locals {
  # Unix timestamp for unique branch naming
  # Note: time_static only has second-level precision, so we use unix seconds
  unix_timestamp = time_static.dispatch_time.unix

  # We name the branch after the dispatcher and the unix timestamp
  branch_name = "dispatch-${var.dispatcher_name}-${local.unix_timestamp}"

  # The full name of the target repo, computed a priori from input variables
  target_repo_full_name = "${var.github_owner}/${var.target_repo_name}"
}

# Capture the current time at resource creation for consistent naming
resource "time_static" "dispatch_time" {}

data "github_repository" "source_repo" {
  full_name = var.source_repo
}

# Create the new target repository where the work will be pushed
resource "github_repository" "target_repo" {
  name        = var.target_repo_name
  description = var.description
  visibility  = var.visibility
  auto_init   = false
}

# Create the isolation branch in terraform before executing the bash script
resource "github_branch" "containment_branch" {
  repository    = github_repository.target_repo.name
  branch        = local.branch_name
  source_branch = "main"

  depends_on = [terraform_data.dispatch_first_work]
}

resource "terraform_data" "dispatch_first_work" {
  provisioner "local-exec" {
    command = "${path.module}/init_containment_branch.sh > /tmp/loglog.txt 2>&1"

    environment = {
      UNIX_TIMESTAMP      = local.unix_timestamp
      BRANCH_NAME         = local.branch_name
      SLOPSPACES_WORK_DIR = var.slopspaces_working_dir
      SOURCE_REPO_URL = replace(
        "https://github.com/${var.source_repo}.git", "https://", "https://${var.github_pat}@"
      )
      TARGET_REPO_URL = replace(
        github_repository.target_repo.http_clone_url, "https://", "https://${var.github_pat}@"
      )
      DISPATCHER_NAME  = var.dispatcher_name
      INSTRUCTION      = var.instruction
      INSTRUCTION_MODE = var.instruction_mode
    }
  }

  depends_on = [github_repository.target_repo]
}

# Create a pull request from the containment branch to main so that we can have a PR to work with in the next steps
resource "github_repository_pull_request" "containment_pr" {
  title           = "Dispatch: ${var.dispatcher_name} (${local.unix_timestamp})"
  body            = <<-EOT
    This is a PR for AI-generated changes in the new target repository.

    **Source repository:** ${var.source_repo}
    **Target repository:** ${local.target_repo_full_name}
  EOT
  head_ref        = local.branch_name
  base_ref        = "main"
  base_repository = github_repository.target_repo.name

  depends_on = [github_branch.containment_branch]
}

# Fetch all comments and PR state using the shared Python script.
# The native Terraform GitHub provider data source does not expose merged_at or
# closed_at attributes, so we use the REST API via external data source.
#
# This data source executes during the plan phase. It uses the PR number from
# the resource, which is known from state on subsequent applies. The script
# handles errors gracefully when the PR doesn't exist yet.
data "external" "pr_comments" {
  program = ["python3", "${path.module}/../scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = local.target_repo_full_name
    pr_number = tostring(github_repository_pull_request.containment_pr.number)
  }
}

# Compute conclusion state from the external data source
locals {
  pr_comments_result = data.external.pr_comments.result
  pr_is_merged       = local.pr_comments_result.pr_merged == "true"
  conclusion_state   = local.pr_comments_result.conclusion_state
}

# When REVISE: comments are detected, post a status comment to the PR before
# beginning the revision work.
resource "terraform_data" "post_revise_comment" {
  triggers_replace = {
    revise_instructions = local.pr_comments_result.revise_instructions_json
  }

  provisioner "local-exec" {
    command = "python3 ${path.module}/../scripts/post_revise_comments.py"

    environment = {
      REVISE_INSTRUCTIONS_JSON = local.pr_comments_result.revise_instructions_json
      GITHUB_PAT               = var.github_pat
      REPO                     = local.target_repo_full_name
      PR_NUMBER                = tostring(github_repository_pull_request.containment_pr.number)
    }
  }

  depends_on = [data.external.pr_comments]
}

# For each REVISE: comment found on the PR, clone the repo at the HEAD of the
# working branch, dispatch an AI work unit, and push the result back to the branch.
resource "terraform_data" "handle_revise_comments" {
  triggers_replace = {
    revise_instructions = local.pr_comments_result.revise_instructions_json
  }

  provisioner "local-exec" {
    command = "bash ${path.module}/../scripts/handle_revise_comments.sh > /tmp/revise_log.txt 2>&1"

    environment = {
      REVISE_INSTRUCTIONS_JSON = local.pr_comments_result.revise_instructions_json
      BRANCH_NAME              = local.branch_name
      SOURCE_REPO_URL = replace(
        "https://github.com/${local.target_repo_full_name}.git", "https://", "https://${var.github_pat}@"
      )
      SLOPSPACES_WORK_DIR = var.slopspaces_working_dir
      DISPATCHER_NAME     = var.dispatcher_name
      UNIX_TIMESTAMP      = local.unix_timestamp
    }
  }

  depends_on = [terraform_data.post_revise_comment]
}
