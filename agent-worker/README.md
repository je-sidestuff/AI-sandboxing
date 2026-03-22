# agent-worker

The core work unit executor. Polls the input directory for INSTRUCTION.json and REPORT.json files, invokes AI agents to process them, and produces output artifacts with full audit trails.

## Overview

`agent-worker` runs as a daemon, continuously watching `INPUT_DIR/any/` for work unit folders. When a work unit is detected, the worker:

1. Parses the instruction or report specification
2. Writes `PROCESSING.md` to claim the unit (prevents duplicate processing)
3. Invokes the configured AI agent via `invoke-agent.sh`
4. Moves output content to `OUTPUT_DIR/content/`
5. Moves metadata to `OUTPUT_DIR/records/`
6. Writes a JSON record to `RECORDS_DIR/worker/`

## Usage

```bash
cd agent-worker
go build -o agent-worker .
./agent-worker
```

The worker runs indefinitely until interrupted. Use `CTRL+C` to stop.

## Work Unit Types

### Instruction Work Units

A folder containing `INSTRUCTION.json` or `INSTRUCTION.md`:

**INSTRUCTION.json:**
```json
{
  "instruction": "Describe the task here",
  "mode": "execute",
  "agent": "claude",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

- `mode`: `"prompt"` (read-only) or `"execute"` (allow file modifications)
- `agent`: optional override; defaults to `AGENT_PRESET`
- `INSTRUCTION.md` is auto-converted to JSON with `mode: "prompt"`
- When both exist, `INSTRUCTION.json` takes precedence and `INSTRUCTION.md` is removed

### Report Work Units

A folder containing `REPORT.json` or `REPORT.md`:

**REPORT.json:**
```json
{
  "type": "daily",
  "agent": "claude",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

- `type`: `"daily"`, `"weekly"`, `"monthly"`, or `"custom"`
- For `"custom"` reports, include a `"content"` field with the report prompt
- `REPORT.md` is auto-converted to JSON with `type: "custom"` and the file contents as the prompt
- When both exist, `REPORT.json` takes precedence and `REPORT.md` is removed
- Reports always run in execute mode to allow file creation

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Root input directory (watches `INPUT_DIR/any/`) |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | Output directory for content and records |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Worker audit records |
| `AGENT_PRESET` | `claude` | Default AI agent (`claude`, `copilot`, `gemini`, `codex`, `opencode`) |
| `WORKER_INSTRUCTION_ENABLED` | `true` | Set to `false` to disable instruction work unit processing |
| `WORKER_REPORT_ENABLED` | `true` | Set to `false` to disable report work unit processing |

## Output Structure

After processing a work unit at `INPUT_DIR/any/<unit-id>/`, the worker produces:

```
OUTPUT_DIR/
  content/<unit-id>/
    <agent-produced files>
    PROCESSED-<timestamp>.md    # Processing summary
  records/<unit-id>-<timestamp>/
    INSTRUCTION.json             # Original instruction (moved from input)
    PROCESSING.md               # Processing marker
    PROCESSED.md                # Processing summary
RECORDS_DIR/
  worker/
    <worker-id>_<unit-id>_<unix-ts>.json  # Structured audit record
```

## Concurrency and Safety

- Work units are claimed by writing `PROCESSING.md` before processing begins
- Any folder with an existing `PROCESSING.md` is skipped during scanning
- Only one worker should run per `INPUT_DIR` to avoid race conditions
- If a worker crashes mid-processing, the `PROCESSING.md` file prevents the unit from being picked up again until manually removed

## Agent Invocation

The worker locates `invoke-agent.sh` by searching (in order):
1. Same directory as the worker binary
2. Current working directory
3. `ambiguous-agent/` subdirectory of cwd
4. `ambiguous-agent/` in the parent directory
5. `PATH`

The script is invoked with the mode flag (`-p` for prompt, `-e` for execute) and the instruction written to a temporary file to safely handle multi-line prompts.

## Inactivity Logging

When no work units are available, the worker logs inactivity at exponentially increasing intervals to reduce noise:

| Time since last activity | Log interval |
|--------------------------|--------------|
| 0–30s | 30s |
| 30s–5m | 5m |
| 5m–1h | 1h |
| 1h+ | 24h |

## Building

```bash
cd agent-worker
go build -o agent-worker .
```

## Testing

Manual test fixtures are provided in `tests/manual/`:

```bash
# Place a work unit in the input directory and run the worker
cp -r tests/manual/sample-work-unit-1 $INPUT_DIR/any/test-unit-1
./agent-worker
```
