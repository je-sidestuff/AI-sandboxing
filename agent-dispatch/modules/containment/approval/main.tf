locals {
  # Unix timestamp for unique branch naming
  unix_timestamp = time_static.dispatch_time.unix

  # We name the branch after the dispatcher and the unix timestamp
  branch_name = "approval-${var.dispatcher_name}-${local.unix_timestamp}"

  # The full name of the approval repo
  approval_repo_full_name = "${var.github_owner}/${var.approval_repo}"

  # ---------------------------------------------------------------------------------------------------------------------
  # SEQUENCE DETECTION AND FORMATTING
  # These locals support displaying sequence plans in approval PRs for sequence-to-new-repo dispatches.
  # ---------------------------------------------------------------------------------------------------------------------

  # Detect if this is a sequence dispatch (has commands)
  is_sequence = length(var.sequence_commands) > 0

  # Pre-compute sequence metadata for display
  sequence_total_steps         = length(var.sequence_commands)
  sequence_total_duration_mins = local.sequence_total_steps * var.sequence_minutes_between

  # Build the numbered step list for PR body
  # Format: "1. (T+0m) First command\n2. (T+20m) Second command\n..."
  sequence_step_list = join("\n", [
    for i, cmd in var.sequence_commands :
    "${i + 1}. (T+${i * var.sequence_minutes_between}m) ${cmd}"
  ])

  # ---------------------------------------------------------------------------------------------------------------------
  # APPROVAL FILE CONTENT
  # Base approval data always included, sequence data conditionally merged.
  # ---------------------------------------------------------------------------------------------------------------------

  # Base approval data (always included)
  _approval_base = {
    type                = "approval-request"
    dispatcher_name     = var.dispatcher_name
    source_context      = var.source_context
    pending_instruction = var.pending_instruction
    pending_mode        = var.pending_mode
    pending_agent       = var.pending_agent
    request_time        = time_static.dispatch_time.rfc3339
    metadata            = jsondecode(var.metadata_json)
  }

  # Merged result - conditionally include sequence fields only when this is a sequence dispatch
  # We use merge with a for-expression filter to avoid Terraform's "inconsistent conditional result types" error.
  # The for-expression produces a map only when is_sequence is true, otherwise an empty map.
  approval_file_content = jsonencode(merge(
    local._approval_base,
    { for k, v in {
      sequence_commands        = var.sequence_commands
      sequence_minutes_between = var.sequence_minutes_between
    } : k => v if local.is_sequence }
  ))
}

# Capture the current time at resource creation for consistent naming
resource "time_static" "dispatch_time" {}

# Lookup the approval repository to ensure it exists
data "github_repository" "approval_repo" {
  full_name = local.approval_repo_full_name
}

# Get the default branch's latest commit SHA for creating the new branch
data "github_branch" "main" {
  repository = var.approval_repo
  branch     = "main"
}

# Create the approval branch
resource "github_branch" "approval_branch" {
  repository    = var.approval_repo
  branch        = local.branch_name
  source_branch = "main"
  source_sha    = data.github_branch.main.sha
}

# Create the approval request file in the branch
resource "github_repository_file" "approval_request" {
  repository          = var.approval_repo
  branch              = github_branch.approval_branch.branch
  file                = "requests/${var.dispatcher_name}-${local.unix_timestamp}.json"
  content             = local.approval_file_content
  commit_message      = "Approval request: ${var.dispatcher_name} (${local.unix_timestamp})"
  overwrite_on_create = true

  depends_on = [github_branch.approval_branch]
}

# ---------------------------------------------------------------------------------------------------------------------
# PR BODY CONTENT LOCALS
# Pre-compute sequence plan content to avoid heredoc-in-ternary Terraform parser issues.
# See: FULL_SEQUENCE_TODAY_AND_TOMORROW.md section "⚠️ CRITICAL PITFALL: Terraform Heredoc + Ternary"
# ---------------------------------------------------------------------------------------------------------------------

locals {
  # Pre-compute the sequence plan as a separate local (no ternary with heredoc)
  _sequence_plan_content = <<-EOT
### Proposed Sequence

**Execution Steps:**
${local.sequence_step_list}

**Timing:** ${var.sequence_minutes_between} minutes between steps
**Total Duration:** ~${local.sequence_total_duration_mins} minutes (${local.sequence_total_steps} steps)
EOT

  # Now use simple ternary with pre-computed strings
  sequence_plan_markdown = local.is_sequence ? local._sequence_plan_content : ""

  # Conditional header text
  instruction_header = local.is_sequence ? "Initial Setup" : "Pending Instruction"

  # Conditional merge action text
  merge_action = local.is_sequence ? "begin sequence execution" : "execute the instruction"
}

# Create a pull request for the approval
resource "github_repository_pull_request" "approval_pr" {
  title           = "Approval: ${var.dispatcher_name} (${local.unix_timestamp})"
  body            = <<-EOT
## Approval Request

**Source:** ${var.source_context}
**Dispatcher:** ${var.dispatcher_name}
**Requested at:** ${time_static.dispatch_time.rfc3339}

### ${local.instruction_header}

```
${var.pending_instruction}
```

${local.sequence_plan_markdown}
### Details

- **Mode:** ${var.pending_mode}
${var.pending_agent != "" ? "- **Agent:** ${var.pending_agent}" : ""}

---

**Merge this PR to approve and ${local.merge_action}.**
**Close this PR to reject the request.**
EOT
  head_ref        = local.branch_name
  base_ref        = "main"
  base_repository = var.approval_repo

  depends_on = [github_repository_file.approval_request]
}

# Fetch all comments and PR state using the shared Python script.
data "external" "pr_comments" {
  program = ["python3", "${path.module}/../scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = local.approval_repo_full_name
    pr_number = tostring(github_repository_pull_request.approval_pr.number)
  }
}

# Compute conclusion state from the external data source.
locals {
  pr_comments_result = data.external.pr_comments.result
  conclusion_state   = local.pr_comments_result.conclusion_state
  approval_pr_url    = "https://github.com/${local.approval_repo_full_name}/pull/${github_repository_pull_request.approval_pr.number}"
}
