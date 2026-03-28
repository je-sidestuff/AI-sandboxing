# ERR Implementation Strategies
## Evolutionary Reconstitutional Render — Dispatch Framework Proposals

---

## Background

The **Evolutionary Reconstitutional Render (ERR)** is an AI execution strategy for transforming functional content through three sequential phases:

1. **Decomposition** — Analyze source content and isolate its minimum viable functional units into *atoms* (no dependencies) and *molecules* (has dependencies). The content is now in an *atomized state*.
2. **Distillation** — Subject each particle to verification and improvement individually, producing the *distilled state*. Optionally, one or more particles may undergo *turbulence*: iterated man-in-the-loop feedback before being finalized.
3. **Recomposition** — Reassemble the distilled particles back into a coherent whole, optionally passing through *intermediate molecules*. At any recomposition stage, *lensing* (instruction-driven verification/improvement) or *turbulence* (man-in-the-loop review) may be applied. When complete, the content is *recrystallized*.

This document proposes two implementation strategies for wiring ERR into the existing dispatch/approval framework, which supports four dispatch types:

| Dispatch Type | Description |
|---|---|
| `direct` | Fire-and-forget; transforms to `INSTRUCTION.json` for agent-worker pickup |
| `in-repo` | PR-based containment; agent works on a branch in the target repository |
| `repo-isolation` | Fully isolated private repo; maximum containment, with optional reintegration PR |
| `approval` | PR-based approval gate; merging the PR approves and executes the pending dispatch |

---

## Strategy 1 — Option A: Direct Path (Single-Approval Fan-Out)

### Overview

Option A minimizes friction. A **single approval gate** kicks off the entire ERR pipeline. At the moment of approval, the full set of phase tasks is decomposed and queued as discrete async work units that run to completion without further human intervention.

This strategy is appropriate when the ERR transformation is well-understood and trusted enough to run end-to-end without checkpoints.

### Execution Flow

```
[Operator creates approval dispatch]
        │
        ▼  GATE: Approval PR opened in approval repo
[Human reviews & merges PR]  ←── single approval
        │
        ▼  Approval conclusion: "merged"
[ERR Orchestrator dispatched as direct async task]
        │
        ├─ Phase 1: Decomposition Task (direct, async)
        │         └── Analyzes source content
        │         └── Produces atom/molecule manifest
        │
        ├─ Phase 2: Distillation Tasks (direct, async, one per particle)
        │         └── Queued after decomposition completes
        │         └── Each particle distilled independently
        │         └── All particles run concurrently
        │
        └─ Phase 3: Recomposition Task (direct, async)
                  └── Queued after all distillation tasks complete
                  └── Reassembles distilled particles
                  └── Produces recrystallized output
```

### Wiring into the Dispatch/Approval Framework

#### Step 1 — Create the Approval-Gated Entry Point

Issue an `approval` dispatch whose `pending_dispatch` is the ERR orchestrator instruction:

```json
{
  "type": "approval",
  "instruction": "Execute the Evolutionary Reconstitutional Render (ERR) strategy on <target content>.",
  "source_context": "err-direct-path",
  "pending_dispatch": {
    "type": "direct",
    "instruction": "You are the ERR orchestrator. Execute the full ERR pipeline on <target content>: (1) Decompose into atoms/molecules, (2) Distill each particle, (3) Recompose into recrystallized output. Use agent-dispatch --async for each phase task and coordinate sequencing.",
    "mode": "execute",
    "agent": "err-orchestrator"
  }
}
```

The `approval` module creates a PR in the approval repository. Merging this PR triggers the `pending_dispatch`.

#### Step 2 — ERR Orchestrator Spawns Phase Tasks

Upon approval, the orchestrator agent runs as a `direct` dispatch and is responsible for spawning and sequencing the three phase tasks using `agent-dispatch --once --async`:

```
# Phase 1: Decomposition
DECOMP_ID=$(agent-dispatch --once --async -i "Decompose <target content> into ERR atoms and molecules. Output manifest to <output path>." -m execute)

# Wait for decomposition to complete
agent-dispatch --once --wait "$DECOMP_ID"

# Phase 2: Distillation — queue one task per particle from the manifest
for PARTICLE in $(read_manifest <output path>); do
  agent-dispatch --once --async -i "Distill ERR particle: $PARTICLE" -m execute
done
# ... wait for all distillation tasks ...

# Phase 3: Recomposition
agent-dispatch --once --async -i "Recompose ERR distilled particles from <distilled state path> into recrystallized output." -m execute
```

#### Step 3 — Async Invocation Requirement

The approval dispatch **must not be triggered directly from a shell command or foreground process**. It should be placed as a `DISPATCH.json` in the watched input directory (`INPUT_DIR/any/<dispatch-id>/DISPATCH.json`) and picked up by the agent-dispatch watcher:

```bash
# Write the dispatch file — do NOT use agent-dispatch --once
mkdir -p "$INPUT_DIR/any/err-direct-$(date +%s)"
cat > "$INPUT_DIR/any/err-direct-$(date +%s)/DISPATCH.json" << 'EOF'
{
  "type": "approval",
  "instruction": "Execute ERR on <target>",
  ...
}
EOF
```

The watcher processes this asynchronously without blocking any foreground process, satisfying the async invocation requirement.

### Phase Task Summary

| Phase | Dispatch Type | Async | Triggers After |
|---|---|---|---|
| ERR Orchestrator | `direct` | yes | Approval PR merged |
| Decomposition | `direct` | yes | Orchestrator start |
| Distillation (×N) | `direct` | yes | Decomposition complete |
| Recomposition | `direct` | yes | All distillation tasks complete |

### Approval Gate Count

**One gate only** — the initial approval PR. All downstream phase tasks run with `skip_approval: true` (implicit for `direct` dispatches) and require no further human interaction.

---

## Strategy 2 — Option B: Comprehensive Path (Multi-Gate, Revisable-in-Transit)

### Overview

Option B takes **full advantage of the approval mechanisms** already present in the framework. Each major ERR phase transition and key decision point surfaces an optional approval gate, enabling man-in-the-loop turbulence throughout execution.

Approvals along the way can:
- **Redirect** — modify the instruction before the next phase proceeds
- **Prune** — remove specific particles from the distillation set
- **Expand** — add new particles or apply lensing at intermediate stages
- **Select variations** — choose granularity (atoms vs molecules), distillation strategy, or recomposition approach

This strategy is appropriate when the ERR transformation touches high-stakes content, when early phases may surface unexpected structure, or when iterative human guidance improves the final quality.

### Execution Flow

```
[Operator creates approval dispatch for full ERR pipeline]
        │
        ▼  GATE 1: Pre-execution approval
[Approve ERR on this content?]
[Optional: specify molecule vs atom granularity preference]
        │ merged
        ▼
[Phase 1: Decomposition executes (in-repo or repo-isolation)]
        │
        ▼  GATE 2 (optional): Post-decomposition review
[Approve atom/molecule manifest?]
[Options: adjust granularity, merge atoms, split molecules, prune]
        │ merged
        ▼
[Phase 2: Distillation — each particle dispatched]
        │
        ├── GATE 3a (optional, per-particle): Turbulence gate
        │   [Approve distilled particle X?]
        │   [Options: iterate, accept, skip]
        │   └── merged → particle accepted
        │
        ▼  All particles distilled
        │
        ▼  GATE 4 (optional): Pre-recomposition review
[Review distilled state. Choose recomposition strategy:]
[Options: linear, molecule-first, lensed-intermediates]
        │ merged
        ▼
[Phase 3: Recomposition executes]
        │
        ├── GATE 5a (optional): Intermediate molecule review
        │   [Approve intermediate molecule M before continuing?]
        │   [Options: apply lensing, continue, revise]
        │
        ▼  Full recomposition reached
        │
        ▼  GATE 6 (optional): Final lensing gate
[Apply final lensing before recrystallization?]
[Specify lensing instructions or approve as-is]
        │ merged
        ▼
[Recrystallized output produced]
```

### Leveraging Existing Approval Flow Hooks

#### Gate 1 — Pre-Execution Approval (existing `approval` dispatch)

Use the standard `approval` dispatch type with `source_context` describing the content and ERR intent. The approval PR body (auto-generated by the `approval` terraform module) already surfaces `pending_instruction`, `pending_mode`, and `pending_agent`. No new infrastructure needed.

The `metadata` field can carry operator preferences (granularity hints, lensing flags) that downstream phases read from the approval request file.

#### Gate 2 — Post-Decomposition Review (`approval` dispatch, triggered by orchestrator)

After Phase 1 completes, the orchestrator dispatches a **new `approval`** whose pending instruction is the distillation phase. The approval PR body includes the decomposed manifest, giving the operator a chance to:

- Accept the manifest as-is (merge PR)
- Comment on the PR to signal modifications — comments are polled by `prpoller` and can trigger a terraform re-apply that updates the pending instruction before the distillation dispatch is queued
- Close the PR to abort the pipeline

The `prpoller` integration (already present in the dispatch watcher via `PRRegistration.OnChange`) handles comment-driven updates without any new tooling.

#### Gate 3a — Per-Particle Turbulence (`approval` dispatch, one per particle)

Each particle distillation can optionally be surfaced as an `approval` dispatch rather than a `direct` dispatch. This is controlled by a flag in the decomposition manifest or the Gate 2 approval metadata:

```json
{
  "type": "approval",
  "instruction": "Distill ERR particle: <particle-id> — <particle content>",
  "pending_dispatch": {
    "type": "direct",
    "instruction": "Accept distilled particle <particle-id> and continue pipeline."
  },
  "metadata": { "err_phase": "distillation", "particle_id": "<id>" }
}
```

This is most useful for high-value molecules (those with many dependencies), while atoms can be dispatched as `direct` tasks without gating.

#### Gate 4 — Pre-Recomposition Review and Strategy Selection (`approval` dispatch)

The approval PR for this gate presents:
- The complete distilled state (list of verified particles)
- Recomposition strategy options encoded in the PR description

The operator selects a strategy by commenting on the PR (e.g., `strategy: molecule-first` or `strategy: linear`). The `prpoller` detects the comment, and the `TerraformAction` callback updates the `pending_dispatch` accordingly before approval is recorded.

New optional gate vs. existing hook: this gate uses the **existing `approval` module** without modification, but requires the orchestrator to construct the pending dispatch dynamically based on the comment payload parsed from `prpoller.ChangeEvent.NewComments`.

#### Gate 5a — Intermediate Molecule Review (`approval` dispatch, optional per molecule)

During recomposition, when an intermediate molecule is assembled, the orchestrator can optionally surface it for review. The operator can:
- Apply lensing (specify an instruction to be applied to the intermediate molecule before continuing)
- Approve and continue
- Redirect (replace the intermediate molecule with a revised version)

This leverages the standard `approval` PR + comment flow.

#### Gate 6 — Final Lensing Gate (`approval` or `in-repo` dispatch)

Before declaring the output *recrystallized*, an optional final approval surfaces the fully recomposed result. If lensing is requested:

- Use `in-repo` dispatch to apply the lensing instruction as a PR against the recomposed output in the target repository — the PR diff itself is the lensing artifact, reviewable before merge
- Or use `approval` with the pending instruction being the lensing operation followed by recrystallization declaration

The `in-repo` dispatch type is particularly well-suited here because lensing at the recomposed stage benefits from the full PR review workflow (diff view, comments, CI checks).

### Where New Optional Gates Could Be Inserted

| Gate | Phase | Dispatch Hook | Existing or New |
|---|---|---|---|
| Gate 1 | Pre-execution | `approval` dispatch | Existing |
| Gate 2 | Post-decomposition | `approval` dispatch (orchestrator-issued) | Existing hook, new usage |
| Gate 3a | Per-particle distillation | `approval` dispatch (per-particle, optional) | Existing hook, new usage |
| Gate 3b | Batch distillation review | `approval` dispatch (batch manifest review) | Existing hook, new usage |
| Gate 4 | Pre-recomposition | `approval` + `prpoller` comment parsing | Existing hooks, new orchestrator logic |
| Gate 5a | Intermediate molecule review | `approval` dispatch (optional per molecule) | Existing hook, new usage |
| Gate 6 | Final lensing | `in-repo` or `approval` dispatch | Existing |

### Granularity and Variation Options Per Stage

| Stage | Variation Option | Mechanism |
|---|---|---|
| Decomposition | Atom-only vs. molecule-aware | Gate 1 `metadata` field carries preference |
| Distillation | Per-particle turbulence on/off | Gate 2 manifest flags per particle |
| Distillation | Molecule vs. atom granularity for distillation order | Gate 2 approval comment |
| Recomposition | Linear / molecule-first / lensed-intermediates | Gate 4 approval comment parsed by `prpoller` |
| Recomposition | Lensing instructions per intermediate stage | Gate 5a approval `pending_instruction` |
| Final | Lensing on/off, lensing instruction | Gate 6 approval body |

---

## Comparison

| Dimension | Option A (Direct Path) | Option B (Comprehensive Path) |
|---|---|---|
| Approval gates | 1 | 1–6 (operator-configurable) |
| Human intervention after kickoff | None | At each optional gate |
| Dispatch types used | `approval`, `direct` | `approval`, `direct`, `in-repo` |
| Turbulence support | No | Yes (Gate 3a, Gate 5a) |
| Lensing support | No | Yes (Gate 5a, Gate 6) |
| Granularity choices | Fixed at invocation | Revisable at Gate 2 and Gate 4 |
| Pipeline revisability | None after approval | Redirect/prune/expand at any gate |
| Async invocation | Required (via DISPATCH.json watcher) | Required (same watcher mechanism) |
| Suitable for | Trusted, well-understood content | High-stakes, exploratory, or iterative work |

---

## Implementation Notes

### Async Invocation (Both Options)

Both strategies require that the entry-point `approval` dispatch be placed as a `DISPATCH.json` file into the watched input directory, **not** invoked via `agent-dispatch --once` from a foreground shell. This ensures the dispatch watcher picks it up asynchronously and the terraform lifecycle is managed by the running watcher daemon.

### Orchestrator Agent

Both options rely on an ERR orchestrator agent that understands how to:
1. Read and produce ERR manifests (atomized state, distilled state)
2. Issue downstream dispatches using `agent-dispatch --once --async`
3. (Option B) Construct `approval` dispatch files dynamically with phase-appropriate pending instructions

### `skip_approval` Usage

For `direct` phase tasks in both options, `skip_approval` is implicitly `true` (direct dispatches do not go through the approval module). For Option A, the pending dispatch after the single approval gate carries `skip_approval: false` only at the top-level approval — all child tasks are `direct` and bypass the approval module by design.

### PR Comment Turbulence (Option B)

Option B leverages the existing `prpoller` infrastructure in the dispatch watcher. The `PRRegistration.OnChange` callback and optional `TerraformAction` callback are the primary hooks for responding to operator comments mid-execution. No changes to the approval terraform module are required — only orchestrator-level logic to construct updated `pending_dispatch` payloads from comment content.
