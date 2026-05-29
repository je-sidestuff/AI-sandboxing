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
