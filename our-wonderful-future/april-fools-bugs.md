# April Fools Bugs in Our Dispatch Process

Two bugs have been identified in our broader dispatch process, specifically in the sequence-to-new-repo flow. This document explains our best guesses as to why these issues are occurring and proposes hypothetical fixes.

---

## Bug 1: Duplicate Repository Creation

### Symptom
When executing a sequence-to-new-repo flow and attempting to create a repo with a specific name, we accidentally create two new repositories instead of just one with our intended name.

### Best Guesses as to Why This Is Happening

**Guess 1: Terraform Apply Race Condition**

The dispatcher runs `terraform apply` both during initial setup and during periodic polling in `pollAllMonitoringFlows()`. If the polling cycle fires before the initial apply completes, or if two polling cycles overlap, Terraform might process the same configuration twice. While Terraform is generally idempotent, if the state file hasn't been written yet from the first apply, the second apply wouldn't know the resource already exists.

Relevant code in `agent-dispatch/main.go`:
- Initial apply happens in `processSequenceToNewRepoDispatch()` (line 1172)
- Polling apply happens in `pollAllMonitoringFlows()` (line 2872)

**Guess 2: Default Repo Creation in Containment Module**

Looking at `modules/containment/to-repo/main.tf`, the `github_repository.target_repo` resource creates the repo with `var.target_repo_name`. However, if there's a fallback or default resource elsewhere (perhaps in a parent module or a separate apply), we could be creating both:
1. The specifically-named repo from our terraform config
2. A default-named repo from some other codepath

**Guess 3: Terraform State File Corruption or Loss**

If the terraform state file (`.tfstate`) becomes corrupted or is deleted between applies, Terraform loses track of what resources it has already created. On the next apply, it would attempt to create the resource again. Since repo names must be unique, GitHub would either error or (if we're using different names each time due to timestamp jitter) create a second repo.

### Hypothetical Fixes

**Fix 1: Add Locking/Mutex Around Terraform Operations**

```go
// In main.go, add a per-flow mutex
type FlowRecord struct {
    // ... existing fields ...
    applyMutex sync.Mutex `json:"-"` // Don't serialize
}

func (d *Dispatcher) runTerraformWithLock(record *FlowRecord, command string, args ...string) error {
    record.applyMutex.Lock()
    defer record.applyMutex.Unlock()
    return d.runTerraform(record.TFConfigDir, command, args...)
}
```

**Fix 2: Check for Existing Repo Before Creation**

Add a data source lookup in terraform to check if repo already exists:

```hcl
# In modules/containment/to-repo/main.tf
data "github_repository" "existing" {
  count     = 1
  full_name = "${var.github_owner}/${var.target_repo_name}"
}

resource "github_repository" "target_repo" {
  count       = length(data.github_repository.existing) == 0 ? 1 : 0
  name        = var.target_repo_name
  description = var.description
  # ...
}
```

**Fix 3: Add Idempotency Guard in Go Code**

Before running terraform apply, check if repo already exists via GitHub API:

```go
func (d *Dispatcher) repoExists(owner, name string) (bool, error) {
    _, resp, err := d.githubClient.Repositories.Get(context.Background(), owner, name)
    if err != nil {
        if resp != nil && resp.StatusCode == 404 {
            return false, nil
        }
        return false, err
    }
    return true, nil
}
```

### Code Analysis Findings (April 2026)

After reviewing the actual codebase, **Guess 1 (Race Condition)** appears to be **incorrect**:

1. The `DISPATCHING.md` marker (created at line 871 of main.go) prevents re-processing of the same dispatch unit within a single dispatcher instance
2. The initial terraform apply completes synchronously before `NeedsMonitoring` is set to `true`, so polling cannot overlap with initial setup
3. Terraform applies are serialized per-flow since the polling loop processes flows sequentially

**Guesses 2 and 3 remain possible** but lack supporting evidence.

**Additional Possible Causes** not yet documented:

1. **Multiple dispatcher instances**: If two dispatcher processes with different `dispatcherID`s monitor the same input directory, both could pick up the same dispatch unit simultaneously (TOCTOU race on `DISPATCHING.md` check)

2. **File system latency**: The `os.Stat()` check for `DISPATCHING.md` followed by `os.WriteFile()` is not atomic - two processes could both pass the check before either creates the file

3. **Repo name generation is unique**: Each dispatch generates a unique repo name (`seq-YYYYMMDD-HHMMSS-uuid8`) with timestamp and UUID, so two repos would have **different names**, not duplicates of the **same name**

### Information Needed to Diagnose

Before attempting a fix, collect the following evidence:

| Data Point | How to Collect | What It Tells Us |
|------------|----------------|------------------|
| **Names of both repos** | Check GitHub org for repos created around incident time | Were they identical names (state issue) or different names (duplicate dispatch)? |
| **Timestamps of creation** | GitHub API: `gh repo list --json name,createdAt` | Did they happen simultaneously (race) or sequentially (retry)? |
| **Dispatcher logs** | Check log output for `processSequenceToNewRepoDispatch` entries | Was the same dispatch unit processed twice? |
| **Terraform state files** | Check `dispatcher-live/flows/sequence-to-new-repo/*/` for `.tfstate` | Are there multiple flow directories? Is state intact? |
| **Number of dispatchers running** | Check running processes: `ps aux \| grep agent-dispatch` | Were multiple dispatcher instances active? |
| **Flow records** | Check `records/dispatch-watch/*.json` | Are there duplicate flow records for the same dispatch? |

### Investigation Update (April 3, 2026)

**Reproduction Rate**: The bug reproduces **100% of the time** when a heuristic dispatch triggers a sequence-to-new-repo flow. This is not a sporadic race condition but a consistent behavior.

**Logging Added**: Instrumented `checkForDispatchUnits` function (line 720 of main.go) with:
```go
log.Printf("[%s] Found dispatch unit: %s (DISPATCHING.md exists: %v)", d.dispatcherID, entry.Name(), dispatchingMDExists)
```

**Next Step**: Run the sequence again to collect logs and identify the exact duplicate creation mechanism.

### Recommended Next Steps

1. **Immediate**: Run `gh repo list je-sidestuff --json name,createdAt,description -L 100` to identify the duplicate repos and their creation timestamps

2. ~~**Add logging**: Instrument the `checkForDispatchUnits` function to log when a dispatch unit is detected~~ ✅ Done

3. **Add atomic locking**: Replace the TOCTOU-vulnerable `DISPATCHING.md` check with file-based locking using `flock` or a `.lock` file with `O_EXCL`

4. **Run sequence and collect logs**: With logging now in place, trigger a heuristic dispatch → sequence-to-new-repo flow and capture the logs to identify the duplicate creation mechanism

### Notes Taken During Test Run

The anonymously named repo (seq-20260403-105901-52e183fb) is created first, before the first work unit is done.

### Follow-up Observation (April 3, 2026 - Afternoon)

**New Finding**: When running a subsequent test, the duplicate repo creation bug did NOT occur, but a different issue appeared:

- **Expected behavior**: When the heuristic layer dispatches a sequence-to-new-repo, it should be able to specify a custom repo name
- **Actual behavior**: The repo was created with an auto-generated name (`seq-20260403-123748-6b2fc2ed`) instead of a custom name

**Key Evidence from Terraform Output** (from `ignored-scratch/no-name-no-bug.txt`):
```
+ name = "seq-20260403-123748-6b2fc2ed"
```

This suggests either:
1. The heuristic layer is not passing a custom repo name in the dispatch unit
2. The worker layer (Go dispatcher) is not reading the custom repo name from the dispatch unit
3. The Terraform variable for custom repo name is not being set properly

**Note**: The absence of the duplicate repo bug in this run is interesting - it may indicate the bug is intermittent or was partially fixed by earlier changes.

### Fix Applied (April 3, 2026)

**Root Cause Identified**: Both the heuristic layer and worker layer lacked support for custom repo names:
1. The `Dispatch` struct in `agent-dispatch/main.go` had no field for custom sequence repo names
2. The `processSequenceToNewRepoDispatch` function always generated an auto-name
3. The heuristic prompt template didn't include a field for custom repo names
4. The ERR-EXECUTION.md examples instructed agents to include "Create repository X" in the instruction, causing duplicate repos

**Status: IMPLEMENTED ✅**

The following changes have been applied:

1. **agent-dispatch/main.go:106** - Added `SequenceRepoName` field to `Dispatch` struct:
   ```go
   SequenceRepoName string `json:"sequence_repo_name,omitempty"` // For sequence-to-new-repo: custom repo name
   ```

2. **agent-dispatch/main.go:1191-1200** - Updated `processSequenceToNewRepoDispatch` to use custom name when provided:
   ```go
   var targetRepoName string
   if unit.Dispatch.SequenceRepoName != "" {
       targetRepoName = unit.Dispatch.SequenceRepoName
       log.Printf("[%s] Using custom repo name for sequence-to-new-repo: %s", d.dispatcherID, targetRepoName)
   } else {
       // Auto-generate name as before
   }
   ```

3. **heuristic-request/main.go:281** - Updated prompt template to include `sequence_repo_name` as required:
   ```json
   {
     "type": "sequence-to-new-repo",
     "sequence_repo_name": "descriptive-repo-name",
     ...
   }
   ```
   Also added guidance: "Do NOT embed the repo name in the instruction text; that causes duplicate repo creation."

4. **heuristic-request/ERR-EXECUTION.md** - Updated all example dispatches to:
   - Include `sequence_repo_name` field
   - Remove "Create repository X" from instruction text
   - Add explicit warning about duplicate repo creation

---

### How Custom Repo Naming Works NOW (Fixed April 3, 2026)

Custom repo naming now works correctly through the `sequence_repo_name` field:

**The Fixed Flow:**
1. The heuristic layer generates a `sequence-to-new-repo` DISPATCH.json with `sequence_repo_name` field
2. The Go dispatcher reads `sequence_repo_name` and uses it as the target repo name
3. If `sequence_repo_name` is empty, the dispatcher falls back to auto-generated name
4. The instruction text should NOT mention "create repository X" - just set up the structure within the already-created repo

**Example of Correct Dispatch:**
```json
{
  "type": "sequence-to-new-repo",
  "sequence_repo_name": "AI-evo-experimental1",
  "instruction": "Set up ERR working structure with folders...",
  ...
}
```

**Result:** One repo created with the custom name, no duplicates.

**Previous Behavior (Bug):**

Before this fix, the heuristic prompt taught agents to include repo names in the instruction text:
```json
{
  "type": "sequence-to-new-repo",
  "instruction": "Create the repository AI-evo-experimental1 with ERR working structure...",
  ...
}
```

This caused duplicate repos because:
1. The dispatcher created `seq-YYYYMMDD-HHMMSS-uuid` as the containment repo
2. The AI executing the instruction created `AI-evo-experimental1` as instructed



---

## Bug 2: Conclusory State Not Detected, Terraform Config Not Removed

### Symptom
When we reach the conclusory state for a sequence-to-new-repo flow, the system does not detect it and fails to remove the terraform configuration and associated resources.

### Best Guesses as to Why This Is Happening

**Guess 1: Sequence Complete vs PR State Mismatch**

Looking at the cleanup logic in `main.go` (lines 2587-2597), cleanup only triggers when BOTH conditions are met:
1. `sequence_complete == "true"` (all steps have executed)
2. `record.ConclusionState == ConclusionStateMerged` OR `ConclusionStateClosed`

If the sequence completes but the PR is still in "active" state, cleanup won't trigger. The comment in the code confirms this: "Sequence is complete but PR is still active - keep monitoring for PR merge/close"

The bug might be that `ConclusionState` is not being updated properly when the PR is merged or closed. The state update likely depends on polling the GitHub API or terraform refreshing the PR state, which might be failing silently.

**Guess 2: Output Calculation Never Returns True**

The `sequence_complete` output is calculated as:
```hcl
output "sequence_complete" {
  value = alltrue([for k, v in module.sequence_execution.step_readiness : v])
}
```

This requires ALL step readiness values to be true simultaneously. However, if the step_readiness map includes steps beyond our `SequenceTotalSteps`, those extra steps might never become "ready" (since they have no commands), causing `alltrue()` to always return false.

**Guess 3: Terraform Output Retrieval Failure**

The check uses `d.getTerraformOutput(record.TFConfigDir, "sequence_complete")`. If this function fails silently (returns empty string instead of error), the comparison `sequenceComplete == "true"` would always be false, preventing cleanup from ever triggering.

**Guess 4: Flow Record Not Being Written After State Update**

Even if the conclusion state is detected correctly, if `d.writeFlowRecord(*record)` fails or isn't called after updating `ConclusionState`, the next poll cycle would still see the old state and skip cleanup.

### Hypothetical Fixes

**Fix 1: Handle Sequence Complete Independently of PR State**

For sequence-to-new-repo, we might want to cleanup when sequence is complete regardless of PR state:

```go
func (d *Dispatcher) pollSequenceFlow(record *FlowRecord) error {
    sequenceComplete, err := d.getTerraformOutput(record.TFConfigDir, "sequence_complete")
    if err != nil {
        log.Printf("[%s] Warning: failed to get sequence_complete output: %v", d.dispatcherID, err)
        return nil // Don't fail, just skip this cycle
    }

    if sequenceComplete == "true" {
        log.Printf("[%s] Flow %s sequence complete", d.dispatcherID, record.FlowID)
        // For sequence-to-new-repo, cleanup when sequence is done
        // The created repo has served its purpose
        if record.DispatchType == DispatchTypeSequenceToNewRepo {
            return d.performFlowCleanup(record, "sequence-complete")
        }
    }
    return nil
}
```

**Fix 2: Fix the Sequence Complete Calculation**

Limit the `alltrue` calculation to only the steps that are actually being used:

```hcl
output "sequence_complete" {
  description = "Whether all sequence steps have completed"
  value = alltrue([
    for idx, v in module.sequence_execution.step_readiness : v
    if idx <= var.total_steps
  ])
}
```

**Fix 3: Add Error Handling and Logging to Output Retrieval**

```go
func (d *Dispatcher) getTerraformOutput(configDir, outputName string) (string, error) {
    cmd := exec.Command("terraform", "output", "-raw", outputName)
    cmd.Dir = configDir

    output, err := cmd.Output()
    if err != nil {
        log.Printf("[%s] Failed to get terraform output %s: %v", d.dispatcherID, outputName, err)
        return "", fmt.Errorf("terraform output %s failed: %w", outputName, err)
    }

    result := strings.TrimSpace(string(output))
    log.Printf("[%s] Terraform output %s = %q", d.dispatcherID, outputName, result)
    return result, nil
}
```

**Fix 4: Add Explicit Cleanup Trigger**

Add a timeout-based cleanup as a fallback:

```go
func (d *Dispatcher) checkForStaleFlows(record *FlowRecord) bool {
    // If flow has been active for more than 24 hours with no progress, mark for cleanup
    startTime, _ := time.Parse(time.RFC3339, record.StartTime)
    if time.Since(startTime) > 24*time.Hour && record.SequenceLastCompletedIdx == record.SequenceTotalSteps {
        log.Printf("[%s] Flow %s appears complete but stale, triggering cleanup", d.dispatcherID, record.FlowID)
        return true
    }
    return false
}
```

---

## Summary

| Bug | Root Cause Hypothesis | Recommended Fix Priority |
|-----|----------------------|--------------------------|
| Duplicate Repos | Race condition or state file issues | Add mutex locking + pre-creation check |
| No Cleanup | Output never true OR PR state not updating | Fix output calculation + add fallback cleanup |

Both bugs likely stem from the same underlying issue: our state management isn't robust enough for the concurrent, long-running nature of sequence flows. Adding better logging, explicit state checks, and fallback mechanisms should resolve both issues.
