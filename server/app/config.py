from functools import lru_cache
from pydantic import BaseModel
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="CLASSROOM_", env_file=".env", extra="ignore")

    database_url: str = "sqlite+aiosqlite:///./classroom.db"
    heartbeat_interval_seconds: int = 30
    offline_after_seconds: int = 90
    admin_token: str = "change-me-admin-token"
    agent_token: str = "change-me-agent-token"
    speedtest_payload_bytes: int = 5 * 1024 * 1024


class RuntimeCounters(BaseModel):
    online: int = 0
    offline: int = 0


@lru_cache
def get_settings() -> Settings:
    return Settings()
