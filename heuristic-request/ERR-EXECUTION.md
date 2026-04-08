# Evolutionary Recompositional Render (ERR) Execution Guide

This document provides comprehensive guidance for preparing `sequence-to-new-repo` proposals to execute an Evolutionary Recompositional Render (ERR).

---

## What is ERR?

ERR is an AI execution strategy for transforming functional content through three coarse phases:

1. **Decomposition** — Break content into minimum viable functional units (quarks, atoms, molecules)
2. **Distillation** — Verify and improve each particle individually
3. **Recomposition** — Rebuild content from distilled particles

ERR is ideal for `sequence-to-new-repo` dispatch because the entire process can be planned upfront in a Pure Open Loop execution pattern.

---

## Recognizing ERR Requests

Detect ERR execution requests when the heuristic input mentions:

- "evolutionary recompositional render" or "ERR"
- "decompose and recompose"
- "break down, verify, rebuild"
- "particle distillation"
- "atomize" + "recrystallize"
- "fission and fusion" (in a content context)

The target repository and destination will typically be specified in one of these patterns:

- `"Perform an ERR of <source> into <destination>"`
- `"ERR <source> → <destination>"`
- `"Evolutionary recompositional render of this repository into <new-repo-name>"`
- `"ERR this codebase into <new-repo-name>"`

---

## Detail Tuning Dial

When preparing an ERR proposal, adjust the decomposition granularity based on the detail level requested (or inferred from context).

**Step Budget:** The total step count across all phases must not exceed **80 steps**. Scale the steps within each detail level based on repository size and complexity—the baselines below are calibrated for small-medium repositories and can be expanded for larger codebases as long as the 80-step ceiling is respected.

### LOW Detail (Minimum Effective Decomposition)

Use when: Quick transformation, simple content, time-constrained, or explicitly requested.

**Decomposition depth:** Molecules only (skip quark extraction)
**Distillation scope:** Molecular distillation only
**Recomposition stages:** Direct molecular synthesis

**Typical step count:** ~5-8 steps (flexible based on content scope; small-medium repos work well at this baseline)

```
Phase 1: Decomposition (1-2 steps)
  - Analyze source and identify major functional molecules
  - Document molecule boundaries and dependencies

Phase 2: Distillation (2-3 steps)
  - Distill molecule group A (highest-level functional units)
  - Distill molecule group B (supporting functional units)
  - [Optional] Distill molecule group C

Phase 3: Recomposition (2-3 steps)
  - Synthesize core structure from distilled molecules
  - Integrate supporting molecules
  - Final verification and cleanup
```

### MEDIUM Detail (Balanced Decomposition)

Use when: Standard transformation, typical codebase, no specific detail requirements.

**Decomposition depth:** Atoms with limited quark exposure
**Distillation scope:** Atomic distillation with molecular verification
**Recomposition stages:** Atomic bonding → Molecular synthesis

**Typical step count:** ~10-15 steps (scale up for larger codebases; this baseline suits small-medium repos)

```
Phase 1: Decomposition (2-3 steps)
  - Analyze source and perform fission (molecules → atoms)
  - Document atomic inventory and dependency graph
  - Selective quark extraction for complex atoms

Phase 2: Distillation (4-7 steps)
  - Distill foundational atoms (no dependencies)
  - Distill dependent atoms group A
  - Distill dependent atoms group B
  - [Continue for additional atom groups]
  - Verify distilled atoms against original functionality

Phase 3: Recomposition (4-5 steps)
  - Atomic bonding: Create intermediate molecules
  - Apply lensing to intermediate molecules
  - Molecular synthesis: Combine into full structure
  - Integration verification
  - Final recrystallization and cleanup
```

### HIGH Detail (Maximum Decomposition)

Use when: Critical transformation, complex content, quality is paramount, or explicitly requested.

**Decomposition depth:** Full quark extraction
**Distillation scope:** Quark → Atomic → Molecular distillation
**Recomposition stages:** Quark fusion → Atomic bonding → Molecular synthesis → Lensing

**Typical step count:** ~20-40 steps (scale as needed; this baseline suits small-medium repos)

```
Phase 1: Decomposition (3-5 steps)
  - Initial analysis and molecule identification
  - Fission: Break molecules into atoms
  - Document atomic inventory with dependency mapping
  - Quark extraction: Expose constituent quarks within atoms
  - Document quark-level inventory

Phase 2: Distillation (10-20 steps)
  - Distill quark group A (foundational capabilities)
  - Distill quark group B
  - [Continue for all quark groups]
  - Atomic distillation: Verify quarks form valid atoms
  - Distill atom group A (independent units)
  - Distill atom group B (dependent units)
  - [Continue for all atom groups]
  - Molecular distillation: Verify atoms form valid molecules

Phase 3: Recomposition (7-15 steps)
  - Quark fusion: Recombine quarks into atoms
  - Verify fused atoms
  - Atomic bonding: Create intermediate molecules
  - Apply lensing to intermediate molecules
  - Verify intermediate structures
  - Molecular synthesis phase 1: Core structure
  - Molecular synthesis phase 2: Supporting structure
  - [Continue synthesis phases]
  - Full integration
  - Apply final lensing
  - Recrystallization verification
  - Final cleanup and documentation
```

---

## Preparing the Dispatch

When generating a `sequence-to-new-repo` dispatch for ERR execution:

### Step 1: Parse the Request

Extract from the heuristic input:
- **Source:** The repository or content to transform (may be "this repository", a specific repo name, or a path)
- **Destination:** The new repository name to create
- **Detail level:** LOW, MEDIUM, or HIGH (default to MEDIUM if unspecified)
- **Special instructions:** Any specific verification requirements, lensing instructions, or constraints

### Step 2: Determine Repository Naming

The destination repository name comes from the request. Common patterns:

| Request Pattern | Destination |
|----------------|-------------|
| `"...into AI-evo-experimental1"` | `AI-evo-experimental1` |
| `"...into a new repo called foo-refactored"` | `foo-refactored` |
| `"ERR this → my-new-repo"` | `my-new-repo` |
| No destination specified | `<source-name>-err-<timestamp>` |

### Step 3: Build the Initial Instruction

The `instruction` field should:
1. Set up ERR working directories (`decomposition/`, `distillation/`, `recrystallization/`)
2. Document the source being transformed
3. Establish the detail level

**IMPORTANT:** Do NOT include "Create the repository X" in the instruction - the dispatcher creates the repo using the `sequence_repo_name` field. Including repo creation in the instruction causes duplicate repos.

Example:
```
Set up ERR working structure with folders: decomposition/, distillation/, recrystallization/.
Document in README.md that this is an ERR transformation of <source> at <detail> detail level.
Include a MANIFEST.md to track particle inventory.
```

### Step 4: Build the Sequence Commands

Each command should be self-contained and reference the working directory structure.

**Phase 1 commands** should:
- Analyze the source content
- Identify functional units at the appropriate granularity
- Document particles in `decomposition/` folder
- Create dependency graphs where applicable

**Phase 2 commands** should:
- Reference particles in `decomposition/`
- Perform verification/improvement
- Write distilled particles to `distillation/` folder
- Document any issues discovered

**Phase 3 commands** should:
- Reference particles in `distillation/`
- Perform fusion/bonding/synthesis operations
- Write recomposed content to `recrystallization/` folder (then to root)
- Apply lensing as specified
- Perform final verification

### Step 5: Set Timing

The `sequence_minutes_between` value should reflect the detail level:

| Detail Level | Minutes Between Steps |
|-------------|----------------------|
| LOW | 10-15 |
| MEDIUM | 15-25 |
| HIGH | 20-30 |

---

## Example Dispatch: MEDIUM Detail ERR

Given heuristic input:
> "Perform an evolutionary recompositional render of agent-events into AI-evo-experimental1"

```json
{
  "type": "sequence-to-new-repo",
  "sequence_repo_name": "AI-evo-experimental1",
  "instruction": "Set up ERR working structure with folders: decomposition/, distillation/, recrystallization/. Create README.md documenting this as an ERR transformation of agent-events at MEDIUM detail level. Create MANIFEST.md to track particle inventory throughout the process.",
  "sequence_commands": [
    "Phase 1.1 - Fission: Analyze agent-events repository. Identify all functional molecules (major feature areas). Break molecules into atoms (individual functional units). Document in decomposition/MOLECULES.md and decomposition/ATOMS.md with dependency graph.",
    "Phase 1.2 - Quark Extraction: For complex atoms identified in Phase 1.1, extract constituent quarks. Document in decomposition/QUARKS.md. Update MANIFEST.md with complete particle inventory.",
    "Phase 2.1 - Distill Foundational Atoms: Process atoms with no dependencies. Verify each atom functions correctly in isolation. Write distilled versions to distillation/atoms/. Document improvements in distillation/DISTILLATION-LOG.md.",
    "Phase 2.2 - Distill Dependent Atoms Group A: Process atoms that depend only on foundational atoms. Verify with dependencies available. Write to distillation/atoms/. Update DISTILLATION-LOG.md.",
    "Phase 2.3 - Distill Dependent Atoms Group B: Process remaining dependent atoms. Complete atomic distillation. Verify full atom inventory is distilled.",
    "Phase 2.4 - Atomic Verification: Run verification across all distilled atoms. Ensure no regressions. Document verification results in distillation/VERIFICATION.md.",
    "Phase 3.1 - Atomic Bonding: Combine distilled atoms into intermediate molecules. Write to recrystallization/molecules/. Document molecular structure.",
    "Phase 3.2 - Apply Lensing: Apply instruction-driven verification to intermediate molecules. Ensure molecules meet original functional requirements. Document lensing results.",
    "Phase 3.3 - Molecular Synthesis: Combine intermediate molecules into full structure. Write recomposed content to repository root. Maintain ERR working folders for reference.",
    "Phase 3.4 - Final Verification: Verify recrystallized content against original agent-events functionality. Document any differences. Mark recrystallization complete in MANIFEST.md.",
    "Phase 3.5 - Cleanup and Documentation: Update README.md with ERR completion summary. Document the transformation process. Retain or archive working folders as appropriate."
  ],
  "sequence_minutes_between": 20,
  "mode": "execute"
}
```

---

## Example Dispatch: LOW Detail ERR

Given heuristic input:
> "Quick ERR of my-utils into utils-v2 - keep it simple"

```json
{
  "type": "sequence-to-new-repo",
  "sequence_repo_name": "utils-v2",
  "instruction": "Set up ERR structure with folders: decomposition/, distillation/, recrystallization/. Document as LOW detail ERR of my-utils in README.md.",
  "sequence_commands": [
    "Phase 1 - Molecular Analysis: Analyze my-utils and identify major functional molecules. Skip quark extraction. Document molecules and dependencies in decomposition/MOLECULES.md.",
    "Phase 2.1 - Distill Core Molecules: Distill the primary functional molecules. Write to distillation/. Verify functionality.",
    "Phase 2.2 - Distill Supporting Molecules: Distill remaining molecules. Complete molecular distillation.",
    "Phase 3.1 - Molecular Synthesis: Directly synthesize full structure from distilled molecules. Write to recrystallization/ then root.",
    "Phase 3.2 - Final Verification and Cleanup: Verify recrystallized content. Update README.md. Complete."
  ],
  "sequence_minutes_between": 12,
  "mode": "execute"
}
```

---

## Example Dispatch: HIGH Detail ERR

Given heuristic input:
> "Perform a thorough evolutionary recompositional render of auth-service into auth-service-v2 with maximum decomposition"

```json
{
  "type": "sequence-to-new-repo",
  "sequence_repo_name": "auth-service-v2",
  "instruction": "Set up full ERR structure with decomposition/, distillation/, recrystallization/ and subdirectories quarks/, atoms/, molecules/. Create comprehensive MANIFEST.md. Document as HIGH detail ERR of auth-service in README.md.",
  "sequence_commands": [
    "Phase 1.1 - Initial Analysis: Analyze auth-service repository structure. Identify all major functional areas. Create initial documentation in decomposition/OVERVIEW.md.",
    "Phase 1.2 - Fission: Break identified functional areas into molecules. Document in decomposition/molecules/. Create dependency graph.",
    "Phase 1.3 - Atomic Decomposition: Break molecules into atoms. Document each atom in decomposition/atoms/. Map atom-to-molecule relationships.",
    "Phase 1.4 - Quark Extraction: Extract quarks from each atom. Document in decomposition/quarks/. Map quark-to-atom relationships. Complete MANIFEST.md particle inventory.",
    "Phase 2.1 - Distill Quark Group A: Distill foundational quarks (authentication primitives). Write to distillation/quarks/. Document in DISTILLATION-LOG.md.",
    "Phase 2.2 - Distill Quark Group B: Distill security-related quarks. Continue documentation.",
    "Phase 2.3 - Distill Quark Group C: Distill utility quarks. Complete quark distillation.",
    "Phase 2.4 - Quark Verification: Verify all distilled quarks. Ensure test content validates each quark.",
    "Phase 2.5 - Distill Foundational Atoms: Distill atoms with no dependencies using distilled quarks. Write to distillation/atoms/.",
    "Phase 2.6 - Distill Dependent Atoms A: Distill first tier of dependent atoms.",
    "Phase 2.7 - Distill Dependent Atoms B: Distill second tier of dependent atoms.",
    "Phase 2.8 - Distill Dependent Atoms C: Complete atomic distillation.",
    "Phase 2.9 - Atomic Verification: Full verification of distilled atoms. Document results.",
    "Phase 3.1 - Quark Fusion: Recombine distilled quarks into fused atoms. Write to recrystallization/atoms/.",
    "Phase 3.2 - Verify Fused Atoms: Ensure fused atoms match distilled atom specifications.",
    "Phase 3.3 - Atomic Bonding A: Bond atoms into core intermediate molecules. Write to recrystallization/molecules/.",
    "Phase 3.4 - Atomic Bonding B: Bond atoms into supporting intermediate molecules.",
    "Phase 3.5 - Apply Lensing to Intermediates: Instruction-driven verification of intermediate molecules.",
    "Phase 3.6 - Verify Intermediate Structures: Comprehensive intermediate verification.",
    "Phase 3.7 - Molecular Synthesis Core: Synthesize core application structure.",
    "Phase 3.8 - Molecular Synthesis Supporting: Synthesize supporting structures.",
    "Phase 3.9 - Full Integration: Integrate all synthesized components. Write to repository root.",
    "Phase 3.10 - Final Lensing: Apply final instruction-driven verification to complete structure.",
    "Phase 3.11 - Recrystallization Verification: Full verification against original auth-service.",
    "Phase 3.12 - Documentation and Cleanup: Complete README.md, archive working folders, mark ERR complete."
  ],
  "sequence_minutes_between": 25,
  "mode": "execute"
}
```

---

## Handling Ambiguous Requests

When the heuristic input is ambiguous:

| Ambiguity | Resolution |
|-----------|------------|
| No detail level specified | Default to MEDIUM |
| No destination specified | Use `<source>-err` or `<source>-recrystallized` |
| Source is "this repository" | Require clarification or use context to determine actual repo |
| Source is a path not a repo | Treat as content within current repo to be transformed |
| Multiple sources mentioned | Create separate dispatches or ask for clarification |

---

## Key Principles

1. **Each step must be self-contained** — Later steps may reference earlier work but shouldn't require knowledge of step implementation details

2. **Use the working folder structure** — `decomposition/`, `distillation/`, `recrystallization/` provide clear organization

3. **Document throughout** — MANIFEST.md tracks particles, DISTILLATION-LOG.md tracks improvements, VERIFICATION.md tracks test results

4. **Match detail level to content** — Simple utilities → LOW; typical applications → MEDIUM; critical systems → HIGH

5. **Preserve the journey** — The ERR process is as valuable as the result; don't squash history

---

## CRITICAL: Execute, Do Not Document

**ERR is a transformation process that produces transformed code, not documentation about transformation.**

The instructions you generate (both the initial `instruction` and each `sequence_command`) must clearly communicate to the worker agent that it should produce actual artifacts, not descriptions. The worker does not know it's doing an ERR unless you tell it clearly what output is expected.

### Communicating Worker Role Through Instructions

Your instructions ARE the worker's understanding of its task. To prevent workers from defaulting to documentation mode:

1. **Be explicit about expected output types** — Say "create quarks/Q001.ts containing the extracted function code" not "document the quarks"

2. **Reference concrete source content** — Name specific files and functions from the source repository. Don't say "identify functional molecules"; say "extract parseConfig() and validateInput() from config/parser.go"

3. **Specify file extensions** — ".ts", ".go", ".py" files containing code, NOT ".md" files describing code

4. **Use action verbs that demand artifacts** — "Extract", "Write", "Create", "Implement" not "Analyze", "Document", "Describe", "Plan"

### What Success Looks Like

✅ **Quarks** = Actual code files (Q001.ts, Q002.go) containing extracted functions/capabilities
✅ **Atoms** = Actual code files (atoms/auth.ts, atoms/parser.go) containing independent functional units
✅ **Molecules** = Actual code files with working code that has dependencies
✅ **Distillation** = Taking Q001.ts and improving/verifying the actual code
✅ **Recrystallization** = Writing new main.go that integrates the distilled code

### What Failure Looks Like

❌ Q001.md that says "This quark would contain the authentication primitives..."
❌ atoms/README.md that catalogs "The atoms in this system are: 1. Auth atom, 2. Parser atom..."
❌ DECOMPOSITION-PLAN.md that describes what decomposition would look like
❌ Creating a directory structure with markdown files describing what would go in each directory

### The Litmus Test

After each step, ask: "Did I produce working code, or did I describe what code would exist?"

If the answer is "I described it" — you have failed. Delete the description and write the actual code.

### Command Phrasing

When generating sequence_commands, phrase them to demand execution:

**WRONG:** "Analyze the codebase and document the quark structure"
**RIGHT:** "Extract functions from auth.ts into quarks/Q001.ts, Q002.ts. Each quark file must contain the actual function code."

**WRONG:** "Create a decomposition plan for the dispatcher"
**RIGHT:** "Decompose dispatcher.ts: extract the routeMessage() function into quarks/Q003.ts, extract the validatePayload() function into quarks/Q004.ts"

### Grounding in Concrete Repository Content

When generating ERR dispatches, you have access to a READ SPACE containing the actual source repository. Use this to:

1. **Inventory what exists** — Before generating sequence_commands, identify the actual files, directories, and functions in the source

2. **Name specific artifacts** — Reference real filenames like "agent-worker/main.go" or "dispatcher/handlers.ts", not abstract concepts like "the worker module"

3. **Avoid abstract pass-through** — Don't restate the request in different words. "Decompose the codebase into quarks" is useless. "Extract the processRequest() and handleError() functions from server.go into quarks/" is actionable.

4. **If you can't identify specific content** — State that explicitly in the dispatch rather than generating vague instructions that will confuse the worker

---

## Reference: ERR Terminology

| Term | Definition |
|------|------------|
| **Quarks** | Foundational capability pieces too partial to be independently exercised |
| **Atoms** | Fully independent functional units with no dependencies |
| **Molecules** | Functional units with dependencies on other units |
| **Fission** | Breaking molecules into atoms |
| **Quark Extraction** | Exposing constituent quarks within atoms |
| **Distillation** | Verification and improvement of particles |
| **Quark Fusion** | Recombining quarks into atoms |
| **Atomic Bonding** | Combining atoms into molecules |
| **Molecular Synthesis** | Combining molecules into larger structures |
| **Lensing** | Instruction-driven verification at intermediate or final stages |
| **Turbulence** | Man-in-the-loop verification (not applicable in Pure Open Loop) |
| **Recrystallized** | Final state after reconstituted verification is complete |
