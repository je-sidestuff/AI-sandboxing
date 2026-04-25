# Prompt

Next we will create 'research/AI-evo1/ambiguous-agent'.

The 'ambiguous-agent' sub-project is a go binary that is the spiritual successor to 'sandbox/AI-sandboxing/ambiguous-agent/invoke-agent.sh'. It is NOT responsible to cover any interactive-shell-like-CLI any longer (like the old 'sandbox/AI-sandboxing/ambiguous-agent/' go functionality or ambiguous-shell).

Ambiguous agent will provide a generic interface for a call to be made to an agent without knowing which agent/model type will fulfill it. It will be session-aware and will always wrap calls with 'clauditable'. It will optionally provide access to agent records for one or more sessions with 'add dir' style functionality. Unlike the previous implementation which accepted flags p/e for prompt/execute - this time we accept p/r/w/x for prompt/read/write/execute. This corresponds to passing config to the underlying agent to (only chat without even reading files/read files only/read and write files/read and write files, and execute commands). If no 'mode' flag is passed it will default to 'read'.

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile) In this case we need to consider that our Docker file must include 'clauditable' as it is a runtime dependency.

For this increment we will support agent listing and selection, but we will not yet support models other than the default. We will use visual flare similar to 'sandbox/AI-sandboxing/ambiguous-agent/main.go' to reflect which agent we are using. We will support the same set of agents we supported in 'sandbox/AI-sandboxing'.

The test suite should be very simple to begin and should leverage 'clod' as the harnessed agent.


# Reply

The project is complete. Let me provide a summary of what was created:

## Summary: `research/AI-evo1/ambiguous-agent` Created

I've implemented the `ambiguous-agent` project as specified. Here's what was created:

### Files Created

| File | Description |
|------|-------------|
| `main.go` | Core CLI implementation with agent invocation, mode flags, clauditable wrapping |
| `main_test.go` | Test suite covering agent configs, modes, arg building, and clod integration |
| `go.mod` / `go.sum` | Go module files with lipgloss dependency |
| `Makefile` | Build targets: `build`, `docker-build`, `run`, `clean`, `test` |
| `Dockerfile` | Multi-stage build with clauditable as runtime dependency |
| `.gitignore` | Ignores binary, test artifacts, .env |
| `.github/workflows/ambiguous-agent-test.yml` | CI workflow on `test/ambiguous-agent` branch |

### Key Features

1. **Mode Flags (p/r/w/x)**:
   - `-p`: Prompt only (chat without file access)
   - `-r`: Read mode (default - read files only)
   - `-w`: Write mode (read and write files)
   - `-x`: Execute mode (full access including commands)

2. **Agent Support**: copilot, gemini, claude, opencode, codex, grok, clod (test agent)

3. **Clauditable Integration**: All agent calls are wrapped with `clauditable` for record-keeping. Falls back to direct invocation with a warning if clauditable is not found.

4. **Session Awareness**: Auto-generates session IDs or uses `AGENT_SESSION` env var

5. **Visual Flare**: Agent-specific colors matching the original implementation (claude=orange, gemini=blue, etc.)

6. **Add-Dir Support**: Passes session directory and additional directories to agents that support `--add-dir`

### Usage Examples

```bash
# Read mode (default)
ambiguous-agent -r "What files are in this directory?"

# Write mode with specific agent
ambiguous-agent -w -a gemini "Update the README"

# Execute mode (full access)
ambiguous-agent -x "Run tests and fix failures"

# List available agents
ambiguous-agent --list-agents
```

Records saved to: /workspaces/agent-records/session-2026-04-23_18-49-19_1776970159/2026-04-25_13-56-31_1777125391


## Reprompt A

We have a bug when we use 'make run' with 'research/AI-evo1/ambiguous-agent':
vscode ➜ /workspaces/research/AI-evo1/ambiguous-agent (main) $ make run
go build -o ambiguous-agent .
Running ambiguous-agent...
./ambiguous-agent -r -p "What files are in this directory?"
Error: only one mode flag (-p, -r, -w, -x) can be specified
make: *** [Makefile:18: run] Error 1

We must also adjust a few things:
 - We need to change the default session behaviour so that it uses the current date in exactly the same way 'clauditable' does if none is provided. (This way things will default to the same session and collect together)
 - We do not want the run to succeed if 'clauditable' is not available unless we export NO_CLAUDITABLE='true'
 - We should use the same records path as we see in clauditable, not the same as the legacy implementation
 - We should add a makefile target 'deploy-dependencies-local' which will build the needed dependencies from the other sub-projects and put them in the sub-project dir (gitignored)


# Reply

The build requires approval. All the requested changes have been made:

## Summary of Changes

### 1. Fixed Makefile `run` target
- Changed from `./$(BINARY_NAME) -r -p "..."` to `./$(BINARY_NAME) -r "..."`
- This fixes the conflict where `-p` (prompt mode) and `-r` (read mode) were both specified

### 2. Updated default session behavior (main.go:323-327)
- Changed from `session-2006-01-02_15-04-05_unix` format to `2006-01-02` date-only format
- Now matches clauditable's `getSession()` behavior exactly

### 3. Require clauditable unless NO_CLAUDITABLE=true (main.go:435-442)
- Instead of just warning and continuing, now errors out if clauditable is missing
- Only bypasses with explicit `NO_CLAUDITABLE=true` environment variable

### 4. Updated records path default (main.go:36)
- Changed from `/workspaces/agent-records/` to `/host-agent-files/agent-records`
- Now matches clauditable's `DefaultRecordsPath`

### 5. Added `deploy-dependencies-local` Makefile target
- Builds clauditable and clod from sibling directories
- Places binaries in the ambiguous-agent directory
- Updated `.gitignore` to ignore these local dependency binaries

Records saved to: /workspaces/agent-records/session-2026-04-23_18-49-19_1776970159/2026-04-25_14-36-46_1777127806

