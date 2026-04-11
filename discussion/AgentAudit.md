# Agent Audit Subsystem

## What this is

The agent-audit subsystem captures a full snapshot of an agent's universe at the moment it is invoked. The goal is observability: given any past invocation, a human (or future tool) should be able to reconstruct exactly what the agent saw and what it was asked to do.

## Scope of this increment

Instrumented components:
- **heuristic-request** — captures a snapshot each time a heuristic unit is processed
- **agent-worker** — captures a snapshot for both instruction invocations and report invocations

## How it works

### Activation

Auditing is controlled by the `AGENT_AUDIT` environment variable:

| Value | Behaviour |
|-------|-----------|
| unset / empty | No audit written (zero overhead) |
| `FULL` | Full audit written to `/agent-audit/` |

No other values are defined at this time.

### Audit location

```
/agent-audit/
├── heuristic-request/
│   └── <watcherID>/
│       └── 2026-04-08_19-04-05.123/
│           ├── audit.json       # metadata: type, id, agent, timestamp, paths
│           ├── prompt.txt       # the complete prompt fed to the agent
│           └── filesystem.txt   # tree listing of every path the agent can see
└── agent-worker/
    └── <workerID>/
        └── 2026-04-08_19-04-05.456/
            ├── audit.json
            ├── prompt.txt
            └── filesystem.txt
```

Path structure: `/agent-audit/<type>/<id>/<timestamp>/`

- `<type>`: component name (`heuristic-request` or `agent-worker`)
- `<id>`: the long-lived watcher/worker instance ID (stable across invocations of the same process)
- `<timestamp>`: UTC timestamp with milliseconds (`2006-01-02_15-04-05.000`), unique per invocation

### What is captured

**`prompt.txt`**
The exact string passed to the agent — including the workspace preamble that describes filesystem boundaries.

**`filesystem.txt`**
A tree listing (filenames + structure, no file contents) of every path the agent has access to at invocation time.

For heuristic-request this includes:
- `workspace.RootPath` (`/agent/heuristic-request/`) — the isolated workspace with read/ and write/ sub-trees
- `folderPath` — the pending heuristic folder (the work unit that triggered this invocation)

For agent-worker this includes:
- `workspace.RootPath` (`/agent/agent-worker/`) — the full workspace containing read/default, read/workunit, and write/primary
- `folderPath` — the input work unit folder

**`audit.json`**
Metadata: agent type, instance ID, agent name, timestamp (RFC3339), list of fs_paths captured, audit directory path.

### Code location

| File | Role |
|------|------|
| `pkg/agentaudit/agentaudit.go` | Core package: `Capture()`, `IsEnabled()`, tree walker |
| `pkg/agentaudit/go.mod` | Module declaration (no external deps) |
| `heuristic-request/main.go` | Call site in `executeAgent()` |
| `agent-worker/main.go` | Call sites in `executeAgent()` and `executeReportAgent()` |

Both `heuristic-request/go.mod` and `agent-worker/go.mod` reference agentaudit via a local `replace` directive, matching the existing pattern used by `pkg/filestory`.

### Failure handling

Audit failures are non-fatal. The capture call logs a warning and execution continues. An audit failure never blocks an agent invocation.

---

## Open questions for humans

1. **Write permissions on `/agent-audit/`**
   The audit code writes to `/agent-audit/<type>/...` as whatever user runs heuristic-request / agent-worker (typically root in host mode). This directory needs to exist and be writable. Should it be created by `make setup-dirs`, a Dockerfile step, or something else? It is not currently provisioned anywhere.

2. **Capturing at the right moment**
   The current capture happens *after* `prepareWorkspace()` but *before* the agent subprocess is launched. This means the filesystem snapshot reflects the populated workspace (read/default and read/workunit are already populated, write/primary contains only `.prompt.tmp`). Is this the right moment? An alternative would be to snapshot *before* workspace preparation to capture the raw input state.

3. **File contents in the snapshot**
   The filesystem snapshot currently records only names/structure (no file contents). For small workunit files (INSTRUCTION.json, REPORT.json, HEURISTIC.md) including their contents would make the audit fully self-contained for replay. Worth adding in a future increment?

4. **Audit for replay**
   The stated future goal is replay capability. A replay-ready audit would also need: the agent binary version/model, the environment variables passed to invoke-agent.sh, and the full content of small read-space files. None of this is captured yet — flagging so the schema stays open to extension.

5. **Retention / rotation**
   No cleanup or rotation is implemented. `/agent-audit/` will grow unboundedly. Is there an existing rotation mechanism (log rotation, cron cleanup) that this should plug into, or should the subsystem own its own TTL logic?

6. **The `<id>` level in the path**
   Currently `<id>` is the watcher/worker instance ID, which is stable for the lifetime of the process. This groups all invocations from the same process under one directory. An alternative would be to use the per-invocation heuristic ID or work-unit ID instead, making each audit directory correspond to a single work item. Which grouping is more useful to you?
