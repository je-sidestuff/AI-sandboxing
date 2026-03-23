# agent-worker

Processes AI agent work units from the input directory. Watches for `INSTRUCTION` and `REPORT` work units, invokes the configured agent, and archives the results.

## How It Works

1. Polls `INPUT_DIR/any/` every 10 seconds for new work unit directories.
2. When a work unit is found (identified by `INSTRUCTION.json`, `INSTRUCTION.md`, `REPORT.json`, or `REPORT.md`), it creates a `PROCESSING.md` marker to claim the unit.
3. Invokes `invoke-agent.sh` with the appropriate mode flag and instruction.
4. On completion, moves content to `OUTPUT_DIR/content/<id>/` and metadata to `OUTPUT_DIR/records/<id>-<timestamp>/`.
5. Writes a JSON record to `RECORDS_DIR/worker/`.

## Work Unit Formats

### Instruction

**`INSTRUCTION.json`** (takes precedence):
```json
{
  "instruction": "Run the test suite and report any failures.",
  "mode": "execute",
  "agent": "claude"
}
```

**`INSTRUCTION.md`** (auto-converted to `INSTRUCTION.json` with `mode: "prompt"`):
```markdown
Run the test suite and report any failures.
```

Fields:
- `instruction` — the prompt text (required)
- `mode` — `"prompt"` (default) or `"execute"` (grants file write access)
- `agent` — override the default agent preset (optional)

### Report

**`REPORT.json`** (takes precedence):
```json
{
  "type": "daily",
  "agent": "claude"
}
```

**`REPORT.md`** (auto-converted to `REPORT.json` with `type: "custom"`):
```markdown
Summarize the key decisions made this week.
```

Report types:
- `custom` — uses the `content` field as the prompt
- `daily` — summarize today's agent activity
- `weekly` — summarize the past week's activity
- `monthly` — summarize the past month's activity

## Output Layout

```
OUTPUT_DIR/
├── content/<work-unit-id>/         # Files created by the agent
│   └── PROCESSED-<timestamp>.md   # Processing summary
└── records/<work-unit-id>-<ts>/    # Metadata files
    ├── INSTRUCTION.json            # Original instruction
    └── PROCESSED.md                # Processing summary

RECORDS_DIR/worker/
└── <worker-id>_<work-unit-id>_<unix-ts>.json  # Structured record
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Directory to watch for work units |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | Directory for completed work output |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Directory for worker records |
| `AGENT_PRESET` | `claude` | Default agent (`claude`, `copilot`, `gemini`, `opencode`, `codex`) |
| `WORKER_INSTRUCTION_ENABLED` | `true` | Set to `false` to skip instruction work units |
| `WORKER_REPORT_ENABLED` | `true` | Set to `false` to skip report work units |

## Building and Running

```bash
cd agent-worker
make build   # produces ./agent-worker binary
make run     # build and run
```

The worker locates `invoke-agent.sh` by searching:
1. Same directory as the binary
2. Current working directory
3. `./ambiguous-agent/invoke-agent.sh`
4. `../ambiguous-agent/invoke-agent.sh`
5. `$PATH`

## Logging

The worker uses exponential backoff for idle-state logging to reduce noise:
- First log after 30 seconds of no activity
- Then every 5 minutes
- Then every hour
- Then every 24 hours

Activity resets the backoff counter.
