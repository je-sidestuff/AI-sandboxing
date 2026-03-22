# agent-events

A timer and schedule-based event manager that automatically creates report work units for `agent-worker` to process. Enables hands-free, recurring AI report generation.

## Overview

`agent-events` reads JSON configuration files from `EVENTS_CONFIG_DIR` and fires work units into `INPUT_DIR/any/` on a schedule. It supports two event types:

- **timer**: Fires at a fixed interval (e.g., every 6 hours)
- **schedule**: Fires once per day at a specific time of day (e.g., 09:00)

On first run with no existing configs, the manager creates two default configurations:
- A daily report scheduled at 09:00
- A custom "heartbeat" report on a 6-hour timer with a randomly selected topic

## Usage

```bash
cd agent-events
go build -o agent-events .
./agent-events
```

The manager runs indefinitely until interrupted. It checks for due events every 10 seconds.

## Event Configuration

Event configs are JSON files stored in `EVENTS_CONFIG_DIR`. Each file defines one event.

### Timer Event

Fires at a fixed interval. On startup, a timer event fires immediately on its first check, then repeats at the configured interval.

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

- `interval`: Go duration string (e.g., `"30m"`, `"6h"`, `"24h"`)
- `report_type`: `"daily"`, `"weekly"`, `"monthly"`, or `"custom"`
- `topic_style`: Set to `"random_words"` to auto-generate a topic from adjective+noun pairs

### Schedule Event

Fires once per day when the current time reaches the configured time. Tracks the last run date to avoid duplicate reports.

```json
{
  "name": "daily-report",
  "type": "schedule",
  "description": "Daily activity report at 9am",
  "schedule_at": "09:00",
  "report_type": "daily",
  "enabled": true
}
```

- `schedule_at`: 24-hour time string (`"HH:MM"`)
- For `"daily"` reports, the report covers yesterday's date

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EVENTS_CONFIG_DIR` | `/workspaces/slopspaces/events/config` | Directory containing event config JSON files |
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Input directory where work units are created |

## How It Works

1. At startup, `agent-events` creates default configs if none exist
2. All `.json` files in `EVENTS_CONFIG_DIR` are loaded (disabled events are skipped)
3. Every 10 seconds, the manager evaluates each loaded event against its schedule
4. When an event is due, a `REPORT.json` work unit is created in `INPUT_DIR/any/<work-unit-name>/`
5. `agent-worker` picks up the work unit and generates the report

> **Note:** Configs are loaded once at startup. To add or modify events, restart the manager.

## Work Unit Names

Generated work unit folder names use the pattern:

```
<event-name>_<timestamp>_<short-uuid>
```

For example: `custom-heartbeat-report_2024-03-21_09-00-00_a1b2c3d4`

## Deduplication

For `schedule` events with `report_type: "daily"`, the manager:
1. Tracks the last processed date in memory
2. Checks `INPUT_DIR/any/` for any existing pending `REPORT.json` with the same type and date

This prevents duplicate daily reports even if the manager restarts.

## Building

```bash
cd agent-events
go build -o agent-events .
```
