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

### Changes Required

#### 1. Enhanced Approval PR Content

Current approval PRs show the pending instruction and basic metadata. We need to:

**a) Create a human-readable sequence plan in the approval PR:**
```markdown
## Proposed Sequence

**Initial Setup:** Create project scaffold with README.md

**Execution Steps:**
1. (T+0m) Write chapter 1 of documentation in docs/CHAPTER1.md
2. (T+20m) Write chapter 2 of documentation in docs/CHAPTER2.md
3. (T+40m) Write chapter 3 of documentation in docs/CHAPTER3.md

**Total Duration:** ~60 minutes (3 steps @ 20 min intervals)

**Approval Action:**
- Merge this PR to approve and begin execution
- Close this PR to reject the sequence
```

**b) Include structured data for re-dispatch:**
The approval JSON already contains `pending_instruction`, but we need to ensure `sequence_commands` and `sequence_minutes_between` are preserved and re-emitted on approval.

#### 2. Approval Module Updates

File: `/agent-dispatch/modules/containment/approval/`

- Add `sequence_commands` and `sequence_minutes_between` to the approval request JSON
- Generate the human-readable sequence plan in PR body
- Ensure re-dispatch includes all sequence parameters

#### 3. Dispatcher Updates

File: `/agent-dispatch/main.go`

- When creating approval-gated dispatch for sequence-to-new-repo:
  - Pass through all sequence parameters
  - Format the approval PR body with the sequence plan
- When processing approved dispatch (from PR merge):
  - Extract and use preserved sequence parameters

#### 4. Testing the Flow

Create test cases:
1. Dispatch sequence-to-new-repo with approval required
2. Verify approval PR contains readable sequence plan
3. Merge approval PR
4. Verify re-dispatch includes all sequence commands
5. Verify sequence executes correctly

### Optional Enhancements

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

### The Heuristic-Request Stage

The heuristic-request stage receives high-level user requests and must:
1. Understand the intent
2. Determine if a sequence is appropriate
3. Construct the sequence of steps
4. Create the dispatch with approval required

### Changes Required

#### 1. Sequence Planning Capability

Heuristic-request needs logic to break down requests into sequences:

**Input:** "Write a comprehensive tutorial on building REST APIs"

**Output:**
```json
{
  "type": "sequence-to-new-repo",
  "instruction": "Create tutorial repository structure with outline",
  "sequence_commands": [
    "Write introduction and setup guide in docs/01-introduction.md",
    "Write endpoint design section in docs/02-endpoints.md",
    "Write authentication section in docs/03-auth.md",
    "Write testing section in docs/04-testing.md",
    "Write deployment section in docs/05-deployment.md"
  ],
  "sequence_minutes_between": 25,
  "skip_approval": false
}
```

#### 2. Heuristic Triggers

Define when heuristic-request should propose a sequence vs. a single dispatch:

**Sequence Indicators:**
- Request mentions "multi-part", "series", "chapters", "phases"
- Task is naturally decomposable (tutorial, documentation, refactoring)
- Estimated scope exceeds single-session capacity
- Request explicitly asks for staged/phased approach

**Single Dispatch Indicators:**
- Clearly bounded task
- Simple bug fix or feature
- User wants immediate execution

#### 3. Step Decomposition Strategy

How should heuristic-request break down tasks?

**Option A: Template-Based**
Predefined templates for common request types:
- Tutorial: intro → concepts → examples → advanced → conclusion
- Refactoring: analyze → plan → execute-phase-1 → execute-phase-2 → verify
- Documentation: overview → API-ref → guides → examples

**Option B: AI-Driven**
Use an AI agent to analyze the request and generate steps:
- More flexible but less predictable
- May produce inconsistent step granularity
- Requires validation/guardrails

**Option C: Hybrid**
Templates provide structure, AI fills in specifics:
- Template: "For documentation, use: overview → details → examples"
- AI: Determines what "details" means for this specific request

#### 4. Quality Guardrails

Ensure generated sequences are well-formed:

**Validation Rules:**
- Minimum 2 steps, maximum 20 steps
- Each step has clear, actionable instruction
- Steps have logical dependency ordering
- Total estimated time is reasonable
- No duplicate or redundant steps

**Human Review:**
- All auto-generated sequences require approval by default
- Approval PR shows sequence was auto-generated
- Reviewer can modify before approving

#### 5. Feedback Loop

Learn from approval outcomes:

**Track:**
- Which auto-generated sequences get approved vs. rejected
- Common modifications reviewers make
- Step execution success rates

**Improve:**
- Adjust heuristics based on approval rates
- Refine step decomposition strategies
- Update templates based on patterns

### Integration Points

```
User Request
     ↓
[Heuristic-Request Stage]
     ↓
Analyze request type and scope
     ↓
Is sequence appropriate?
     ├─ NO → Single dispatch
     └─ YES ↓
          Generate sequence steps
               ↓
          Construct dispatch JSON
               ↓
          Write DISPATCH.json to input directory
               ↓
[Existing Flow Takes Over]
     ↓
Approval PR created
     ↓
Human reviews auto-generated sequence
     ↓
Approve/Reject/Modify
     ↓
Execute or terminate
```

### Testing Strategy

1. **Unit Tests:** Step decomposition logic, validation rules
2. **Integration Tests:** End-to-end from request to approval PR
3. **Acceptance Tests:** Real requests produce sensible sequences
4. **A/B Testing:** Compare approval rates of different decomposition strategies

### Rollout Plan

**Phase 1: Opt-In**
- Add flag to enable auto-sequence generation
- Default off, users explicitly request sequences
- Gather feedback and metrics

**Phase 2: Suggested**
- Heuristic-request suggests sequences when appropriate
- User confirms before dispatch
- Refine based on suggestions accepted/rejected

**Phase 3: Automatic**
- Auto-generate sequences for appropriate requests
- Still requires human approval before execution
- Full feedback loop operational

---

## Summary

| Phase | Focus | Key Deliverable |
|-------|-------|-----------------|
| TODAY | Foundation | Working sequence-to-new-repo with approval |
| TOMORROW | Enhanced Proposals | Human-readable sequence plans in approval PRs |
| THE NEXT DAY | Automation | Heuristic-driven sequence generation |

The path forward builds incrementally: first we make sequences easy to review (TOMORROW), then we automate their creation (THE NEXT DAY). Each phase adds value independently while moving toward full automation.
