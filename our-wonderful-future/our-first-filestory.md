# Our First Filestory: The First Error-Free Read-and-Write Spaces Execution

**Date**: April 6, 2026
**Test**: `2026-04-06-test1.md`
**Duration**: ~37 minutes (15:18:16 - 15:55:41)

---

## Executive Summary

This filestory documents the first successful full execution of the agent orchestration system with read-and-write spaces fully integrated. Two complete heuristic-to-sequence flows executed without errors, demonstrating the viability of our file-based agent isolation model.

---

## What is a Filestory?

A filestory is a timestamped log of all file operations performed across the entire agent ecosystem during an execution session. Each entry records:
- **Timestamp**: When the operation occurred
- **Actor**: Which component performed the operation (e.g., `agent-worker`, `heuristic-request`, `agent-dispatch`)
- **Operation**: The type of file operation (`listdir`, `read`, `copy-in`, `copy-out`, `create`, `delete`)
- **Path**: The target filesystem location
- **Content hashes**: Abbreviated SHA hashes identifying file contents

This creates a complete audit trail of the system's behavior, making it possible to understand, debug, and verify the flow of data through the multi-agent pipeline.

---

## The Read-and-Write Spaces Model

The successful execution validated our read-and-write spaces architecture:

### Read Spaces (`/agent/{type}/read/`)
- **Mechanism**: Content copied into the agent's scope at invocation time
- **Security**: Agent cannot modify these files (OS-level permissions enforce this)
- **Purpose**: Provides reference material, codebase access, and work unit context
- **Subdirectories**:
  - `default/`: AI-sandboxing codebase copy (minus `.git`)
  - `workunit/`: Work unit content for the current task

### Write Spaces (`/agent/{type}/write/`)
- **Mechanism**: Agent-owned directories for output
- **Security**: Only place where agent writes have impact
- **Purpose**: Isolated workspace for task execution
- **Subdirectories**:
  - `primary/`: Main working directory, agent's CWD

### File Brokering Pattern
1. **Before invocation**: Copy content into read spaces (root-owned, read-only to agent)
2. **During execution**: Agent works in write space
3. **After invocation**: Copy write space content back to work unit folder
4. **Cleanup**: Clear workspace for next invocation

---

## Flow 1: The First Complete Sequence (15:18 - 15:39)

### Phase 1: Idle Polling (15:18:16 - 15:18:46)
The `agent-worker` polls `/workspaces/slopspaces/input/any` every 10 seconds, seeing two pre-existing work units from earlier test sessions:
- `2026-04-05_22-29-34_test-bugs-1775428151/`
- `2026-04-06_14-47-54_test-bugs-1775486848/`

### Phase 2: Heuristic Processing (15:18:49 - 15:19:12)
A new heuristic request arrives:

```
[15:18:49] heuristic-request/listdir: /workspaces/slopspaces/heuristic/pending
  test-bugs-1775488728/
    HEURISTIC.md (77b16084)

[15:18:49] heuristic-request/read: HEURISTIC.md

[15:18:53] heuristic-request/copy-in: /agent/heuristic-request/read/default
  [3 files, 19 dirs - abbreviated]
  .gitignore, LICENSE, futuristic_workspace.png...

[15:18:53] heuristic-request/copy-in: /agent/heuristic-request/read/workunit
  HEURISTIC.md (77b16084)
  PROCESSING.md (ca823737)

[15:19:12] heuristic-request/copy-out: /agent/heuristic-request/write/primary
  .prompt.tmp, agent_output.txt

[15:19:12] heuristic-request/create: .../2026-04-06_15-19-12_test-bugs-1775488728/
  DISPATCH.json (4f57d89f)
  HEURISTIC_SOURCE.md (77b16084)
```

**Key insight**: The heuristic agent:
1. Received the AI-sandboxing codebase in its read space
2. Received the work unit files in a separate read space
3. Produced a DISPATCH.json that triggers downstream work

### Phase 3: Dispatch Approval (15:19:16 - 15:21:55)
The dispatcher creates an approval flow:

```
[15:21:55] agent-dispatch/delete: .../approval_2026-04-06_15-19-16_eca18993
  [9 files, 1 dirs - terraform configuration + DISPATCH_RECORD.json]

[15:21:57] New work unit appears:
  approved-approval_2026-04-06_15-19-16_eca18993-20260406-152155/
    DISPATCH.json (678770c7)
```

The approval process took ~2.5 minutes, during which a human (or automated approval) confirmed the dispatch.

### Phase 4: Sequence Dispatch (15:22:12)
A sequence-to-new-repo flow begins:

```
[15:22:12] agent-dispatch/create: sequence-to-new-repo_2026-04-06_15-22-12_6ee5c600
  DISPATCH_RECORD.json, main.tf, outputs.tf, providers.tf,
  terraform.tfvars, variables.tf
```

### Phase 5: Containment Step (15:24:37 - 15:24:47)
The containment phase prepares the work environment:

```
[15:24:37] containment_sequence-to-new-repo_..._1775489018 appears
  [7 files, 15 dirs - full codebase structure]
  INSTRUCTION.json (b96bff26)

[15:24:38] agent-worker/copy-in: /agent/agent-worker/read/default
  [3 files, 19 dirs - abbreviated]

[15:24:38] agent-worker/copy-in: /agent/agent-worker/read/workunit
  [8 files, 15 dirs - abbreviated]

[15:24:47] agent-worker/copy-out: /agent/agent-worker/write/primary
  .prompt.tmp (b4d1ad23)
```

### Phase 6: Sequence Steps (15:26:47 - 15:30:46)
Two sequence steps execute:

**Step 001** (15:26:47):
```
[15:26:47] single_sequence-...-sequence_step_001_1775489190
  [10 files, 15 dirs]
  INSTRUCTION.json (90c50d5b)

[15:26:49] copy-in: read/default, read/workunit
[15:26:54] copy-out: write/primary
  .prompt.tmp (067d2d37)
```

**Step 002** (15:30:35):
```
[15:30:35] single_sequence-...-sequence_step_002_1775489417
  [11 files, 15 dirs]
  INSTRUCTION.json (980a6307)

[15:30:36] copy-in: read/default, read/workunit
[15:30:46] copy-out: write/primary
  .prompt.tmp (477992a1)
```

---

## Flow 2: The Second Complete Sequence (15:41 - 15:55)

The second flow follows the same pattern but with new content:

### Heuristic Processing (15:41:40 - 15:42:09)
```
[15:41:40] heuristic-request/listdir: heuristic/pending
  test-bugs-1775490090/
    HEURISTIC.md (527b6541)  <- Different content hash!

[15:41:51] copy-in: read/default [6 files now visible]
[15:41:51] copy-in: read/workunit
[15:42:09] copy-out: write/primary
  .prompt.tmp (144be5be), agent_output.txt (aefd5b0a)
```

### Approval & Sequence (15:42:11 - 15:45:10)
```
[15:44:53] approval deleted
[15:45:10] sequence-to-new-repo_..._308de865 created
```

### Containment & Steps (15:47:51 - 15:52:40)

**Containment** (15:47:51):
```
[15:48:13] copy-out: write/primary
  .opencode.json (db9f6fc6)   <- New file type!
  .prompt.tmp (71e5f5dc)
  README.md (571b6974)        <- New file type!
```

**Step 001** (15:50:13):
```
[15:50:22] copy-out: write/primary
  .opencode.json, .prompt.tmp
  hello-world-1.md (706c338d) <- Agent created a hello-world file!
```

**Step 002** (15:52:32):
```
[15:52:40] copy-out: write/primary
  .opencode.json, .prompt.tmp
  hello-world-2.md (e5eb3222) <- Another hello-world file!
```

---

## Notable Observations

### 1. Perfect Read/Write Isolation
Every agent invocation shows the clear pattern:
- `copy-in` to read spaces (before agent runs)
- `copy-out` from write space (after agent completes)

No direct writes to source locations. The file brokering works.

### 2. Accumulated State Through Sequences
The sequence steps show file accumulation:
- Containment: 7 files, 15 dirs
- Step 001: 10 files (containment output + step output)
- Step 002: 11 files (previous + more output)

The write space content is preserved and passed forward through sequence steps.

### 3. Agent Output Artifacts
The agents produced various outputs:
- `.prompt.tmp` - Prompt artifacts (every invocation)
- `agent_output.txt` - Heuristic output text
- `.opencode.json` - Configuration file
- `README.md` - Documentation
- `hello-world-1.md`, `hello-world-2.md` - Task outputs

### 4. Clean Input Queue Management
Work units appear, get processed, and the queue returns to baseline between major operations. No orphaned work units, no stuck items.

### 5. Consistent 10-Second Polling
The `agent-worker/listdir` operations occur precisely every 10 seconds, showing reliable polling behavior during idle periods.

---

## Timing Analysis

| Phase | Duration | Notes |
|-------|----------|-------|
| Heuristic processing | ~23 seconds | Fast AI inference |
| Approval wait | ~2.5 minutes | Human/external approval |
| Containment step | ~8 seconds | Copy + quick task |
| Sequence step | ~7-10 seconds | Copy + task execution |
| Full flow | ~20 minutes | Dominated by approval time |

---

## What This Proves

1. **Read/write spaces work end-to-end**: The file brokering pattern successfully isolates agent execution while enabling data flow.

2. **Multi-agent orchestration is viable**: Heuristic agents can dispatch work to worker agents through the approval pipeline.

3. **Sequence execution functions**: Multi-step workflows execute correctly with state accumulation.

4. **The filestory format is useful**: We can reconstruct exactly what happened from the audit trail.

---

## What's Next

With the first error-free execution complete, we can now:

1. **Stress test**: Run longer sequences with more complex tasks
2. **Error injection**: Test failure modes and recovery
3. **Performance optimization**: Reduce copy overhead for large codebases
4. **Container mode**: Move from OS-user isolation to full containerization

This execution represents a significant milestone: the first time our agent orchestration system ran a complete flow with the new security model in place, and it worked perfectly.
