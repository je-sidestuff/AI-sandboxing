# Prompt

We will create the 'research/AI-evo1/federation-command' sub-project next.

The 'federation-command' sub-project is a go binary that is the spiritual successor to 'sandbox/AI-sandboxing/ambiguous-agent/ambiguous-shell'. It is responsible for covering the interactive-shell-like-CLI functionality (the old 'sandbox/AI-sandboxing/ambiguous-agent/' go functionality).

Federation command is a CLI with the same appearance as the previous ambiguous-shell and roughly the same functionality. It now wraps all commands with 'clauditable' - setting the agent to 'none' when the human/keyboard driver is using non-agentic commands. We will add a functionality to clauditable where we prevent double-wrapping by setting an environment variable 'IS_CLAUDITABLE' within the scope of clauditable and detecting it on a subsequent invocation so we can prevent double-logging.

For this first increment we will replicate the visual appearance, the agent-selection, and the 'agent' invocation command (now with -p/r/w/x instead of -p/e). We will also add a NOT_YET_IMPLEMENTED.md describing the functions not yet brought over from legacy.

Make sure the visual style is consistent with 'research/AI-evo1/ambiguous-agent'.

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile) In this case we need to consider that our Docker file must include 'clauditable' as it is a runtime dependency. We'll want a similar Makefile capability to deploy dependencies locally, like we have in ambiguous-agent.

For our test suite we will simply support a test of a 'version' entrypoint for now, not the interative mode. In the reply please discuss possible options for testing the interactive mode.


## Reply

I've created the `research/AI-evo1/federation-command` sub-project with all the requested components:

### Files Created

1. **`main.go`** (~900 lines) - The interactive shell with:
   - Same visual appearance as the legacy shell (lipgloss styling, agent colors)
   - Agent selection (`set-agent`, `list-agents`)
   - Model selection (`set-model`, `list-models`, `clear-model`)
   - Mode-based agent invocation: `-p` (prompt), `-r` (read), `-w` (write), `-x` (execute)
   - All commands wrapped with `clauditable`, setting agent to "none" for non-agentic commands
   - `IS_CLAUDITABLE` environment variable for double-wrap prevention
   - Multi-line input (backslash continuation, heredoc, unclosed quotes)
   - Tab completion for file paths
   - `cd` and `export` built-in commands
   - `--version` / `-v` flag for non-interactive version check

2. **`main_test.go`** - Tests for:
   - Version flag verification
   - Agent validation
   - Variable name validation
   - Path abbreviation
   - Argument parsing with quotes
   - Multi-line continuation detection
   - Mode descriptions

3. **`go.mod`** - Go module file with dependencies

4. **`Makefile`** - Standard targets:
   - `build`, `docker-build`, `run`, `clean`, `test`, `all`
   - `deploy-dependencies-local` for building sibling dependencies

5. **`Dockerfile`** - Multi-stage build including clauditable as runtime dependency

6. **`.gitignore`** - Ignoring binaries and local dependencies

7. **`.github/workflows/federation-command-test.yml`** - CI workflow

8. **`NOT_YET_IMPLEMENTED.md`** - Documents:
   - Pending features from legacy shell
   - Interactive mode testing options
   - Environment variables
   - Migration notes

### Changes to `clauditable`

Added double-wrap prevention in `/workspaces/workspace/research/AI-evo1/clauditable/main.go`:
- New constant `EnvIsClauditable = "IS_CLAUDITABLE"`
- Early exit passthrough when `IS_CLAUDITABLE=true` is set
- Sets `IS_CLAUDITABLE=true` for child processes

### Interactive Mode Testing Discussion

The tests focus on non-interactive functionality (version flag, helper functions). For testing interactive mode, options include:

1. **PTY-based testing** - Most accurate but platform-dependent
2. **Expect-style testing** - Use goexpect library
3. **Interface refactoring** - Accept io.Reader for testability
4. **Docker integration tests** - End-to-end but slow
5. **Snapshot testing** - Record and replay terminal sessions

The recommended approach is to refactor the readline handling to accept interfaces (option 3) for unit tests, combined with occasional PTY-based integration tests.

Records saved to: /workspaces/agent-records/session-2026-04-23_18-49-19_1776970159/2026-04-25_15-00-50_1777129250
