import asyncio
import os
import secrets
from datetime import datetime, timedelta

from fastapi import Depends, FastAPI, Header, HTTPException, WebSocket, WebSocketDisconnect
from fastapi.responses import HTMLResponse, PlainTextResponse, Response
from fastapi.templating import Jinja2Templates
from fastapi import Request
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from .config import get_settings
from .connection_manager import manager
from .database import AsyncSessionLocal, Base, engine, get_db
from .models import SpeedtestResult, Station, Ticket
from .schemas import AdminAction, AgentTicketCreate, ConfigUpdate, HeartbeatPayload, StationStats, TicketCreate

app = FastAPI(title="Classroom Control Server")
templates = Jinja2Templates(directory=os.path.join(os.path.dirname(__file__), "templates"))
settings = get_settings()


async def require_admin(x_admin_token: str = Header(default="")) -> None:
    if not secrets.compare_digest(x_admin_token, settings.admin_token):
        raise HTTPException(status_code=401, detail="invalid admin token")


async def require_agent(x_agent_token: str = Header(default="")) -> None:
    if not secrets.compare_digest(x_agent_token, settings.agent_token):
        raise HTTPException(status_code=401, detail="invalid agent token")


@app.on_event("startup")
async def startup() -> None:
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    app.state.worker = asyncio.create_task(offline_worker())


@app.on_event("shutdown")
async def shutdown() -> None:
    app.state.worker.cancel()


async def offline_worker() -> None:
    while True:
        cutoff = datetime.utcnow() - timedelta(seconds=settings.offline_after_seconds)
        async with engine.begin() as conn:
            await conn.execute(
                Station.__table__.update()
                .where(Station.last_seen_at < cutoff)
                .values(is_online=False)
            )
        await asyncio.sleep(10)


async def upsert_station(session: AsyncSession, hb: HeartbeatPayload) -> Station:
    station = (
        await session.execute(select(Station).where(Station.computer_id == hb.computer_id))
    ).scalar_one_or_none()
    now = datetime.utcnow()
    if station is None:
        station = Station(
            computer_id=hb.computer_id,
            room_id=hb.room_id,
            version=hb.version,
            language=hb.language,
        )
        session.add(station)
    station.room_id = hb.room_id
    station.version = hb.version
    station.language = hb.language
    station.cpu_usage = hb.cpu_usage
    station.ram_usage = hb.ram_usage
    station.is_online = True
    station.last_seen_at = now
    station.updated_at = now
    await session.commit()
    await session.refresh(station)
    return station


@app.websocket("/ws/agent/{computer_id}")
async def ws_agent(websocket: WebSocket, computer_id: str) -> None:
    token = websocket.query_params.get("token", "")
    if not secrets.compare_digest(token, settings.agent_token):
        await websocket.close(code=4401)
        return
    await manager.connect(computer_id, websocket)
    await manager.flush_pending(computer_id)
    try:
        while True:
            payload = await websocket.receive_json()
            kind = payload.get("type")
            if kind == "heartbeat":
                hb = HeartbeatPayload.model_validate(payload["data"])
                async with AsyncSessionLocal() as db:
                    await upsert_station(db, hb)
                await manager.touch(computer_id)
                await websocket.send_json({"type": "heartbeat_ack"})
            elif kind == "ticket_create":
                body = TicketCreate.model_validate(payload["data"])
                async with AsyncSessionLocal() as db:
                    station = (
                        await db.execute(select(Station).where(Station.computer_id == computer_id))
                    ).scalar_one_or_none()
                    if station is None:
                        continue
                    db.add(Ticket(station_id=station.id, category=body.category, description=body.description))
                    await db.commit()
                await websocket.send_json({"type": "ticket_created"})
            elif kind == "action_result" and payload.get("action") == "run_speedtest":
                result = payload.get("data", {})
                async with AsyncSessionLocal() as db:
                    station = (
                        await db.execute(select(Station).where(Station.computer_id == computer_id))
                    ).scalar_one_or_none()
                    if station:
                        db.add(
                            SpeedtestResult(
                                station_id=station.id,
                                download_mbps=float(result.get("download_mbps", 0)),
                                upload_mbps=float(result.get("upload_mbps", 0)),
                                ping_ms=float(result.get("ping_ms", 0)),
                            )
                        )
                        await db.commit()
    except WebSocketDisconnect:
        pass
    finally:
        await manager.disconnect(computer_id)


@app.get("/agent/tickets/overview", dependencies=[Depends(require_agent)])
async def ticket_overview(db: AsyncSession = Depends(get_db)) -> list[dict]:
    rows = (
        await db.execute(
            select(Ticket, Station.room_id, Station.computer_id)
            .join(Station, Station.id == Ticket.station_id)
            .where(Ticket.status == "open")
            .order_by(Ticket.created_at.desc())
            .limit(200)
        )
    ).all()
    return [
        {
            "ticket_id": t.id,
            "category": t.category,
            "description": t.description,
            "room_id": room,
            "computer_id": comp,
            "created_at": t.created_at.isoformat(),
        }
        for t, room, comp in rows
    ]


@app.post("/agent/tickets", dependencies=[Depends(require_agent)])
async def create_ticket(payload: AgentTicketCreate, db: AsyncSession = Depends(get_db)) -> dict:
    station = (await db.execute(select(Station).where(Station.computer_id == payload.computer_id))).scalar_one_or_none()
    if station is None:
        raise HTTPException(status_code=404, detail="station not found")
    ticket = Ticket(station_id=station.id, category=payload.category, description=payload.description)
    db.add(ticket)
    await db.commit()
    await db.refresh(ticket)
    return {"id": ticket.id, "status": ticket.status}


@app.get("/admin/stats", dependencies=[Depends(require_admin)], response_model=StationStats)
async def admin_stats(db: AsyncSession = Depends(get_db)) -> StationStats:
    total_online = await db.scalar(select(func.count()).select_from(Station).where(Station.is_online.is_(True)))
    total_offline = await db.scalar(select(func.count()).select_from(Station).where(Station.is_online.is_(False)))
    open_tickets = await db.scalar(select(func.count()).select_from(Ticket).where(Ticket.status == "open"))
    speed = (
        await db.execute(
            select(
                func.avg(SpeedtestResult.download_mbps),
                func.avg(SpeedtestResult.upload_mbps),
                func.avg(SpeedtestResult.ping_ms),
            )
        )
    ).one()
    return StationStats(
        online=total_online or 0,
        offline=total_offline or 0,
        open_tickets=open_tickets or 0,
        avg_download_mbps=round(float(speed[0] or 0), 2),
        avg_upload_mbps=round(float(speed[1] or 0), 2),
        avg_ping_ms=round(float(speed[2] or 0), 2),
    )


@app.get("/admin/stations", dependencies=[Depends(require_admin)])
async def stations(db: AsyncSession = Depends(get_db)) -> list[dict]:
    rows = (await db.execute(select(Station).order_by(Station.room_id, Station.computer_id))).scalars().all()
    return [
        {
            "computer_id": s.computer_id,
            "room_id": s.room_id,
            "version": s.version,
            "is_online": s.is_online,
            "last_seen_at": s.last_seen_at.isoformat() if s.last_seen_at else None,
        }
        for s in rows
    ]


@app.post("/admin/stations/{computer_id}/config", dependencies=[Depends(require_admin)])
async def push_config(computer_id: str, update: ConfigUpdate) -> dict:
    sent = await manager.send_action(
        computer_id,
        {"type": "action", "action": "push_config", "payload": update.model_dump(exclude_none=True)},
    )
    return {"queued": not sent, "sent": sent}


@app.post("/admin/stations/{computer_id}/actions", dependencies=[Depends(require_admin)])
async def trigger_action(computer_id: str, action: AdminAction) -> dict:
    sent = await manager.send_action(
        computer_id,
        {"type": "action", "action": action.action, "payload": action.payload},
    )
    return {"queued": not sent, "sent": sent}


@app.get("/dashboard", response_class=HTMLResponse)
async def dashboard(request: Request, db: AsyncSession = Depends(get_db)) -> HTMLResponse:
    stats = await admin_stats(db)
    station_rows = await stations(db)
    tickets = await ticket_overview(db)
    return templates.TemplateResponse(
        "dashboard.html",
        {
            "request": request,
            "stats": stats.model_dump(),
            "stations": station_rows,
            "tickets": tickets,
        },
    )


@app.get("/agent/update/manifest", dependencies=[Depends(require_agent)])
async def update_manifest(current_version: str) -> dict:
    latest_version = "1.0.0"
    return {
        "latest_version": latest_version,
        "update_available": latest_version != current_version,
        "download_url": "/agent/update/binary",
        "checksum_sha256": "",
    }


@app.get("/agent/update/binary", dependencies=[Depends(require_agent)])
async def update_binary() -> PlainTextResponse:
    return PlainTextResponse("placeholder binary stream")


@app.get("/agent/speedtest/download", dependencies=[Depends(require_agent)])
async def speedtest_download() -> PlainTextResponse:
    return PlainTextResponse("0" * settings.speedtest_payload_bytes)


@app.post("/agent/speedtest/upload", dependencies=[Depends(require_agent)])
async def speedtest_upload(_: Request) -> Response:
    return Response(status_code=204)


@app.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok"}
