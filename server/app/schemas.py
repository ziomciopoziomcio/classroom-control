from datetime import datetime
from typing import Literal

from pydantic import BaseModel, Field


class HeartbeatPayload(BaseModel):
    computer_id: str
    room_id: str
    version: str
    language: Literal["en", "pl"] = "en"
    cpu_usage: float = Field(ge=0, le=100)
    ram_usage: float = Field(ge=0, le=100)


class TicketCreate(BaseModel):
    category: str
    description: str = Field(min_length=4, max_length=2000)

class AgentTicketCreate(TicketCreate):
    computer_id: str


class TicketRead(BaseModel):
    id: int
    station_id: int
    category: str
    description: str
    status: str
    created_at: datetime


class ConfigUpdate(BaseModel):
    room_id: str | None = None
    language: Literal["en", "pl"] | None = None
    heartbeat_interval_seconds: int | None = Field(default=None, ge=10, le=3600)


class AdminAction(BaseModel):
    action: Literal["update_check", "force_update", "run_shell", "run_speedtest", "push_config"]
    payload: dict = Field(default_factory=dict)


class StationStats(BaseModel):
    online: int
    offline: int
    open_tickets: int
    avg_download_mbps: float
    avg_upload_mbps: float
    avg_ping_ms: float
