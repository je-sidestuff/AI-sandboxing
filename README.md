# slopspaces

A system for safely dispatching and executing AI agent work units in isolated, contained environments. It manages the full lifecycle of AI-driven code generation tasks — from scheduling and dispatch through execution and result handling — with strong containment guarantees.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                      slopspaces                         │
│                                                         │
│  ┌──────────────┐     ┌──────────────┐                  │
│  │ agent-events │────►│              │                  │
│  │  (scheduler) │     │  INPUT_DIR   │                  │
│  └──────────────┘     │  /any/<id>/  │                  │
│                       │              │                  │
│  ┌──────────────┐     │ INSTRUCTION  │                  │
│  │  heuristic-  │────►│   .json      │                  │
│  │   request    │     │ DISPATCH     │                  │
│  └──────────────┘     │   .json      │◄──────────┐      │
│                       │ REPORT.json  │           │      │
│  ┌──────────────┐     └──────┬───────┘           │      │
│  │ ambiguous-   │            │                   │      │
│  │    agent     │     ┌──────▼───────┐           │      │
│  │ (interactive)│     │agent-dispatch│           │      │
│  └──────────────┘     │  (watcher)   │           │      │
│                       └──────┬───────┘           │      │
│                              │                   │      │
│               ┌──────────────┼──────────────┐    │      │
│               ▼              ▼              ▼    │      │
│         [direct]        [in-repo]    [repo-      │      │
│               │         terraform   isolation]   │      │
│               ▼                     terraform    │      │
│       ┌──────────────┐                           │      │
│       │ agent-worker │                           │      │
│       │  (executor)  │                           │      │
│       └──────────────┘                           │      │
│                                                  │      │
│                       ┌──────────────┐           │      │
│                       │  prpoller    │───────────┘      │
│                       │(PR watcher)  │                  │
│                       └──────────────┘                  │
└─────────────────────────────────────────────────────────┘
```

## Services

| Service | Description |
|---------|-------------|
| [`agent-dispatch`](agent-dispatch/) | Central dispatcher. Watches for work units and routes them to the appropriate execution strategy. |
| [`agent-worker`](agent-worker/) | Executes instruction and report work units by invoking the configured AI agent. |
| [`agent-events`](agent-events/) | Scheduler that generates periodic work units (timer-based or at scheduled times). |
| [`ambiguous-agent`](ambiguous-agent/) | Interactive REPL shell for manually invoking agents with a chat-like interface. |
| [`heuristic-request`](heuristic-request/) | Watches a heuristic directory for pending requests and dispatches them as instructions. |

## Containment Strategies

When dispatching work to AI agents, three containment levels are available:

### Direct
Fire-and-forget execution. The agent runs in the current input directory with no special isolation. Suitable for low-risk tasks.

### In-Repo (`in-repo`)
The agent works on a branch within the **target repository itself**. Changes are submitted as a pull request for human review before merging.

### Repo-Isolation (`repo-isolation`)
A **separate private repository** is created as an isolated clone of the target. The agent works inside this sandbox without access to the original repo's git history. Results are surfaced via a pull request to the target repo after human review.

See [`agent-dispatch/modules/containment/`](agent-dispatch/modules/containment/) for the Terraform modules implementing each strategy.

## Work Unit Types

Work units are directories placed in `INPUT_DIR/any/<id>/`:

| File | Type | Description |
|------|------|-------------|
| `INSTRUCTION.json` / `INSTRUCTION.md` | Instruction | A task for the agent to execute |
| `REPORT.json` / `REPORT.md` | Report | A report generation request (daily, weekly, monthly, custom) |
| `DISPATCH.json` / `DISPATCH.md` | Dispatch | A routing directive (for agent-dispatch watch mode) |

`.json` takes precedence over `.md` when both are present. `.md` files are automatically converted to `.json`.

## Quick Start

```bash
# Build all services
cd agent-dispatch && make build
cd agent-worker  && make build
cd agent-events  && make build

# Run the dispatcher (watch mode)
agent-dispatch

# Run the worker (in a separate terminal)
agent-worker

# Dispatch a one-shot instruction
agent-dispatch --once -i "Run the test suite" -m execute

# Start an interactive agent session
ambiguous-agent/invoke-agent.sh
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Where work units are dropped |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | Where completed work is stored |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Execution records and logs |
| `AGENT_PRESET` | `claude` | Default AI agent (`claude`, `copilot`, `gemini`, `opencode`, `codex`) |
| `GITHUB_PAT` | — | GitHub token (required for in-repo / repo-isolation dispatch) |

## Directory Structure

```
.
├── agent-dispatch/          # Dispatcher service and containment Terraform modules
│   ├── cmd/prpoller/        # Standalone PR comment polling service
│   ├── modules/containment/ # Terraform modules for each containment strategy
│   │   ├── approval/        # Approval request workflow
│   │   ├── in-repo/         # In-repo branch + PR workflow
│   │   ├── orig/            # Original containment module
│   │   └── repo-isolation/  # Isolated private repo workflow
│   └── prpoller/            # GitHub PR comment polling library
├── agent-events/            # Event scheduler (timer & schedule-based)
├── agent-worker/            # Work unit executor
├── ambiguous-agent/         # Interactive agent shell
├── heuristic-request/       # Heuristic-based request dispatcher
└── deprecated-agent-dispatch-watch/  # Legacy watcher (superseded by agent-dispatch)
```

## License

MIT — see [LICENSE](LICENSE).
