# Agent Viewer

A real-time web viewer for slopspaces agent sessions. Browse past sessions, inspect
their files, and watch live file-system activity as agents work.

## Quick Start

```bash
cd agent-viewer
go run .
```

Open **http://localhost:8080** in your browser.

> **Note:** The frontend loads React 18 and Babel from unpkg CDN, so internet
> access is required on first load.

## Features

- **Session Browser** — lists sessions from every slopspaces subdirectory, grouped
  and filterable by category (input, heuristic, output, working, requests, dispatcher)
- **Record Browser** — browses `agent-records/` invocations, sessions, and worker JSON files
- **File Viewer** — click any file in a session to read its contents; JSON is
  syntax-highlighted and pretty-printed
- **Live Feed** — WebSocket connection streams every file-system event under
  `/workspaces/slopspaces/` in real time; changed sessions and files are briefly
  highlighted
- **Auto-refresh** — new sessions created while the app is running are picked up
  automatically via the watcher

## API Endpoints

| Method | Path                       | Description                              |
|--------|----------------------------|------------------------------------------|
| `GET`  | `/api/sessions`            | List all sessions (JSON array)           |
| `GET`  | `/api/sessions/{id}`       | Get a single session with its file list  |
| `GET`  | `/api/file?path=<abs>`     | Read a file (restricted to slopspaces/)  |
| `GET`  | `/api/records`             | List agent-records entries               |
| `GET`  | `/api/records/{id}`        | Get a single record                      |
| `WS`   | `/ws`                      | WebSocket; pushes `file_event` messages  |

### WebSocket message format

```json
{ "type": "file_event", "path": "input/any/session-id/DISPATCH.json", "op": "write", "time": "2026-03-24T23:00:00Z" }
```

`op` is one of `create`, `write`, `remove`, `rename`.

## Slopspaces Directory Structure

```
/workspaces/slopspaces/
├── input/any/                         # Incoming sessions
│   └── <session-id>/
│       ├── DISPATCH.json              # Simple dispatch: { type, instruction, mode }
│       ├── INSTRUCTION.json           # Complex dispatch: { mode, instruction }
│       ├── DISPATCHING.md             # Set by dispatcher while processing
│       └── HEURISTIC_SOURCE.md        # Raw text that produced the dispatch
│
├── heuristic/
│   ├── pending/<session-id>/          # Awaiting heuristic processing
│   └── processed/<session-id>/        # Completed heuristic runs
│       ├── HEURISTIC.md
│       ├── PROCESSING.md
│       ├── PROCESSED.md
│       └── agent_output.txt
│
├── output/                            # Completed task output
│   ├── <session-id>/
│   │   ├── PROCESSING.md
│   │   └── PROCESSED.md
│   └── content/                       # Report content
│
├── working/                           # In-progress git-isolated tasks
│   └── <session-id>/
│       ├── branch_name
│       └── git_state/.git/
│
├── requests/                          # Approval request tracking
│   └── <session-id>/
│       └── DISPATCH.json
│
├── dispatcher/live/flows/             # Terraform dispatch flows
│   ├── approval/<flow-id>/
│   ├── repo-isolation/<flow-id>/
│   └── in-repo/<flow-id>/
│
├── agent-records/                     # All agent invocation logs
│   ├── <YYYY-MM-DD_HH-MM-SS_id>/     # Single invocations
│   │   ├── metadata.txt               # date, agent, prompt, duration, exit code
│   │   └── raw_output.txt
│   ├── session-<id>/                  # Multi-call sessions
│   │   ├── <call-id>/
│   │   │   ├── metadata.txt
│   │   │   └── raw_output.txt
│   │   └── session.jsonl
│   └── worker/                        # Worker completion records
│       └── <worker_id>_<work_unit>.json
│
└── events/config/                     # Scheduled event definitions
    ├── default-daily-report.json
    └── custom-heartbeat-report.json
```

## Building a standalone binary

The React frontend is embedded into the Go binary at compile time, so `go build`
produces a single self-contained executable:

```bash
cd agent-viewer
go build -o agent-viewer .
./agent-viewer
```

## Configuration

The slopspaces root path is hardcoded to `/workspaces/slopspaces`. To change it,
edit the `slopspacesRoot` constant in `main.go`. The listen address defaults to
`:8080` and can similarly be changed via the `listenAddr` constant.
