# Read Spaces and Write Spaces

This document defines the filesystem access model for our heuristic agents, using the **slopspaces** directory tree as the working environment. We approach this from a **configurability-first** perspective—configuration is a first-class concern, not an afterthought—while using a paradigm of **convention over configuration** where absent configuration cascades into intelligent defaults.

---

## Core Concepts

### Read Spaces

Read spaces provide **material for agents to read**. The key mechanism: content is either safely copied into the agent's scope, or mounted at the container/system level in a **read-only manner**. From the security boundary perspective, the agent has no power to write these files. In practice, the underlying files may or may not be mutable, but any mutations are discarded and have no effect.

### Write Spaces

Write spaces are directories where agents can create, modify, and delete files. These are typically per-invocation unique directories, ensuring clean isolation between agent runs.

The defining characteristic of write spaces is that modifications have an impact on the wider world. Write spaces are the only places agents perform impacting writes.

### Configurability-First Design

Rather than defining specific long-lived directories upfront, we design the system to be configurable from the start:

- **Explicit configuration** takes precedence when provided
- **Intelligent defaults** apply when configuration is absent
- **Per-dispatch-type overrides** allow fine-grained control
- **Environment variables** enable runtime customization

This means the specific directories mentioned in this document are *defaults*, not hardcoded requirements.

---

## Current State

### Heuristic-Request (`heuristic-request/main.go`)

- Runs agents in **prompt-only mode** (`-p` flag) - effectively read-only
- Current working directory: set to the heuristic folder being processed
- No explicit directory restrictions configured yet
- Default paths:
  - `HEURISTIC_DIR`: `/workspaces/slopspaces/heuristic`
  - `REQUEST_DIR`: `/workspaces/slopspaces/input/any`
  - `RECORDS_DIR`: `/workspaces/slopspaces/agent-records/`

### Agent-Worker (`agent-worker/main.go`)

- Runs agents in **prompt** (`-p`) or **execute** (`-e`) mode
- **Implements read/write spaces model** with host-mode user isolation
- Agent workspace: `/agent/agent-worker/` (fixed location, cleared per invocation)
- Agent working directory: `/agent/agent-worker/write/primary/`
- Default paths for work units:
  - `INPUT_DIR`: `/workspaces/slopspaces/input/`
  - `OUTPUT_DIR`: `/workspaces/slopspaces/output/`
  - `RECORDS_DIR`: `/workspaces/slopspaces/agent-records/`
- Workspace structure (at `/agent/agent-worker/`):
  ```
  /agent/agent-worker/
  ├── read/
  │   └── default/    # Copy of work unit content (no .git)
  └── write/
      └── primary/    # Agent's working directory
  ```
- **Host mode isolation**: AI runs as restricted `agent-worker` OS user
- Makefile targets: `make setup` (create user/dirs), `make verify-setup`, `make run`

### invoke-agent.sh (`ambiguous-agent/invoke-agent.sh`)

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

**Read needs:**
- The heuristic folder being processed (for context files alongside HEURISTIC.json/md)
- Agent records (for understanding past decisions/context)

### Additional Read Space Content

Read spaces support several categories of content:

- **Repositories**: Sets of one-or-more repositories for the agent to see or work on
- **Reference materials**: Directories containing peripheral reference material such as documents or resource files
- **Declarative tool results**: Directories containing results from submissions of declarative-tool read manifests

### Codebase Access

When this project is in a more complete state, runtime binaries will not normally see dev-time directories on a regular basis. The AI-sandboxing codebase is a development-time resource, not a runtime dependency.

### External Repository Access

External repositories (e.g., cloned repos in slopspaces/working) are sometimes included in read spaces, depending on the task at hand.

### Shared Reference Resources

Read spaces require a standard directory form. When sharing directories across invocations, we use copies of directories or read-only mounting techniques to maintain isolation.

### Configuring Read Spaces

Read spaces are configured through a hierarchy:
1. **Per-invocation configuration** (highest priority)
2. **Per-dispatch-type configuration**
3. **System-wide defaults** (lowest priority)

---

## Write Spaces

Write spaces are directories where agents can create, modify, and delete files. The defining characteristic is that modifications in write spaces have an impact on the wider world—these are the only places agents perform impacting writes.

### Design Principles

- **Isolation by default**: Each invocation gets its own write space
- **Explicit boundaries**: Write access is granted only to specified directories
- **Configurable scope**: The extent of write access can be adjusted per dispatch type

### For Agent-Worker (Read-Write Agent in Execute Mode)

Agent-worker executes tasks that need to produce output.

**Write needs:**
- The work unit folder (current working directory)
- Output content directory
- Agent records directory

### For Heuristic-Request

Heuristic-request runs in prompt-only mode, so it has **no write spaces**. All writing is done by the Go program itself, not the agent.

### Work Unit Write Spaces

Write access includes a dedicated write-space directory for each work unit at minimum. This space may be detached and preserved for future sequential work units in multi-step workflows.

### Shared Scratch Space

Agent-worker has access to a shared "scratch" or "temp" directory in slopspaces for temporary working files.

### Sequence-to-New-Repo Dispatches

For sequence-to-new-repo dispatches that create new repos in slopspaces/working, we use a flexibility-first approach. We avoid over-optimizing for this case initially, but will develop efficiency measures when the same write space is used repeatedly.

### Output Content Handling

In the immediate system, any writes that occur "on the outside world" happen through the submission of declarative manifests executed by detached deterministic agents. The content in the immediate write-space is "filtered" by these agents, so the immediate agent may write whatever it wants to content within the write space.

### Configuring Write Spaces

Write spaces follow the same configuration hierarchy as read spaces:
1. **Per-invocation configuration** (highest priority)
2. **Per-dispatch-type configuration**
3. **System-wide defaults** (lowest priority)

---

## Implementation Approach

### Directory Preparation

Before invoking an agent:
1. Create a working directory in slopspaces (if needed)
2. `cd` to that directory
3. Pass `--add-dir` flags for additional read/write access

### Write Space Reuse Strategy

We use a hybrid approach:

- **Write spaces**: Per-invocation unique directories are the primary space, ensuring clean isolation between agent runs
- **Read spaces**: Appropriate for sharing and multi-read access across invocations

Agents start fresh each time (write space), while having access to shared reference material (read spaces).

Write space reuse is triggered by:
- Continuation of a multi-step sequence
- Retry of a failed invocation

### Working Directory Naming

Working directories are named using some combination of timestamp, UUID, and heuristic ID. The exact naming convention can evolve based on operational needs.

### Symbolic Links

Since we can always assume we are running on Linux or in a Linux container, symbolic links are a viable mechanism for referencing materials in the working directory. Implementation requires care to use them safely.

### Read-Only vs Read-Write Access

Claude Code's `--add-dir` grants both read AND write access. The file system location differentiates which is which for the agent. We don't need separate flags because:
- Read-space content writing is harmless since the content is either unwritable or disposable
- The directory structure itself communicates the intended usage pattern

---

## Security Considerations

### Sensitive Directory Protection

Agents do not have access to sensitive directories (e.g., ~/.ssh, ~/.aws, ~/.config). Our approach is **non-inclusion rather than tracked prevention**—we simply don't add these directories to the agent's allowed paths.

The security model works by explicit inclusion (allowlist), not by blocking specific paths. This aligns with the principle of least privilege: agents only see what we explicitly grant, rather than seeing everything except what we block.

### Auditing and Logging

Logging or alerting when an agent attempts to access paths outside its allowed spaces is a potential future feature for debugging and auditing purposes.

### Blocklist Consideration

Given the allowlist approach, a blocklist would only serve as a safety net or for documentation purposes. This is a future consideration.

### Filesystem Isolation

Slopspaces deployment normally uses a separate filesystem or filesystem isolation (containers, namespaces). The application itself does not need to know about this—it's an infrastructure concern. We avoid design decisions that would preclude this deployment model.

### Symlink and Traversal Protection

Protection against sandbox escape via symlinks or `..` traversal is a documented security requirement. The system maintains secure defaults while specific mitigation strategies are developed.

---

## Configuration

Configuration is a first-class concern in this system. While we use convention-over-configuration (intelligent defaults when nothing is specified), explicit configuration always takes precedence.

### Configuration Hierarchy

1. **Per-invocation overrides**: Specified in the dispatch/instruction itself
2. **Per-dispatch-type defaults**: Different dispatch types (repo-isolation, direct, sequence) have different defaults
3. **System-wide defaults**: Fallback when no specific configuration exists

### Environment Variables

Environment variables may provide defaults, but read/write spaces are more normally a per-invocation concern.

### Per-Dispatch-Type Configuration

Different dispatch types (repo-isolation, direct, sequence) have their own configurations.

### Glob Pattern Support

Glob pattern support (e.g., /workspaces/slopspaces/**) is not needed up-front and can be added later if necessary.

---

## Current Implementation Status

### Agent-Worker: IMPLEMENTED

The `agent-worker` component implements the full read/write spaces model with host-mode user isolation:

**Workspace Location**: `/agent/agent-worker/`
- This is a fixed location at the root of the filesystem
- One agent per host, so no invocation-specific subdirectories needed
- Content is cleared and repopulated for each invocation

**Directory Structure**:
```
/agent/agent-worker/
├── read/                  # Read-only (root-owned, 755)
│   ├── default/           # Copy of AI-sandboxing codebase (minus .git)
│   └── workunit/          # Copy of work unit content
└── write/                 # Writable (agent-user-owned, 755)
    └── primary/           # Agent's working directory
```

**Permission Model**:
The agent sees `/agent/agent-worker/` as its entire world. OS-level user permissions enforce the security boundary:
- `/agent/agent-worker/` - root:root 755 (agent can traverse)
- `/agent/agent-worker/read/` - root:root 1777 (sticky bit - binary user can create subdirs, agent can read all)
- `/agent/agent-worker/write/` - agent-worker:agent-worker 1777 (sticky bit - agent can write here)

The sticky bit (1777) allows the binary (running as vscode or any user) to create and clean up workspace subdirectories without requiring root privileges, while still preventing users from deleting each other's files.

Claude's `--add-dir` grants access to `/agent/agent-worker/` as a single tree, but the filesystem permissions determine what the agent can actually modify.

**File Brokering (Host Mode)**:
1. Before invocation:
   - Copy AI-sandboxing codebase → `/agent/agent-worker/read/default/`
   - Copy work unit content → `/agent/agent-worker/read/workunit/`
2. Agent runs with `/agent/agent-worker/write/primary/` as working directory
3. After invocation: Copy `/agent/agent-worker/write/primary/` content → work unit folder
4. Cleanup: Remove read and write directory contents

**User Isolation (Host Mode)**:
- The `agent-worker` binary runs as a privileged user (or root)
- The binary populates read/ (files are root-owned, so read-only to agent)
- The binary chowns write/ to the agent user
- The AI itself runs as the restricted `agent-worker` OS user
- Makefile targets manage user creation: `make setup`, `make check-user`

**Environment Variables**:
- `AGENT_USER`: Override the restricted user (set to empty to disable isolation)
- `AGENT_ADD_DIRS`: Workspace root passed to invoke-agent.sh (now just `/agent/agent-worker/`)

### Heuristic-Request: Pending

Heuristic-request dispatches work through agent-worker, so it inherits the read/write space model. Future work may add direct support for heuristic-specific read spaces.

### Next Steps

1. Test the agent-worker implementation end-to-end
2. Apply similar patterns to heuristic-request if needed
3. Implement container mode (mount-based isolation) as an alternative to host mode

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
    # Add any additional directories from AGENT_ADD_DIRS (colon-separated)
    if [[ -n "${AGENT_ADD_DIRS:-}" ]]; then
        IFS=':' read -ra extra_dirs <<< "$AGENT_ADD_DIRS"
        for extra_dir in "${extra_dirs[@]}"; do
            if [[ -n "$extra_dir" && -d "$extra_dir" ]]; then
                args+=("$add_dir_flag" "$extra_dir")
            fi
        done
    fi
fi
```

This means:
- In prompt mode: agent has records path added (but can't write anyway)
- In execute mode: agent has records path AND current directory added
- Additional directories can be passed via `AGENT_ADD_DIRS` environment variable (colon-separated list)

---

## Implementation Plan

This section documents how we'll proceed with implementing read/write spaces support, starting with heuristic-request. The implementation follows our principles of backward compatibility and auto-create paradigms.

### Target Agent Environment

When a heuristic agent runs in a fully isolated mode, the agent's environment follows this structure:

```
/agent/<type>/                    # Working directory for the agent (e.g., /agent/heuristic-request/)
├── read/                         # Read space(s) mounted here
│   ├── default/                  # Example: AI-sandboxing checkout copy
│   └── reference/                # Example: reference materials
└── write/                        # Write space(s) mounted here (for writing agents only, ie: worker)
    └── primary/                  # Always present if parent dir is; the main write space
```

**Key principles:**
- The agent's working directory is `/agent/<type>/` (e.g., `/agent/heuristic-request/` or `/agent/agent-worker/`)
- From the agent's perspective, only `/agent/<type>/` and its contents are visible
- Read spaces appear under `/agent/<type>/read/` with their alias as the subdirectory name
- Write spaces may appear under `/agent/<type>/write/`, with `primary` always present

The `<type>` component (e.g., `heuristic-request`, `agent-worker`) enables OS-level user/group permissions for isolation. Each agent type runs as a dedicated user, and since requests run serially within each type, this provides filesystem-level security boundaries without full containerization.

**Deployment modes:**
1. **Container mode**: Mount directories into the container filesystem at `/agent/<type>/read` and `/agent/<type>/write`
2. **Lightweight mode**: Use OS-level user/group permissions to ensure compromised agent/model code cannot violate trust. Each agent type runs as a dedicated user (e.g., `heuristic-request`, `agent-worker`), with filesystem permissions restricting access. Directories outside the agent's scope are made invisible to the running user. Use `--add-dir` flags with symbolic links or direct paths to grant explicit access

### Phase 1: Heuristic-Request Read Spaces (Lightweight Mode)

**Goal**: Enable heuristic-request agents to see new repository checkouts (starting with the AI-sandboxing repository itself), using the lightweight deployment approach.

**Approach**: Extend HEURISTIC.json with an optional `readSpaces` field that can designate additional directories for the agent to access. When absent, the 'default' readspace is attached.

We want to make sure even the 'direct launchers' (like 'make run') act as a new restricted user we've created.

We also want to create the concept of creating readspaces and assigning readspaces up-front. If we do not explicitly perform this process then the AI-sandboxing repository gets added to a 'default' readspace with 'copy' mode so that an ephemeral copy with the '.git' removed is dispatched.

#### HEURISTIC.json Extension

```json
{
  "heuristic": "...",
  "prompt": "...",
  "useReadSpaces": ["default"], // This is also the default
  "createReadSpaces" : {}
}
```

**Field semantics:**
- `useReadSpaces` (optional): Array of read space handles to attach. Defaults to `["default"]`
- `createReadSpaces` (optional): Object mapping read space names to their creation configuration (semantics defined later)

#### Backward Compatibility

Because the default readspace is auto-attached when configuration is absent, things are FUNCITONALLY backwards compatible.

This is better than maintaining explicitly identical behaviour.

#### Implementation Changes to heuristic-request/main.go

1. **Parse readSpaces from HEURISTIC.json**: Extend the heuristic parsing to recognize the `useReadSpaces` field
1. **Copy the read-space directories into place**: Bring the readspace into place with a copy-and-remove-git.

#### First Use-Case: AI-sandboxing Repository

The immediate goal is a heuristic that can examine the AI-sandboxing repository. Example HEURISTIC.json:

```json
{
  "heuristic": "review-codebase",
  "prompt": "Send a task to the downstream agent based on the structure you can see in AI-sanboxing.",
  "useReadSpaces": ["default"]
}
```

This allows the heuristic agent to read files from the AI-sandboxing repository while maintaining the prompt-only security model.

### Phase 2: Working Directory Isolation (Lightweight Mode)

**Goal**: Change the agent's working directory to a prepared directory structure that mirrors the target `/agent/<type>/` layout, while still using the lightweight deployment approach.

**Approach**: Before invoking the agent, heuristic-request prepares a temporary directory structure that mirrors the target `/agent/<type>/` layout:

```
/workspaces/slopspaces/agent-workspaces/heuristic-request/{invocation-id}/
├── read/
│   └── {alias} -> {actual-path}   # Prepared and then copied into the correct path
└── write/
    └── primary/                    # Actual directory for writes (for agent-worker)
```

**Implementation:**
1. Create the workspace directory with the invocation ID under the appropriate type directory
2. Create `read/` subdirectory right away. Create `write/primary/` subdirectory later when we implement agent-worker
3. Create symbolic links in `read/` pointing to each read space path
4. Change to this directory before invoking the agent
5. Pass `--add-dir` for the workspace and each linked read space
6. Run the agent as a dedicated OS user (e.g., `heuristic-request` user) with appropriate group permissions

### Phase 3: Write Spaces for Agent-Worker

**Goal**: Extend write spaces support to agent-worker for execute mode agents.

**Approach**: Similar to read spaces, add `writeSpaces` configuration to work-unit files. We can consider this schema later.

### Phase 4: Container Mode

**Goal**: Support full container isolation with mounted directories.

**Approach**: When running in container mode:
1. Mount read spaces read-only at `/agent/<type>/read/{alias}`
2. Mount write spaces read-write at `/agent/<type>/write/{alias}`
3. Set working directory to `/agent/<type>/`
4. No `--add-dir` flags needed; the mount points define the boundaries

This phase requires container orchestration integration and is deferred until the lightweight mode is proven.

### Design Decisions

1. **Repository paths are absolute**: We use absolute paths to avoid ambiguity. The system running heuristic-request knows its filesystem layout.

2. **No glob patterns initially**: Keeping it simple with explicit paths. Glob support can be added later if needed.

3. **Aliases are optional but recommended**: They provide meaningful names in the agent's environment. When absent, a sanitized version of the path basename is used.

4. **Read-only by construction in Phase 1**: Since heuristic-request uses `-p` (prompt mode), the read spaces are inherently read-only regardless of what `--add-dir` technically allows.

5. **Copies first for lightweight mode**: Disposable copies are safe and simple, they need to have no .git present.

6. **Phased rollout**: Each phase builds on the previous, allowing us to validate the approach incrementally before adding complexity.

7. **Makefile target for user/group setup**: The Makefile includes a target to ensure the presence of (and create if needed) the dedicated OS users and groups for each agent type. This supports the lightweight mode's reliance on OS-level user/group permissions for isolation.