from datetime import datetime

from sqlalchemy import Boolean, DateTime, Float, ForeignKey, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from .database import Base


class Station(Base):
    __tablename__ = "stations"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    computer_id: Mapped[str] = mapped_column(String(64), unique=True, index=True)
    room_id: Mapped[str] = mapped_column(String(64), index=True)
    version: Mapped[str] = mapped_column(String(32), default="0.0.0")
    language: Mapped[str] = mapped_column(String(8), default="en")
    cpu_usage: Mapped[float] = mapped_column(Float, default=0.0)
    ram_usage: Mapped[float] = mapped_column(Float, default=0.0)
    is_online: Mapped[bool] = mapped_column(Boolean, default=False, index=True)
    last_seen_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)

    tickets: Mapped[list["Ticket"]] = relationship(back_populates="station")


class Ticket(Base):
    __tablename__ = "tickets"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    station_id: Mapped[int] = mapped_column(ForeignKey("stations.id"), index=True)
    category: Mapped[str] = mapped_column(String(64), index=True)
    description: Mapped[str] = mapped_column(Text)
    status: Mapped[str] = mapped_column(String(16), default="open", index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)

    station: Mapped[Station] = relationship(back_populates="tickets")


class SpeedtestResult(Base):
    __tablename__ = "speedtest_results"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    station_id: Mapped[int] = mapped_column(ForeignKey("stations.id"), index=True)
    download_mbps: Mapped[float] = mapped_column(Float)
    upload_mbps: Mapped[float] = mapped_column(Float)
    ping_ms: Mapped[float] = mapped_column(Float)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
