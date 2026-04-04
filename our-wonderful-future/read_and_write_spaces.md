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
- Current working directory: set to the work unit folder
- Default paths:
  - `INPUT_DIR`: `/workspaces/slopspaces/input/`
  - `OUTPUT_DIR`: `/workspaces/slopspaces/output/`
  - `RECORDS_DIR`: `/workspaces/slopspaces/agent-records/`

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

## Next Steps

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

---

## Implementation Plan

This section documents how we'll proceed with implementing read/write spaces support, starting with heuristic-request. The implementation follows our principles of backward compatibility and auto-create paradigms.

### Target Agent Environment

When a heuristic agent runs in a fully isolated mode, the agent's environment follows this structure:

```
/agent/                    # Working directory for the agent
├── read/                  # Read space(s) mounted here
│   ├── ai-sandboxing/     # Example: repository checkout
│   └── reference/         # Example: reference materials
└── write/                 # Write space(s) mounted here
    └── primary/           # Always present; the main write space
```

**Key principles:**
- The agent's working directory is `/agent`
- From the agent's perspective, only `/agent` and its contents are visible
- Read spaces appear under `/agent/read/` with their alias as the subdirectory name
- Write spaces appear under `/agent/write/`, with `primary` always present

**Deployment modes:**
1. **Container mode**: Mount directories into the container filesystem at `/agent/read` and `/agent/write`
2. **Lightweight mode**: Use `--add-dir` flags with symbolic links or direct paths, achieving similar isolation without full containerization

### Phase 1: Heuristic-Request Read Spaces (Lightweight Mode)

**Goal**: Enable heuristic-request agents to see new repository checkouts (starting with the AI-sandboxing repository itself), using the lightweight deployment approach.

**Approach**: Extend HEURISTIC.json with an optional `readSpaces` field that can designate additional directories for the agent to access. When absent, the current behavior applies unchanged.

#### HEURISTIC.json Extension

```json
{
  "heuristic": "...",
  "prompt": "...",
  "readSpaces": {
    "repositories": [
      {
        "path": "/workspaces/workspace/sandbox/AI-sandboxing",
        "alias": "ai-sandboxing"
      }
    ]
  }
}
```

**Field semantics:**
- `readSpaces` (optional): Object containing read space configuration
- `readSpaces.repositories` (optional): Array of repository paths to include as read spaces
  - `path`: Absolute path to the repository
  - `alias` (optional): A friendly name for the repository (used in agent context)

#### Backward Compatibility

When `readSpaces` is absent or empty:
- Heuristic-request behaves exactly as it does today
- No additional `--add-dir` flags are passed
- No changes to working directory handling

This follows our auto-create paradigm: the feature exists when configured, but the absence of configuration yields default behavior identical to current functionality.

#### Implementation Changes to heuristic-request/main.go

1. **Parse readSpaces from HEURISTIC.json**: Extend the heuristic parsing to recognize the `readSpaces` field
2. **Pass additional --add-dir flags**: For each repository in `readSpaces.repositories`, add the corresponding `--add-dir` flag when invoking the agent
3. **No working directory changes yet**: Phase 1 keeps the current working directory behavior; we only add read access to additional paths

#### First Use-Case: AI-sandboxing Repository

The immediate goal is a heuristic that can examine the AI-sandboxing repository. Example HEURISTIC.json:

```json
{
  "heuristic": "review-codebase",
  "prompt": "Review the AI-sandboxing codebase structure and provide observations.",
  "readSpaces": {
    "repositories": [
      {
        "path": "/workspaces/workspace/sandbox/AI-sandboxing",
        "alias": "ai-sandboxing"
      }
    ]
  }
}
```

This allows the heuristic agent to read files from the AI-sandboxing repository while maintaining the prompt-only security model.

### Phase 2: Working Directory Isolation (Lightweight Mode)

**Goal**: Change the agent's working directory to a prepared directory structure that mirrors the target `/agent` layout, while still using the lightweight deployment approach.

**Approach**: Before invoking the agent, heuristic-request prepares a temporary directory structure:

```
/workspaces/slopspaces/agent-workspaces/{invocation-id}/
├── read/
│   └── {alias} -> {actual-path}   # Symbolic links to read spaces
└── write/
    └── primary/                    # Actual directory for writes
```

**Implementation:**
1. Create the workspace directory with the invocation ID
2. Create `read/` and `write/primary/` subdirectories
3. Create symbolic links in `read/` pointing to each read space path
4. Change to this directory before invoking the agent
5. Pass `--add-dir` for the workspace and each linked read space

### Phase 3: Write Spaces for Agent-Worker

**Goal**: Extend write spaces support to agent-worker for execute mode agents.

**Approach**: Similar to read spaces, add `writeSpaces` configuration to DISPATCH.json/INSTRUCTION.json:

```json
{
  "writeSpaces": {
    "primary": "/workspaces/slopspaces/working/{work-unit-id}",
    "additional": [
      {
        "path": "/workspaces/slopspaces/output/content",
        "alias": "output"
      }
    ]
  }
}
```

### Phase 4: Container Mode

**Goal**: Support full container isolation with mounted directories.

**Approach**: When running in container mode:
1. Mount read spaces read-only at `/agent/read/{alias}`
2. Mount write spaces read-write at `/agent/write/{alias}`
3. Set working directory to `/agent`
4. No `--add-dir` flags needed; the mount points define the boundaries

This phase requires container orchestration integration and is deferred until the lightweight mode is proven.

### Design Decisions

1. **Repository paths are absolute**: We use absolute paths to avoid ambiguity. The system running heuristic-request knows its filesystem layout.

2. **No glob patterns initially**: Keeping it simple with explicit paths. Glob support can be added later if needed.

3. **Aliases are optional but recommended**: They provide meaningful names in the agent's environment. When absent, a sanitized version of the path basename is used.

4. **Read-only by construction in Phase 1**: Since heuristic-request uses `-p` (prompt mode), the read spaces are inherently read-only regardless of what `--add-dir` technically allows.

5. **Symbolic links for lightweight mode**: Using symlinks allows us to present a clean directory structure to the agent without copying files, while maintaining the target environment layout.

6. **Phased rollout**: Each phase builds on the previous, allowing us to validate the approach incrementally before adding complexity.
