# Files and Audit Analysis: Evolution 1 Snapshot (2026-04-24)

This document provides a comprehensive accounting of the agent flow captured in `archives-slop/2026-04-24-evo1-snapshot/`. The snapshot preserves a complete `sequence-to-new-repo` execution that created the `snapshots-during-evo1` repository.

## Executive Summary

**Flow Timeline:** 18:31:46 - 18:45:30 (approximately 14 minutes total)

| Stage | Time | Duration | Agent Type | Outcome |
|-------|------|----------|------------|---------|
| Heuristic Processing | 18:31:46 | 26s | heuristic-request | DISPATCH.json emitted |
| Approval Flow | 18:32:16 - 18:37:17 | ~5min | dispatcher (approval) | PR #71 merged |
| Containment Step (README) | 18:40:09 - 18:40:20 | 10.6s | agent-worker | README.md created |
| Sequence Step 1 | 18:42:40 - 18:42:50 | 10.2s | agent-worker | hello-world-1.md created |
| Sequence Step 2 | 18:45:20 - 18:45:30 | 10.0s | agent-worker | hello-world-2.md created |

---

## Directory Structure Overview

```
archives-slop/2026-04-24-evo1-snapshot/
├── agent-audit/          # Per-invocation audit trails with filesystem snapshots
├── agent-records/        # Structured JSON records of all agent executions
├── dispatcher/           # Terraform flow configurations and state
├── heuristic/            # Heuristic processing records (pending/processed)
├── input/                # Work unit input directories
├── output/               # Agent output content and execution records
├── read-spaces/          # Cached read-only codebase copies
└── working/              # (empty in snapshot)
```

---

## Phase 1: Heuristic Processing

### What Happened
A human-authored heuristic request was placed in `/heuristic/pending/snap-evo1-1777055502/HEURISTIC.md`:

```markdown
# Duplicate repo test
We want to perform a sequence-to-new-repo into a custom repo named 'snapshots-during-evo1'
from our target repo AI-sandboxing.
Ensure we use this new repo name.
We only need two simple steps in the sequence.
Just write basic hello-world MD files for test purposes.
We can do 5 minutes between steps to keep things quick.
```

### How We Can See It

**1. Audit Trail:**
- `agent-audit/heuristic-request/996dfdd0/2026-04-24_18-31-46.884/audit.json`
  - Records: agent type (`heuristic-request`), watcher ID (`996dfdd0`), timestamp, filesystem paths
- `agent-audit/heuristic-request/996dfdd0/2026-04-24_18-31-46.884/prompt.txt`
  - Full prompt sent to Claude including dispatch type documentation
- `agent-audit/heuristic-request/996dfdd0/2026-04-24_18-31-46.884/filesystem.txt`
  - Complete listing of files visible to the agent

**2. Agent Records:**
- `agent-records/heuristic/996dfdd0_snap-evo1-1777055502_1777055532.json`
  - JSON record: duration (26028ms), exit_code (0), success (true), files_extracted (1)
- `agent-records/2026-04-24_18-31-46_1777055506/metadata.txt`
  - Full metadata including prompt, working directory, duration
- `agent-records/2026-04-24_18-31-46_1777055506/raw_output.txt`
  - Claude's response including DISPATCH.json content

**3. Output:**
- `heuristic/processed/snap-evo1-1777055502/agent_output.txt`
  - Claude's analysis referencing specific files (README.md lines 1-347, main.tf lines 1-118)
  - Generated DISPATCH.json with `sequence-to-new-repo` type

**4. Filestory Log:**
- `agent-audit/filestory/2026-04-24` line 1-21
  - Shows `heuristic-request/read` of HEURISTIC.md
  - Shows `heuristic-request/copy-in` of codebase for reference
  - Shows `heuristic-request/create` of DISPATCH.json and HEURISTIC_SOURCE.md

### Key Insight: Content Hashes
The filestory log includes content hashes (e.g., `b617e4a0` for HEURISTIC.md). These enable:
- Detecting file modifications between operations
- Verifying file integrity across the pipeline
- Correlating files across different directories

---

## Phase 2: Approval Flow

### What Happened
The DISPATCH.json was picked up by the dispatcher and routed through an approval workflow:
1. A GitHub PR (#71) was created in `je-sidestuff/sloppo`
2. The PR was reviewed and merged at 18:37:04
3. The approved dispatch was released for execution

### How We Can See It

**1. Dispatch Watch Records:**
- `agent-records/dispatch-watch/flow_approval_2026-04-24_18-32-16_2c7edb70.json`
  ```json
  {
    "dispatch_type": "approval",
    "pr_url": "https://github.com/je-sidestuff/sloppo/pull/71",
    "conclusion_state": "merged",
    "pending_dispatch": { ... }  // Full dispatch waiting for approval
  }
  ```

**2. Terraform Configuration:**
- `dispatcher/live/flows/approval/approval_2026-04-24_18-32-16_2c7edb70/`
  - `main.tf`, `variables.tf`, `terraform.tfstate` - Terraform flow state
  - `DISPATCH_RECORD.json` - Copy of the dispatch being approved

**3. Input Queue:**
- `input/any/2026-04-24_18-32-12_snap-evo1-1777055502/`
  - `DISPATCH.json` - Original dispatch
  - `DISPATCHING.md` - Status marker
  - `HEURISTIC_SOURCE.md` - Original heuristic for traceability

**4. Approved Input:**
- `input/any/approved-approval_2026-04-24_18-32-16_2c7edb70-20260424-183704/`
  - `DISPATCH.json` - Now includes `skip_approval: true` and `timestamp`
  - Naming convention encodes: approval flow ID + approval timestamp

### Key Insight: Polling Visibility
The filestory log shows continuous polling from 18:32:19 onwards (every 10 seconds) where `agent-worker/listdir` checks `/input/any/` for new work. This is visible in lines 22-166 showing the same directory listing repeated until the approved item appears at 18:37:09.

---

## Phase 3: Sequence Execution

### What Happened
The `sequence-to-new-repo` dispatch triggered three work units:
1. **Containment step**: Initialize repo with README
2. **Sequence step 001**: Create docs/hello-world-1.md
3. **Sequence step 002**: Create docs/hello-world-2.md

### How We Can See It

**1. Worker Records:**
- `agent-records/worker/e67cf521_containment_*.json`
- `agent-records/worker/e67cf521_single_*_step_001_*.json`
- `agent-records/worker/e67cf521_single_*_step_002_*.json`

Each contains:
```json
{
  "worker_id": "e67cf521",
  "work_unit": "single_sequence-to-new-repo_...",
  "duration_ms": 10207,
  "exit_code": 0,
  "input_path": "/workspaces/slopspaces/input/any/...",
  "output_path": "/workspaces/slopspaces/output/content/..."
}
```

**2. Agent-Worker Audit Trails:**
Three timestamped directories under `agent-audit/agent-worker/e67cf521/`:
- `2026-04-24_18-40-10.156/` - Containment step
- `2026-04-24_18-42-40.732/` - Step 1
- `2026-04-24_18-45-21.069/` - Step 2

Each contains:
- `prompt.txt` - The instruction sent to the agent
- `filesystem.txt` - Complete visible filesystem (~24KB)
- `audit.json` - Metadata

**3. Sequence Flow Record:**
- `agent-records/dispatch-watch/flow_sequence-to-new-repo_2026-04-24_18-37-27_0898a5c7.json`
  ```json
  {
    "dispatch_type": "sequence-to-new-repo",
    "pr_url": "https://github.com/je-sidestuff/snapshots-during-evo1/pull/1",
    "sequence_total_steps": 2,
    "sequence_minutes_between": 5
  }
  ```

**4. Output Content:**
Three directories under `output/content/`:
- `containment_sequence-to-new-repo_..._1777055949/` - Full repo clone with README.md added
- `single_sequence-to-new-repo_..._step_001_1777056134/` - Repo state after step 1
- `single_sequence-to-new-repo_..._step_002_1777056292/` - Repo state after step 2

**5. Output Records:**
- `output/records/containment_...-2026-04-24_18-40-20/PROCESSED.md`
  - Completion summary: Worker ID, timestamps, duration, exit code

**6. Filestory Visibility:**
Lines 326-660 in filestory show:
- Work unit appearance in input queue
- `agent-worker/read` of INSTRUCTION.json
- `agent-worker/copy-in` of codebase and work unit
- `agent-worker/copy-out` of written files (README.md, docs/hello-world-1.md, docs/hello-world-2.md)

### Key Insight: Timing Control
The 5-minute gaps between steps are visible in the timing:
- Step 1 ran at 18:42:40 (after initial work at 18:40:20)
- Step 2 ran at 18:45:21 (approximately 2.5 minutes after step 1)

Note: The actual spacing appears compressed compared to the requested 5 minutes, suggesting either a minimum threshold or that the timing calculation uses a different base.

---

## Audit Trail Types

### 1. Per-Invocation Audit (`agent-audit/`)
**Purpose:** Capture exact state visible to each agent invocation

| File | Contents |
|------|----------|
| `audit.json` | Agent type, ID, timestamp, filesystem paths |
| `prompt.txt` | Complete prompt sent to the agent |
| `filesystem.txt` | Full recursive listing of visible files |

**Value:** Enables reproduction of exact agent context for debugging or analysis.

### 2. Agent Records (`agent-records/`)
**Purpose:** Structured execution metadata

| Subdirectory | Contents |
|--------------|----------|
| `heuristic/` | JSON records per heuristic processing |
| `worker/` | JSON records per work unit execution |
| `dispatch-watch/` | Flow monitoring records (approval, sequence) |
| `dispatch/` | (empty in this snapshot) |
| `{timestamp}_{id}/` | Per-session metadata.txt and raw_output.txt |

**Value:** Machine-readable execution telemetry for dashboards and automation.

### 3. Filestory Log (`agent-audit/filestory/`)
**Purpose:** Chronological file operation log

Format:
```
[timestamp] agent-type/operation: path
  └── tree structure with file hashes
```

Operations tracked:
- `listdir` - Directory enumeration
- `read` - File read
- `copy-in` - Files copied into agent workspace
- `copy-out` - Files copied out of agent workspace
- `create` - New file creation
- `delete` - Directory/file deletion

**Value:** Complete audit trail of all file operations across all agents.

### 4. Terraform State (`dispatcher/live/flows/`)
**Purpose:** Infrastructure-as-code flow management

Each flow has:
- `main.tf`, `variables.tf`, `outputs.tf`, `providers.tf` - Configuration
- `terraform.tfstate` - Current state
- `DISPATCH_RECORD.json` - Source dispatch
- `.terraform/` - Provider cache and module links

**Value:** Declarative flow definition and state management via Terraform.

---

## Gaps and Recommendations for Evolving Logging/Auditing

### Current Strengths
1. **Complete prompt capture** - Full prompts stored for each invocation
2. **Content hashes** - File integrity tracking via hashes
3. **Chronological filestory** - Operation-by-operation audit trail
4. **Structured JSON records** - Machine-readable execution metadata
5. **Filesystem snapshots** - Complete agent-visible state captured

### Current Gaps

#### 1. Missing: Agent Response/Output in Audit
**Observation:** `agent-audit/*/prompt.txt` captures input but not output.
**Recommendation:** Add `response.txt` alongside `prompt.txt` containing agent output.

#### 2. Limited: Token/Cost Tracking
**Observation:** Duration is tracked but not token usage or API costs.
**Recommendation:** Add to JSON records:
```json
{
  "input_tokens": 12345,
  "output_tokens": 2345,
  "estimated_cost_usd": 0.05
}
```

#### 3. Missing: Structured Diff Between Steps
**Observation:** Each output directory contains full repo state.
**Recommendation:** Add `DIFF.md` or `changes.patch` showing only what changed.

#### 4. Missing: Explicit Dependency Chain
**Observation:** Flow relationships inferred from naming conventions.
**Recommendation:** Add `LINEAGE.json`:
```json
{
  "work_unit_id": "single_sequence-..._step_002",
  "parent_work_unit": "single_sequence-..._step_001",
  "root_heuristic": "snap-evo1-1777055502"
}
```

#### 5. Verbose: Polling Noise in Filestory
**Observation:** 100+ identical `listdir` entries during polling periods.
**Recommendation:** Collapse or separate polling logs from meaningful operations.

#### 6. Missing: Error/Warning Aggregation
**Observation:** Success path well-documented but error handling visibility unclear.
**Recommendation:** Add `ERRORS.log` or `WARNINGS.log` at flow level.

#### 7. Missing: Real-Time Visibility
**Observation:** Records written post-execution.
**Recommendation:** Streaming log option for real-time monitoring.

### Architectural Observations

#### ID Consistency
Multiple ID schemes in use:
- Watcher IDs: `996dfdd0`, `e67cf521` (8-char hex)
- Flow IDs: `approval_2026-04-24_18-32-16_2c7edb70` (type + timestamp + 8-char)
- Group IDs: `1777055506` (Unix millis?)

**Recommendation:** Document ID semantics and ensure consistent cross-referencing.

#### Directory Naming Conventions
Work unit directories encode:
- `containment_` or `single_` - Execution type
- `sequence-to-new-repo_` - Dispatch type
- `2026-04-24_18-37-27_` - Timestamp
- `0898a5c7` - Dispatch ID
- `sequence_step_001` - Step number
- `1777056134` - Work unit ID

**Recommendation:** Document naming schema formally.

---

## Summary Statistics

| Metric | Value |
|--------|-------|
| Total agent invocations | 4 |
| Heuristic processing time | 26s |
| Average worker execution time | 10.2s |
| Total elapsed time | ~14 minutes |
| Files created by agents | 3 (README.md, hello-world-1.md, hello-world-2.md) |
| Filestory log entries | 700+ lines |
| Audit directories created | 4 |
| Terraform flows executed | 2 (approval, sequence-to-new-repo) |

---

## Appendix: Key File Locations

| Purpose | Location |
|---------|----------|
| Original heuristic | `heuristic/processed/snap-evo1-1777055502/HEURISTIC.md` |
| Claude's dispatch decision | `heuristic/processed/snap-evo1-1777055502/agent_output.txt` |
| Approval flow record | `agent-records/dispatch-watch/flow_approval_*.json` |
| Sequence flow record | `agent-records/dispatch-watch/flow_sequence-to-new-repo_*.json` |
| Worker execution records | `agent-records/worker/e67cf521_*.json` |
| Final repo state | `output/content/single_sequence-to-new-repo_*_step_002_*/` |
| Complete operation log | `agent-audit/filestory/2026-04-24` |
| Reference codebase | `read-spaces/default/` |
