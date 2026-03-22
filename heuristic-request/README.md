# heuristic-request

Converts unstructured natural language inputs into structured dispatch or instruction requests. Acts as an AI-powered front-end for the orchestration pipeline, translating vague intent into actionable work units.

## Overview

`heuristic-request` watches `HEURISTIC_DIR/pending/` for folders containing `HEURISTIC.md` files. When one is detected, it invokes an AI agent in prompt-only mode to analyze the content and decide what kind of work unit to create. The structured output is placed in `REQUEST_DIR` for downstream processing.

The key value: you can drop an informal note into the pipeline and `heuristic-request` will determine whether it needs a full dispatch workflow (with branch/PR creation) or a simple instruction, and produce the correctly formatted file.

## Usage

```bash
cd heuristic-request
go build -o heuristic-request .

# Watch mode (default) - continuous polling
./heuristic-request

# Single-shot mode - process pending units once and exit
./heuristic-request --once
```

## Input Format

Place a folder containing `HEURISTIC.md` in `HEURISTIC_DIR/pending/`:

```
HEURISTIC_DIR/pending/
  my-request-001/
    HEURISTIC.md
```

**HEURISTIC.md** contains free-form text describing what needs to be done:

```markdown
The login page is broken on mobile. The button text overflows the container
and the form doesn't submit when the user taps outside the input field.
Can you investigate and fix these issues?
```

The agent reads this and determines whether to create:
- `INSTRUCTION.json` / `INSTRUCTION.md` — for direct agent tasks
- `DISPATCH.json` / `DISPATCH.md` — for tasks requiring branch/PR workflows

## Output

Successfully processed units produce a folder in `REQUEST_DIR`:

```
REQUEST_DIR/
  <timestamp>_<heuristic-id>/
    INSTRUCTION.json     # (or DISPATCH.json, INSTRUCTION.md, DISPATCH.md)
    HEURISTIC_SOURCE.md  # Copy of the original input for reference
```

The original folder is moved to `HEURISTIC_DIR/processed/<heuristic-id>/` with a `PROCESSED.md` summary.

## Decision Logic

The AI agent chooses the output type based on these guidelines:

| Output Type | When to Use |
|-------------|-------------|
| `DISPATCH.json` | Multi-step workflows requiring branch creation, PRs, or repository isolation |
| `DISPATCH.md` | Same as above, but in markdown format |
| `INSTRUCTION.json` | Direct, single-step agent tasks with structured metadata |
| `INSTRUCTION.md` | Same as above, in plain markdown format |

The structured JSON formats allow specifying mode (`prompt`/`execute`), agent preset, and dispatch type. Markdown formats are simpler and default to prompt mode.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HEURISTIC_DIR` | `/workspaces/slopspaces/heuristic` | Root directory; watches `HEURISTIC_DIR/pending/` |
| `REQUEST_DIR` | `/workspaces/slopspaces/requests` | Output directory for extracted requests |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Records for audit and debugging |
| `AGENT_PRESET` | `claude` | Agent to use for heuristic analysis |

## Records

Each processed unit writes a JSON record to `RECORDS_DIR/heuristic/`:

```json
{
  "watcher_id": "a1b2c3d4",
  "heuristic_id": "my-request-001",
  "start_time": "2024-03-21T09:00:00Z",
  "end_time": "2024-03-21T09:00:45Z",
  "duration_ms": 45000,
  "agent": "claude",
  "exit_code": 0,
  "request_id": "2024-03-21_09-00-00_my-request-001",
  "files_extracted": 1,
  "success": true
}
```

## Error Handling

If the agent fails or produces no recognizable output, the unit is marked with a `FAILED.md` file in the input folder and the `PROCESSING.md` marker is removed. The unit can then be retried or manually corrected.

## Building

```bash
cd heuristic-request
go build -o heuristic-request .
```
