# ambiguous-agent

An interactive REPL shell for direct AI agent interaction. Combines a familiar shell-like experience with AI agent invocation, session recording, and multi-line input support.

## Overview

`ambiguous-agent` provides a readline-based interactive shell where you can run shell commands and invoke AI agents side by side. Every session is recorded with structured metadata including timing, exit codes, and git context. Sessions are grouped into 20-minute windows for easy review.

The component also provides two scripts used by the broader orchestration system:
- **`invoke-agent.sh`** — universal agent invocation wrapper used by `agent-worker` and other components
- **`agent-report.sh`** — generates summary reports from recorded session history

## Usage

```bash
cd ambiguous-agent
go build -o ambiguous-agent .
./ambiguous-agent
```

### Shell Commands

Inside the REPL, you can run any shell command. The working directory is tracked and persists across commands:

```
● session: /workspaces/agent-records/session-2024-03-21_09-00-00_1711015200
  agent: claude | 'set-agent <name>' to change | 'list-agents' for options
  type 'exit!' to end | 'agent <prompt>' to invoke AI
  multi-line: trailing \, unclosed quotes, or <<<DELIMITER

/workspaces/slopspaces [claude] > ls -la
/workspaces/slopspaces [claude] > cd agent-worker
/workspaces/slopspaces/agent-worker [claude] > make build
```

### AI Agent Invocation

Prefix any prompt with `agent` to invoke the configured AI agent:

```
/workspaces/slopspaces [claude] > agent What files are in this directory?
/workspaces/slopspaces [claude] > agent -e Refactor the main function in main.go
```

### Built-in Commands

| Command | Description |
|---------|-------------|
| `set-agent <name>` | Switch the active AI agent |
| `list-agents` | Show all available agents |
| `cd <path>` | Change directory (supports `-` for previous directory) |
| `exit!` | End the session |

### Multi-line Input

Three methods for multi-line input:

1. **Backslash continuation:** End a line with `\` to continue on the next line
2. **Heredoc:** Start with `<<<DELIMITER`, end with `DELIMITER` on its own line
3. **Unclosed quotes:** Open a quote without closing it to span lines

## invoke-agent.sh

The universal agent wrapper script used by all components in the system.

### Usage

```bash
./invoke-agent.sh [-p|-e] [-a <agent>] [-f <prompt-file>] [<prompt>]
```

**Flags:**
- `-p` — prompt mode (read-only)
- `-e` — execute mode (allow file modifications)
- `-a <agent>` — agent preset (claude, copilot, gemini, opencode, codex)
- `-f <file>` — read prompt from file instead of argument

### Supported Agents

| Preset | Description |
|--------|-------------|
| `claude` | Anthropic's Claude via Claude Code CLI |
| `copilot` | GitHub Copilot |
| `gemini` | Google Gemini |
| `opencode` | OpenCode AI |
| `codex` | OpenAI Codex |

### Records

Each invocation appends a record to `AGENT_RECORDS_PATH` (default: `/workspaces/slopspaces/agent-records/`). Records are grouped into 20-minute session windows, with each group sharing a directory. Record files capture:

- Command invoked
- Prompt and mode
- Git context (branch, commit, dirty state)
- Start time, duration, and exit code
- Group ID for session correlation

## agent-report.sh

Generates reports from agent session records.

```bash
./agent-report.sh [daily|weekly|monthly|custom] [options]
```

Supported modes:
- `daily` — report on yesterday's sessions
- `weekly` — report on the past 7 days
- `monthly` — report on the past 30 days
- `custom <start-date> <end-date>` — report on a specific date range

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_RECORDS_PATH` | `/workspaces/agent-records/` | Directory for session records |
| `AGENT_PRESET` | `claude` | Default AI agent |

## Session Records

Each session creates a directory at `AGENT_RECORDS_PATH/session-<timestamp>/` containing:

- `session.jsonl` — newline-delimited JSON log of every command
- One record file per agent invocation with full context

Records are organized into 20-minute groups so related commands appear together when reviewing history.

## Building

```bash
cd ambiguous-agent
go build -o ambiguous-agent .
```
