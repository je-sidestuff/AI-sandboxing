# Read Spaces and Write Spaces

This document defines the filesystem access model for our heuristic agents, using the **slopspaces** directory tree as the working environment. We approach this from a **configurability-first** perspective—configuration is a first-class concern, not an afterthought—while using a paradigm of **convention over configuration** where absent configuration cascades into intelligent defaults.

---

## Core Concepts

### Read Spaces

Read spaces provide **material for agents to read**. The key mechanism: content is either safely copied into the agent's scope, or mounted at the container/system level in a **read-only manner**. From the security boundary perspective, the agent has no power to write these files. In practice, the underlying files may or may not be mutable, but any mutations are discarded and have no effect.

### Write Spaces

Write spaces are directories where agents can create, modify, and delete files. These are typically per-invocation unique directories, ensuring clean isolation between agent runs.

### Configurability-First Design

Rather than defining specific long-lived directories upfront, we design the system to be configurable from the start:

- **Explicit configuration** takes precedence when provided
- **Intelligent defaults** apply when configuration is absent
- **Per-dispatch-type overrides** allow fine-grained control
- **Environment variables** enable runtime customization

This means the specific directories mentioned in this document are *defaults*, not hardcoded requirements.

---

## What We Know Already

### Current State

1. **Heuristic-request** (`heuristic-request/main.go`)
   - Runs agents in **prompt-only mode** (`-p` flag) - effectively read-only
   - Current working directory: set to the heuristic folder being processed
   - No explicit directory restrictions configured yet
   - Default paths:
     - `HEURISTIC_DIR`: `/workspaces/slopspaces/heuristic`
     - `REQUEST_DIR`: `/workspaces/slopspaces/input/any`
     - `RECORDS_DIR`: `/workspaces/slopspaces/agent-records/`

2. **Agent-worker** (`agent-worker/main.go`)
   - Runs agents in **prompt** (`-p`) or **execute** (`-e`) mode
   - Current working directory: set to the work unit folder
   - Default paths:
     - `INPUT_DIR`: `/workspaces/slopspaces/input/`
     - `OUTPUT_DIR`: `/workspaces/slopspaces/output/`
     - `RECORDS_DIR`: `/workspaces/slopspaces/agent-records/`

3. **invoke-agent.sh** (`ambiguous-agent/invoke-agent.sh`)
   - Supports `--add-dir` flag for claude and copilot presets
   - Currently adds `$records_path` as an allowed directory
   - In execute mode, also adds `$call_pwd` (the working directory)
   - The `--add-dir` flag maps to Claude's `--add-dir` which adds to allowed read/write paths

### The Slopspaces Directory Tree

The intended working structure:
```
/workspaces/slopspaces/
├── heuristic/
│   ├── pending/          # Input: HEURISTIC.json or HEURISTIC.md folders
│   └── processed/        # Output: Completed heuristic processing
├── input/
│   └── any/              # Input: DISPATCH.json, INSTRUCTION.json, REPORT.json
├── output/
│   ├── content/          # Output: Completed work results
│   └── records/          # Work unit completion records
├── agent-records/
│   ├── worker/           # Records from agent-worker
│   ├── dispatch/         # Records from agent-dispatch
│   ├── dispatch-watch/   # Records from agent-dispatch (watch mode)
│   └── heuristic/        # Records from heuristic-request
├── dispatcher/
│   └── live/             # Terraform configurations for dispatches
│       └── flows/
├── working/              # Working directory for sequence executions
└── (other directories?)
```

### Key Insight: Directory Change Before Invocation

The plan is to **change directory** to a prepared slopspaces subdirectory before invoking the agent. This makes the agent's "current directory" the constrained working space, and we use `--add-dir` to explicitly grant access to additional read/write locations.

---

## Read Spaces

Read spaces provide content for agents to reference without modification capability. The security model ensures that even if the underlying storage is technically mutable, changes from within the agent's scope are discarded or have no effect.

### Implementation Mechanism

Read spaces use one of two approaches:
1. **Copy-on-provision**: Content is copied into the agent's scope at invocation time
2. **Read-only mount**: Content is mounted at the container/system level with read-only permissions

From the agent's perspective, these files simply exist and cannot be written. The mechanism is transparent to the agent.

### For Heuristic-Request (Read-Only Agent)

Since heuristic-request uses prompt-only mode, the entire invocation is conceptually read-only. However, specifying what the agent can *see* remains important for both security and clarity.

**Currently known read needs:**
- The heuristic folder being processed (for context files alongside HEURISTIC.json/md)
- Agent records (for understanding past decisions/context)

### Configuring Read Spaces

Read spaces are configured through a hierarchy:
1. **Per-invocation configuration** (highest priority)
2. **Per-dispatch-type configuration**
3. **System-wide defaults** (lowest priority)

### Questions About Read Spaces

#### Q1: What additional directories should heuristic-request be able to read?

Directories like sets of one-or-more repositories to see or work on.
Directories containing peripheral reference material, maybe documents or resource files.
Directories containing results from submissions of declarative-tool-tool read manifests.

#### Q2: Should heuristic-request have read access to the main codebase (AI-sandboxing)?

When this project is in a more complete state it will not be usual for runtime binaries to see dev-time directories on a regular basis.

#### Q3: Should read spaces include access to external repositories (e.g., cloned repos in slopspaces/working)?

Sometimes, yes.

#### Q4: Should there be a dedicated "reference" directory in slopspaces for shared read-only resources?

This question is unclear -- there should be standard directory form for read-spaces to exist in. We must remember that we need copies of directories or read-only mounting techniques if they are shared.

---

## Write Spaces

Write spaces are directories where agents can create, modify, and delete files. Unlike read spaces, write spaces are typically **per-invocation unique**, providing clean isolation between agent runs.

<EDIT: The more important distinction is that when agents write into these directories the modifications will have an impact on the wider world. Write spaces are the only places agents will be performing impacting writes.>

### Design Principles

- **Isolation by default**: Each invocation gets its own write space
- **Explicit boundaries**: Write access is granted only to specified directories
- **Configurable scope**: The extent of write access can be adjusted per dispatch type

### For Agent-Worker (Read-Write Agent in Execute Mode)

Agent-worker executes tasks that need to produce output.

**Currently known write needs:**
- The work unit folder (current working directory)
- Output content directory
- Agent records directory

### For Heuristic-Request

Heuristic-request runs in prompt-only mode, so it should have **no write spaces**. All writing is done by the Go program itself, not the agent.

### Configuring Write Spaces

Write spaces follow the same configuration hierarchy as read spaces:
1. **Per-invocation configuration** (highest priority)
2. **Per-dispatch-type configuration**
3. **System-wide defaults** (lowest priority)

### Questions About Write Spaces

#### Q5: For agent-worker, should write access be limited to only the current work unit folder?

-- Possibly. It should be in a dedicated write-space directory dedicated to this work unit at a minimum (although potentially detached/preserved for future sequential work units).

#### Q6: Should agent-worker be able to write to a shared "scratch" or "temp" directory in slopspaces?

-- Yes. Good idea. Not sure of the details butg this seems like a good measure.

#### Q7: How should write access work for sequence-to-new-repo dispatches? (They create new repos in slopspaces/working)

-- This is uncertain. We don't want to over-optimize for this case, but it's also an important example.

We want to start with a flexibility-first approach but will want to make things efficient eventually if the same write space will be used repeatedly.

#### Q8: Should there be explicit write access to output/content, or should the Go program move files there after the agent completes?

-- In the immediate system any writes that occur "on the outside world" will happen through the submission of declarative manifests which are executed by detached deterministic agents. The content in the immediate write-space will be "filtered" by these agents and so the immediate agent may write whatever it wants to content within the write space.

---

## Implementation Approach

### Proposed Directory Preparation

Before invoking an agent:
1. Create a working directory in slopspaces (if needed)
2. `cd` to that directory
3. Pass `--add-dir` flags for additional read/write access

### Questions About Implementation

#### Q9: Should we create a new subdirectory per invocation, or reuse a consistent working directory?

**Answer**: We use a hybrid approach:

- **Write spaces**: Per-invocation unique directories are the primary space, ensuring clean isolation between agent runs
- **Read spaces**: Appropriate for sharing and multi-read access across invocations

This means agents start fresh each time (write space), while having access to shared reference material (read spaces). We may develop strategies for selective write space reuse in specific scenarios, but isolation is the default.

**Follow-up**: What criteria would trigger write space reuse? (e.g., continuation of a multi-step sequence, retry of a failed invocation)

-- Both of these.

#### Q10: How should the working directory be named? (timestamp, UUID, heuristic ID, etc.)

-- Some combination of these probably, we don't need to get it perfect up-front.

#### Q11: Should the Go programs (heuristic-request, agent-worker) create symbolic links in the working directory to reference materials?

-- We can always assume we are running on linux/in a linux container; so this might be a worthwhile measure. We just have to figure out how to use it safely.

#### Q12: Should there be separate read-spaces and write-spaces flags, or a single allowed-directories concept?

Note: Claude Code's `--add-dir` grants both read AND write access. For read-only access, we'd need prompt-only mode or a different mechanism.

-- The file system location will differentiate which is which for the agent. Remember that we don't care if read-space content writing is performed because the content is either unwritable or disposable

---

## Security Considerations

### Questions About Security

#### Q13: Should agents be prevented from accessing sensitive directories (e.g., ~/.ssh, ~/.aws, ~/.config)?

**Answer**: Yes, agents should not have access to sensitive directories. Our approach is **non-inclusion rather than tracked prevention**—we simply don't add these directories to the agent's allowed paths. The security model works by explicit inclusion (allowlist), not by blocking specific paths.

This aligns with the principle of least privilege: agents only see what we explicitly grant, rather than seeing everything except what we block.

**Follow-up**: Should we log or alert when an agent attempts to access paths outside its allowed spaces? (For debugging/auditing purposes)

-- Maybe, but let's note this and worry about it later.

#### Q14: Should there be a blocklist of paths that are never allowed?

Note: Given the allowlist approach established in Q13, a blocklist would only serve as a safety net or for documentation purposes.

-- Maybe, but let's note this and worry about it later.

#### Q15: Should slopspaces be on a separate filesystem or use filesystem isolation (containers, namespaces)?

-- Yes, this should be a normal means of deploying the system, but the application should not need to know about this. We won't think about this unless we are trying to do something that would otherwise preclude it.

#### Q16: How do we prevent agents from escaping their sandbox via symlinks or .. traversal?

-- Another 'document for now and keep secure by default' scneraio.

---

## Configuration

Configuration is a first-class concern in this system. While we use convention-over-configuration (intelligent defaults when nothing is specified), explicit configuration always takes precedence.

### Configuration Hierarchy

1. **Per-invocation overrides**: Specified in the dispatch/instruction itself
2. **Per-dispatch-type defaults**: Different dispatch types (repo-isolation, direct, sequence) may have different defaults
3. **System-wide defaults**: Fallback when no specific configuration exists

### Questions About Configuration

#### Q17: Should read/write spaces be configurable via environment variables?

-- Maybe defaults, but it will be more normal for them to be a per-invocation concern.

#### Q18: Should there be per-dispatch-type configurations (repo-isolation vs direct vs sequence)?

-- Yes.

#### Q19: Should the configuration support glob patterns (e.g., /workspaces/slopspaces/**)?

-- We probably don't need this up-front.

---

## Next Steps After This Brainstorm

Once the questions above are answered, we'll:

1. Update `heuristic-request/main.go` to:
   - Change to a prepared working directory before invoking the agent
   - Pass appropriate `--add-dir` flags for read access (heuristic-request is already read-only via `-p`)

2. Update `agent-worker/main.go` similarly for execute mode agents

3. Document the final read/write space model

---

## Appendix: Claude Code --add-dir Behavior

From Claude Code documentation:
- `--add-dir <path>` adds a directory to the allowed paths
- These paths allow BOTH reading AND writing
- For read-only access, use prompt mode (`-p`) which prevents all file modifications
- Claude Code's sandbox operates at the session level

### Current invoke-agent.sh Logic

```bash
if [[ -n "$add_dir_flag" ]]; then
    args+=("$add_dir_flag" "$records_path")
    if [[ "$mode" == "-e" ]]; then
        args+=("$add_dir_flag" "$call_pwd")
    fi
fi
```

This means:
- In prompt mode: agent has records path added (but can't write anyway)
- In execute mode: agent has records path AND current directory added
