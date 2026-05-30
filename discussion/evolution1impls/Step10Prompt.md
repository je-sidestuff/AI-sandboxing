# Prompt

We are creating a new capability for continuity in interactive AI collaboration called 'condocs', or 'conversational documents'.

These are documents which provide an interaction surface for a human and an agent. They are an 'md' based format where there is a top level document and one or more layers of nested child documents.

The top level document is divided into coarse 'steps' which represent stages of collaborative development being performed by the human-AI pair (henceforth 'the pair').

Each 'step' heading optionally contains comments and must contain triple-backtic 'prompt' block. This is the major initiating prompt which starts a new logical division of work.

When we use the handler to instruct the AI to begin a new step it automatically creates a child discussion document and metadata document, populates the header metadata and the parent link back, copies the prompt into the child discussion under the heading '## Prompt', and instructs the AI to execute the prompt.

The output of the AI is captured and placed in the discussion child doc after the heading '## Reply'.

In order to tell the agent that the condoc is in a state where it is time for it to act we can add the line !HANDOFF! to the end of the document. (As long as only whitespace is following the '!HANDOFF!' string the handler wiull recognize this.) When the agent picks up the handoff it will replace the '!HANDOFF!' string with <>

We want to implement condocs in federation-command in a very decoupled and non-invasive way. This implementation is an early iteration and we want to be able to cleanly remove everything without being disruptive if we want to try again.

For our condocs implementation we will look at the 'AI-sandboxing/discussion/Evolution1.md'/'AI-sandboxing/discussion/evolution1impls' and 'AI-sandboxing/discussion/SlopspaceAdventures.md'/'AI-sandboxing/discussion/slopspaceadventuresimpls' for inspiration. These capture the idea of the interaction but are not character-for-character examples. Also look at 'AI-sandboxing/discussion/condocs-examples' for character-for-character examples -- beginning with 'AI-sandboxing/discussion/condocs-examples/simple'. (Note that explanation.md is not part of an interaction - iut is there to explain what is happening)

Note that initially we WILL NOT IMPLEMENT THE RETRY PATH, ONLY THE REVISE.

We can look at our riealong handler dynapane for inspiration on federation-command UI design. When our UI is in 'condoc' mode it has a similar appearance to the ridealong handler but the flashing dot is green and yellow instead of blue and red, and the emoji is not a cop-car.

We will now implement enough of the condoc handler in 'AI-evo1/federation-command' to perform the steps we see in 'AI-sandboxing/discussion/condocs-examples/simple'.


## Reply

The condoc handler has been implemented in `AI-evo1/federation-command` across three files: `condoc.go` (962 lines, self-contained handler with all phases, file manipulation, git sequencing, dynapane, and agent prompt builders), `blinker.go` (extended with `BlinkerCondoc` state that alternates green/yellow), and `main.go` (integrated command parsing, message dispatch, key handling, and view rendering). The implementation covers the full lifecycle visible in `condocs-examples/simple` — proposal acceptance, step templating, agent execution, revision cycle, step completion, and condoc finalization — while deliberately omitting the Retry path. All condoc code is isolated so it can be removed by deleting `condoc.go` and stripping a small number of references from `main.go` and `blinker.go`.


# Reprompt A

The initial implementation is on the right track, but there are a few flaws.

The first flaw is that when we enter condoc mode it will accept a file path which is not a '.md'. Let's be strict about md extensions for now.

The next flaw is that in our dynapane we cannot select menu items - we may as well allow the user to select options. Instead of ctrl-c to exit we'll make an exit option, for now the only other option can simply be 'placeholder' (which does nothing).

The next flaw is that when we submit a prompt we do not see that from the condoc dialog/handler window. We only see the response.

And finally - although we create the local branch successfully when we are procesing condocs - we are not pushing to the remote. We want to be pushing to the remote automatically as well.

Let's fix all of these flaws then I will try again.


## Reply 

All four fixes are implemented. Here's a summary of what changed:

**Fix 1 — Strict `.md` extension** (`condoc.go:654`): `NewCondocSession` now rejects any file path that doesn't end in `.md` before doing anything else.

**Fix 2 — Dynapane menu selection** (`condoc.go`, `main.go`):
- Added `condocMenuAction` type with `condocMenuExit` and `condocMenuPlaceholder` constants
- Added `menuIndex int` to `CondocDynapane` with `MenuUp()`, `MenuDown()`, `MenuAction()` methods
- `View()` now renders a two-item menu (◈ for selected item) with "↑↓ navigate  enter to select" hint instead of the old ctrl+c hint
- Added `handleCondocKey()` in `main.go` routing Up/Down to menu navigation, Enter to selection, Ctrl+C still exits as a fallback

**Fix 3 — Show submitted prompt** (`condoc.go:832`, `921`): `condocStartStep` prints the step prompt text to the scrollback before the agent runs; `condocRunRevision` prints the extracted revision text the human wrote.

**Fix 4 — Push to remote** (`condoc.go:739`, `881`, `948`, `970`): Every git commit sequence now includes a push — the first (branch creation) uses `--set-upstream origin <branch>`, subsequent ones use `push origin <branch>`.


# Reprompt B

The implementaiton is getting better - two more adjustments need to be made:
- We still need the condoc handler to be regularly attempting to 'pull --rebase' the remote branch  (this allows contribution from a remote location).
- We should adjust the document spacing around the headings slightly. Note that I have updated the examples accordingly.


## Reply

Both fixes are implemented. Here's what changed:

**Periodic `pull --rebase`** (`condoc.go`):
- New `condocPullDoneMsg` type and `condocPullInterval = 30 * time.Second` constant
- New `lastPullAt time.Time` field on `CondocSession`
- New `runGitPullRebase(dir, branch)` function — fires a `condocPullDoneMsg` when done
- `handleCondocTick` now checks if 30s have elapsed while in `condocPhaseAwaitingStep` or `condocPhaseAwaitingAction` (not during branching, committing, or agent runs), and kicks off the pull; returns `tea.Batch(condocTickCmd(), pullCmd)` so both the tick and the pull fire
- New `handleCondocPullDone` — only surfaces errors as a status message; silent on success
- `main.go`: added `case condocPullDoneMsg` dispatch

**Heading spacing** (`condoc.go`): Changed all places where a `##`/`###` heading follows content from one blank line (`\n\n`) to two blank lines (`\n\n\n`):
- `writeProposalFile` — before `## Human-Prompt`
- `addHumanPrompt` — before `## Human-Prompt` (affects all callers)
- `templateStep` — before `### Step N`
- `addRevisionTemplate` — before `## <REPLACE-Revision|Retry> A` and before `## Human-Prompt`
- `addNextRevisionTemplate` — same two locations
- `finalizeStepFile` — before `## Step Completed`
- `finalizeMainFile` — before `## Condoc Completed`
- Agent prompts — updated to request two blank lines before `## Reply` / `## Reply X`


# Reprompt C

We have now updated our examples in AI-sandboxing/discussion/condocs-examples/ so that they properly create the links between the parent condocs and the child step files. We have also articulated the difference between verbose and standard presentation. (Note that we originally implemented verbose mode but did not add a flag for it.)

Now we will update the condocs implementation in AI-evo1/federation-command/ to cover verbose mode and to properly add the links.


## Reply

Both features are implemented in `AI-evo1/federation-command/condoc.go` with a small update to `main.go`.

**Verbose mode** (`condoc.go:61,759`): Added `verbose bool` field to `CondocSession`. `parseCondocCommand` now returns a third `verbose bool` value and strips `-v`/`--verbose` from the argument list before parsing the file path. `NewCondocSession` accepts `verbose` and stores it on the session. All five Human-Prompt-generating functions (`writeProposalFile`, `templateStep`, `updateMainAfterStepStart`, `addRevisionTemplate`, `addNextRevisionTemplate`) now branch on `cs.verbose`: verbose mode keeps the existing detailed text; simple mode uses the brief one-liner text matching the `simple/` examples.

**Ordinal helper** (`condoc.go:116`): Added `ordinal(n int) string` returning "first"–"tenth" for n=1–10, falling back to `"step N"` for larger values. Used by `templateStep` and `updateMainAfterStepStart` to produce "first step", "second step", etc. in Human-Prompt text.

**Backlink in step file** (`condoc.go:362`): `writeStepFile` now takes `mainFilePath` as an additional parameter and inserts `[BaseName](../BaseName.md)` between the `# Prompt` heading and the prompt text, matching the format shown in all `simpleImpls/Step1Prompt.snp*.md` examples.

**Step link in main file** (`condoc.go:374`): New `insertStepLink` helper uses a compiled regex to find the filled-in `### Step N - Title` heading (skipping `<REPLACE-TITLE>` templates) and appends `[Step N Prompt](simpleImpls/StepNPrompt.md)` directly after it. `updateMainAfterStepStart` calls this before writing the Human-Prompt section, producing the snp4 layout seen in the examples.

**`main.go`**: Updated the `condoc` command block to destructure the new `verbose` return value from `parseCondocCommand` and pass it to `NewCondocSession`; updated the usage error message to show `[-v|--verbose]`.

Done. Here's a summary of what changed across the two files:

**`condoc.go`**:
- `CondocSession` gains `verbose bool`
- New `ordinal(n)` helper (1→"first", 2→"second", …10→"tenth")
- `parseCondocCommand` returns `(filePath, description, verbose)` and strips `-v`/`--verbose` flags
- `NewCondocSession` signature gains `verbose bool` parameter
- `writeStepFile` gains `mainFilePath` param — inserts `[BaseName](../BaseName.md)` after `# Prompt`
- New `insertStepLink` — regex-finds the filled-in step heading and appends the `[Step N Prompt](...)` link
- `updateMainAfterStepStart` gains `stepNum, verbose` — calls `insertStepLink` then writes verbose or brief Human-Prompt
- `writeProposalFile`, `templateStep`, `addRevisionTemplate`, `addNextRevisionTemplate` all accept `verbose` and branch between the existing detailed text (verbose) and the brief one-liners from the `simple/` examples (standard)

**`main.go`**: Updated the `condoc` command handler to receive `verbose` from `parseCondocCommand` and pass it to `NewCondocSession`; usage message updated to show `[-v|--verbose]`.


# Reprompt D

We are having an issue with our implementation where the agent is writing directly to the condoc files - we should add some context so that the agent knows it should never write directly to the condoc files and that the response it sends to the terminal will go there automatically.

Somehow when we executed last time the agent sent one response to the terminal and another went into the condoc. We don't want the agent to ever write to the condoc, only the handler should do that.

Let's update the implementation in `AI-evo1/federation-command/` so agents do not write to the condocs.


## Reply

Four changes were made to `AI-evo1/federation-command/condoc.go` to fully decouple agent output from condoc file writes:

**Agent prompt updates** (`buildCondocStepPrompt`, `buildCondocRevisionPrompt`): Removed the instruction to write a `## Reply` section to the step file. Both prompts now carry an explicit `IMPORTANT: Do NOT write to or modify the step file or any other condoc files — the condoc handler will automatically capture your terminal response as the reply.` directive.

**Terminal output capture** (`condocStartStep`, `condocRunRevision`): Each agent invocation now generates a temp file path (`/tmp/condoc-reply-<nanos>.txt`), stores it as `cs.replyTmpPath`, and wraps the agent command with `teeCommand(agentCmd, replyTmpPath)` so the agent's full terminal output is captured alongside the normal display.

**Handler-side reply writing** (`handleCondocAgentDone`): After the agent exits, the handler reads the temp file, strips ANSI escape codes and CR-overwritten lines via the new `stripANSI` helper, and writes the cleaned text under `## Reply` (or `## Reply X` for revisions) using the new `appendReplyToStepFile` helper. The temp file is deleted after use. The prior logic that inferred the reply letter by re-reading the step file is replaced by the new `cs.pendingRevLetter` field (set to `""` in `condocStartStep` and to the revision letter in `condocRunRevision`).

**New helpers** (`stripANSI`, `appendReplyToStepFile`, `ansiEscRe`): `stripANSI` handles both `ESC[…` SGR sequences and bare escape sequences, then collapses any `\r`-overwritten lines. `appendReplyToStepFile` is the single authoritative place that writes reply content into a step file.


# Reprompt D

Things are looking better now.

We need just a couple more adjustments, then this phase of implementation looks good:
- When we go use the !COMPLETE! directive we see that the template for the most recent step is not cleaned up when the step file is marked complete (true with one or two invocations within a step)
- When we echo the agent's response we end up getting the metadata lines 'invoking claude in write (files read/write) mode...' etc -- we don't want to include that. We should add a path to silence that metadata.


## Reply

Both changes look correct. Here's a summary of what was implemented:

**Fix 1 — Unfilled revision template cleanup** (`condoc.go:481`):
- New `removeUnfilledRevisionTemplates` function strips any `## <REPLACE-Revision|Retry> X` block still containing `<REPLACE-PROMPT>` (i.e., the human added `!COMPLETED!` without filling it in).
- `finalizeStepFile` now calls it after `removeHumanPromptSection`, so the completed step file contains only real content followed by `## Step Completed`.
- Works for both the "one invocation" case (un-revised `## <REPLACE-Revision|Retry> A`) and the "two invocations" case (post-revision `## <REPLACE-Revision|Retry> B`).

**Fix 2 — Silence agent metadata** (`ambiguous-agent/main.go:419`, `condoc.go:989,1096`):
- `ambiguous-agent` now reads `AA_QUIET=true` and suppresses all its metadata output: the "invoking … mode…", "● session:", "● providing records from:", "cleaned up records temp dir:", "agent completed successfully / exited with code N" lines.
- Both `condocStartStep` and `condocRunRevision` append `AA_QUIET=true` to the agent command's environment before wrapping with `teeCommand`, so the captured reply contains only the agent's actual response.


# Reprompt E

Our last iteration broke the '## Reply' process is now not working for '## Revision' steps.

I was testing in a remote case, where I pushed updates to the repo from a rmeote client and they were pulled down. I have now tested in both local and remote however, and it fails in both cases - but only for revisions, not for the original invocation in a new step.

Please fix.
