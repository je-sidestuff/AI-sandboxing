# Prompt

We want slopspaces to now support a concept of 'proposing'. We want to have each slopspace have a 'proposal mode' -- no new arguments are required, and if the optional 'proposal mode' argument is not provided then this mode will default to 'auto-propose'.e'. 

The proposal mode of a slopspace is inherited by all writespaces in that slopspace. (In the future we may have individual proposal modes but for now we'll stick with the single slopspace level)

Each type of writespace proposes its changes differently. A writespace repo pushes its changes to its branch as part of the 'sync' phase which happens before proposing but after returning the slopspace. In order to propose the change it will use its 'interaction surface'. Different writespaces allow different interaction surfaces - the two types we will support in our initial implementation are the 'filesystem' and 'github PR'.

Respond first with a proposed implementation with all gaps filled in by assumptions (which are stated), and then respond with a series of clarifying questions so we can refine this implementation.

Assume we want to support all existing content with only minor change to behaviour and that we will also create a new example for claudomation and new dist
inct ridealongs for both dungeon-keeper and claudomation to explore this new functionality.


## Reply

Now I have a thorough understanding of the codebase. Here's my full implementation proposal, followed by clarifying questions.

---

## Implementation Proposal: Slopspace Proposal Mode

### Core Concepts

**Proposal modes** (stored on the slopspace, inherited by all its writespaces):
- `auto-propose` (default): `slopspace write` automatically triggers `propose` after sync
- `manual`: `propose` must be called explicitly
- `none`: no proposal step ever runs

**Interaction surfaces** (derived from writespace type, not configurable per-writespace):
- Repo writespaces → `github-pr`
- All other writespace types (files, dtt-canvas, agent-records) → `filesystem`

---

### Assumption A: "Filesystem" proposals write a manifest file

For non-repo writespaces, the proposal writes a `proposals/<type>/<id>.json` into the slopspace root:

```json
{
  "created_at": "...",
  "writespace_type": "files",
  "slopspace_id": "...",
  "status": "pending"
}
```

This gives an external signal that changes exist and are ready for review — useful for dtt-canvas outputs or file trees that an external process might pick up.

---

### Assumption B: Proposal metadata is tracked per-writespace in the slopspace

A new `WRITESPACE_REPOS.json` (or equivalent section in `SLOPSPACE.json`) tracks:

```json
{
  "repos": [
    {
      "owner": "org",
      "repo": "my-repo",
      "branch": "slopspace-abc123-feature",
      "proposed_at": null,
      "proposal_url": null
    }
  ]
}
```

This lets `propose` find the branch name without needing a live `.git` directory.

---

### Assumption C: GitHub PR creation uses the REST API with the existing PAT

New `propose.go` in `dungeon-keeper/pkg/slopspace/` calls `https://api.github.com/repos/<owner>/<repo>/pulls` using `TF_VAR_github_pat`. Default PR title = the commit message from the sync phase. Default PR body includes the slopspace ID, agent type, model, and the original prompt from the work signal (if available).

PR creation is idempotent: if a PR already exists for the branch, the command returns the existing URL without error.

---

### Assumption D: Auto-propose wires into `slopspace write`, not a separate orchestration call

When `slopspace write repo <id> <owner/repo>` completes a sync and the slopspace's proposal mode is `auto-propose`, it immediately runs the proposal step. Scripts calling `write` get the full behavior without changes. The `propose` command remains callable independently for `manual` mode.

---

### Schema changes (`types.go`)

```go
type ProposalMode string

const (
    ProposalModeAutoPropose ProposalMode = "auto-propose"
    ProposalModeManual      ProposalMode = "manual"
    ProposalModeNone        ProposalMode = "none"
)

// SlopspaceMetadata gets one new field:
type SlopspaceMetadata struct {
    // ...existing fields...
    ProposalMode ProposalMode `json:"proposal_mode"`
}
```

`ProposalMode` zero-value is empty string; reading metadata treats `""` as `auto-propose` for backwards compatibility with existing slopspaces.

---

### CLI changes (`main.go`)

```
dungeon-keeper slopspace create [--proposal-mode <mode>]

dungeon-keeper slopspace propose <id>                          # all writespaces
dungeon-keeper slopspace propose repo <id> <owner/repo>        # specific repo
    [--pr-title <title>]   # defaults to commit message
    [--pr-body  <body>]    # defaults to generated template
```

`slopspace write` behavior is unchanged in interface; the auto-propose trigger is internal.

---

### Full lifecycle with proposal

```
1. create slopspace  [--proposal-mode auto-propose]
2. add-writespace repo ... --ref slopspace-<id>-<feature>
3. deploy
4. agent works
5. return          ← slopspace returned, .git restored
6. write all       ← sync: git add -A, commit, push branch to remote
                      if auto-propose: automatically runs step 7
7. propose         ← github-pr: POST /repos/.../pulls
                      filesystem: write proposals/<type>/<id>.json
```

---

### New files

| File | Purpose |
|---|---|
| `dungeon-keeper/pkg/slopspace/propose.go` | `ProposeRepoChanges()`, `ProposeFilesystem()`, `ProposeAll()`, GitHub API helper |
| `dungeon-keeper/docs/proposal-ridealong.md` | dungeon-keeper ridealong: manual proposal flow |
| `claudomation/examples/proposal/main.tf` | Terraform example using auto-propose |
| `claudomation/examples/proposal/ridealong.md` | Claudomation ridealong: auto-propose creating a PR |

---

### Backwards compatibility

- Existing slopspaces with no `proposal_mode` field are treated as `auto-propose`
- `write` commands already in scripts get auto-propose behavior for free
- No changes to `deploy`, `return`, or readspace commands

---

## Clarifying Questions

1. **What are the other proposal modes besides `auto-propose`?** I assumed `manual` and `none` — is that right, or are there others (e.g. `dry-run`, `draft-pr`)?

2. **Should the GitHub PR be created as a draft PR?** A draft PR is less disruptive for automated changes, but it requires explicit review to mark ready. Which should be the default?

3. **Where does the PR title/body come from in the automated path?** My assumption: the commit message from the sync phase becomes the PR title, and the work signal prompt becomes the body. Is there a better source, or should these be set explicitly per-execution?

4. **Should existing `slopspace write` scripts pick up auto-propose automatically** (my current assumption), or should the proposal step only run when `propose` is called explicitly, even in `auto-propose` mode? The distinction matters for scripts that currently call `write` and whose callers don't expect a PR to appear.

5. **For the claudomation example**: should it use a real repo (like the existing `AI-evo1` examples do), or a dummy/placeholder repo that readers substitute in?

6. **Filesystem proposal format**: I assumed a JSON manifest file in the slopspace. Is there a specific format or location that makes more sense for how non-repo writespace outputs are currently consumed?

7. **Should `propose` record its output (PR URL, etc.) back into the slopspace metadata**, or into the ledger only? I assumed both — the slopspace tracks `proposal_url` per writespace repo, and the ledger gets an event.

8. **Interaction surface configurability**: I assumed the surface is fixed per writespace type (repos always use `github-pr`). Should it be overridable at the slopspace or writespace level (e.g. a repo writespace that proposes via filesystem for testing)?
