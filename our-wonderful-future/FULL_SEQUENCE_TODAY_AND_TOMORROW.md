# Full Sequence Flow: Today and Tomorrow

This document outlines our journey from the current sequence-to-new-repo implementation to a complete heuristic-driven dispatch system.

---

## TODAY

### How It Works

The sequence-to-new-repo flow currently operates with explicit enumeration of steps at dispatch time. Here's the complete flow:

#### 1. Dispatch Entry

A dispatch file (DISPATCH.json) is placed in the input directory with:
```json
{
  "type": "sequence-to-new-repo",
  "instruction": "Initial setup instruction",
  "sequence_commands": [
    "Step 1: Do the first thing",
    "Step 2: Do the second thing",
    "Step 3: Do the third thing"
  ],
  "sequence_minutes_between": 20,
  "skip_approval": false
}
```

#### 2. Approval Gate (Default)

Unless `skip_approval: true`, all dispatches go through approval:
- Creates a PR in the approval repo (sloppo by default)
- PR contains the pending instruction and all sequence commands
- Human reviews and either:
  - **Merges PR** → Approval granted → Re-dispatch with `skip_approval=true`
  - **Closes PR** → Rejection → Flow terminates

#### 3. Sequence Execution

After approval, `processSequenceToNewRepoDispatch()` handles execution:
1. Creates a unique repository: `seq-<timestamp>-<uuid>`
2. Initializes terraform in `/dispatcher/live/flows/sequence-to-new-repo/<flowID>/`
3. First terraform apply:
   - Creates the new repository
   - Creates PR #1 with the initial instruction
   - Captures `actual_start_time_millis`
4. Monitoring loop takes over with periodic terraform applies

#### 4. Time-Based Step Activation

Steps activate based on elapsed time:
- Step 1: Immediate (0 minutes elapsed)
- Step 2: After `minutes_between` (default 20)
- Step 3: After `2 * minutes_between` (40 minutes)
- And so on...

Each step:
1. Clones the repo at the current branch HEAD
2. Dispatches the instruction to an AI agent
3. Waits for completion
4. Pushes results to the branch

#### 5. Completion & Cleanup

When all steps complete AND the PR is merged/closed:
- `performFlowCleanup()` removes terraform configs
- Flow record updated to status: "completed"

### Key Components

| Component | Location |
|-----------|----------|
| Main dispatcher | `/agent-dispatch/main.go` |
| Approval module | `/agent-dispatch/modules/containment/approval/` |
| To-repo module | `/agent-dispatch/modules/containment/to-repo/` |
| Sequence module | `/agent-dispatch/modules/execution/sequence/` |
| Single execution | `/agent-dispatch/modules/execution/sequence/single/` |

### Current Limitations

1. **All-or-Nothing Approval**: Cannot approve/reject individual steps - the entire sequence must be approved or rejected

2. **Static Step Definition**: Sequence commands must be fully enumerated at dispatch time - no dynamic step generation

3. **No Mid-Sequence Modification**: Once approved and running, steps cannot be added, removed, or modified

4. **Known Terraform Count Bug**: First apply has "count cannot be determined" issues when count depends on values that aren't known until apply time. Workarounds exist but add complexity.

5. **No Step-Level Feedback**: Human cannot provide guidance between steps without external intervention (REVISE: comments work but weren't designed for inter-step feedback)

6. **Timing Precision**: Steps may execute slightly off-schedule depending on when the monitoring loop runs its terraform apply

---

## TOMORROW

### Goal

Support **proposed sequences** where:
1. Steps are explicitly enumerated in a proposal
2. Human reviews and approves the entire sequence before execution begins
3. Upon approval, the sequence executes as it does today

This is an evolutionary step - we enhance the approval experience without changing execution.

### Backward Compatibility Strategy

The existing `sequence-to-new-repo` flow works today with `skip_approval: true`. Our changes must:
1. **NOT break existing flows** - all current variables remain optional or have defaults
2. **Be additive only** - new variables are optional; existing dispatches without them work unchanged
3. **Preserve data through the cycle** - approval → merge → re-dispatch must not lose sequence parameters

### Changes Required

#### 1. Approval Module Variables (`variables.tf`)

Add two new **optional** variables with defaults that maintain backward compatibility:

```hcl
variable "sequence_commands" {
  description = "For sequence-to-new-repo: list of commands to execute in sequence"
  type        = list(string)
  default     = []  # Empty list = not a sequence dispatch
}

variable "sequence_minutes_between" {
  description = "For sequence-to-new-repo: minutes between steps"
  type        = number
  default     = 20  # Sensible default
}
```

**Why this is safe:** Existing callers don't pass these, so they get empty list / default. No breaking changes.

#### 2. Approval Module Locals (`main.tf`)

Add computed locals to detect if this is a sequence dispatch and build the display content:

```hcl
locals {
  # ... existing locals ...

  # Detect if this is a sequence dispatch (has commands)
  is_sequence = length(var.sequence_commands) > 0

  # Pre-compute sequence metadata for display
  sequence_total_steps = length(var.sequence_commands)
  sequence_total_duration_mins = local.sequence_total_steps * var.sequence_minutes_between

  # Build the numbered step list for PR body
  # Format: "1. (T+0m) First command\n2. (T+20m) Second command\n..."
  sequence_step_list = join("\n", [
    for i, cmd in var.sequence_commands :
    "${i + 1}. (T+${i * var.sequence_minutes_between}m) ${cmd}"
  ])
}
```

#### 3. Approval File Content (JSON in the request file)

Extend `approval_file_content` to include sequence data **conditionally**:

```hcl
locals {
  # Base approval data (always included)
  _approval_base = {
    type                = "approval-request"
    dispatcher_name     = var.dispatcher_name
    source_context      = var.source_context
    pending_instruction = var.pending_instruction
    pending_mode        = var.pending_mode
    pending_agent       = var.pending_agent
    request_time        = time_static.dispatch_time.rfc3339
    metadata            = jsondecode(var.metadata_json)
  }

  # Sequence-specific data (only included when is_sequence = true)
  _approval_sequence = local.is_sequence ? {
    sequence_commands        = var.sequence_commands
    sequence_minutes_between = var.sequence_minutes_between
  } : {}

  # Merged result
  approval_file_content = jsonencode(merge(local._approval_base, local._approval_sequence))
}
```

**Why separate locals:** Terraform's ternary operator struggles with complex inline expressions (especially heredocs). Breaking into separate locals avoids parser ambiguity.

#### 4. PR Body Content

##### ⚠️ CRITICAL PITFALL: Terraform Heredoc + Ternary

**DO NOT** write code like this:
```hcl
# BAD - Terraform parser cannot handle heredoc as ternary branch
sequence_plan = local.is_sequence ? <<-EOT
  some content
EOT : ""
```

The Terraform parser sees the first `:` in the heredoc content and thinks it's the ternary separator, causing "Missing false expression in conditional" errors.

**SOLUTION:** Pre-compute content in separate locals, then use simple string interpolation:

```hcl
locals {
  # Pre-compute the sequence plan as a separate local (no ternary)
  _sequence_plan_content = <<-EOT
### Proposed Sequence

**Execution Steps:**
${local.sequence_step_list}

**Timing:** ${var.sequence_minutes_between} minutes between steps
**Total Duration:** ~${local.sequence_total_duration_mins} minutes (${local.sequence_total_steps} steps)
EOT

  # Now use simple ternary with pre-computed strings
  sequence_plan_markdown = local.is_sequence ? local._sequence_plan_content : ""
}
```

##### PR Body Template

```hcl
resource "github_repository_pull_request" "approval_pr" {
  title = "Approval: ${var.dispatcher_name} (${local.unix_timestamp})"
  body  = <<-EOT
## Approval Request

**Source:** ${var.source_context}
**Dispatcher:** ${var.dispatcher_name}
**Requested at:** ${time_static.dispatch_time.rfc3339}

### ${local.is_sequence ? "Initial Setup" : "Pending Instruction"}

```
${var.pending_instruction}
```

${local.sequence_plan_markdown}

### Details

- **Mode:** ${var.pending_mode}
${var.pending_agent != "" ? "- **Agent:** ${var.pending_agent}" : ""}

---

**Merge this PR to approve and ${local.is_sequence ? "begin sequence execution" : "execute the instruction"}.**
**Close this PR to reject the request.**
EOT
  # ... other fields
}
```

#### 5. Dispatcher Updates (`main.go`)

##### 5a. Pass sequence parameters when creating approval terraform config

In `createApprovalTerraformConfig()`, add the sequence parameters to the generated HCL:

```go
// In the function that generates approval terraform config
if len(dispatch.SequenceCommands) > 0 {
    // Build HCL list literal for sequence_commands
    cmdItems := make([]string, len(dispatch.SequenceCommands))
    for i, cmd := range dispatch.SequenceCommands {
        cmdItems[i] = fmt.Sprintf("  %q", cmd)
    }
    sequenceCommandsHCL := "[\n" + strings.Join(cmdItems, ",\n") + "\n]"

    tfConfig += fmt.Sprintf(`
sequence_commands        = %s
sequence_minutes_between = %d
`, sequenceCommandsHCL, dispatch.SequenceMinutesBetween)
}
```

##### 5b. Extract sequence parameters from approved dispatch

When processing a merged approval PR, the dispatcher reads the approval JSON file. Ensure sequence parameters are extracted:

```go
// In processApprovedDispatch or similar
type ApprovalRequest struct {
    Type                    string   `json:"type"`
    DispatcherName          string   `json:"dispatcher_name"`
    PendingInstruction      string   `json:"pending_instruction"`
    PendingMode             string   `json:"pending_mode"`
    PendingAgent            string   `json:"pending_agent"`
    SequenceCommands        []string `json:"sequence_commands,omitempty"`
    SequenceMinutesBetween  int      `json:"sequence_minutes_between,omitempty"`
    // ... other fields
}

// When re-dispatching:
newDispatch := DispatchRequest{
    Type:                   inferTypeFromContext(), // or stored in approval
    Instruction:            approval.PendingInstruction,
    Mode:                   approval.PendingMode,
    SequenceCommands:       approval.SequenceCommands,
    SequenceMinutesBetween: approval.SequenceMinutesBetween,
    SkipApproval:           true, // Already approved
}
```

#### 6. Testing the Flow

**Manual Test Procedure:**

1. Create a DISPATCH.json with sequence-to-new-repo and `skip_approval: false`:
   ```json
   {
     "type": "sequence-to-new-repo",
     "instruction": "Create tutorial project structure",
     "sequence_commands": [
       "Write chapter 1 in docs/ch1.md",
       "Write chapter 2 in docs/ch2.md"
     ],
     "sequence_minutes_between": 5,
     "skip_approval": false
   }
   ```

2. Place in `slopspaces/input/any/<folder>/`

3. Verify approval PR is created with:
   - "Initial Setup" header (not "Pending Instruction")
   - Numbered step list with timing
   - Total duration displayed
   - Merge button text says "begin sequence execution"

4. Check the approval JSON file in the PR contains:
   - `sequence_commands` array
   - `sequence_minutes_between` value

5. Merge the approval PR

6. Verify re-dispatch occurs with:
   - All sequence commands preserved
   - `skip_approval: true` set

7. Verify sequence executes correctly with time-based steps

**Automated Test Cases (future):**

- `TestApprovalPRBodyContainsSequencePlan`
- `TestApprovalJSONPreservesSequenceCommands`
- `TestApprovedDispatchReDispatchesWithSequence`
- `TestNonSequenceDispatchUnchanged` (backward compatibility)

### Summary of Files to Modify

| File | Changes |
|------|---------|
| `modules/containment/approval/variables.tf` | Add `sequence_commands` (list), `sequence_minutes_between` (number) with defaults |
| `modules/containment/approval/main.tf` | Add locals for is_sequence detection, step list formatting, PR body generation |
| `agent-dispatch/main.go` | Pass sequence params to approval TF config; extract from approved dispatch |

### Known Pitfalls to Avoid

1. **Heredoc in ternary**: Never put heredoc directly in ternary branches - use separate locals
2. **Count depends on unknown**: Don't use `count = length(something_from_apply)` - use data source queries with a-priori known values
3. **Missing defaults**: Always provide defaults for new variables to maintain backward compatibility
4. **JSON encoding edge cases**: Use `jsonencode()` for complex structures, not string interpolation

### Optional Enhancements (Deferred)

**A. Step-Level Comments in Approval PR**
Allow reviewers to add comments on individual steps that get passed to the agent when that step executes:
```markdown
### Step 2: Write chapter 2
> Reviewer note: Focus on practical examples, not theory
```

**B. Timing Adjustments**
Allow reviewers to modify `sequence_minutes_between` via a comment before merging:
```
TIMING: 30
```

**C. Step Reordering/Removal**
More complex - allow reviewers to modify the sequence before approval. This requires parsing modified step lists from the PR.

---

## THE NEXT DAY

### Goal

Enable the **heuristic-request** stage to automatically construct and dispatch sequence proposals. This closes the loop from user intent to automated multi-step execution.

### Current State of Heuristic-Request

The heuristic-request watcher (`heuristic-request/main.go`) currently:
- Watches `HEURISTIC_DIR/pending/` for folders containing `HEURISTIC.md`
- Invokes an AI agent in **prompt-only mode** (`-p` flag)
- Extracts `DISPATCH.json` or `INSTRUCTION.json` from the agent's output
- Places the extracted file in `REQUEST_DIR` for agent-dispatch to pick up

**Key Insight:** The prompt template (`buildHeuristicPrompt()`) currently documents these dispatch types:
- `repo-isolation`
- `in-repo`
- `direct`
- `INSTRUCTION.json` (simple prompts)

**Missing:** The `sequence-to-new-repo` type is NOT in the prompt. The AI agent doesn't know it exists.

### Changes Required

#### 1. Update Heuristic Prompt Template

File: `heuristic-request/main.go`, function `buildHeuristicPrompt()`

Add the `sequence-to-new-repo` type documentation:

```go
func (w *HeuristicWatcher) buildHeuristicPrompt(heuristicContent string) string {
    return fmt.Sprintf(`You are a heuristic processor...

## Dispatch Types (Execution Patterns)

### repo-isolation
... existing ...

### in-repo
... existing ...

### direct
... existing ...

### sequence-to-new-repo
For multi-step tasks that should be executed as a series of timed steps in a NEW repository.
Creates a fresh repo and executes steps sequentially with configurable delays between them.
Use when:
- Writing multi-chapter documentation or tutorials
- Implementing features in phases
- Tasks that naturally decompose into ordered steps
- Creating content that builds on previous steps

%sjson DISPATCH.json
{
  "type": "sequence-to-new-repo",
  "instruction": "Initial setup instruction (creates repo structure)",
  "sequence_commands": [
    "Step 1: First action to perform",
    "Step 2: Second action (builds on step 1)",
    "Step 3: Third action (builds on previous steps)"
  ],
  "sequence_minutes_between": 20,
  "mode": "execute"
}
%s

**Guidelines for sequence-to-new-repo:**
- Use when the request mentions: "chapters", "phases", "series", "step-by-step", "tutorial", "guide"
- First instruction creates the repo structure (README, folder layout)
- Each sequence_command is a discrete, self-contained step
- Steps should be logically ordered - later steps may reference earlier work
- 20 minutes between steps is the default; adjust based on complexity
- Keep steps focused: 3-10 steps is typical, max 20

... rest of existing prompt ...
`, "` + "`" + "`" + "`", "` + "`" + "`")
}
```

#### 2. Update Guidelines Section

Add explicit guidance for choosing `sequence-to-new-repo`:

```go
## Guidelines

- **repo-isolation**: Default choice for modifying existing repos
- **in-repo**: Use for quick fixes where isolation overhead isn't warranted
- **direct**: Use for local tasks, reports, analysis that don't modify external repos
- **sequence-to-new-repo**: Use for multi-part content creation (tutorials, documentation, phased implementations)
- **INSTRUCTION**: Use for very simple prompts that don't need any orchestration
- Infer repo owner when not specified: "agent-events" → "je-sidestuff/agent-events"
- All dispatches go through approval automatically - you don't need to specify approval

**Sequence detection keywords:** chapters, phases, series, step-by-step, tutorial, guide, multi-part, staged
```

#### 3. Validation in agent-dispatch

File: `agent-dispatch/main.go`

Add validation when processing `sequence-to-new-repo` dispatches:

```go
func (d *Dispatcher) validateSequenceDispatch(dispatch *DispatchRequest) error {
    if dispatch.Type != DispatchTypeSequenceToNewRepo {
        return nil // Not a sequence, nothing to validate
    }

    // Must have at least one command
    if len(dispatch.SequenceCommands) == 0 {
        return fmt.Errorf("sequence-to-new-repo requires at least one command in sequence_commands")
    }

    // Cap at 100 steps (already in code, verify)
    if len(dispatch.SequenceCommands) > 100 {
        return fmt.Errorf("sequence-to-new-repo limited to 100 steps, got %d", len(dispatch.SequenceCommands))
    }

    // Validate no empty commands
    for i, cmd := range dispatch.SequenceCommands {
        if strings.TrimSpace(cmd) == "" {
            return fmt.Errorf("sequence_commands[%d] is empty", i)
        }
    }

    // Ensure minutes_between is reasonable
    if dispatch.SequenceMinutesBetween <= 0 {
        dispatch.SequenceMinutesBetween = 20 // Default
    }
    if dispatch.SequenceMinutesBetween > 1440 { // 24 hours max
        return fmt.Errorf("sequence_minutes_between must be <= 1440 (24 hours), got %d", dispatch.SequenceMinutesBetween)
    }

    return nil
}
```

### Heuristic Triggers (AI Prompt Guidance)

The AI agent itself decides when to use `sequence-to-new-repo` based on the prompt guidance. Key triggers:

**Sequence Indicators (include in prompt):**
- Request mentions "multi-part", "series", "chapters", "phases"
- Task is naturally decomposable (tutorial, documentation, refactoring)
- Request explicitly asks for staged/phased approach
- Keywords: "step-by-step", "tutorial", "guide", "documentation"

**Single Dispatch Indicators (also include):**
- Clearly bounded task ("fix this bug", "add this button")
- Simple feature addition
- Modification to existing repo (use `repo-isolation` instead)

### Step Decomposition Strategy

The AI agent does the decomposition based on the heuristic content. This is **Option B: AI-Driven** from the original plan.

**Guardrails via prompt guidance:**
- Suggest 3-10 steps as typical range
- Each step should be self-contained
- Steps should build logically on previous work
- First step creates structure, later steps add content

**Guardrails via validation (agent-dispatch):**
- Min 1 step (already enforced)
- Max 100 steps (already enforced)
- No empty steps
- Reasonable timing (1-1440 minutes)

### Testing the Flow

**Manual Test - Heuristic to Sequence:**

1. Create `HEURISTIC.md`:
   ```markdown
   # Tutorial Request

   Write a comprehensive guide on building CLI applications in Go.

   The guide should have chapters covering:
   1. Introduction and project setup
   2. Argument parsing with cobra
   3. Configuration management
   4. Interactive prompts and user input
   5. Testing CLI commands

   This should be structured for step-by-step learning.
   ```

2. Place in `slopspaces/heuristic/pending/cli-tutorial/HEURISTIC.md`

3. Wait for heuristic-request to process

4. Verify output in `slopspaces/input/any/<generated-id>/DISPATCH.json`:
   - Should have `type: "sequence-to-new-repo"`
   - Should have `sequence_commands` array with ~5 steps
   - Should have `sequence_minutes_between` set

5. Verify agent-dispatch creates approval PR with sequence plan visible

**Test Cases:**

| Input HEURISTIC.md | Expected Dispatch Type |
|--------------------|----------------------|
| "Add email handling to agent-events" | `repo-isolation` |
| "Write a 5-chapter tutorial on React" | `sequence-to-new-repo` |
| "Fix typo in README" | `direct` or `in-repo` |
| "Create a phased migration plan for database" | `sequence-to-new-repo` |
| "Generate a report on code coverage" | `direct` |

### Integration Flow

```
HEURISTIC.md dropped in pending/
           ↓
[heuristic-request watcher]
           ↓
Invoke AI agent with updated prompt
(prompt now includes sequence-to-new-repo docs)
           ↓
AI analyzes and outputs DISPATCH.json
           ↓
Extract to slopspaces/input/any/
           ↓
[agent-dispatch watcher]
           ↓
Validate dispatch (including sequence validation)
           ↓
Route to approval (skip_approval defaults to false)
           ↓
[approval module] ← TOMORROW's changes make this show sequence plan
           ↓
Human reviews sequence in PR
           ↓
Merge → re-dispatch with skip_approval=true
           ↓
[sequence-to-new-repo handler]
           ↓
Create repo, execute steps with timing
```

### Summary of Files to Modify

| File | Changes |
|------|---------|
| `heuristic-request/main.go` | Add `sequence-to-new-repo` to prompt template, add keyword guidance |
| `agent-dispatch/main.go` | Add/verify sequence validation in dispatch processing |

### Rollout Plan

**Phase 1: Prompt Update Only**
- Update `buildHeuristicPrompt()` to include `sequence-to-new-repo`
- Let AI naturally choose when appropriate
- Monitor what it generates

**Phase 2: Tuning**
- Adjust prompt guidance based on observed outputs
- Add more specific keyword triggers if AI misses obvious cases
- Refine step count guidance if AI over/under-decomposes

**Phase 3: Feedback**
- Track approval rates of auto-generated sequences
- Identify patterns in rejected/modified sequences
- Update prompt based on learnings

---

## Summary

| Phase | Focus | Key Deliverable | Files Modified |
|-------|-------|-----------------|----------------|
| TODAY | Foundation | Working sequence-to-new-repo with approval | (existing) |
| TOMORROW | Enhanced Proposals | Human-readable sequence plans in approval PRs | `approval/variables.tf`, `approval/main.tf`, `main.go` |
| THE NEXT DAY | Automation | Heuristic-driven sequence generation | `heuristic-request/main.go`, `main.go` (validation) |

### Implementation Order

1. **TOMORROW first** - Makes sequences visible in approval PRs
   - Required for human review of auto-generated sequences
   - Backward compatible (new variables have defaults)
   - Low risk - only affects PR display and JSON storage

2. **THE NEXT DAY second** - Enables AI to propose sequences
   - Depends on TOMORROW for human review experience
   - Prompt change only in heuristic-request
   - Validation already mostly exists in agent-dispatch

### Key Pitfalls Documented

| Pitfall | Where | Solution |
|---------|-------|----------|
| Heredoc in ternary | Terraform locals | Pre-compute in separate local, then simple ternary |
| Count depends on unknown | Module resources | Use data sources with a-priori known query values |
| Missing backward compat | New variables | Always provide defaults for new optional vars |
| AI not knowing about type | Heuristic prompt | Add `sequence-to-new-repo` docs to `buildHeuristicPrompt()` |

The path forward builds incrementally: first we make sequences easy to review (TOMORROW), then we automate their creation (THE NEXT DAY). Each phase adds value independently while moving toward full automation.
