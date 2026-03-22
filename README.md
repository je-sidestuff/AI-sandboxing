# containment-dispatcher

A multi-layered AI agent orchestration and containment system for the slopspaces platform.
The dispatcher routes work units to AI agents using configurable containment strategies,
manages PR-based workflows via Terraform, schedules periodic reports, and processes
high-level heuristic inputs into concrete agent tasks.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         slopspaces platform                      │
│                                                                   │
│  heuristic-request  ──►  requests/          (heuristic → task)   │
│                                │                                  │
│  agent-events       ──►  input/any/         (scheduled reports)   │
│                                │                                  │
│  agent-dispatch     ──►  input/any/         (dispatch → instruct) │
│  agent-dispatch-watch          │                                  │
│                                ▼                                  │
│                        agent-worker          (executes tasks)     │
│                                │                                  │
│                                ▼                                  │
│                     output/content/  +  output/records/           │
└─────────────────────────────────────────────────────────────────┘
```

### Components

| Component | Binary | Purpose |
|-----------|--------|---------|
| `agent-worker` | `agent-worker` | Polls `input/any/` for work units and executes them via an AI agent |
| `agent-dispatch` | `agent-dispatch` | Unified dispatcher: watch mode + single-shot CLI |
| `agent-dispatch-watch` | `agent-dispatch-watch` | Watches for `DISPATCH.json/md` files and manages their lifecycle |
| `agent-events` | `agent-events` | Schedules periodic reports (timers and daily schedules) |
| `heuristic-request` | `heuristic-request` | Converts high-level `HEURISTIC.md` inputs into `DISPATCH`/`INSTRUCTION` requests |
| `ambiguous-agent` | `ambiguous-agent` | Interactive REPL shell for manual agent invocation |

---

## Work Unit Types

### Instruction

An instruction is a direct task for an agent to complete.

**`INSTRUCTION.json`:**
```json
{
  "instruction": "Refactor the authentication module to use JWT tokens",
  "mode": "execute",
  "agent": "claude",
  "timestamp": "2026-01-01T09:00:00Z"
}
```

Fields:
- `instruction` (required): The task to perform
- `mode`: `"prompt"` (read-only, returns output) or `"execute"` (can modify files). Default: `"prompt"`
- `agent`: Override the default agent. One of: `copilot`, `gemini`, `claude`, `opencode`, `codex`

**`INSTRUCTION.md`** (auto-converts to JSON with `mode: "prompt"`):
```markdown
Explain the purpose of the prpoller package and how it batches GraphQL queries.
```

### Report

A report asks an agent to generate a summary based on agent records and history.

**`REPORT.json`:**
```json
{
  "type": "daily",
  "date": "2026-01-01",
  "timestamp": "2026-01-01T09:00:00Z"
}
```

Report types:
- `daily` — summary of yesterday's activity
- `weekly` — week-in-review
- `monthly` — monthly summary
- `custom` — arbitrary topic supplied via the `content` field

**`REPORT.md`** (auto-converts to JSON with `type: "custom"`).

### Dispatch

A dispatch unit routes work through a configurable containment strategy.

**`DISPATCH.json`:**
```json
{
  "type": "direct",
  "instruction": "Run the full test suite",
  "mode": "execute",
  "agent": "claude"
}
```

Dispatch types:
- `direct` — transforms to `INSTRUCTION.json` in place, picked up by `agent-worker`
- `in-repo` — creates a PR branch in `target_repo` via Terraform (requires `GITHUB_PAT`)
- `repo-isolation` — creates a completely isolated clone of `target_repo` via Terraform

**`DISPATCH.md`** (auto-converts to JSON with `type: "direct"`).

---

## Data Flow

### Instruction / Report processing

```
input/any/<work-unit>/
  ├── INSTRUCTION.json  ──►  agent-worker  ──►  output/content/<work-unit>/
  └── REPORT.json                          └──  output/records/<work-unit>-<ts>/
```

### Dispatch processing

```
input/any/<dispatch-id>/DISPATCH.json
  ├─[direct]────────────► INSTRUCTION.json  (worker picks it up)
  ├─[in-repo]──────────►  Terraform: create PR branch  ──►  output/<dispatch-id>/
  └─[repo-isolation]───►  Terraform: create isolation repo  ──►  output/<dispatch-id>/
```

### Heuristic processing

```
heuristic/pending/<id>/HEURISTIC.md
  ──►  agent (prompt mode)
  ──►  requests/<request-id>/DISPATCH.json  (or INSTRUCTION.json)
  ──►  heuristic/processed/<id>/
```

### Event scheduling

```
events/config/*.json  (timer / schedule events)
  ──►  agent-events
  ──►  input/any/<work-unit>/REPORT.json
```

---

## Environment Variables

| Variable | Default | Used by | Description |
|----------|---------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | all | Root input directory |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | worker, dispatch | Output for completed work |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | all | Operational records / logs |
| `DISPATCHER_LIVE` | `/workspaces/slopspaces/dispatcher/live` | dispatch-watch | Terraform config storage |
| `EVENTS_CONFIG_DIR` | `/workspaces/slopspaces/events/config` | agent-events | Event config files |
| `HEURISTIC_DIR` | `/workspaces/slopspaces/heuristic` | heuristic-request | Heuristic input directory |
| `REQUEST_DIR` | `/workspaces/slopspaces/requests` | heuristic-request | Extracted request output |
| `GITHUB_PAT` / `GH_TOKEN` | — | dispatch-watch | GitHub token (in-repo / repo-isolation dispatch) |
| `TERRAFORM_BINARY` | `terraform` | dispatch-watch | Path to terraform binary |
| `AGENT_PRESET` | `claude` | all | Default AI agent |
| `AGENT_RECORDS_PATH` | `/workspaces/agent-records/` | workers | Session records path |
| `WORKER_INSTRUCTION_ENABLED` | `true` | agent-worker | Enable instruction work units |
| `WORKER_REPORT_ENABLED` | `true` | agent-worker | Enable report work units |

---

## Building

Each component is an independent Go module. Build with:

```bash
cd <component>
go build -o <component> .
```

For example:

```bash
cd agent-worker && go build -o agent-worker .
cd agent-dispatch && go build -o agent-dispatch .
cd agent-dispatch-watch && go build -o agent-dispatch-watch .
cd agent-events && go build -o agent-events .
cd heuristic-request && go build -o heuristic-request .
cd ambiguous-agent && go build -o ambiguous-agent .
```

---

## Quick Start

### 1. Start the worker

```bash
agent-worker
# [abc12345] Agent worker started
# [abc12345] Input: /workspaces/slopspaces/input/any
# [abc12345] Default agent: claude
```

### 2. Dispatch a task (single-shot)

```bash
agent-dispatch --once -i "Summarise recent changes" -m prompt
# === Dispatch Result ===
# Work Unit ID: dispatch-inst_2026-01-01_09-00-00_abc12345
# Success:      true
# Exit Code:    0
```

### 3. Dispatch a task via file

```bash
mkdir -p /workspaces/slopspaces/input/any/my-task
cat > /workspaces/slopspaces/input/any/my-task/INSTRUCTION.json <<'EOF'
{
  "instruction": "List all Go files in the project",
  "mode": "prompt"
}
EOF
# agent-worker picks it up automatically
```

### 4. Schedule events

```bash
agent-events
# Creates default configs (daily report at 09:00 + custom heartbeat every 6h)
# and begins scheduling work units
```

### 5. Process a heuristic

```bash
mkdir -p /workspaces/slopspaces/heuristic/pending/my-heuristic
cat > /workspaces/slopspaces/heuristic/pending/my-heuristic/HEURISTIC.md <<'EOF'
The project needs better error handling in the authentication flow.
EOF
heuristic-request --once
# Invokes agent to decide DISPATCH vs INSTRUCTION, writes result to requests/
```

---

## Records

All operations produce structured JSON records for auditing and debugging:

| Path | Contents |
|------|----------|
| `agent-records/worker/` | Per-work-unit records (agent, duration, exit code) |
| `agent-records/dispatch-watch/` | Dispatch operation records |
| `agent-records/dispatch-watch/flow_*.json` | Terraform flow records (in-repo/repo-isolation) |
| `agent-records/heuristic/` | Heuristic processing records |
| `agent-records/session-*/` | Interactive session transcripts (ambiguous-agent) |

---

## License

MIT — see [LICENSE](LICENSE).
