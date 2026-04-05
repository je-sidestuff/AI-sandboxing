# heuristic-request

Watches for heuristic inputs and processes them via prompt-only agents to generate dispatch requests.

## File Overview

### Input Files (User Creates)

| File | Purpose |
|------|---------|
| `pending/<folder>/HEURISTIC.json` | Heuristic input with message and optional model specification |
| `pending/<folder>/HEURISTIC.md` | Alternative: plain markdown heuristic input (fallback if no .json) |

### Processing Files (Watcher Creates)

| File | Purpose |
|------|---------|
| `pending/<folder>/PROCESSING.md` | Lock file indicating a watcher is processing this unit |
| `pending/<folder>/.prompt.tmp` | Temporary file containing the agent prompt (deleted after use) |
| `pending/<folder>/agent_output.txt` | Captured output from the agent invocation |

### Completion Files (Watcher Creates)

| File | Purpose |
|------|---------|
| `processed/<folder>/PROCESSED.md` | Summary of successful processing (timestamps, duration, request ID) |
| `processed/<folder>/FAILED.md` | Created on failure with error details |

### Output Files (Agent Creates → Watcher Extracts)

The agent analyzes the heuristic and produces one of these files in its output:

| File | Purpose |
|------|---------|
| `DISPATCH.json` | Dispatch instruction for agent-dispatch (repo-isolation, in-repo, direct, sequence-to-new-repo) |
| `INSTRUCTION.json` | Simple instruction that doesn't need dispatch orchestration |

These are extracted from agent output and placed in the request directory for agent-dispatch pickup.

### Request Directory Output

| File | Purpose |
|------|---------|
| `<request-id>/DISPATCH.json` or `INSTRUCTION.json` | Extracted dispatch/instruction from agent |
| `<request-id>/HEURISTIC_SOURCE.md` | Copy of original heuristic for reference |

### Records

| File | Purpose |
|------|---------|
| `agent-records/heuristic/<watcher>_<id>_<timestamp>.json` | Processing record with timing and outcome |

## Agent Worker Expected Files

When the dispatched agent-worker processes a request, it typically creates:

| File | Purpose |
|------|---------|
| `DISPATCH.md` or `DISPATCH.json` | The dispatch specification (created by heuristic processor) |
| `APPROVED.md` | Created when dispatch is approved for execution |
| `EXECUTING.md` | Created when execution begins |
| `COMPLETED.md` | Created on successful completion |
| `FAILED.md` | Created on execution failure |
| `PR_URL.md` | Contains PR URL for repo-isolation/in-repo dispatches |

For `sequence-to-new-repo` dispatches, additional files appear:

| File | Purpose |
|------|---------|
| `SEQUENCE_PROGRESS.md` | Tracks which steps have been executed |
| `step_<n>_output.txt` | Output from each sequence step |

## Usage

```bash
# Watch mode (default)
./heuristic-request --watch

# Process once and exit
./heuristic-request --once
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `HEURISTIC_DIR` | `/workspaces/slopspaces/heuristic` | Directory containing pending/processed folders |
| `REQUEST_DIR` | `/workspaces/slopspaces/input/any` | Output directory for agent-dispatch |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Records directory |
| `AGENT_PRESET` | `claude` | Default agent (copilot, gemini, claude, opencode, codex) |
