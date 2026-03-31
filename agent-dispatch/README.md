# agent-dispatch

A unified dispatcher for agent work units. Combines continuous watch mode for processing DISPATCH files with single-shot dispatch operations.

## Modes of Operation

### Watch Mode (Default)

By default, `agent-dispatch` runs as a continuous watcher, monitoring the input directory for DISPATCH.json/md files and processing them:

```bash
agent-dispatch
```

This mode:
- Monitors `INPUT_DIR/any/` for DISPATCH.json or DISPATCH.md files
- Processes dispatch units based on their type (direct, in-repo, or repo-isolation)
- Transforms direct dispatches into INSTRUCTION.json for agent-worker pickup
- Manages terraform lifecycle for in-repo and repo-isolation dispatches
- Uses exponential backoff logging to reduce noise during idle periods

### Single-Shot Mode (--once)

Use the `--once` flag (or simply provide dispatch flags like `-i` or `-r`) for one-time dispatch operations:

```bash
# Dispatch an instruction
agent-dispatch --once -i "Run the test suite" -m execute

# Dispatch a report
agent-dispatch --once -r daily

# Async dispatch (returns immediately)
agent-dispatch --once -i "Build the project" --async

# Check status of a dispatched work unit
agent-dispatch --once --check <work-unit-id>

# Wait for a work unit to complete
agent-dispatch --once --wait <work-unit-id>
```

## Usage

```
agent-dispatch: Watch for and process dispatch work units

By default, runs in watch mode, continuously monitoring for DISPATCH.json/md files.
Use --once for single-shot dispatch operations.

WATCH MODE (default):
  agent-dispatch

SINGLE-SHOT MODE (--once):
  agent-dispatch --once -i "instruction" [-m mode] [-a agent] [-t timeout] [--async]
  agent-dispatch --once -r type [-c content] [-a agent] [-t timeout] [--async]
  agent-dispatch --once --check <work-unit-id>
  agent-dispatch --once --wait <work-unit-id> [-t timeout]

Flags:
  -a string        Agent to use (optional, requires --once)
  -async           Dispatch asynchronously without waiting (requires --once)
  -c string        Content for custom reports (requires --once)
  -check string    Check status of a work unit by ID (requires --once)
  -i string        Instruction to dispatch (requires --once)
  -m string        Mode for instructions: prompt or execute (default "prompt")
  -once            Single-shot mode: dispatch one work unit and exit
  -r string        Report type: custom, daily, weekly, monthly (requires --once)
  -t duration      Timeout for dispatch operation (default 30m0s)
  -wait string     Wait for a work unit to complete by ID (requires --once)
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/workspaces/slopspaces/input/` | Input directory for work units |
| `OUTPUT_DIR` | `/workspaces/slopspaces/output/` | Output directory for completed work |
| `RECORDS_DIR` | `/workspaces/slopspaces/agent-records/` | Records directory for dispatch logs |
| `DISPATCHER_LIVE` | `/workspaces/slopspaces/dispatcher/live` | Directory for terraform configs |
| `GITHUB_PAT` | - | GitHub Personal Access Token (required for in-repo/repo-isolation dispatch) |
| `GH_TOKEN` | - | Alternative to GITHUB_PAT |
| `TERRAFORM_BINARY` | `terraform` | Path to terraform binary |

## Watch Mode Dispatch Types

### Direct Dispatch

Creates work units that are picked up by agent-worker directly. Fire-and-forget style.

**DISPATCH.json:**
```json
{
  "type": "direct",
  "instruction": "Run the test suite and report results",
  "mode": "execute",
  "agent": "test-runner"
}
```

**DISPATCH.md** (auto-converts to direct):
```markdown
Run the test suite and report results.
Make sure all tests pass before continuing.
```

### In-Repo Dispatch

Creates a PR-based containment workflow using terraform. The dispatcher manages the full terraform lifecycle. Work is done on a branch in the target repository.

**DISPATCH.json:**
```json
{
  "type": "in-repo",
  "instruction": "Implement the new feature",
  "target_repo": "owner/repo",
  "pr_title": "Add new feature",
  "pr_body": "This PR adds..."
}
```

Requires `GITHUB_PAT` or `GH_TOKEN` environment variable.

### Repo-Isolation Dispatch

Creates a completely separate private isolation repository for maximum containment. The AI works in a cloned copy of the target repo, isolated from the original. This provides stronger isolation guarantees than in-repo dispatch.

**DISPATCH.json:**
```json
{
  "type": "repo-isolation",
  "instruction": "Implement the new feature",
  "target_repo": "owner/repo",
  "isolation_name": "my-isolation-repo"
}
```

If `isolation_name` is not specified, a unique name is auto-generated.

Requires `GITHUB_PAT` or `GH_TOKEN` environment variable.

### Sequence-to-New-Repo Dispatch

Creates a brand new repository and executes a sequence of commands over time. Each command activates after a configurable interval (default: 20 minutes). The dispatcher's watch loop performs `terraform apply` periodically, activating steps as time progresses.

This flow is useful for:
- Multi-step tasks that need human review time between steps
- Long-running workflows with natural breakpoints
- Tasks requiring incremental commits and feedback

**DISPATCH.json:**
```json
{
  "type": "sequence-to-new-repo",
  "instruction": "Initial setup - create the project scaffold",
  "sequence_commands": [
    "Write chapter 1 of the documentation in docs/CHAPTER1.md",
    "Write chapter 2 of the documentation in docs/CHAPTER2.md",
    "Write chapter 3 of the documentation in docs/CHAPTER3.md"
  ],
  "sequence_minutes_between": 20
}
```

**Fields:**
| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `type` | Yes | - | Must be `"sequence-to-new-repo"` |
| `instruction` | Yes | - | Initial instruction for the containment PR |
| `sequence_commands` | Yes | - | Array of instructions to execute in sequence |
| `sequence_minutes_between` | No | 20 | Minutes between each step |
| `mode` | No | "execute" | Mode for all instructions ("execute" or "prompt") |
| `skip_approval` | No | false | Skip the approval gate (use with caution) |

**How it works:**
1. Dispatcher creates a new repository named `seq-<timestamp>-<id>`
2. Initial terraform apply creates the repo, PR #1, and captures the start time
3. The watch loop runs terraform apply every minute (periodic poll)
4. Steps activate based on elapsed time since first apply:
   - Step 1: immediately after repo/PR creation
   - Step 2: after `minutes_between` minutes
   - Step 3: after `2 * minutes_between` minutes
   - etc.
5. Flow completes when all steps have executed and PR is merged/closed

**Monitoring progress:**
The dispatcher logs sequence progress during polling:
```
[abc12345] Flow sequence-to-new-repo_2024-03-21_12-30-00_xyz Flow sequence progress: 2/3 steps ready
[abc12345] Flow sequence-to-new-repo_2024-03-21_12-30-00_xyz next step in ~8 minutes
```

Requires `GITHUB_PAT` or `GH_TOKEN` environment variable.

## Data Flow

### Watch Mode Flow

```
INPUT_DIR/any/<dispatch-id>/
  └── DISPATCH.json (or DISPATCH.md)
        │
        ▼ (agent-dispatch watch mode)
        │
        ├─[direct]──────────► Transform to INSTRUCTION.json
        │                     └── Stays in place for agent-worker pickup
        │
        ├─[in-repo]─────────► Create terraform config (branch in target repo)
        │                     └── Run terraform init/apply
        │                     └── Move to OUTPUT_DIR with DISPATCH_PROCESSED.md
        │
        ├─[repo-isolation]──► Create terraform config (separate isolation repo)
        │                     └── Run terraform init/apply
        │                     └── Move to OUTPUT_DIR with DISPATCH_PROCESSED.md
        │
        └─[sequence-to-new-repo]──► Create new repo + terraform config
                              └── Run terraform init/apply (creates repo + PR)
                              └── Watch loop runs apply every ~1 min
                              └── Steps activate based on elapsed time
                              └── Cleanup after all steps complete + PR merged/closed
```

### Single-Shot Mode Flow

```
agent-dispatch --once -i "instruction"
        │
        ▼
Creates INPUT_DIR/any/<work-unit-id>/INSTRUCTION.json
        │
        ▼
Polls OUTPUT_DIR/content/<work-unit-id>/ for PROCESSED-*.md
        │
        ▼
Returns result with exit code and output files
```

## Records

Dispatch operations are recorded for tracking and debugging:

- **Single-shot records:** `RECORDS_DIR/dispatch/`
- **Watch mode records:** `RECORDS_DIR/dispatch-watch/`
- **Flow records (in-repo/repo-isolation):** `RECORDS_DIR/dispatch-watch/flow_*.json`

## Building

```bash
cd agent-dispatch
go build -o agent-dispatch .
```

## Examples

### Run as a daemon (watch mode)

```bash
# Start watching for dispatch files
agent-dispatch

# Output:
# [abc12345] Dispatch watcher started
# [abc12345] Watching: /workspaces/slopspaces/input/any
# [abc12345] Output: /workspaces/slopspaces/output/
# [abc12345] GitHub PAT: configured (in-repo dispatch enabled)
```

### Dispatch an instruction synchronously

```bash
agent-dispatch -i "Run database migrations" -m execute -t 10m

# Output:
# === Dispatch Result ===
# Work Unit ID: dispatch-inst_2024-03-21_12-30-00_abc12345
# Output Path:  /workspaces/slopspaces/output/content/dispatch-inst_2024-03-21_12-30-00_abc12345
# Success:      true
# Exit Code:    0
# Duration:     45.123s
```

### Dispatch asynchronously and check later

```bash
# Dispatch
agent-dispatch -i "Run full test suite" -m execute --async
# Output: Dispatched work unit: dispatch-inst_2024-03-21_12-30-00_abc12345

# Check status later
agent-dispatch --check dispatch-inst_2024-03-21_12-30-00_abc12345

# Or wait for completion
agent-dispatch --wait dispatch-inst_2024-03-21_12-30-00_abc12345 -t 1h
```

### Generate a daily report

```bash
agent-dispatch -r daily

# Output:
# === Dispatch Result ===
# Work Unit ID: dispatch-report_2024-03-21_12-30-00_abc12345
# Success:      true
```

## Testing the Sequence-to-New-Repo Flow

### Prerequisites

1. Ensure `GITHUB_PAT` or `GH_TOKEN` is set with repo creation permissions
2. Ensure `GITHUB_OWNER` is set (defaults to `je-sidestuff`)
3. Build agent-dispatch: `cd agent-dispatch && go build -o agent-dispatch .`
4. Start the dispatcher: `./agent-dispatch` (runs in watch mode)

### Quick Test (2-minute intervals)

For testing purposes, use a short interval. Create a test dispatch:

```bash
# Create the dispatch directory
mkdir -p /workspaces/slopspaces/input/any/test-sequence-$(date +%s)

# Write the DISPATCH.json
cat > /workspaces/slopspaces/input/any/test-sequence-$(date +%s)/DISPATCH.json << 'EOF'
{
  "type": "sequence-to-new-repo",
  "instruction": "Create a simple test project scaffold with a README.md file",
  "sequence_commands": [
    "Add a file docs/step1.md with content about step 1",
    "Add a file docs/step2.md with content about step 2",
    "Add a file docs/step3.md with content about step 3"
  ],
  "sequence_minutes_between": 2,
  "skip_approval": true
}
EOF
```

### What to Expect

1. **Initial dispatch** (within 10 seconds):
   - Dispatcher picks up the DISPATCH.json
   - Creates a new repo: `seq-<timestamp>-<id>`
   - Creates PR #1 with the initial instruction
   - Captures start time for sequencing

2. **Step activation** (every 2 minutes):
   - Watch loop polls every minute
   - Step 1 activates immediately after repo creation
   - Step 2 activates after 2 minutes
   - Step 3 activates after 4 minutes

3. **Monitoring logs**:
   ```
   [abc123] Processing sequence-to-new-repo dispatch: test-sequence-...
   [abc123] Running terraform init in .../flows/sequence-to-new-repo/...
   [abc123] Running terraform apply in ... (initial apply)
   [abc123] Captured sequence start time: 1711032600000 ms
   [abc123] Sequence-to-new-repo dispatch created, PR URL: https://github.com/owner/seq-.../pull/1
   [abc123] Target repo: owner/seq-... (3 steps, 2 min intervals)
   ...
   [abc123] Flow sequence-to-new-repo_... sequence progress: 1/3 steps ready
   [abc123] Flow sequence-to-new-repo_... next step in ~2 minutes
   ```

4. **Cleanup**:
   - After all steps complete AND the PR is merged/closed, the flow is cleaned up
   - Flow record is updated with `status: completed`

### Verifying State

Check active flows:
```bash
# List flow records
ls -la /workspaces/slopspaces/agent-records/dispatch-watch/

# View a specific flow record
cat /workspaces/slopspaces/agent-records/dispatch-watch/flow_sequence-to-new-repo_*.json | jq .

# Check terraform state
cd /workspaces/slopspaces/dispatcher/live/flows/sequence-to-new-repo/<flow-id>
terraform output
```

### Manual Step Verification

Force an immediate terraform apply to check step status:
```bash
cd /workspaces/slopspaces/dispatcher/live/flows/sequence-to-new-repo/<flow-id>
terraform apply -auto-approve
terraform output step_readiness
terraform output sequence_complete
```
