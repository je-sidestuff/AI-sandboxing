# Idiomatizing

## Overview

Idiomatizing is an AI execution strategy for transforming target content toward an idiomatic state defined by an **archetype** — a hypothetical ideal that represents what the content should look and behave like when done right.

The process measures the gap between the current state of the target and the archetype, produces a plan to close that gap, and executes it. If the gap is already small, the process may terminate early.

---

## Terminology

| Term | Definition |
|------|------------|
| **Archetype** | A hypothetical idiomatic state used as the transformation target |
| **Example** | An impression-based archetype input — a reference artifact that embodies the desired idiom |
| **Explanation** | A description-based archetype input — a resource that describes the desired idiom |
| **Error signal** | An accounting of how the target content differs from the archetype |
| **Reconciliation plan** | A proposed change set that would make the target more like the archetype |
| **Early termination** | Stopping the process when the error signal is sufficiently small |
| **Quantification** | Optional measurement applied to make error signal generation more deterministic and results more verifiable |

---

## Archetype Definition

The archetype is defined before transformation begins. It may be composed of:

- **One or more examples** — reference artifacts that convey the desired idiomatic style, structure, or behavior through impression
- **One or more explanations** — resources that describe the desired idiom directly (documentation, guidelines, specifications)
- **A combination of both** — examples and explanations used together to define the archetype from multiple angles

The archetype does not need to be a real artifact — it is a hypothetical ideal synthesized from its inputs.

---

## Process

### Step 1: Define the Archetype

Establish what the idiomatic target state looks like by assembling examples, explanations, or both. The archetype serves as the reference point for all subsequent steps.

---

### Step 2: Generate the Error Signal

Analyze the target content against the archetype and produce an accounting of the differences. The error signal captures:

- What the target currently looks like
- What the archetype prescribes
- Where and how they diverge

If **quantification** is employed, measurements are applied to make the error signal more deterministic and the degree of divergence more objectively verifiable.

If the error signal is sufficiently small — meaning the target is already close enough to idiomatic — the process may terminate early here without proceeding to planning or execution.

---

### Step 3: Generate the Reconciliation Plan

Produce a proposed change set that would bring the target into closer alignment with the archetype. The plan is derived directly from the error signal and describes concrete modifications to apply.

---

### Step 4: Execute the Plan

Apply the reconciliation plan to the target content, transforming it toward the idiomatic state defined by the archetype.

---

## Early Termination

The process can end after Step 2 if the error signal is small. This avoids unnecessary work when the target is already sufficiently idiomatic. The threshold for early termination may be defined explicitly (via quantification) or assessed qualitatively.

---

## Quantification

Quantification measures may optionally be applied during error signal generation to:

- Make the assessment more deterministic and repeatable
- Produce a measurable score or distance metric
- Enable verification that the reconciliation has succeeded

Quantification is not required — the process can proceed qualitatively — but it improves rigor when verifiability matters.

---

## Repo Isolation

When the target is a repository, **repo-isolation** should be used to avoid interfering with it directly. The repository is worked on in an isolated copy, ensuring the original is not modified until changes are intentional and reviewed.

---

## State Progression

```
[Archetype Definition]
      │  examples and/or explanations
      ▼
[Error Signal]  ◄── accounting of divergence from archetype
      │               (± quantification)
      │
      ├── [small error signal] ──► Early Termination
      │
      ▼
[Reconciliation Plan]  ◄── proposed changes to close the gap
      │
      ▼
[Execution]  ◄── target transformed toward idiomatic state
```
