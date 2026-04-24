# Prompt

The next sub-project to build is 'clauditable'. This is the sub-project that will replace parts of agent-recorder and agent-events in 'sandbox/AI-sandboxing'. The project will run with a paradigm similar to the linux 'time' binary. (So we would call 'clauditable claude -p <prompt>' or 'clauditable clod -p <prompt>)

The clauditable sub-project is also a go binary.

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile)

The concept of records and reports will be used throughout the projects to refer to directly collected signals as records and significantly processed/composed/adjusted records as reports.

The purpose of the overall record keeping utility is to wrap arbitrary calls - often an agent, but not always, and to organize the records of these calls in a way which is effective for agents and humans to use together. The system, in the longer term, will organize, refine, and augment the collected records.

The 'clauditable' binary will use input from environment variables to drive some known locations like the 'agent records path' in AGENT_RECORDS_PATH, defaulting to '/host-agent-files/agent-records', where records will be sent automatically. The AGENT_SESSION will define the session that the logs will be added to - the current date in the form 2026-12-13 will be used if none is provided, and will automatically update as the day ticks. The AGENT_CONSOLIDATE_RECORDS will be true by default.

For its first functionality 'clauditable' will:
 - Wrap call passed after its binary name, without interfering at all (similar to 'time'), and call the passed call while teeing both input and error stream, and relaying input stream.
 - Record the command
 - Record the length of time the command took
 - Add an event marker and the raw record of the interaction in a session log. This will use a concise simple event schema inspired by the one in 'sandbox/AI-sandboxing'.
 - The raw record will go to <records-path>/<session>/<unix-timestamp>
 - If AGENT_CONSOLIDATE_RECORDS is true the recorder will 'collect' any '<records-path>/<session>/<unix-timestamp>' format logs when the record it writes is complete, and will append the data from them to '<records-path>/<session>/session.log' before deleting them.
 - The tests should be in go and we want only a very basic couple to begin.

In general this is a spiritual successor to the records functionality found in 'sandbox/AI-sandboxing', so feel free to use patterns there to eliminate ambiguities that arise.


# Reply

I've created the `clauditable` project. Here's a summary of what was built:

## Project Structure

```
/workspaces/workspace/research/AI-evo1/clauditable/
├── .dockerignore
├── .gitignore
├── Dockerfile
├── Makefile
├── go.mod
├── main.go
└── main_test.go
```

Plus the CI workflow at:
```
/workspaces/workspace/research/AI-evo1/.github/workflows/clauditable-test.yml
```

## Features Implemented

**main.go** - The core binary that:
- Wraps any command passed after its name (like `time`)
- Tees both stdout and stderr while relaying to the terminal
- Connects stdin passthrough to the wrapped command
- Records: command, duration, exit code, stdout, stderr
- Uses a concise event schema with `Event` and `RawRecord` types
- Writes raw records to `<AGENT_RECORDS_PATH>/<session>/<unix-timestamp>`
- Consolidates timestamp files into `session.log` (JSONL format) when `AGENT_CONSOLIDATE_RECORDS` is true

**Environment Variables:**
- `AGENT_RECORDS_PATH` - defaults to `/host-agent-files/agent-records`
- `AGENT_SESSION` - defaults to current date (YYYY-MM-DD), auto-updates daily
- `AGENT_CONSOLIDATE_RECORDS` - defaults to `true`

**Bootstrapping Files:**
- `Makefile` with `build`, `docker-build`, `run`, `test`, `clean`, `all` targets
- `Dockerfile` using Go 1.25
- `.gitignore` and `.dockerignore` for binaries and test files
- `clauditable-test.yml` CI workflow triggering on `test/clauditable` branch

**Tests (main_test.go):**
- `TestGetSession` - verifies session resolution from env or date
- `TestGetConsolidateRecords` - tests boolean parsing with defaults
- `TestIsUnixTimestamp` - validates timestamp file detection
- `TestWriteRawRecord` - tests record file creation
- `TestConsolidateRecords` - tests log consolidation and cleanup

To build and test, run:
```bash
make -C /workspaces/workspace/research/AI-evo1/clauditable build
make -C /workspaces/workspace/research/AI-evo1/clauditable test
```

Records saved to: /workspaces/agent-records/session-2026-04-23_18-49-19_1776970159/2026-04-24_11-58-11_1777031891

