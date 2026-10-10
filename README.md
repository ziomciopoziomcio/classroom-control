# classroom-control

School laboratory management suite with a FastAPI backend and a Go Windows service agent.

## Repository layout

- `/server/app`: FastAPI server (WebSocket hub, admin APIs, dashboard, DB models)
- `/client`: Go agent (Windows service lifecycle, heartbeat, remote actions, localized UI + notifications)
- `/tests`: focused backend tests

## Server (FastAPI)

### Features

- Agent WebSocket endpoint: `ws://.../ws/agent/{computer_id}`
- 30s heartbeat payload support (`cpu_usage`, `ram_usage`, `version`, `room_id`, `computer_id`, `language`)
- Offline worker marks clients offline if silent for >90s
- Tickets storage + all-rooms overview endpoint for agents
- Admin endpoints for:
  - station list + online/offline stats
  - push config updates to selected stations
  - trigger remote actions (`update_check`, `force_update`, `run_shell`, `run_speedtest`)
- Dashboard at `/dashboard` showing counters, stations, and open tickets
- Speedtest helper endpoints (`/agent/speedtest/download`, `/agent/speedtest/upload`)

### Run locally

```bash
cd server
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload
```

Auth headers:

- Admin: `X-Admin-Token`
- Agent/API: `X-Agent-Token`

Defaults can be overridden with env vars prefixed by `CLASSROOM_`.

## Client Agent (Go, Windows service)

### Implemented capabilities

- Service lifecycle with `golang.org/x/sys/windows/svc`
- Config loaded from `C:\Program Files\SchoolAgent\config.json`
- Heartbeat + automatic reconnect loop over WebSocket
- Bilingual strings (English/Polish)
- Local UI page (teacher-facing) with:
  - About section (Room/Station ID + app version)
  - Helpdesk ticket submit form
  - Tickets overview table across rooms (read-only)
- Localized toast notifications (Windows PowerShell toast script)
- Remote actions:
  - secure config push/write
  - run shell command and stream output to server
  - run network speedtest and return metrics
  - check/force update with updater `.bat` flow (`sc stop`, replace binary, `sc start`)

### Build

```powershell
cd client
go mod tidy
go build -o school-agent.exe ./cmd/agent
```

### Register service under SYSTEM

```powershell
sc create SchoolAgent binPath= "C:\Program Files\SchoolAgent\school-agent.exe" start= auto obj= "NT AUTHORITY\SYSTEM"
sc description SchoolAgent "Classroom Control Agent"
sc start SchoolAgent
```

### Lock config permissions (teachers/students read-only)

```powershell
New-Item -ItemType Directory -Force "C:\Program Files\SchoolAgent" | Out-Null
# Place config.json there first
icacls "C:\Program Files\SchoolAgent\config.json" /inheritance:r
icacls "C:\Program Files\SchoolAgent\config.json" /grant:r "SYSTEM:(F)" "Administrators:(F)"
icacls "C:\Program Files\SchoolAgent\config.json" /grant:r "Users:(R)"
```

## Tests

```bash
cd server
pytest -q ../tests/test_server.py
```
