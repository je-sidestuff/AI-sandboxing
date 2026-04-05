# File Streamlining for Heuristic Flow

This document analyzes the excessive file proliferation observed during a single heuristic-request → agent-worker interaction, and proposes streamlining measures.

---

## The Problem: File Explosion

A single test interaction (`test-bugs-1775347935`) produced the following file structure:

### Files Created During Heuristic-Request Phase

```
slopspaces/heuristic/processed/test-bugs-1775347935/
├── HEURISTIC.json       # Input configuration
├── PROCESSING.md        # Status marker: processing started
├── agent_output.txt     # Raw agent output
└── PROCESSED.md         # Status marker: processing complete

slopspaces/agent-records/heuristic/
└── 807bddc1_test-bugs-1775347935_1775347963.json    # Agent record

slopspaces/agent-records/2026-04-05_00-12-25_1775347945/
├── raw_output.txt       # Duplicate of agent output
└── metadata.txt         # Agent metadata
```

### Files Created During Agent-Worker Phase

```
slopspaces/output/content/2026-04-05_00-12-43_test-bugs-1775347935/
├── HEURISTIC_SOURCE.md              # Origin tracking
└── PROCESSED-2026-04-05_00-24-49.md # Completion marker

slopspaces/output/records/2026-04-05_00-12-43_test-bugs-1775347935-2026-04-05_00-24-49/
├── INSTRUCTION.json     # Work unit instruction
├── PROCESSING.md        # Status marker: processing started
└── PROCESSED.md         # Status marker: processing complete

slopspaces/agent-records/2026-04-05_00-24-43_1775348683/
├── raw_output.txt       # Agent output
└── metadata.txt         # Agent metadata

slopspaces/agent-records/worker/
└── 53b5671e_2026-04-05_00-12-43_test-bugs-1775347935_1775348689.json    # Agent record
```

### Total Count

For **one interaction**:
- **~15 files** created across **~10 directories**
- Multiple levels of nesting
- Several redundant/duplicate files

---

## Why This Is a Problem

### 1. Cognitive Overhead

The directory structure is difficult to navigate and understand. Finding "what happened" requires traversing multiple locations.

### 2. Redundant Information

- `PROCESSING.md` and `PROCESSED.md` appear in multiple places with nearly identical content
- `raw_output.txt` in timestamped directories duplicates content elsewhere
- Agent records appear in both timestamped directories AND role-specific directories

### 3. Naming Inconsistency

- Some directories use timestamps: `2026-04-05_00-12-25_1775347945`
- Some use heuristic IDs: `test-bugs-1775347935`
- Some combine both: `2026-04-05_00-12-43_test-bugs-1775347935-2026-04-05_00-24-49`

### 4. Unclear Hierarchy

It's not obvious which directories are:
- Primary records vs. auxiliary
- Required vs. optional
- For human consumption vs. machine processing

---

## Proposed Streamlining

### Principle 1: Single Source of Truth

Each piece of information should exist in exactly one canonical location. Other references should be symbolic links or database references.

### Principle 2: Flat Over Nested

Prefer fewer directory levels. Deep nesting increases cognitive load and makes glob patterns harder.

### Principle 3: Separate Concerns

- **Records** (for debugging/auditing) should be separate from **state** (for workflow coordination)
- Workflow state files should be minimal: status, timestamps, references

### Proposed Structure

```
slopspaces/
├── work/
│   └── {flow-id}/                          # One directory per workflow
│       ├── STATE.json                      # Single state file: status, timestamps, references
│       ├── input/                          # Input files (immutable after creation)
│       │   └── heuristic.json
│       └── output/                         # Output files (written by worker)
│           └── {whatever the work produces}
│
└── records/
    └── {flow-id}/                          # Records directory mirrors work/
        ├── heuristic-agent.log             # Agent output for heuristic phase
        ├── worker-agent.log                # Agent output for worker phase
        └── metadata.json                   # Combined metadata for all phases
```

### Key Changes

1. **Eliminate separate `processed/` and `pending/` directories**
   - Use `STATE.json` status field instead
   - Single directory per workflow, state tracked in file

2. **Consolidate agent records**
   - One records directory per flow, not per-agent-type
   - Named logs instead of timestamped directories

3. **Eliminate redundant status markers**
   - No more `PROCESSING.md` / `PROCESSED.md` pairs
   - Single `STATE.json` with status field and timestamps

4. **Consistent naming**
   - Use flow-id everywhere
   - Flow-id assigned at heuristic creation time
   - Format: `{date}_{heuristic-id}` (e.g., `2026-04-05_test-bugs`)

### STATE.json Example

```json
{
  "flowId": "2026-04-05_test-bugs-1775347935",
  "status": "completed",
  "phases": {
    "heuristic": {
      "started": "2026-04-05T00:12:25Z",
      "completed": "2026-04-05T00:12:43Z",
      "agentId": "807bddc1"
    },
    "worker": {
      "started": "2026-04-05T00:24:43Z",
      "completed": "2026-04-05T00:24:49Z",
      "agentId": "53b5671e",
      "exitCode": 0
    }
  }
}
```

---

## Migration Path

### Phase 1: Records Consolidation

- Modify agent-records output to use per-flow directories
- Eliminate role-specific subdirectories (`worker/`, `heuristic/`)
- Keep timestamped subdirectories temporarily for backward compatibility

### Phase 2: State File Introduction

- Add `STATE.json` to track workflow status
- Stop creating `PROCESSING.md` / `PROCESSED.md` files
- Update watchers to check `STATE.json` instead of marker files

### Phase 3: Directory Consolidation

- Merge `input/any/`, `heuristic/pending/`, `heuristic/processed/` into unified `work/` structure
- Update all Go programs to use new paths

---

## Compatibility Notes

The current structure was designed for flexibility and debugging during development. The proposed structure optimizes for:
- Operational clarity
- Reduced file count
- Single source of truth

This trade-off is appropriate as the system matures from experimentation to regular use.

---

## Questions for Further Discussion

1. Should we keep the timestamped agent-records directories for debugging, or are consolidated logs sufficient?
2. Should `STATE.json` be the only state tracking, or keep filesystem-based state (directory existence) as a backup?
3. How to handle multi-worker scenarios where one heuristic spawns multiple parallel workers?
