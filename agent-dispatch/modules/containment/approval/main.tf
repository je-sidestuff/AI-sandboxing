locals {
  # Unix timestamp for unique branch naming
  unix_timestamp = time_static.dispatch_time.unix

  # We name the branch after the dispatcher and the unix timestamp
  branch_name = "approval-${var.dispatcher_name}-${local.unix_timestamp}"

  # The full name of the approval repo
  approval_repo_full_name = "${var.github_owner}/${var.approval_repo}"

  # Generate the approval file content as JSON
  approval_file_content = jsonencode({
    type               = "approval-request"
    dispatcher_name    = var.dispatcher_name
    source_context     = var.source_context
    pending_instruction = var.pending_instruction
    pending_mode       = var.pending_mode
    pending_agent      = var.pending_agent
    request_time       = time_static.dispatch_time.rfc3339
    metadata           = jsondecode(var.metadata_json)
  })
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

# Create a pull request for the approval
resource "github_repository_pull_request" "approval_pr" {
  title           = "Approval: ${var.dispatcher_name} (${local.unix_timestamp})"
  body            = <<-EOT
    ## Approval Request

    **Source:** ${var.source_context}
    **Dispatcher:** ${var.dispatcher_name}
    **Requested at:** ${time_static.dispatch_time.rfc3339}

    ### Pending Instruction

    ```
    ${var.pending_instruction}
    ```

    ### Details

    - **Mode:** ${var.pending_mode}
    ${var.pending_agent != "" ? "- **Agent:** ${var.pending_agent}" : ""}

    ---

    **Merge this PR to approve and execute the instruction.**
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
