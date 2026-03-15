# slopspaces agent orchestration

A lightweight system for dispatching work units to AI agents and collecting their outputs. Four cooperating binaries form a processing pipeline:

```
agent-events ──┐
               ├──► input/any/{work-unit}/ ──► agent-worker ──► output/{work-unit}/
agent-dispatch ┘                                    │
                                                    └──► invoke-agent.sh ──► AI agent
```

## Components

### `agent-dispatch`
CLI tool for submitting work units and waiting for results. Use it to send instructions or request reports from agents programmatically or from scripts.

```sh
# Run an instruction and wait for completion
agent-dispatch -i "Summarize the recent changes" -m execute

# Dispatch asynchronously, then poll
agent-dispatch -i "Long running task" --async
agent-dispatch --check <work-unit-id>
agent-dispatch --wait  <work-unit-id>

# Request a report
agent-dispatch -r daily
agent-dispatch -r custom -c "Analyse performance trends"
```

### `agent-worker`
Long-running daemon that polls `input/any/` for work unit directories and processes them one at a time. For each work unit it:
1. Writes `PROCESSING.md` to claim the unit
2. Invokes `invoke-agent.sh` with the appropriate agent and mode
3. Moves the completed folder to `output/` and writes `PROCESSED.md`
4. Saves a JSON record under `agent-records/worker/`

```sh
# Environment variables
AGENT_PRESET=claude              # which agent to use (default: claude)
WORKER_INSTRUCTION_ENABLED=true  # process INSTRUCTION work units (default: true)
WORKER_REPORT_ENABLED=true       # process REPORT work units (default: true)
INPUT_DIR=/workspaces/slopspaces/input/
OUTPUT_DIR=/workspaces/slopspaces/output/
RECORDS_DIR=/workspaces/slopspaces/agent-records/
```

### `agent-events`
Long-running daemon that generates work units on a schedule. Two event types are supported:

| Type       | Config field   | Behaviour                                          |
|------------|----------------|----------------------------------------------------|
| `timer`    | `interval`     | Fires once immediately, then every `interval`      |
| `schedule` | `schedule_at`  | Fires once per day after the specified time (HH:MM)|

Default configs are created automatically on first run:
- **`default-daily-report`** — daily report at 09:00
- **`custom-heartbeat-report`** — custom report every 6 hours with a random "adjective noun" topic

Add custom events by dropping JSON files into `events/config/`.

### `ambiguous-agent`
Interactive readline shell for manual agent invocations. All commands and their outputs are recorded to a session JSONL file.

```sh
# Special commands
set-agent claude     # switch active agent
list-agents          # show available presets
agent <prompt>       # invoke the active agent
exit!                # end the session
# Anything else is passed to bash
```

### `invoke-agent.sh`
Wrapper script that normalises invocations across agent presets (claude, copilot, gemini, opencode, codex). Every call is recorded with metadata (agent, mode, git context, duration, exit code) grouped by a 20-minute session window.

```sh
invoke-agent.sh -e -a claude "Fix the failing tests"   # execute mode
invoke-agent.sh -p -a claude "What does this do?"      # prompt-only mode
invoke-agent.sh -e -f instruction.txt                  # read prompt from file
```

## Work unit format

A work unit is a directory placed in `input/any/`. The worker detects and processes it based on the files it contains.

### Instruction work unit
```
input/any/my-task/
└── INSTRUCTION.json   # takes precedence over INSTRUCTION.md
```

`INSTRUCTION.json`:
```json
{
  "instruction": "Refactor the auth module to use the new SDK",
  "mode": "execute",
  "agent": "claude"
}
```
`mode` must be `"prompt"` (read-only) or `"execute"` (allows file edits). `agent` is optional and overrides the worker default.

An `INSTRUCTION.md` file is also accepted; the worker converts it to JSON automatically (defaulting to `prompt` mode).

### Report work unit
```
input/any/my-report/
└── REPORT.json   # takes precedence over REPORT.md
```

`REPORT.json`:
```json
{
  "type": "daily",
  "date": "2026-03-14",
  "agent": "claude"
}
```
`type` must be one of `custom`, `daily`, `weekly`, or `monthly`. Custom reports require a `content` field with the report prompt.

## Output structure

After processing, the work unit directory is moved to `output/` and a `PROCESSED.md` file is added:

```
output/my-task/
├── INSTRUCTION.json
├── PROCESSING.md
├── PROCESSED.md      ← added by worker; contains exit code, duration, etc.
└── <any files written by the agent>
```

## Records

| Path                              | Contents                                      |
|-----------------------------------|-----------------------------------------------|
| `agent-records/worker/*.json`     | Per-work-unit metadata (timing, exit code)    |
| `agent-records/dispatch/*.json`   | Dispatch operation records                    |
| `agent-records/<date>/<id>/`      | Raw agent output + invocation metadata        |
| `agent-records/session-<id>/`     | ambiguous-agent session transcripts           |

## Development

Each component is an independent Go module. From any component directory:

```sh
make build   # compile binary
make run     # build and run
make test    # run tests
make fmt     # gofmt
make vet     # go vet
```

Go 1.21 or later is required.
