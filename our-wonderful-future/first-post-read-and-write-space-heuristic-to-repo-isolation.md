# First Post-Read-and-Write-Space Heuristic-to-Repo-Isolation Flow

**Date**: 2026-04-06
**Status**: Successful end-to-end execution

This document chronicles our first successful heuristic-to-repo-isolation flow after implementing the read and write spaces model. The file-story provides a detailed accounting of every file operation that occurred during the flow, demonstrating the visibility and traceability we now have over agent activities.

---

## The File Story

The file-story below captures every filesystem operation performed by our agents during this flow. Each line records a timestamp, the responsible agent, the operation type, and the path(s) involved. Content hashes (SHA256 prefixes) provide cryptographic verification of file contents.

```
[2026-04-06 21:29:44] heuristic-request/listdir: /workspaces/slopspaces/heuristic/pending
  └── test-bugs-1775510980/
     └── HEURISTIC.json (2b4c3a97)
[2026-04-06 21:29:44] heuristic-request/read: /workspaces/slopspaces/heuristic/pending/test-bugs-1775510980/HEURISTIC.json (2b4c3a97)
[2026-04-06 21:29:46] heuristic-request/copy-in: /agent/heuristic-request/read/default
  [6 files, 19 dirs - abbreviated]
  ├── .gitignore (dc0cbd34)
  ├── LICENSE (d8df369b)
  ├── futuristic_workspace.png (6293122a)
  ├── ... and 3 more files
  ├── .devcontainer/
  ├── .grok/
  └── ... and 17 more directories
[2026-04-06 21:29:46] heuristic-request/copy-in: /agent/heuristic-request/read/workunit
  ├── HEURISTIC.json (2b4c3a97)
  └── PROCESSING.md (397cc2a5)
[2026-04-06 21:30:20] heuristic-request/copy-out: /agent/heuristic-request/write/primary
  ├── .prompt.tmp (85951632)
  └── agent_output.txt (72eec517)
[2026-04-06 21:30:20] heuristic-request/create: /workspaces/slopspaces/input/any/2026-04-06_21-30-20_test-bugs-1775510980/DISPATCH.json (16a644bd)
[2026-04-06 21:30:20] heuristic-request/create: /workspaces/slopspaces/input/any/2026-04-06_21-30-20_test-bugs-1775510980/HEURISTIC_SOURCE.md (0179105b)
[2026-04-06 21:30:27] agent-worker/listdir: /workspaces/slopspaces/input/any
  └── 2026-04-06_21-30-20_test-bugs-1775510980/
     ├── DISPATCH.json (16a644bd)
     ├── DISPATCHING.md (f6d717c5)
     └── HEURISTIC_SOURCE.md (0179105b)
[2026-04-06 21:32:03] agent-dispatch/delete: /workspaces/slopspaces/dispatcher/live/flows/approval/approval_2026-04-06_21-30-24_daa5bca7
  [9 files, 1 dirs - abbreviated]
  ├── .terraform.lock.hcl (91ba4328)
  ├── DISPATCH_RECORD.json (bda481fe)
  ├── main.tf (7dcf0b1e)
  ├── ... and 6 more files
  ├── .terraform/
[2026-04-06 21:32:07] agent-worker/listdir: /workspaces/slopspaces/input/any
  ├── 2026-04-06_21-30-20_test-bugs-1775510980/
  │  ├── DISPATCH.json (16a644bd)
  │  ├── DISPATCHING.md (f6d717c5)
  │  └── HEURISTIC_SOURCE.md (0179105b)
  └── approved-approval_2026-04-06_21-30-24_daa5bca7-20260406-213203/
     └── DISPATCH.json (681bada0)
[2026-04-06 21:32:21] agent-dispatch/copy-in: /workspaces/slopspaces/input/any/approved-approval_2026-04-06_21-30-24_daa5bca7-20260406-213203
  ├── DISPATCH.json (681bada0)
  └── DISPATCHING.md (8e2de7a6)
[2026-04-06 21:32:21] agent-dispatch/create: /workspaces/slopspaces/dispatcher/live/flows/repo-isolation/repo-isolation_2026-04-06_21-32-21_e4477c7c
  ├── DISPATCH_RECORD.json (681bada0)
  ├── main.tf (a2e0809c)
  ├── outputs.tf (0f7b1d6b)
  ├── providers.tf (aaa5e5b5)
  ├── terraform.tfvars (f54eba76)
  └── variables.tf (fd2b25c2)
[2026-04-06 21:33:27] agent-worker/listdir: /workspaces/slopspaces/input/any
  ├── 2026-04-06_21-30-20_test-bugs-1775510980/
  │  ├── DISPATCH.json (16a644bd)
  │  ├── DISPATCHING.md (f6d717c5)
  │  └── HEURISTIC_SOURCE.md (0179105b)
  ├── approved-approval_2026-04-06_21-30-24_daa5bca7-20260406-213203/
  │  ├── DISPATCH.json (681bada0)
  │  └── DISPATCHING.md (8e2de7a6)
  └── containment_repo-isolation_2026-04-06_21-32-21_e4477c7c_1775511147/
     [7 files, 15 dirs - abbreviated]
     ├── .gitignore (dc0cbd34)
     ├── INSTRUCTION.json (54a8cb49)
     ├── LICENSE (d8df369b)
     ├── ... and 4 more files
     ├── .devcontainer/
     ├── agent-advisor/
     └── ... and 13 more directories
[2026-04-06 21:33:27] agent-worker/read: /workspaces/slopspaces/input/any/containment_repo-isolation_2026-04-06_21-32-21_e4477c7c_1775511147/INSTRUCTION.json (54a8cb49)
[2026-04-06 21:33:28] agent-worker/copy-in: /agent/agent-worker/read/default
  [6 files, 19 dirs - abbreviated]
  ├── .gitignore (dc0cbd34)
  ├── LICENSE (d8df369b)
  ├── futuristic_workspace.png (6293122a)
  ├── ... and 3 more files
  ├── .devcontainer/
  ├── .grok/
  └── ... and 17 more directories
[2026-04-06 21:33:29] agent-worker/copy-in: /agent/agent-worker/read/workunit
  [8 files, 15 dirs - abbreviated]
  ├── .gitignore (dc0cbd34)
  ├── INSTRUCTION.json (54a8cb49)
  ├── LICENSE (d8df369b)
  ├── ... and 5 more files
  ├── .devcontainer/
  ├── agent-advisor/
  └── ... and 13 more directories
[2026-04-06 21:33:40] agent-worker/copy-out: /agent/agent-worker/write/primary
  ├── .opencode.json (53c9fb2a)
  ├── .prompt.tmp (ab967476)
  ├── agent-dispatch/
  │  └── TODO.md (d00433c8)
  └── our-wonderful-future/
     └── STORY.md (2eb999eb)
[2026-04-06 21:40:19] agent-dispatch/delete: /workspaces/slopspaces/dispatcher/live/flows/repo-isolation/repo-isolation_2026-04-06_21-32-21_e4477c7c
  [9 files, 1 dirs - abbreviated]
  ├── .terraform.lock.hcl (91ba4328)
  ├── DISPATCH_RECORD.json (681bada0)
  ├── main.tf (a2e0809c)
  ├── ... and 6 more files
  ├── .terraform/
```

*(Note: The file-story contains many repeated `agent-worker/listdir` entries showing the polling behavior as the worker waits for work. These have been condensed above for readability but are present in the raw accounting.)*

---

## How the Flow Unfolded

The flow demonstrates the full chain from heuristic submission to repo-isolation execution. Here's a detailed walkthrough of each phase:

### Phase 1: Heuristic Discovery and Processing (21:29:44 - 21:30:20)

**Lines 1-4**: The `heuristic-request` agent discovers a pending heuristic.

The flow begins when `heuristic-request` scans the pending directory and finds `test-bugs-1775510980/HEURISTIC.json`. The content hash `2b4c3a97` provides a fingerprint of the exact heuristic being processed.

**Lines 5-16**: Read spaces are populated.

This is where the new read-and-write-space model becomes visible. The system copies the AI-sandboxing codebase into `/agent/heuristic-request/read/default` (lines 5-13) and the work unit into `/agent/heuristic-request/read/workunit` (lines 14-16). The agent now has:
- A pristine copy of the codebase (without `.git`) for context
- The heuristic request materials for processing

**Lines 17-19**: The heuristic agent produces output.

After 34 seconds of processing, the agent's output appears in `/agent/heuristic-request/write/primary`. This includes the prompt working file and the actual output. The write space isolation means the agent could only write here—not to the read spaces or anywhere else.

**Lines 20-21**: Dispatch request is created.

The heuristic processing completes by creating a `DISPATCH.json` in the input queue. Hash `16a644bd` identifies this specific dispatch request. The `HEURISTIC_SOURCE.md` preserves provenance of where this dispatch originated.

### Phase 2: Dispatch Polling and Approval (21:30:27 - 21:32:21)

**Lines 22-72 (condensed in story above)**: The agent-worker polls.

The `agent-worker` enters its polling loop, checking `/workspaces/slopspaces/input/any` every 10 seconds. This is visible as repeated `listdir` operations. The work unit sits waiting in the queue.

**Line 72-78**: Approval flow completes.

At 21:32:03, the `agent-dispatch` component cleans up the approval flow directory. The Terraform files (`.terraform.lock.hcl`, `main.tf`, etc.) and the `DISPATCH_RECORD.json` are deleted, indicating the approval process has concluded.

**Lines 79-85**: Approved work appears.

By 21:32:07, a new directory `approved-approval_2026-04-06_21-30-24_daa5bca7-20260406-213203/` appears in the input queue. The naming convention encodes:
- That it was approved (`approved-`)
- The original approval flow identifier (`approval_2026-04-06_21-30-24_daa5bca7`)
- The approval timestamp (`20260406-213203`)

### Phase 3: Repo-Isolation Flow Execution (21:32:21 - 21:33:40)

**Lines 93-102**: Repo-isolation Terraform is created.

The `agent-dispatch` reads the approved dispatch and creates the repo-isolation flow. The Terraform configuration files appear in `/workspaces/slopspaces/dispatcher/live/flows/repo-isolation/`. These files define the isolated container environment where the actual work will execute.

**Lines 159-167**: Containment directory appears.

At 21:33:27, the isolated work environment materializes. The `containment_repo-isolation_2026-04-06_21-32-21_e4477c7c_1775511147/` directory contains:
- A copy of the AI-sandboxing codebase
- An `INSTRUCTION.json` defining the work to be done
- All the supporting directories and files needed

**Lines 168-186**: Agent-worker picks up the contained work.

The worker reads the `INSTRUCTION.json` (hash `54a8cb49`), then populates its read/write spaces:
- `/agent/agent-worker/read/default` gets the codebase
- `/agent/agent-worker/read/workunit` gets the full contained work unit (including the codebase copy that the repo-isolation flow placed there)

**Lines 187-193**: Work output emerges.

The agent produces output in its write space. The output includes:
- Configuration files (`.opencode.json`)
- Work artifacts (`agent-dispatch/TODO.md`, `our-wonderful-future/STORY.md`)

### Phase 4: Cleanup (21:40:19)

**Lines 506-512**: Flow cleanup.

The `agent-dispatch` cleans up the repo-isolation Terraform directory, indicating the flow has completed its lifecycle.

---

## Visibility Analysis

The file-story demonstrates several key aspects of our visibility over agent operations:

### What We Can See

1. **Complete operation trace**: Every file read, write, copy, and delete is logged with timestamps. We know exactly what happened and when.

2. **Content verification**: SHA256 hash prefixes on files let us verify content hasn't changed between operations. When `DISPATCH.json (16a644bd)` appears multiple times, we know it's the same content.

3. **Agent attribution**: Each operation is tagged with the responsible agent (`heuristic-request`, `agent-worker`, `agent-dispatch`). We can trace which component performed each action.

4. **Space isolation proof**: The read/write space model is visible in the paths:
   - Copy-in operations populate read spaces (`/agent/*/read/`)
   - Copy-out operations capture write space contents (`/agent/*/write/primary`)
   - The agent never directly touches files outside its spaces

5. **Flow correlation**: Directory naming embeds timestamps and identifiers, allowing us to trace work units through the entire flow.

### What We Learn

The polling pattern (lines 22-72 in the condensed story) shows the agent-worker checking every 10 seconds. This reveals:
- The system is working as designed (poll-based discovery)
- No work was missed or delayed unexpectedly
- The timing between heuristic completion (21:30:20) and approval (21:32:03) was about 103 seconds

The approval gap suggests human intervention or a deliberate delay in the approval flow—exactly the kind of insight the file-story enables.

---

## Reflections on Process Visibility

### The Value of Comprehensive Logging

Before the read-and-write-space model, understanding what agents did required examining scattered logs, trusting agent-reported outputs, and hoping nothing was missed. Now we have:

1. **Ground truth at the filesystem level**: The file-story is generated from actual filesystem operations, not agent self-reports.

2. **Cryptographic verification**: Content hashes mean we can prove what content existed at each point.

3. **Isolation guarantees**: The space model means agents physically cannot access areas outside their designated zones—not through policy, but through filesystem permissions.

### Limitations to Address

The current file-story has some gaps:

1. **Network operations aren't captured**: If an agent makes API calls or network requests, those don't appear in the file-story.

2. **In-memory processing is opaque**: What the agent does between reading input and writing output isn't visible.

3. **Polling noise**: The repeated `listdir` entries (67 occurrences of the same check!) obscure the meaningful events. Future iterations should either:
   - Deduplicate repeated identical operations
   - Separate polling from substantive operations
   - Use a different logging level for routine polling

---

## Future Evolution

Based on this successful flow, we can envision several improvements:

### Short-term Enhancements

1. **File-story deduplication**: Collapse repeated identical operations (like polling) into a single entry with a count or time range.

2. **Operation categorization**: Tag operations as "routine" vs "substantive" to allow filtered views.

3. **Flow visualization**: Generate diagrams showing the work unit's journey through the system with timestamps.

### Medium-term Capabilities

1. **Content diffing**: For modified files, show what changed between versions.

2. **Real-time monitoring**: Stream the file-story during execution for live visibility.

3. **Anomaly detection**: Flag unexpected file accesses or unusual patterns automatically.

### Long-term Vision

1. **Full provenance chain**: Link file-story entries to the git commits, container images, and configurations that produced them.

2. **Replay capability**: Given a file-story and the original inputs, reproduce the exact execution.

3. **Cross-flow correlation**: Track how outputs from one flow become inputs to another.

---

## Conclusion

This first post-read-and-write-space heuristic-to-repo-isolation flow succeeded in demonstrating:

1. **The isolation model works**: Agents operated within their designated spaces, producing outputs only in their write zones while reading from prepared read zones.

2. **The dispatch chain functions**: Work flowed from heuristic → dispatch request → approval → repo-isolation → execution → cleanup.

3. **Visibility is dramatically improved**: The file-story provides a complete, verifiable record of all filesystem operations.

The 11-minute flow (21:29:44 to 21:40:19) processed a heuristic, gained approval, spun up an isolated execution environment, performed work, and cleaned up. Every step is documented, timestamped, and content-verified.

This is the foundation for trusted autonomous agent execution—not through blind faith in AI behavior, but through comprehensive observability and structural isolation guarantees.
