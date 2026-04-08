# ERR Attempt 1 Assessment

**Date:** 2026-04-07
**Outcome:** Failure — agents produced documentation *about* ERR instead of *executing* ERR

---

## What Happened

Two distinct failures occurred:

### Failure 1: Heuristic Request Receiver Passed Through Instead of Contextualizing

The agent that received the heuristic request was supposed to:
- Understand they were in a **read space** containing a specific repository
- Break down the ERR execution strategy into tasks **within the context of that repository**
- Generate task definitions that reference actual files, actual code, actual content in the repo

Instead, the agent:
- Passed along the ERR request as an abstract description
- Did not ground the decomposition in the concrete content of their read space
- Created tasks that described ERR phases generically rather than identifying what quarks/atoms/molecules exist in the specific repo

### Failure 2: Task Receivers Documented Instead of Executed

The agents receiving the ERR tasks were supposed to:
- Act on the repository content
- Actually decompose files into quarks, distill them, recompose them
- Produce transformed artifacts

Instead, the agents:
- Wrote detailed markdown documentation describing what ERR *is*
- Created files like `Q001.md`, `Q002.md` that describe quarks conceptually
- Built a catalog of what the system *would* look like if decomposed, without actually doing the work

The output in `ignored-scratch/err-attempt/` shows this clearly: 125+ quark files, 11 atoms, 4 organisms — all documentation about ERR particles, not actual distilled code.

---

## Root Cause Theory

**Agents defaulted to "explain" mode rather than "execute" mode.**

When given a complex, abstract request like "perform ERR on this codebase," the natural LLM tendency is to:
1. Demonstrate understanding by explaining the concept
2. Create organizational structures that mirror the concepts
3. Produce documentation that looks like work was done

This is the "write a report about the assignment" failure mode rather than "do the assignment."

**The read space was informational rather than actionable.**

The agents likely received:
- The ERR specification document (explaining what ERR is)
- Possibly a pointer to the repository
- No explicit instruction that their output must be *transformed code*, not *description of code*

Without explicit grounding, agents interpreted their task as "understand and document ERR" rather than "apply ERR to produce transformed artifacts."

---

## What Needs to Change

### Fix 1: Heuristic requests must force contextualization

The heuristic request receiver must be prompted with explicit constraints:

> "You have a read space. Your task is to generate a plan that references **specific files** in that read space. Do not describe processes abstractly. Every task you emit must name concrete artifacts that exist in your read space."

The request itself should include a forcing function like:
- "List the top 10 files you will decompose in this ERR"
- "Name the first 3 quarks you have identified in the codebase"

### Fix 2: Task definitions must specify output type

Tasks should explicitly state:
- **Output format:** Transformed code, not documentation
- **Success criteria:** "This task is complete when file X has been rewritten as file Y"
- **Anti-pattern warning:** "Do not describe what you would do. Do it."

Example task that would fail less:
```
Task: Decompose dispatcher/direct-dispatch.ts into quarks
Output: Create dispatcher/quarks/Q001.ts, Q002.ts, etc. containing extracted functions
NOT: Create Q001.md describing what a quark would be
```

### Fix 3: Early checkpoints with concrete artifact verification

After the first task completes, verify:
- Did the agent produce code files or documentation files?
- Does the output contain actual transformed content from the source repo?
- Can we diff the input against the output to see transformations?

If verification fails at step 1, abort and re-prompt with stronger constraints.

---

## Next Steps

**Option A: Strengthen prompting and retry**
Add the forcing functions described above. Re-run ERR with explicit "do, don't document" instructions.

**Option B: Build scaffolding that makes wrong behavior harder**
Create a dispatch type specifically for ERR that:
- Pre-populates the write space with stub files to be filled in
- Validates output file extensions (must be .ts/.py/etc., not .md)
- Rejects tasks that only produce markdown

**Option C: Smaller scope test**
Instead of ERR on the whole repo, try ERR on a single file. The smaller scope makes "execute vs explain" more obvious and easier to verify.

---

## Artifacts to Review

- `ignored-scratch/err-attempt/` — the failed attempt output
- `our-wonderful-future/EVOLUTIONARY-RECOMPOSITIONAL-RENDER.md` — the ERR spec that was (mis)interpreted
