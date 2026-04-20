# AI-Sandboxing

A multi-component orchestration framework for safely dispatching, executing, and monitoring AI agents. The system provides multiple containment strategies including in-repo branching, repository isolation, approval gates, and sequenced execution workflows.

## Overview

AI-Sandboxing enables controlled AI agent execution through an event-driven architecture. Work units flow through a pipeline from heuristic input to dispatch to isolated execution, with comprehensive auditing at every step.

```
Heuristic Input → heuristic-request → agent-dispatch → agent-worker → Output
                                            ↓
                              Terraform Containment Workflows
                        (in-repo | repo-isolation | approval | sequence)
```

## Quick Start

```bash
# Build core components
cd agent-dispatch && make build
cd agent-worker && make build
cd heuristic-request && make build

# For agent-worker/heuristic-request, run setup first (requires sudo)
make setup
make run
```

---

## Dependency Graph

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER INPUT LAYER                                │
├─────────────────────────────────────────────────────────────────────────────┤
│   agent-scribe (voice)  ──────┐                                             │
│   HEURISTIC.md/json     ──────┼──→ heuristic-request ──→ DISPATCH/INSTRUCTION│
│   Manual submission     ──────┘                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            DISPATCH LAYER                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                          agent-dispatch                                      │
│   ┌─────────────┬───────────────┬──────────────┬─────────────────────┐      │
│   │   direct    │   in-repo     │ repo-isolation│ sequence-to-new-repo│      │
│   │  (no TF)    │  (branch)     │  (clone)      │  (multi-step)       │      │
│   └──────┬──────┴───────┬───────┴───────┬──────┴──────────┬──────────┘      │
│          │              │               │                 │                  │
│          │              └───────────────┴─────────────────┘                  │
│          │                              │                                    │
│          │                    Terraform Workflows                            │
│          │                   (dispatcher/live/flows/)                        │
└──────────┼──────────────────────────────┼───────────────────────────────────┘
           │                              │
           ▼                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           EXECUTION LAYER                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                          agent-worker                                        │
│          ┌──────────────────────────────────────────────┐                   │
│          │  Restricted Unix User Environment            │                   │
│          │  ┌─────────────┐  ┌────────────────────┐    │                   │
│          │  │ Read-only   │  │ Writable workspace │    │                   │
│          │  │ workspace   │  │ (output)           │    │                   │
│          │  └─────────────┘  └────────────────────┘    │                   │
│          └──────────────────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          MONITORING LAYER                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│   agent-viewer (Web UI)         agent-records/         pkg/filestory        │
│   (WebSocket live feed)         (session logs)         (file audit trail)   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Sub-Project Dependencies

```
                              ┌──────────────────┐
                              │  SHARED PACKAGES │
                              └────────┬─────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        │
    ┌─────────────────┐     ┌─────────────────┐                │
    │  pkg/filestory  │     │  pkg/agentaudit │                │
    │  (file logging) │     │ (context audit) │                │
    └────────┬────────┘     └────────┬────────┘                │
             │                       │                         │
   ┌─────────┼───────────────────────┼─────────────────┐       │
   │         │                       │                 │       │
   ▼         ▼                       ▼                 ▼       │
┌──────────────────┐  ┌──────────────────┐  ┌─────────────────┐│
│  agent-dispatch  │  │   agent-worker   │  │heuristic-request││
│                  │  │                  │  │                 ││
│ google/uuid      │  │ google/uuid      │  │ google/uuid     ││
│ pkg/filestory    │  │ pkg/filestory    │  │ pkg/filestory   ││
│                  │  │ pkg/agentaudit   │  │ pkg/agentaudit  ││
└──────────────────┘  └──────────────────┘  └─────────────────┘│
                                                               │
┌──────────────────┐  ┌──────────────────┐  ┌─────────────────┐│
│   agent-viewer   │  │  agent-scribe    │  │ ambiguous-agent ││
│                  │  │   (Node.js)      │  │                 ││
│ fsnotify         │  │ @aws-sdk         │  │ charmbracelet   ││
│ gorilla/websocket│  │ express          │  │ chzyer/readline ││
│ golang.org/x/net │  │ socket.io        │  │ google/uuid     ││
└──────────────────┘  │ uuid             │  └─────────────────┘│
                      └──────────────────┘                     │
                                                               │
┌──────────────────┐  ┌──────────────────┐                     │
│  agent-executor  │──│  agent-recorder  │                     │
│   (depends on)   │  │   (local pkg)    │                     │
└──────────────────┘  └──────────────────┘                     │
```

### Terraform Module Hierarchy

```
agent-dispatch/
├── modules/
│   ├── containment/
│   │   ├── in-repo/           ─────────────────────────────────────┐
│   │   │   ├── main.tf        Branch-based containment             │
│   │   │   └── README.md      Creates PR in target repo            │
│   │   │                                                           │
│   │   ├── repo-isolation/    ─────────────────────────────────────┤
│   │   │   ├── main.tf        Private repo clone                   │
│   │   │   └── README.md      Full isolation, PR back to original  │
│   │   │                                                           ├─► GitHub
│   │   ├── approval/          ─────────────────────────────────────┤   Provider
│   │   │   └── main.tf        Human approval gate                  │
│   │   │                      Creates approval PR for review       │
│   │   │                                                           │
│   │   ├── to-repo/           ─────────────────────────────────────┤
│   │   │   └── main.tf        Repository targeting                 │
│   │   │                                                           │
│   │   └── scripts/           ─────────────────────────────────────┘
│   │       ├── fetch_pr_comments.py      GitHub API interactions
│   │       ├── post_revise_comments.py   Post revision status
│   │       ├── handle_revise_comments.sh Handle REVISE: flow
│   │       └── init_containment_branch.sh Branch initialization
│   │
│   └── execution/
│       ├── single/            ─────────────────────────────────────┐
│       │   └── main.tf        Single command execution             │
│       │                                                           │
│       └── sequence/          ─────────────────────────────────────┤
│           ├── main.tf        Multi-step time-based execution      │
│           └── single/        Per-step handling                    │
│               └── main.tf                                         │
│                                                                   │
└── examples/                  ─────────────────────────────────────┘
    ├── containment/           Example configurations
    │   ├── in-repo/main.tf
    │   ├── repo-isolation/main.tf
    │   ├── approval/main.tf
    │   └── to-repo/main.tf
    └── execution/
        ├── single/main.tf
        └── sequence/main.tf
```

---

## Filesystem Layout

### Repository Structure

```
AI-sandboxing/
│
├── Core Orchestration ─────────────────────────────────────────────
│   ├── agent-dispatch/          Central dispatcher for work units
│   │   ├── main.go              Watch/dispatch logic (3,423 lines)
│   │   ├── modules/             Terraform modules (see above)
│   │   └── examples/            Terraform examples
│   │
│   ├── agent-worker/            Isolated execution environment
│   │   └── main.go              Worker logic (1,569 lines)
│   │
│   └── heuristic-request/       Heuristic → DISPATCH conversion
│       ├── main.go              Processor logic (1,176 lines)
│       └── roles/               Role definitions (code-implementer, etc.)
│
├── Visualization & Monitoring ─────────────────────────────────────
│   ├── agent-viewer/            Web UI for session browsing
│   │   ├── main.go              Server + API (33,514 lines)
│   │   └── frontend/            Embedded React application
│   │
│   └── agent-scribe/            Voice transcription service
│       ├── server.js            Express + Socket.IO
│       └── package.json         Node.js dependencies
│
├── Supporting Utilities ───────────────────────────────────────────
│   ├── agent-events/            Scheduled event management
│   ├── agent-executor/          Execution orchestration stub
│   ├── agent-recorder/          Session recording library
│   ├── ambiguous-agent/         Interactive agent selection CLI
│   └── declarative-tool-tool/   Declarative tool creation (T3s)
│
├── Shared Packages ────────────────────────────────────────────────
│   └── pkg/
│       ├── filestory/           File operation logging
│       └── agentaudit/          Agent context snapshots
│
└── Documentation ──────────────────────────────────────────────────
    └── discussion/              Design documents
```

### Runtime Data Flow (slopspaces)

```
/workspaces/slopspaces/
│
├── Input Pipeline ─────────────────────────────────────────────────
│   ├── heuristic/
│   │   ├── pending/             ← Incoming heuristics
│   │   │   └── <id>/HEURISTIC.json
│   │   └── processed/           → Processed results
│   │       └── <id>/PROCESSED.md
│   │
│   └── input/any/               ← Dispatcher input queue
│       └── <work-id>/
│           └── DISPATCH.json | INSTRUCTION.json
│
├── Terraform Workflows ────────────────────────────────────────────
│   └── dispatcher/live/flows/
│       ├── approval/<flow-id>/
│       ├── in-repo/<flow-id>/
│       ├── repo-isolation/<flow-id>/
│       └── sequence-to-new-repo/<flow-id>/
│
├── Execution State ────────────────────────────────────────────────
│   ├── working/                 Git-isolated task workspaces
│   │   └── <session-id>/
│   │       ├── branch_name
│   │       └── git_state/.git/
│   │
│   └── requests/                Approval request tracking
│       └── <session-id>/DISPATCH.json
│
├── Output ─────────────────────────────────────────────────────────
│   └── output/
│       ├── <session-id>/
│       │   ├── PROCESSING.md
│       │   └── PROCESSED.md
│       └── content/<work-id>/PROCESSED-*.md
│
└── Audit & Records ────────────────────────────────────────────────
    ├── agent-records/
    │   ├── <timestamp_id>/      Session logs
    │   │   ├── metadata.txt
    │   │   └── raw_output.txt
    │   ├── dispatch/            Single-shot records
    │   ├── dispatch-watch/      Watch mode records
    │   └── worker/              Worker completion records
    │
    └── events/config/           Scheduled event definitions
        ├── default-daily-report.json
        └── custom-heartbeat-report.json
```

---

## Component Details

| Component | Type | Purpose | Key Dependencies |
|-----------|------|---------|------------------|
| **agent-dispatch** | Go | Central work dispatcher | google/uuid, pkg/filestory |
| **agent-worker** | Go | Isolated execution | google/uuid, pkg/filestory, pkg/agentaudit |
| **heuristic-request** | Go | Heuristic processing | google/uuid, pkg/filestory, pkg/agentaudit |
| **agent-viewer** | Go + React | Web monitoring UI | fsnotify, gorilla/websocket |
| **agent-scribe** | Node.js | Voice transcription | @aws-sdk, express, socket.io |
| **ambiguous-agent** | Go | Interactive agent selection | charmbracelet/lipgloss |
| **declarative-tool-tool** | Go | Tool creation CLI | standard library |
| **pkg/filestory** | Go library | File operation audit | standard library |
| **pkg/agentaudit** | Go library | Context snapshots | standard library |

---

## Containment Strategies

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          CONTAINMENT LEVELS                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   DIRECT          No isolation - immediate execution                     │
│   ────────────────────────────────────────────────────────────────────  │
│                                                                          │
│   IN-REPO         Branch-based containment in target repository          │
│   ─────────       ┌──────────┐    ┌──────────┐    ┌──────────┐          │
│                   │ Create   │───▶│ Execute  │───▶│ Create   │          │
│                   │ Branch   │    │ on Branch│    │ PR       │          │
│                   └──────────┘    └──────────┘    └──────────┘          │
│                                                                          │
│   REPO-ISOLATION  Full clone to private isolation repository             │
│   ──────────────  ┌──────────┐    ┌──────────┐    ┌──────────┐          │
│                   │ Clone to │───▶│ Execute  │───▶│ PR to    │          │
│                   │ Priv Repo│    │ Isolated │    │ Original │          │
│                   └──────────┘    └──────────┘    └──────────┘          │
│                                                                          │
│   APPROVAL-GATED  Human review before execution                          │
│   ──────────────  ┌──────────┐    ┌──────────┐    ┌──────────┐          │
│                   │ Create   │───▶│ Wait for │───▶│ Execute  │          │
│                   │ Approval │    │ Approval │    │ on Merge │          │
│                   │ PR       │    │          │    │          │          │
│                   └──────────┘    └──────────┘    └──────────┘          │
│                                                                          │
│   SEQUENCE        Multi-step with time-based progression                 │
│   ────────        ┌────┐  ┌────┐  ┌────┐  ┌────┐                        │
│                   │ S1 │─▶│ S2 │─▶│ S3 │─▶│ S4 │  (configurable delay)  │
│                   └────┘  └────┘  └────┘  └────┘                        │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Environment Variables

| Variable | Default | Used By | Purpose |
|----------|---------|---------|---------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | agent-dispatch | Work unit input |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | agent-dispatch | Completed output |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | All agents | Execution logs |
| `DISPATCHER_LIVE` | `/workspaces/slopspaces/dispatcher/live` | agent-dispatch | TF configs |
| `HEURISTIC_DIR` | `/workspaces/slopspaces/heuristic` | heuristic-request | Heuristic I/O |
| `GITHUB_PAT` | (required) | agent-dispatch | GitHub auth |
| `FILE_STORY_PATH` | (optional) | worker, heuristic | File audit path |
| `AGENT_AUDIT` | (optional) | worker, heuristic | Set `FULL` for audit |
| `AGENT_USER` | `agent-worker` | worker, heuristic | Restricted Unix user |

---

## License

See individual component licenses.
