from fastapi.testclient import TestClient
from sqlalchemy import delete

from server.app.database import AsyncSessionLocal, Base, engine
from server.app.main import app
from server.app.models import SpeedtestResult, Station, Ticket


ADMIN_HEADERS = {"X-Admin-Token": "change-me-admin-token"}
AGENT_HEADERS = {"X-Agent-Token": "change-me-agent-token"}


def clear_db() -> None:
    import asyncio

    async def _clear() -> None:
        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)
        async with AsyncSessionLocal() as s:
            await s.execute(delete(SpeedtestResult))
            await s.execute(delete(Ticket))
            await s.execute(delete(Station))
            await s.commit()

    asyncio.run(_clear())


def test_admin_requires_token() -> None:
    clear_db()
    with TestClient(app) as client:
        response = client.get("/admin/stats")
    assert response.status_code == 401


def test_websocket_heartbeat_and_ticket_overview() -> None:
    clear_db()
    with TestClient(app) as client:
        with client.websocket_connect("/ws/agent/pc-01?token=change-me-agent-token") as ws:
            ws.send_json(
                {
                    "type": "heartbeat",
                    "data": {
                        "computer_id": "pc-01",
                        "room_id": "A-12",
                        "version": "1.0.0",
                        "language": "pl",
                        "cpu_usage": 20,
                        "ram_usage": 35,
                    },
                }
            )
            ack = ws.receive_json()
            assert ack["type"] == "heartbeat_ack"

        ticket_resp = client.post(
            "/agent/tickets",
            headers=AGENT_HEADERS,
            json={"computer_id": "pc-01", "category": "network", "description": "No internet in room A-12"},
        )
        assert ticket_resp.status_code == 200

        overview = client.get("/agent/tickets/overview", headers=AGENT_HEADERS)
        assert overview.status_code == 200
        payload = overview.json()
        assert payload and payload[0]["room_id"] == "A-12"

        stats = client.get("/admin/stats", headers=ADMIN_HEADERS)
        assert stats.status_code == 200
        assert stats.json()["online"] == 1
        assert stats.json()["open_tickets"] == 1


def test_admin_action_is_queued_when_station_offline() -> None:
    clear_db()
    with TestClient(app) as client:
        response = client.post(
            "/admin/stations/pc-unknown/actions",
            headers=ADMIN_HEADERS,
            json={"action": "run_shell", "payload": {"interpreter": "cmd", "script": "hostname"}},
        )
    assert response.status_code == 200
    assert response.json()["queued"] is True
