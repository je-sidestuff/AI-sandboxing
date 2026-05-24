# Prompt

We want slopspaces (in AI-evo1) to now support a concept of 'syncing'. This is the process by which slopspaces 'expose their potential' to the outside world. In the case of a repo read or writespace this means pushing and pulling to/from the remote. In the case of a dtt read or writespace this means doing a read on any resources referenced (ones for a query or ones that are about to be written) and projecting what the changes would be to arrive at the outcome state. In each case the point is that the content is brought to a state where it is compared against the outside world so the outcome can be predicted in the case of acceptance (conceptually similar to a 'terraform plan').

Syncing in slopspaces nomrally occurs at two points: immediately before a deploy occurs, and immediately after a return occurs. Slopspaces have a 'sync mode' -- no new arguments are required compared to our previous increment, and if the optional 'sync mode' argument is not provided then this mode will default to 'auto-sync'. In this case a sync occurs after the deploy is called before the deploy happens, and also after return is called, after the return occurs but before the overall return process ends.

The sync mode of a slopspace is inherited by all writespaces in that slopspace. (In the future we may have individual sync modes but for now we'll stick with the single slopspace level)

In the future slopspaces will sync in order to make themselves ready to propose their changes. In order to propose the change they will use their 'interaction surface'. Different writespaces allow different interaction surfaces - the two types we will support in our initial implementation are the 'filesystem' and 'github PR'. We will worry about these implementations later - they will be outside the view of dungeon-keeper but will be within the view of claudomation.

For our initial increment we will implement only 'sync' functionality, with only the 'auto-sync' mode, and only for repo-type read and writespaces. We will plumb this functionality into dungeok-keeper, and it should come into claudomation for free and should not impose additional baggage.

Respond first with a proposed implementation with all gaps filled in by assumptions (which are stated), and then respond with a series of clarifying questions so we can refine this implementation. 

Assume we want to support all existing content with only minor change to behaviour and that we will also create a new example for claudomation and new distinct ridealongs for both dungeon-keeper and claudomation to explore this new functionality.

## Reply

Now I have a solid understanding of the codebase. Here is my proposed implementation followed by clarifying questions.

---

## Proposed Implementation

### Assumptions

1. **Pre-deploy sync (pull)**: Freshens both readspace and writespace repos from remote before the agent runs — readspaces get re-pulled from their canonical clones, writespaces get `git pull --rebase` against origin.
2. **Post-return sync (push)**: After the agent returns, all writespace repo changes are committed and pushed to their respective remote branches (equivalent to calling `WriteAllRepoChanges` automatically).
3. **Sync errors are fatal**: If sync fails, the deploy/return call fails and returns an error. This prevents agents running on stale data or changes being silently lost.
4. **Backward compatibility**: Existing slopspaces whose `SLOPSPACE.json` lacks a `SyncMode` field will default to `auto-sync` at read time (Go zero-value → treated as auto-sync).
5. **No new manual sync CLI subcommand** in this increment — sync is an internal concern of Deploy/Return.
6. **DTT sync is a no-op** in this increment (placeholder path, no implementation).
7. **Auto-generated commit message** for post-return pushes: `"slopspace auto-sync: iteration {N}"` using the existing iteration counter.
8. **Claudomation gets sync for free**: `execute.py` calls `dungeon-keeper slopspace deploy`; the watch loop calls `Return` — both will auto-sync without changes to claudomation. No new terraform variables needed.
9. **Sync mode is stored in `SLOPSPACE.json`** so it persists across iterations.
10. **No change to the readspace canonical clone refresh** on pre-deploy — the sync only updates the slopspace copy from the already-cloned canonical; it does not re-pull from GitHub into the canonical (that remains a separate operation). If canonical is stale, it's stale.

> Assumption 10 is the trickiest. See clarifying question #1.

---

### Changes Required

#### 1. `dungeon-keeper/pkg/types/types.go`

Add `SyncMode` type and constant, add field to `SlopspaceMetadata`:

```go
type SyncMode string

const (
    SyncModeAutoSync SyncMode = "auto-sync"
    // Future: SyncModeManual, SyncModeNone
)

type SlopspaceMetadata struct {
    Slopspace
    WorkSignalPath    string
    Iteration         int
    DeployedAgentType AgentType
    SyncMode          SyncMode  // defaults to auto-sync when empty
}
```

`SyncMode` field with empty string → treated as `SyncModeAutoSync` throughout. No migration of existing files needed.

---

#### 2. `dungeon-keeper/pkg/slopspace/slopspace.go`

Add two internal functions and wire them into `Deploy` and `Return`:

```go
// syncPreDeploy runs before deploy: pulls fresh content into readspaces
// and writespaces from their remotes.
func syncPreDeploy(id string) error {
    meta, err := ReadMetadata(id)
    if err != nil { return err }
    if meta.SyncMode != SyncModeAutoSync && meta.SyncMode != "" {
        return nil
    }
    // Pull each readspace repo:
    //   - git pull --rebase on canonical at /host-agent-files/readspaces/repos/<owner>/<repo>
    //   - re-copy content (sans .git) into slopspace readspaces path (overwrite)
    // Pull each writespace repo:
    //   - restore .git from writespaces-secure/<owner>/<repo>
    //   - git pull --rebase origin <branch>
    //   - move .git back to writespaces-secure
    return nil
}

// syncPostReturn runs after return: commits and pushes writespace repo changes.
func syncPostReturn(id string) error {
    meta, err := ReadMetadata(id)
    if err != nil { return err }
    if meta.SyncMode != SyncModeAutoSync && meta.SyncMode != "" {
        return nil
    }
    msg := fmt.Sprintf("slopspace auto-sync: iteration %d", meta.Iteration)
    return WriteAllRepoChanges(id, msg)
}
```

`Deploy` change — insert before the move:

```go
func Deploy(id string, agentType AgentType) error {
    // ... existing validation ...
    if err := syncPreDeploy(id); err != nil {
        return fmt.Errorf("pre-deploy sync: %w", err)
    }
    // ... existing deploy logic (move readspaces/writespaces) ...
}
```

`Return` change — insert before return exits:

```go
func Return(id string) error {
    // ... existing return logic (move writespaces back, discard readspaces) ...
    if err := syncPostReturn(id); err != nil {
        return fmt.Errorf("post-return sync: %w", err)
    }
    return nil
}
```

---

#### 3. `dungeon-keeper/main.go`

Add optional `--sync-mode` flag to `slopspace create`:

```
dungeon-keeper slopspace create [--sync-mode auto-sync]
```

Pass it through to `slopspace.Create()`. All existing callers omit it and get `auto-sync`.

---

#### 4. New: `dungeon-keeper/docs/sync-ridealong.md`

Ridealong demonstrating:
1. Clone a repo into readspaces and writespaces
2. Create a slopspace (default auto-sync)
3. Deploy → watch pre-deploy sync (pull) log output
4. Agent (clod) makes a change to the writespace
5. Return → watch post-return sync commit + push log output
6. Verify the branch on the remote has the new commit

---

#### 5. New: `claudomation/examples/sync/`

Mirrors the structure of `claudomation/examples/execution/` with:
- `main.tf` — references execution module
- `ridealong.md` — same lifecycle as above but driven through terraform/claudomation
- `README.md`

No changes to `claudomation/modules/execution/` or `execute.py` — sync is transparent.

---

## Clarifying Questions

**1. Pre-deploy readspace sync depth** — Assumption 10 above says we only re-copy from the already-cloned canonical; we do *not* `git pull` the canonical from GitHub. Is that right, or should pre-deploy sync also pull the canonical from remote so the agent always gets the freshest data? (The latter is closer to the terraform-plan analogy but makes sync slower and network-dependent.)

**2. Pre-deploy writespace sync direction** — I assumed pre-deploy *pulls* writespaces (fetch latest from remote). But should pre-deploy also *push* the current writespace state to a preview/shadow branch on origin? That would "expose potential" in the sense that someone could inspect the remote branch to see what the slopspace currently contains *before* the agent runs.

**3. Sync failure policy** — I assumed fatal (deploy/return returns error). Should sync failures instead be logged as warnings and allow the operation to continue? The case for non-fatal: network flakiness shouldn't block an agent from running.

**4. Post-return sync when writespace has no changes** — `WriteRepoChanges` already checks for changes and skips commit if tree is clean. Should it also skip the push, or push anyway (to set upstream tracking)? Current assumption: skip both commit and push if no changes.

**5. Commit message for auto-sync** — Is `"slopspace auto-sync: iteration N"` acceptable, or should we pull context from the work signal prompt? Work signal prompt is available in the dungeon-keeper watch loop but not inside `slopspace.Return()` without threading it through.

**6. SyncMode on `slopspace create` vs. at call sites** — Should `dungeon-keeper slopspace deploy` also accept `--sync-mode` (to override per-deploy)? Or is it strictly set once at create time and stored in metadata for all iterations?

**7. Ridealong target repo** — Both existing ridealongs target the `AI-evo1` repo itself. Should the new sync ridealongs do the same (simplest), or target a dedicated toy/demo repo to avoid polluting AI-evo1's branch list?

**8. Claudomation example structure** — The sync example will be functionally identical to the execution example except it will produce an observable git push. Should it use a different prompt that makes the repo change visible (e.g., clod appends a line to a file), or is it fine to use the same generic prompt and just observe the sync side-effect in logs?


## Reprompt A

Question answers:

1 - We DO want to re-pull from the canonical. This probably means we will need to have a readspace-secure paradigm.

2 - The pull is fine, stick with the assumption.

3 - Good assumption, we'll start with that for this iteration.

4 - Skip both if we made no changes, but 'pull --rebase' before the push if we can do so cleanly with no conflicts.

5 - This is a good starting point.

6 - Keep it to on-create for now.

7 - Point to AI-evo1 for now, but we will clean this up soon.

8 - Make it so clod creates or modifies a file (remember the special instruction syntax) so we can see the difference.

Let's implement this understanding! Make the changes in AI-evo1.


## Reply

All done. Here's a summary of every change made:

---

## Changes made

### `dungeon-keeper/pkg/types/types.go`
- Added `SyncMode` type (`"auto-sync"`) and `SyncModeAutoSync` constant
- Added `SyncMode SyncMode \`json:"sync_mode,omitempty"\`` field to `SlopspaceMetadata`; empty value treated as auto-sync everywhere, so existing `SLOPSPACE.json` files need no migration

### `dungeon-keeper/pkg/slopspace/slopspace.go`
- Added `ReadSpacesSecure = "readspaces-secure"` constant (mirrors `WriteSpacesSecure`)
- **`Create(syncMode)`** — new parameter; stores sync mode in metadata
- **`AddReadspaceRepo`** — changed from deleting `.git` to moving it into `readspaces-secure/repos/<owner>/<repo>`, so pre-deploy sync can pull from remote
- **`Deploy`** — calls `syncPreDeploy(rootPath, syncMode)` before moving dirs; pulls latest into all readspace and writespace repos
- **`Return`** — calls `syncPostReturn(id, syncMode, iteration)` after writing metadata; commits and pushes all writespace repo changes
- **`WriteRepoChanges`** — added `git pull --rebase` between commit and push (per Q4 answer: pull cleanly before pushing when changes exist)
- New helpers: `isAutoSync`, `syncPreDeploy`, `syncPostReturn`, `syncRepoDir`, `pullWithSecureGit`, `gitPullRebase`

### `dungeon-keeper/main.go`
- `slopspace create` now accepts optional `--sync-mode auto-sync` flag; displays sync mode in output
- Worker's internal `Create()` call updated to pass `types.SyncModeAutoSync`
- Usage text updated to document the flag and auto-sync behaviour

### `dungeon-keeper/docs/sync-ridealong.md` _(new)_
Ridealong demonstrating the full sync lifecycle: clone → create slopspace → deploy (pre-deploy sync logged) → agent creates file → Return (post-return sync pushes commit) → verify branch on remote with no manual `slopspace write`

### `claudomation/examples/sync/` _(new)_
- `main.tf` / `variables.tf` / `providers.tf` / `outputs.tf` — same structure as the execution example; default prompt has the agent create a file in the writespace repo so the sync push is observable
- `ridealong.md` — walks through repo setup outside terraform, then `terraform apply`, then verifying the auto-pushed commit on the remote branch
- `README.md` — module documentation
