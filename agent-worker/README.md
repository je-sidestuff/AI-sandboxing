# agent-worker

Processes work units from the input directory by invoking AI agents and recording results.

## Overview

`agent-worker` is a long-running service that polls the input directory for new work units (INSTRUCTION.json or REPORT.json files), executes the appropriate AI agent against each unit, then moves the results to the output directory and writes an audit record.

## Building

```bash
cd agent-worker
go build -o agent-worker .
```

## Running

```bash
./agent-worker
```

The worker runs indefinitely, polling every 10 seconds. It uses exponential backoff logging (30s → 5m → 1h → 24h) to reduce noise during idle periods.

## Work Unit Types

### INSTRUCTION.json

Directs the agent to perform a task.

```json
{
  "instruction": "Describe the task here",
  "mode": "prompt",
  "agent": "claude",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `instruction` | yes | The prompt text to send to the agent |
| `mode` | yes | `"prompt"` (read-only) or `"execute"` (allow file modifications) |
| `agent` | no | Agent override; defaults to `AGENT_PRESET` |
| `timestamp` | no | ISO 8601 creation timestamp |

### INSTRUCTION.md (shorthand)

A plain markdown file is auto-converted to `INSTRUCTION.json` with `mode: "prompt"`.

### REPORT.json

Requests an automated report from the agent.

```json
{
  "type": "daily",
  "agent": "claude",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `type` | yes | `"daily"`, `"weekly"`, `"monthly"`, or `"custom"` |
| `content` | for custom | The report prompt (required when `type` is `"custom"`) |
| `agent` | no | Agent override; defaults to `AGENT_PRESET` |
| `timestamp` | no | ISO 8601 creation timestamp |

### REPORT.md (shorthand)

A plain markdown file is auto-converted to `REPORT.json` with `type: "custom"` and the file contents as the `content` field.

## How Work Units Are Processed

1. Worker detects a directory under `INPUT_DIR/any/` containing a recognized work file
2. Creates `PROCESSING.md` to claim the unit and prevent double-processing
3. Invokes the agent via `invoke-agent.sh` in the work unit's directory
4. Moves output files to `OUTPUT_DIR/content/<unit-id>/`
5. Moves metadata files to `OUTPUT_DIR/records/<unit-id>-<timestamp>/`
6. Writes a JSON audit record to `RECORDS_DIR/worker/`

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Watches `INPUT_DIR/any/` for work units |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | Content goes to `OUTPUT_DIR/content/`, metadata to `OUTPUT_DIR/records/` |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Worker audit records directory |
| `AGENT_PRESET` | `claude` | Default agent: `claude`, `copilot`, `gemini`, `codex`, `opencode` |
| `WORKER_INSTRUCTION_ENABLED` | `true` | Set to `false` to disable INSTRUCTION work unit processing |
| `WORKER_REPORT_ENABLED` | `true` | Set to `false` to disable REPORT work unit processing |

## Output Structure

After processing a work unit named `my-task-001`:

```
OUTPUT_DIR/
  content/
    my-task-001/                # Files created by the agent + PROCESSED-<ts>.md
  records/
    my-task-001-2024-01-01_12-00-00/  # Metadata: PROCESSING.md, INSTRUCTION.json, PROCESSED.md

RECORDS_DIR/
  worker/
    <worker-id>_my-task-001_<unix-ts>.json   # Audit record
```

## Audit Record Format

```json
{
  "worker_id": "abc12345",
  "work_unit": "my-task-001",
  "start_time": "2024-01-01T12:00:00Z",
  "end_time": "2024-01-01T12:05:00Z",
  "duration_ms": 300000,
  "agent": "claude",
  "mode": "execute",
  "exit_code": 0,
  "input_path": "/workspaces/slopspaces/input/any/my-task-001",
  "output_path": "/workspaces/slopspaces/output/content/my-task-001"
}
```
