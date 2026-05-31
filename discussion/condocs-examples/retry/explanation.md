# Explanation

This directory contains snapshots of a condoc interaction at different phases.

The syntax of our snapshot files is <file>.snpN.<ext> to denote what is happening at a particular point in time (ie: Simple.snp1.md is the file at snapshot 1).

If we are missing a <file>.snpX.<ext> but we have other numbers with gaps, this simply means the file did not change between those times.

This file explains what is happening in the '## Storyline' section, and each '### Snap N' subsection indicates a point in time.

This VERBOSE mode is triggered by providing the -v or --verbose flag to the condoc command.

Both VERBOSE and standard condocs are readable by the reader in the same way. With VERBOSE mode the handler simply adds inline explanation.

## Differences from the verbose example

This example extends the verbose example to demonstrate the **Retry** path. Two additional features are shown here that do not appear in the verbose example:

1. **Commit hash links** — inserted before every `## Reply` line, linking to the GitHub commit that was HEAD when the agent ran. This provides a stable reference point for traceability and retry targeting.

2. **Retry** — after Revision A completes, the human decides to abandon the whole attempt and retry from start. The handler pushes the current branch state to a `take1` branch, saves a diff of the abandoned commits, resets the main branch HEAD, and runs the agent fresh.

## Storyline

This interaction begins when the user uses the
`condoc AI-sandboxing/discussion/condocs-examples/retry/Simple.md "A simple conversational document where we create great content about cats."`
command in the federation-command CLI.

The federation-command enters condoc mode. Because there is no valid ongoing condoc at that location the handler creates a new file.

The condoc handler finds that the `AI-sandboxing/discussion/condocs-examples/retry/` directory has the repository root `AI-sandboxing`.

The branch is created following the pattern `condoc/Simple-<timestamp>/main`.

### Snap 0

The handler has created the file and templated it with header material so the human can see what is proposed.

The human adds the `!HANDOFF!` directive.

### Snap 1

The human has responded.

The condoc handler sees this, creates the branch, templates out the first step, commits and pushes.

### Snap 2

Step 1 has been templated. The human enters a title and description and adds the `!HANDOFF!` directive.

### Snap 3

The human has given a title and prompt.

The handler creates the step child document `simpleImpls/Step1Prompt.md` and feeds the prompt to the agent.

### Snap 4

The agent has responded with an initial cat story. The step file now shows a **commit hash link** immediately before the `## Reply` heading — this is the HEAD commit at the moment the agent ran, linking to GitHub for traceability.

The human is partially happy but wants to give the cat a name, so they trigger Revision A.

### Snap 5

The human has added `## Revision A` with their request and the `!HANDOFF!` directive.

### Snap 6

The handler ran the agent for Revision A. A new commit hash link appears before `## Reply A`.

The human looks at the result and decides they want to start over entirely — a pompous hunting cat is not the direction they want at all. They add a `## Retry B (from start)` directive.

### Snap 7

The human has filled in the Retry heading and added `!HANDOFF!`.

### Snap 8

The handler detects the Retry. It:

1. Pushes the current `condoc/Simple-1779734500/main` branch HEAD to a new branch `condoc/Simple-1779734500/take1`, preserving the full abandoned history.
2. Runs `git log -p -n 2` (two commits abandoned) and saves the output to `simpleImpls/take1Simple-1779734500diff.txt`.
3. Resets `condoc/Simple-1779734500/main` HEAD back two commits to before any Reply existed.
4. Writes and commits the diff file (so it survives on the reset branch).
5. Runs the agent fresh on the reset step file, using the Retry guidance text.

A new commit hash link appears before `## Reply B` in the step file. The CatStory.md is also fresh (the prior version was in the abandoned commits).

The human is happy with the warm, quiet story and marks the step complete.

### Snap 9

The human has added `!COMPLETED!` to the step file.

### Snap 10

The step is finalized. Simple.md shows the second step templated.

### Snap 11

The human adds `!COMPLETED!` to Simple.md.

### Snap 12

The condoc is marked completed and the handler exits.
