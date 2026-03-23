# agent-events

Schedules recurring AI agent work units. Reads event configurations from a config directory and creates `REPORT.json` work units in the agent-worker input directory when events fire.

## How It Works

1. On startup, creates default event configs if none exist.
2. Polls active event configurations every 10 seconds.
3. For each enabled event, checks whether it should fire (based on timer interval or daily schedule).
4. When an event fires, creates a work unit directory in `INPUT_DIR/any/` containing a `REPORT.json`.
5. `agent-worker` picks up the work unit and executes the report.

## Event Configuration

Event configs are JSON files stored in `EVENTS_CONFIG_DIR`. Each file defines one event.

### Timer Event

Fires at a fixed interval after the previous run (or immediately on first startup):

```json
{
  "name": "custom-heartbeat-report",
  "type": "timer",
  "description": "Custom heartbeat report every 6 hours",
  "interval": "6h",
  "report_type": "custom",
  "topic_style": "random_words",
  "enabled": true
}
```

Fields:
- `interval` — Go duration string (e.g. `"30m"`, `"6h"`, `"24h"`)
- `report_type` — `"custom"`, `"daily"`, `"weekly"`, or `"monthly"`
- `topic_style` — set to `"random_words"` to auto-generate a creative topic (adjective + noun)

### Schedule Event

Fires once per day after a specified time of day:

```json
{
  "name": "default-daily-report",
  "type": "schedule",
  "description": "Creates a daily report for yesterday's activities",
  "schedule_at": "09:00",
  "report_type": "daily",
  "enabled": true
}
```

Fields:
- `schedule_at` — 24-hour time string (`"HH:MM"`)
- `report_type` — currently only `"daily"` is supported for schedule events

### Default Configs

If no config files are found in `EVENTS_CONFIG_DIR`, the following defaults are created automatically:

| Name | Type | Description |
|------|------|-------------|
| `default-daily-report` | schedule @ 09:00 | Daily report for yesterday |
| `custom-heartbeat-report` | timer every 6h | Creative report on a random topic |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `EVENTS_CONFIG_DIR` | `/workspaces/slopspaces/events/config` | Directory containing event config JSON files |
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Where work units are created |

## Building and Running

```bash
cd agent-events
make build   # produces ./agent-events binary
make run     # build and run
```

## Deduplication

For `daily` schedule events, the manager checks both its in-memory state and any existing pending work units in `INPUT_DIR/any/` before creating a new one. This prevents duplicate reports if the process restarts during the day.

Timer events fire immediately on first startup, then respect the configured interval for subsequent runs.
