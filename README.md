# Containment Dispatcher

An AI agent orchestration framework for safely dispatching work to AI agents with multiple containment strategies and comprehensive audit trails.

## Overview

The containment dispatcher manages the full lifecycle of AI-generated work: receiving instructions, executing agents in isolated environments, collecting outputs, and recording results. It supports multiple AI agent backends (Claude, Copilot, Gemini, Codex, OpenCode) and three containment models with increasing isolation.

## Components

| Component | Description |
|-----------|-------------|
| [`agent-dispatch`](agent-dispatch/) | Core dispatcher — watch mode and single-shot dispatch with terraform lifecycle management |
| [`agent-worker`](agent-worker/) | Work unit executor — processes INSTRUCTION.json and REPORT.json files via agent invocation |
| [`agent-dispatch-watch`](agent-dispatch-watch/) | Watch service — monitors for DISPATCH files and drives terraform-based containment workflows |
| [`agent-events`](agent-events/) | Event scheduler — timer and schedule-based automated report generation |
| [`ambiguous-agent`](ambiguous-agent/) | Interactive shell — REPL interface for direct agent interaction |
| [`heuristic-request`](heuristic-request/) | Heuristic processor — converts unstructured inputs into structured dispatch/instruction requests |

## Containment Models

### Direct
Instructions are written to `INPUT_DIR/any/` and picked up by `agent-worker`. Fire-and-forget; no repository isolation.

### In-Repo
Work happens on a branch in the target repository. Terraform manages the branch and PR creation. Provides audit trail via pull request.

### Repo-Isolation
A completely separate private repository is cloned from the target. The agent works in the isolated copy; results are pushed to a branch and a PR is opened on the original repo. Strongest isolation guarantee.

## Data Flow

```
INPUT_DIR/any/<unit-id>/
  ├── INSTRUCTION.json  ──►  agent-worker  ──►  OUTPUT_DIR/content/<unit-id>/
  ├── REPORT.json       ──►  agent-worker  ──►  OUTPUT_DIR/content/<unit-id>/
  └── DISPATCH.json     ──►  agent-dispatch
                                ├─[direct]──────►  INSTRUCTION.json (worker pickup)
                                ├─[in-repo]──────►  terraform → branch + PR
                                └─[repo-isolation]► terraform → isolated repo + PR
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for full system and containment-model sequence diagrams.

## Quick Start

### Run the worker (processes INSTRUCTION.json files)

```bash
cd agent-worker
go build -o agent-worker .
./agent-worker
```

### Dispatch a single instruction

```bash
cd agent-dispatch
go build -o agent-dispatch .
./agent-dispatch --once -i "Run the test suite" -m execute
```

### Run the event scheduler

```bash
cd agent-events
go build -o agent-events .
./agent-events
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Work unit input directory |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | Completed work output directory |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Audit records and logs |
| `AGENT_PRESET` | `claude` | Default AI agent (`claude`, `copilot`, `gemini`, `codex`, `opencode`) |
| `GITHUB_PAT` / `GH_TOKEN` | — | GitHub token (required for in-repo and repo-isolation dispatch) |
| `TERRAFORM_BINARY` | `terraform` | Path to the terraform binary |
| `DISPATCHER_LIVE` | `/workspaces/slopspaces/dispatcher/live` | Terraform working directory |
| `EVENTS_CONFIG_DIR` | `/workspaces/slopspaces/events/config` | Event configuration directory |
| `HEURISTIC_DIR` | `/workspaces/slopspaces/heuristic` | Heuristic input directory |
| `REQUEST_DIR` | `/workspaces/slopspaces/requests` | Extracted request output directory |
| `WORKER_INSTRUCTION_ENABLED` | `true` | Enable instruction work unit processing |
| `WORKER_REPORT_ENABLED` | `true` | Enable report work unit processing |

## Work Unit Formats

### INSTRUCTION.json

```json
{
  "instruction": "Describe the task here",
  "mode": "prompt",
  "agent": "claude",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

- `mode`: `"prompt"` (read-only) or `"execute"` (allow file modifications)
- `agent`: optional override; defaults to `AGENT_PRESET`
- An `INSTRUCTION.md` file is auto-converted to JSON with `mode: "prompt"`

### REPORT.json

```json
{
  "type": "daily",
  "agent": "claude",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

- `type`: `"daily"`, `"weekly"`, `"monthly"`, or `"custom"`
- For `"custom"` reports, add a `"content"` field with the report prompt
- A `REPORT.md` file is auto-converted to JSON with `type: "custom"`

### DISPATCH.json

```json
{
  "type": "direct",
  "instruction": "Task description",
  "mode": "execute"
}
```

See [`agent-dispatch/README.md`](agent-dispatch/README.md) for full dispatch format documentation including `in-repo` and `repo-isolation` types.

## Records

All operations produce structured audit records:

- `RECORDS_DIR/worker/` — JSON records for each processed work unit
- `RECORDS_DIR/dispatch/` — Single-shot dispatch records
- `RECORDS_DIR/dispatch-watch/` — Watch mode dispatch and terraform flow records
- `RECORDS_DIR/heuristic/` — Heuristic processing records
- `RECORDS_DIR/reports/` — Generated report outputs

## License

MIT — see [LICENSE](LICENSE).
