# ambiguous-agent

An interactive REPL shell for manually invoking AI agents. Provides a persistent session with colored output, multi-line input support, command history, and per-session JSON logging.

## Components

- **`invoke-agent.sh`** — Universal wrapper script that invokes any supported AI agent and records metadata (timing, git context, exit code) for each invocation.
- **`main.go`** — Interactive shell built on top of `invoke-agent.sh`. Provides a user-friendly REPL with session management.

## Usage

### Interactive Shell

```bash
./invoke-agent.sh     # Start a session using the default agent (claude)
```

Or build and run the Go shell:

```bash
make build
./ambiguous-shell
```

Session commands:
| Command | Description |
|---------|-------------|
| `agent <prompt>` | Invoke the current agent with a prompt |
| `set-agent <name>` | Switch the active agent preset |
| `list-agents` | Show available agent presets |
| `exit!` | End the session |
| `cd <dir>` | Change working directory |
| `cd -` | Return to previous directory |

### Direct Agent Invocation

Use `invoke-agent.sh` directly from other scripts or services:

```bash
# Prompt mode (read-only)
./invoke-agent.sh -p "Explain the architecture of this repo"

# Execute mode (can write files)
./invoke-agent.sh -e "Add unit tests for the parser"

# Use a specific agent preset
./invoke-agent.sh -a gemini -p "Review this code"

# Read prompt from a file (avoids shell argument parsing issues)
./invoke-agent.sh -e -f /path/to/prompt.txt

# Execute mode with additional context directory
./invoke-agent.sh -e --add-dir /path/to/context "Fix the bug"
```

### Flags

| Flag | Description |
|------|-------------|
| `-p` | Prompt mode (default) — agent can read but not write files |
| `-e` | Execute mode — agent can read and write files |
| `-a <preset>` | Select agent preset (default: `claude`) |
| `-f <file>` | Read the prompt from a file instead of an argument |

## Agent Presets

| Preset | Command | Notes |
|--------|---------|-------|
| `claude` | `claude` | Claude Code CLI |
| `copilot` | `copilot` | GitHub Copilot CLI |
| `gemini` | `gemini` | Gemini CLI |
| `opencode` | `opencode run` | OpenCode |
| `codex` | `codex` | OpenAI Codex CLI |

The active preset can be set via the `AGENT_PRESET` environment variable.

## Session Records

Each invocation is recorded in `AGENT_RECORDS_PATH` (default: `/workspaces/agent-records/`). Records include:

- `metadata.txt` — date, agent, mode, duration, git branch, exit code
- `raw_output.txt` — full agent output

Calls within a 20-minute window are grouped into a shared session directory for easier review.

Interactive shell sessions additionally write a `session.jsonl` log with per-command timing and exit codes.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AGENT_PRESET` | `claude` | Default agent preset |
| `AGENT_RECORDS_PATH` | `/workspaces/agent-records/` | Where session records are written |

## Building

```bash
cd ambiguous-agent
make build   # produces ./ambiguous-shell binary
```
