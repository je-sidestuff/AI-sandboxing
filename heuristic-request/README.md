# heuristic-request

Watches a heuristic input directory for pending requests and uses an AI agent to decide what action to take, then dispatches the result as an instruction or dispatch work unit.

## How It Works

1. Polls `HEURISTIC_DIR/pending/` every 10 seconds for subdirectories containing `HEURISTIC.md`.
2. For each pending heuristic unit, creates a `PROCESSING.md` marker and invokes the configured agent with a structured prompt.
3. The agent reads the heuristic content and outputs either a `DISPATCH.json`, `DISPATCH.md`, or `INSTRUCTION.json` file.
4. The heuristic watcher extracts any files from the agent's output and places them in `REQUEST_DIR` (the agent-worker / agent-dispatch input directory).
5. The original heuristic folder is moved to `HEURISTIC_DIR/processed/` for archival.

## Heuristic Input Format

Place a directory under `HEURISTIC_DIR/pending/` containing a `HEURISTIC.md` file:

```
HEURISTIC_DIR/pending/
└── my-request-001/
    └── HEURISTIC.md
```

**`HEURISTIC.md`** — free-form markdown describing the situation or request:

```markdown
The CI pipeline has been failing for the last 3 builds.
Error logs suggest a dependency version conflict in the auth module.
The issue started after the latest merge to main.
```

The agent will analyse the heuristic and decide whether to create:
- A **`DISPATCH.json`** for orchestrated multi-step work (branches, PRs, etc.)
- A **`DISPATCH.md`** for a simpler prose dispatch
- An **`INSTRUCTION.json`** for a direct agent task

## Directory Layout

```
HEURISTIC_DIR/
├── pending/
│   └── <request-id>/
│       └── HEURISTIC.md        # Input — free-form description
└── processed/
    └── <request-id>/
        ├── HEURISTIC.md        # Original input (archived)
        └── PROCESSED.md        # Processing summary

REQUEST_DIR/  (= INPUT_DIR/any/)
└── <generated-work-unit-id>/
    └── DISPATCH.json           # or INSTRUCTION.json — ready for agent-dispatch / agent-worker
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HEURISTIC_DIR` | `/workspaces/slopspaces/heuristic` | Root directory for heuristic input/output |
| `REQUEST_DIR` | `/workspaces/slopspaces/input/any` | Where generated work units are placed |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | For heuristic processing records |
| `AGENT_PRESET` | `claude` | Agent to use for heuristic analysis |

## Building and Running

```bash
cd heuristic-request
make build   # produces ./heuristic-request binary
make run     # build and run

# Or with custom flags:
./heuristic-request -agent gemini
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-agent` | `$AGENT_PRESET` or `claude` | Agent preset for heuristic analysis |
| `-heuristic-dir` | `$HEURISTIC_DIR` | Override heuristic directory |
| `-request-dir` | `$REQUEST_DIR` | Override request output directory |
