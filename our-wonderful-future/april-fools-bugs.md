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
