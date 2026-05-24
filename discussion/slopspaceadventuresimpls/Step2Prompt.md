# Prompt

Now we need to think about how the process of a slopspace syncing and 'showing the potential of its changes on the outside world' flows into the concept of 'assignments' and the process of 'approval' and/or local and/or remote 'applying' of writes.

An 'assignment' is the high level encapsulation of a sequence of operations managed through slopspaces. Each assignment is broken down into 'phases', which represent distinct sub-goals of the overall operation, which are in turn broken down into 'steps' which represent the smallest pieces of work which are worth encapsulating. Phases and steps progress linearly (we can assume serially now but we will have parallel capbilities in the future) and when we complete one we cannot go back. A step is not necessarily a single invocation and may entail multiple 'retry' or 'revise' invocations.

(A 'revise' is another invocation stacked on top which maintains the previously completed work. A 'retry' discards the previous work but keeps it visible to the agent working on the next attempt for reference.)

We'll describe hypothetical scenarios for reference as we model the next increments of development:


1. A PR is raised by claudomation after an AI performs an increment of work on the single repo writespace. The PR is raised on the target repo itself. The github user merges the PR. This is considered an externally applied write as well as an approval of the work.

This was an assignment with the shorthand 'branch isolation flow'.
- It has a repo as a target which becomes a writespace
- It has one 'step' to execute a prompt on that target repo
  - We can have the option of 'revise' requests or 'retry' requets
  - Should we present 'retry' commits in a readspace?
- Assume we do not allow other writespaces in our simplest case
- Assume we will start with in-repo PR interaction surface
- Assume we will also allow filesystem interaction surface
- Assignment completes on the single 'step' completion
- We do not need sloppo (an AI tracker repo) for this
- Further considerations:
  - If we used the filesystem interaction surface to approve the proposed changes then our PR would be merged by claudomation the next time we did a 'terraform apply' (approval followed by internal apply)


2. A PR is raised in the sloppo repo after an AI performs an increment of work that it intends to write as content to a new github issue. The github user merges the PR - this signals the write to occur. This is considered an approval of the work followed by an internally applied write.

This is a new type of assignment we can shorthand as a 'simple dtt write flow'.
- It has one writespace dtt canvas in this case (but probably could have many)
- It has one 'step' to execute a prompt and create as-code changes to be written to the dtt target
  - We can support 'revise' and 'retry' here as well
- Assume we will use sloppo PR interaction surface
- Assume we will also allow filesystem interaction surface
- Assignment completes on the single 'step' completion
- Further considerations:
  - If we used the filesystem interaction surface to approve the proposed changes then our PR would be merged by claudomation the next time we did a 'terraform apply' (approval followed by internal apply)


3. A PR is raised in the sloppo repo after an AI performs an increment of work that declaratively describes the shape that this assignment will take. This heuristic request will spell out the type of flow to be used for the remainder of the assignment - at this point we will constrain it to one of the aforementioned types. The github user merges the sloppo PR to signal that the proposed assignment continuation proposal is accepted. The declared assignment details contain the writespace and other configuration details, once accepted this proposal will result in the slopspace being reconfigured and the assignment execution to continue. The remaining steps will be akin to what we saw in (1.) and (2.).

This was an assignment with the shorthand 'heuristic request to <x-flow>'.
- The heuristic request 'phase' has a write surface for declaring the assignment details
- The heuristic request (HR) 'phase' may be subject to 'revise' or 'retry' increments as well
- Assume we will use sloppo PR interaction surface for the HR 'phase'
- Assume we will also allow filesystem interaction surface
- The HR 'phase' completes on the single 'step' completion
- The second phase will be basically identical to the single phase in (1.) or (2.)

We will assume that the knowledge of these interactions is distributed as follows:
- dungeon-keeper (and the harnessed agent) only knows about slopspaces and the context of using declarative tools to create effects on the world. Does not *need to* know about its work in the context of an assignment. The work is always handed down in a focused way. Sometimes dungeon-keeper's invocation goals involve crafting new steps, phases, or assignments - in which case it will have the context it needs then -- but only in these cases.
- claudomation has full knowledge of how the high level structures of assignments, phases, and steps translate into the low level details of slopspaces, sessions, and executions. Although claudomation has no continuous behaviour (it is always externally invoked) it is able to complete all operations if it is repeatedly invoked - even a 'while true; terraform apply --auto-approve' would bring all activity to conclusion.
- agent-dispatch has awareness over the high level structures of assignments, phases, and steps, and has the ability to observe signals that indicate it is time to trigger claudomation and have assignments progress. It does not reach into assignments and modify them directly, it always behaves as an orchestrator.

Let's start by crating a plan for an incremental implementation which covers:
- Upgrades to dungeon-keeper and claudomation as-needed to accomplish scenario (1.)
  - Modeling of the assignment, phase, and step data structures
  - Modeling of the revise and retry processes
- No creation of agent-dispatch just yet

In order to plan this implementation we will first create a 'Proposed Implementation' section where we create a specific implementation by making as many assumptions (and stating them) as we need to without stopping to ask for further instructions. After the 'Proposed Implementation' section we will follow up with a 'Clarifying Questions' section where we ask as many questions as are needed to iron out details which were uncertain.


## Reply

Now I have everything I need. Let me write the response.

---

## Proposed Implementation

### Assumptions

1. **Interaction surfaces belong to claudomation, not dungeon-keeper.** The sync step (Step 1) already pushes the work branch. Everything after that — PR creation, polling, approval — is claudomation's domain. No new dungeon-keeper commands are needed for scenario 1.

2. **ASSIGNMENT.json is the single source of truth.** Assignment state is persisted as one JSON file per assignment. Phases, steps, and invocations are embedded arrays (not a directory hierarchy). The file is written atomically via temp-file → `os.replace()`.

3. **Assignment directory** lives at `$ASSIGNMENTS_DIR/<assignment-id>/ASSIGNMENT.json` as a sibling to `$SLOPSPACES_DIR`, not inside it.

4. **The orchestrator is a pure state machine.** `orchestrate.py` is idempotent — each terraform apply advances the state machine by at most one stage, saves ASSIGNMENT.json, and exits. A subsequent apply resumes from saved state. This matches the design principle that claudomation has no continuous behavior.

5. **Revise** = redeploy the same slopspace (same ID, same writespace, prior commits remain on the work branch). A new work signal is created whose prompt is the original prompt with the feedback appended. The open PR on the target repo automatically shows the new commits.

6. **Retry** = claudomation clones the current branch state into a readspace (for reference), force-resets the work branch to the base branch HEAD, creates a **new slopspace** with a fresh writespace and the old attempt in a readspace at `repos/previous-attempt/<owner>/<repo>/`. The work signal prompt notes that the previous attempt is available there.

7. **Retry branch strategy**: force-reset the existing work branch (not a new branch). Simpler PR story — one PR per step. Assumption: rewriting branch history is acceptable for a work branch no one else is tracking.

8. **Revise/retry signals** for the in-repo PR surface come from PR comments with the exact prefixes `REVISE:` (followed by feedback) or `RETRY` (standalone). For the filesystem surface, they come from sentinel files `REVISE.json` and `RETRY` respectively in the `approval_signal_path` directory.

9. **Filesystem approval** = a `APPROVED` sentinel file appears in the `approval_signal_path` directory. On detection, claudomation merges the PR via the GitHub API (internal apply).

10. **No ledger separate from ASSIGNMENT.json** for now. ASSIGNMENT.json is the audit trail. We can add an append-only JSONL ledger later if needed.

11. **A `holder` field** on the current invocation (set to a claudomation invocation UUID) prevents two concurrent terraform applies from double-advancing the same step.

---

### Data Structures

#### ASSIGNMENT.json

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "branch-isolation-flow",
  "name": "Add retry handler to auth service",
  "status": "active",
  "created_at": "2026-05-23T10:00:00Z",
  "updated_at": "2026-05-23T10:05:00Z",
  "config": {
    "target_repo": "myorg/myrepo",
    "base_branch": "main",
    "work_branch": "assignment-550e8400",
    "interaction_surface": "in-repo-pr",
    "approval_signal_path": null
  },
  "current_phase_index": 0,
  "phases": [
    {
      "id": "phase-uuid",
      "index": 0,
      "name": "execution",
      "status": "active",
      "started_at": "2026-05-23T10:00:01Z",
      "completed_at": null,
      "current_step_index": 0,
      "steps": [
        {
          "id": "step-uuid",
          "index": 0,
          "type": "agent-execution",
          "status": "awaiting-approval",
          "prompt": "Add a retry handler to the auth service...",
          "slopspace_id": "abc123",
          "proposal_url": "https://github.com/myorg/myrepo/pull/42",
          "started_at": "2026-05-23T10:00:02Z",
          "completed_at": null,
          "current_invocation_index": 0,
          "invocations": [
            {
              "id": "inv-uuid",
              "index": 0,
              "type": "initial",
              "status": "completed",
              "holder": null,
              "work_signal_id": "ws-uuid",
              "revision_feedback": null,
              "reference_invocation_id": null,
              "reference_slopspace_id": null,
              "started_at": "2026-05-23T10:00:03Z",
              "completed_at": "2026-05-23T10:04:00Z"
            }
          ]
        }
      ]
    }
  ]
}
```

#### Status values

| Entity | States |
|--------|--------|
| Assignment | `pending` → `active` → `completed` / `failed` / `abandoned` |
| Phase | `pending` → `active` → `completed` / `failed` |
| Step | `pending` → `active` → `awaiting-approval` → `completed` / `failed` |
| Invocation | `pending` → `active` → `completed` / `failed` |

#### Invocation `type` values

| Type | Meaning |
|------|---------|
| `initial` | First attempt |
| `revise` | Stacked on prior; `revision_feedback` carries the feedback; same slopspace/branch |
| `retry` | Fresh attempt; `reference_invocation_id` and `reference_slopspace_id` point to prior; prior work visible in readspace |

---

### State Machine (orchestrate.py)

Each terraform apply runs through this once and exits. The `status` fields in ASSIGNMENT.json tell it where to resume.

```
load ASSIGNMENT.json  (or create it if this is the first apply)

current step = phases[current_phase_index].steps[current_step_index]
current inv  = step.invocations[step.current_invocation_index]

match step.status:

  "pending":
    → clone target repo to writespace path
    → create slopspace (auto-sync mode)
    → add writespace repo (work_branch)
    → create initial invocation record, set status "active", set holder
    → write ASSIGNMENT.json
    → deploy slopspace
    → create work signal (prompt = step.prompt)
    → save work signal ID to current invocation
    → exit (next apply will poll)

  "active":
    if current inv has no holder → set holder, write ASSIGNMENT.json
    check work signal status:
      still running → exit (come back next apply)
      completed →
        return slopspace  (auto-sync fires: commits + pushes work_branch)
        clear holder
        create/update interaction surface:
          in-repo-pr  → POST GitHub PR (or find existing for the branch)
          filesystem  → write approval-request JSON to approval_signal_path
        set step.proposal_url (or approval_signal_path)
        set step.status = "awaiting-approval"
        write ASSIGNMENT.json, exit

  "awaiting-approval":
    check for signals on interaction surface:
      REVISE signal found →
        extract feedback text
        close revise signal (delete comment marker or REVISE.json)
        new invocation = {type:"revise", revision_feedback:..., status:"pending"}
        append to step.invocations, advance current_invocation_index
        set step.status = "active"
        write ASSIGNMENT.json
        → immediately re-enter "active" handling (redeploy same slopspace, new work signal)
      RETRY signal found →
        snapshot current branch → readspaces/repos/previous-attempt/<owner>/<repo>/
        force-reset work_branch to base_branch HEAD
        delete old slopspace
        new slopspace = fresh writespace (base branch) + previous-attempt readspace
        new invocation = {type:"retry", reference_invocation_id:..., reference_slopspace_id:..., status:"pending"}
        set step.status = "active"
        write ASSIGNMENT.json
        → immediately re-enter "active" handling
      Approved (PR merged / APPROVED sentinel file) →
        set step.status = "completed", step.completed_at = now
        set phase.status = "completed", phase.completed_at = now (only 1 step)
        set assignment.status = "completed", assignment.updated_at = now
        write ASSIGNMENT.json, exit
      Nothing yet → exit (come back next apply)
```

---

### New Files

```
claudomation/
├── modules/
│   └── assignment/
│       └── branch-isolation/
│           ├── orchestrate.py     ← state machine above
│           ├── main.tf            ← null_resource calling orchestrate.py
│           ├── variables.tf       ← assignment config block + github_pat + dk_binary
│           ├── outputs.tf         ← assignment_id, status, proposal_url
│           ├── versions.tf
│           └── README.md
└── examples/
    └── assignment/
        └── branch-isolation/
            ├── main.tf
            ├── variables.tf
            ├── outputs.tf
            ├── ridealong.md       ← full lifecycle walkthrough
            └── README.md
```

**`variables.tf` shape (module):**
```hcl
variable "assignment_config" {
  type = object({
    assignments_dir         = string
    slopspaces_dir          = string
    work_signals_dir        = string
    name                    = string
    target_repo             = string   # "owner/repo"
    base_branch             = optional(string, "main")
    prompt                  = string
    agent                   = optional(string, "claude")
    model                   = optional(string, "")
    interaction_surface     = optional(string, "in-repo-pr")
    approval_signal_path    = optional(string, "")
  })
}

variable "github_pat"  { type = string; sensitive = true }
variable "dk_binary"   { type = string; default = "dungeon-keeper" }
```

---

### Dungeon-Keeper Changes

**None.** Scenario 1 is fully served by the existing sync implementation. The only claudomation-side additions are the state machine, ASSIGNMENT.json schema, and GitHub API calls for PR creation/polling.

---

## Clarifying Questions

1. **Retry branch strategy** — I assumed force-reset of the work branch (one PR per step, history rewritten). The alternative is a new branch per retry (`assignment-<id>-retry-<n>`), giving a fresh PR each time. Is rewriting the work branch history acceptable, or would you prefer per-retry branches?

2. **Revise/retry signal format for in-repo PR** — I assumed plain text PR comments starting with `REVISE:` or `RETRY`. Should these instead be structured (a GitHub label, a specific PR review type, or a comment with a particular format)? A structured format is more parseable but adds friction.

3. **Slopspace re-deploy on revise** — The existing `Deploy` function on an already-returned slopspace: does it handle "re-deploy after a prior return" cleanly today, or does it assume a fresh slopspace? If there's a guard that prevents re-deploying an already-iterated slopspace, it would need to be relaxed.

4. **Retry: slopspace deletion** — After snapshotting the prior attempt to a readspace, should the old slopspace be deleted (to avoid accumulating orphaned slopspaces), or left in place for post-mortem debugging?

5. **Retry readspace format** — I assumed cloning the branch state into `readspaces/repos/previous-attempt/<owner>/<repo>/` inside the new slopspace. Should this path be versioned per retry (`previous-attempt-1/`, `previous-attempt-2/`) so consecutive retries each have their reference visible?

6. **Revise: accumulated history** — On a second revise, the prompt would be `original + feedback-1 + feedback-2`. Should there be a cap or a summary mechanism, or is accumulation the right behavior for now?

7. **`holder` field and concurrent applies** — If two terraform applies run simultaneously for the same assignment, the holder pattern prevents double-advancement. But what clears a stale holder if a claudomation process crashes mid-invocation? Should there be a timeout after which a held invocation is treated as abandoned?

8. **Filesystem surface: approval signal path** — For the filesystem surface, where should the approval directory live? Options: (a) a path explicitly configured in `assignment_config`, (b) a well-known subdirectory of `ASSIGNMENTS_DIR/<assignment-id>/`, (c) somewhere else. I assumed (a) to keep it flexible.

9. **Ridealong target** — Should the new ridealong target the AI-evo1 repo itself (consistent with existing ridealongs), or a throwaway repo to avoid littering AI-evo1's branch list with `assignment-*` branches?

10. **Take1-claudomation relationship** — The existing `take1-claudomation/modules/execution/branch-isolation/slopspace_flow.py` does most of scenario 1 without the formal data structures. Should the new module replace it, deprecate it, or coexist as a separate example of the older flat approach?

