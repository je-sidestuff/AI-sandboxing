# Evolutionary Recompositional Render (ERR)

## Overview

The Evolutionary Recompositional Render (ERR) is an AI execution strategy for transforming functional content through three coarse phases: **decomposition**, **distillation** (particle distillation), and **recomposition**.

ERR analyzes functional content to identify its minimum viable functional units, recreates those units individually with optional verifiability measures, and then rebuilds the content from the resulting distilled particles. The process enables targeted verification and improvement at the atomic level before reconstituting the whole.

---

## Terminology

| Term | Definition |
|------|------------|
| **Quarks** | Pieces of foundational capability too partial to be independently exercised as functionality, but often testable when test content is developed alongside |
| **Atoms** | Functional units that can be fully independently itemized — no dependencies on other units |
| **Molecules** | Functional units with some dependencies on other units |
| **Atomized state** | Functions fully decomposed but not yet verified or improved |
| **Distilled state** | Itemized functional units that have been subjected to verification and improvement |
| **Turbulence** | Optional iteration of one or more particles with man-in-the-loop feedback (applicable in both distillation and recomposition phases) |
| **Intermediate molecules** | Partially recomposed units before full recomposition is reached |
| **Lensing** | Additional verifiability/improvement applied at intermediary or fully recomposed stages, typically when instructions were a priority |
| **Recrystallized** | The state after reconstituted verification and improvement is complete |

---

## Phases

### Phase 1: Decomposition

Analyze the piece of functional content to identify its minimum viable functional units. The goal is to bring everything down to the smallest meaningful particles — isolating independent units from those with dependencies and identifying foundational sub-capabilities.

- **Quarks** — foundational capability pieces too partial to be independently exercised as functionality; they cannot stand alone but form the building blocks from which atoms are composed
- **Atoms** — fully independent functional units with no dependencies on other units; atoms may be composed of one or more quarks
- **Molecules** — functional units with dependencies on other atoms or molecules

Decomposition proceeds in two sub-phases:
1. **Fission** — breaking molecules down into atoms and identifying inter-atomic dependencies
2. **Quark extraction** — further decomposing atoms to expose their constituent quarks where beneficial for verification

Once fully decomposed but not yet verified or improved, the content is in the **atomized state** (or **quark-exposed state** if quark extraction was performed).

---

### Phase 2: Distillation (Particle Distillation)

Subject the itemized functional units to verification and improvement. Each particle — whether quark, atom, or molecule — is processed individually, leveraging the isolation achieved during decomposition to enable focused, targeted refinement.

Distillation operates at multiple granularities:
- **Quark distillation** — verify and improve individual quarks; often requires test content developed alongside since quarks cannot be independently exercised
- **Atomic distillation** — verify and improve atoms as complete independent units
- **Molecular distillation** — verify and improve molecules with their dependency relationships intact

The distillation process:
- Transforms atomized/quark-exposed content into the **distilled state**
- Optionally, one or more particles may be iterated with man-in-the-loop feedback — this optional iteration is called **turbulence**

---

### Phase 3: Recomposition

Rebuild the content from its distilled particles. Recomposition reassembles the verified and improved units back into a coherent whole, proceeding from the smallest particles upward.

Recomposition proceeds through fusion stages:
1. **Quark fusion** — combine distilled quarks back into their parent atoms
2. **Atomic bonding** — combine distilled atoms into intermediate molecules
3. **Molecular synthesis** — combine intermediate molecules into larger structures until full recomposition is achieved

- May proceed through **intermediate molecules** — partially recomposed units — before reaching full recomposition
- Additional verifiability/improvement may be applied at any recomposition stage:
  - **Lensing** — instruction-driven verification/improvement at intermediary or fully recomposed stages, imposed from the outside in an open-loop way — not from an outside observer through feedback
  - **Turbulence** — man-in-the-loop verification/improvement at any recomposition stage
- When reconstituted verification and improvement is complete, the content is **recrystallized**

---

## State Progression

```
[Source Content]
      │
      ▼ Phase 1a: Fission
[Atomized State]  ◄── atoms + molecules identified
      │
      ▼ Phase 1b: Quark Extraction (optional)
[Quark-Exposed State]  ◄── quarks within atoms exposed
      │
      ▼ Phase 2: Distillation
[Distilled State] ◄── verified + improved particles
      │               (quarks, atoms, molecules)
      │               (± turbulence)
      │
      ▼ Phase 3a: Quark Fusion
[Fused Atoms]     ◄── quarks recombined into atoms
      │
      ▼ Phase 3b: Atomic Bonding
[Intermediate Molecules] ◄── atoms bonded into molecules
      │                       (± lensing / turbulence)
      │
      ▼ Phase 3c: Molecular Synthesis
[Full Recomposition]
      │                (± lensing / turbulence)
      ▼
[Recrystallized]  ◄── reconstituted verification complete
```

---

## Feedback Spectrum

When executing the ERR AI execution strategy, a wide spectrum of feedback levels is available:

### Pure Open Loop

At one end of the spectrum lies **Pure Open Loop** execution. Here we perform the full analysis up front and decide how to break down all particles before beginning. We decide all lensing that will be applied and determine what the appropriate intermediate molecules will be.

This approach is ideally suited to strategies like **sequence-to-new-repo** where we can decide on all steps up front in our heuristic request. The entire decomposition, distillation, and recomposition plan is established before execution begins.

### Arbitrary Turbulence

At the opposite end of the spectrum lies **Arbitrary Turbulence**. Here we have an unknown number of steps and an unknown amount of imposed feedback during the process. The execution path emerges dynamically through continuous human-in-the-loop interaction.

*Note: Robust execution capabilities for the Arbitrary Turbulence end of the spectrum are not yet developed.*

---

## Organizing ERR Execution with Heuristic Dispatch

This section describes how to use heuristic dispatch to organize an ERR execution. (This documentation is tailored for human readers; a subsequent increment will present AI-consumable documentation for heuristic-dispatch itself.)

### Repository Organization During Process

During ERR execution, use top-level folders within your repo scope to organize content by phase:

```
repo/
├── decomposition/    # Atomized content awaiting distillation
├── distillation/     # Particles undergoing verification/improvement
├── recrystallization/ # Content being recomposed
└── ...
```

At the end of recomposition, once recrystallization is complete, these working folders can be removed — retaining only the recomposed output.

**Important:** When cleaning up working folders, be careful not to squash commits. Preserve the commit history that documents the ERR process.
