# Prompt

Now that we have the first path for condocs working in federation command (in AI-evo1) we need to add the rest of the feature.

The next item we will implement is the 'retry' path.

The revision path works as follows:
- We state we want a 'Retry' instead of a 'Revision' when we are iterating on a step
- We optionally state (from X), where X may be 'start' or one of the letters (A-N) -- ie: '## Retry C (from A)'
  - If (from X) is not specified then the previous step is implied (start if current increment is A, A if current increment is B, etc)
- When we retry the sequence is as follows:
  - We push the current state of the branch to a new branch - condoc/<current-identifier>/takeN instead of condoc/<current-identifier>/main (where N is a counter for the full scope of the condoc, not per step, starting at 1)
  - We use 'git log -p -n <steps-back>' and save the output to a file takeN<identifier>diff.txt beside our step file
  - We move the head of our condoc/<current-identifier>/main branch back to the commit we are retrying from
  - We follow the retry prompt the same way we would have for a revision

Note that to facilitate the smooth operation of this mechanism, and to create better traceability generally, we will want to also:
- Add the commit we are currently on BEFORE a '## Reply' line along with a link to the commit
  - For now assume we're using github, we'll generalize later
  - Use a similar style to the step/parent links and similar spacing

Note that a 'Retry' may only retry back as far as the beginning of a step, and that a retry only has meaning within the scope of a step, never at the scope of the outer condoc. (Similar to Revision)

We will first create an example 'AI-sandboxing/discussion/condocs-examples/retry' using 'AI-sandboxing/discussion/condocs-examples/verbose' as a base. In the example we will do a 'Retry' AFTER our first revision and in this case retry from 'start'.


## Reply

Step 11 is fully implemented. Here's a summary of everything that was done and the one bug fixed this session:

**Example files fixed (hash consistency):**
- `Step1Prompt.snp6.md` and `snp7.md` — corrected the commit preamble before `## Reply A` from `9b4c8e1` (the Revision A output hash) to `7f2a9e3` (the initial-reply commit that was HEAD when the Revision A agent ran)
- `take1Simple-1779734500diff.txt` — rewritten to show the two actually-abandoned commits (`9b4c8e1` Revision A output and `7f2a9e3` initial reply output), and fixed the inline preamble hash inside commit `9b4c8e1`'s diff to also use `7f2a9e3`; the step-started commit `3f7a1c2` no longer appears (it's the reset target, not abandoned)

**Bug fixed in condoc.go:**
- `runCondocRetryGitSequence` — changed `git push origin mainBranch` to `git push --force origin mainBranch` after the `git reset --hard HEAD~N`. A regular push would always fail because the take-branch preservation already captured the old history, so force-pushing the main condoc branch is correct here.


# Reprompt A

The example we've created has an error starting at 'AI-sandboxing/discussion/condocs-examples/retry/simpleImpls/Step1Prompt.snp8.md' - when we use a 'Retry' we DO NOT delete text from the document. The idea here is that with a retry we want to have our previous attempt readily apparent.

A retry is a 'soft' action where we are taking a step back while keeping in mind what we already tried.

Please fix the example files so we still see the original prompt and Revision A, then fix the corresponding implementation.

Make sure the INPUT for the Retry also stays present in the example, not just the Reply.


## Reply

All three changes are correct. The implementation:
- `buildCondocRetryPrompt`: updated to describe the full-history step file correctly
- `runCondocRetryGitSequence`: now accepts `stepFilePath`, reads the step file (stripping Human-Prompt) before the reset, and restores it after — both the step file and diff file go into the same commit
- `condocRunRetry`: always uses `buildCondocRetryPrompt` (no more fallback to `buildCondocStepPrompt`) and passes `cs.stepFile` to the git sequence

**Example files (snp8/9/10)** now show the complete history: original prompt → Reply → Revision A → Reply A → Retry B input → Reply B, instead of just Reply B.

**Implementation**: The root cause was that `git reset --hard` reverted the step file along with all project files. The fix saves the step file content before the reset and restores it after, so the agent sees the full history and `appendReplyToStepFile` appends the new Reply B in the right position (after the Retry B heading, not at the top of a blank slate).


## Reprompt B

This implementation is now working well, with one exception.

When we Retry (from start) the mechanism reverts the outer condoc. We only want it to revert to the origin of the step file.

We should be able to resolve this by having a commit and push as the document is created and before the (start) agent invocation.

Let's do that now.


## Reply

**Root cause**: `retryFromStepsBack` counts reply sections in the step file to compute `stepsBack`. The step file deliberately preserves all reply history across retries, but the git history is reset on each "from start" retry. After a first Retry (from start) followed by more work and then a second Retry (from start), `stepsBack` overcounts relative to the actual git commit depth — `git reset --hard HEAD~stepsBack` flies past the "step N started" commit and reverts the outer condoc to a pre-step state.

**Fix — three changes to `condoc.go`**:

1. **`CondocSession.stepStartHash string`** — new field that holds the git commit hash of the "condoc: step N started" commit.

2. **`handleCondocGitDone`, `condocPhaseStepStarting` case** — after the step-start commit completes (but before `condocStartStepAgent` is called), run `git rev-parse HEAD` and store the result in `cs.stepStartHash`. This is the anchor; it stays valid for the entire lifetime of the step across any number of retries.

3. **`runCondocRetryGitSequence`** — added a `resetHash string` parameter. When non-empty (set only for "from start" retries), the function uses `git log -p <resetHash>..HEAD` for the diff capture and `git reset --hard <resetHash>` for the reset — bypassing the stepsBack calculation entirely.

4. **`condocRunRetry`** — sets `resetHash = cs.stepStartHash` when `fromRef == "start"`, then passes it through to `runCondocRetryGitSequence`. For all other `fromRef` values the existing `HEAD~stepsBack` path is unchanged.


## Reprompt C

Let's move on to the next part of the implementation now -- the accompanying condoc *client*, called 'condoccer'.

This will be another sub-project in AI-evo1 beside federation command. A go webserver with a react frontend connecting through a websocket.

The condoccer acts as a handler for the other side of the interaction with the condoc handler. It has an outer menu where it lists the condocs in the repository from where it was launched. The menu shows whether the condocs are completed or ongoing. Completed condocs may be viewed and ongoing ones may be interacted with.

The condoccer may run in 'local' or 'remote' mode -- in local mode it does not need to perform git operations because the condoc handler will do all of the work.

For the first implementation we will implement only local mode.

The condoccer uses buttonology to make it so the user may simply click instead of needing to enter directives by hand, and will also facilitate selection of Revision|Retry with buttonology.

Let's create this utility along with a makefile to run it.


## Reply lost

Condoccer was created in 'AI-evo1/condoccer', but the terminal output was lost.


## Reprompt D

The first cut of condoccer is working, but we need to adjust a few things at a high level to start:
- We want the left hand 'navigator panel' which shows files to perform depth-traversal.
  - When we select a condoc file we want to enter the scope of that condoc
  - When there are child steps in the condoc we want to see those taking over the navigator panel now, and we want an 'up' option
  - Going 'up' from the condoc brings us to the list of condocs
  - When we go 'into' a step the navigator panel displays the iterations in that step -- these simply move the display to that iteration
- We don't want a pair of panes on the right hand side of the navigator panel, just one
- When we are displaying data from the condocs to the condoccer user we want to bring all of the metadata into UI elements
  - The parts of the condoc-yaml should be individual fields
  - For a new step we should get an individual entry for title/prompt
  - We should only see the meaningful text, not the section formatting characters
    - For a step within the condoc file we should see the title and the prompt only in our view
    - We should not see the '<REPLACE>' items, we should simply see view elements that allow us to input what we need

We also need to correct a bug:
- When we submit a new prompt it does not replace the <REPLACE> sections like it should

Make these updates in AI-evo1.
