import asyncio
from collections import defaultdict
from datetime import UTC, datetime

from fastapi import WebSocket


class ConnectionManager:
    def __init__(self) -> None:
        self.active: dict[str, WebSocket] = {}
        self.last_seen: dict[str, datetime] = {}
        self.lock = asyncio.Lock()
        self.pending_actions: dict[str, list[dict]] = defaultdict(list)

    async def connect(self, computer_id: str, websocket: WebSocket) -> None:
        await websocket.accept()
        async with self.lock:
            self.active[computer_id] = websocket
            self.last_seen[computer_id] = datetime.now(UTC)

    async def disconnect(self, computer_id: str) -> None:
        async with self.lock:
            self.active.pop(computer_id, None)
            self.last_seen.pop(computer_id, None)

    async def touch(self, computer_id: str) -> None:
        async with self.lock:
            self.last_seen[computer_id] = datetime.now(UTC)

    async def send_action(self, computer_id: str, action: dict) -> bool:
        async with self.lock:
            ws = self.active.get(computer_id)
            if ws is None:
                self.pending_actions[computer_id].append(action)
                return False
        await ws.send_json(action)
        return True

    async def flush_pending(self, computer_id: str) -> None:
        async with self.lock:
            ws = self.active.get(computer_id)
            pending = self.pending_actions.pop(computer_id, [])
        for action in pending:
            await ws.send_json(action)


manager = ConnectionManager()
