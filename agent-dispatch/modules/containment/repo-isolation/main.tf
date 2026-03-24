locals {
  # Unix timestamp for unique branch naming
  # Note: time_static only has second-level precision, so we use unix seconds
  unix_timestamp = time_static.dispatch_time.unix

  # We name the branch after the dispatcher and the unix timestamp
  branch_name = "dispatch-${var.dispatcher_name}-${local.unix_timestamp}"

  # The full name of the isolation repo, computed a priori from input variables
  isolation_repo_full_name = "${var.github_owner}/${var.name}"
}

# Capture the current time at resource creation for consistent naming
resource "time_static" "dispatch_time" {}

resource "time_static" "detection_time" {
  triggers = {
    "IS_MERGED" = local.pr_is_merged
  }
}

data "github_repository" "target_repo" {
  full_name = var.target_repo
}

# Create the isolation repository where the AI will work
resource "github_repository" "isolation_repo" {
  name       = var.name
  visibility = "private"
  auto_init  = false
}

# Look up isolation repos broadly by owner so the query doesn't depend on the
# repo existing yet (it won't on first apply). We filter the results below.
data "github_repositories" "isolation" {
  query = "user:${var.github_owner} ${var.name} in:name"
}

resource "terraform_data" "dispatch_first_work" {
  provisioner "local-exec" {
    command = "${path.module}/init_containment_branch.sh > /tmp/loglog.txt 2>&1"

    environment = {
      UNIX_TIMESTAMP       = local.unix_timestamp
      BRANCH_NAME          = local.branch_name
      SLOPSPACES_WORK_DIR  = var.slopspaces_working_dir
      SOURCE_REPO_URL      = replace(
        "https://github.com/${var.target_repo}.git", "https://", "https://${var.github_pat}@"
      )
      ISOLATION_REPO_URL   = replace(
        github_repository.isolation_repo.http_clone_url, "https://", "https://${var.github_pat}@"
      )
      DISPATCHER_NAME      = var.dispatcher_name
      INSTRUCTION          = var.instruction
      INSTRUCTION_MODE     = var.instruction_mode
    }
  }

  depends_on = [github_repository.isolation_repo]
}

# Create a pull request in the isolation repo so that we can have a PR to work with in the next steps
resource "github_repository_pull_request" "containment_pr" {
  title           = "Dispatch: ${var.dispatcher_name} (${local.unix_timestamp})"
  body            = "This is a PR to let us test out some AI containment strategies."
  head_ref        = local.branch_name
  base_ref        = "main"
  base_repository = github_repository.isolation_repo.name

  depends_on = [terraform_data.dispatch_first_work]
}

# Detect whether the isolation PR has been closed/merged using only a-priori-known
# strings (var.github_owner and var.name). This keeps the count for reintegration
# resources free of any resource-computed attributes, avoiding the
# "count cannot be determined until apply" error on first apply.
locals {
  # Filter the broad search results to find exactly our repo (empty list = not created yet)
  isolation_repo_match = [for n in data.github_repositories.isolation.names : n if n == var.name]
  repo_exists          = length(local.isolation_repo_match) > 0
}

data "github_repository_pull_requests" "isolation_pr_state" {
  # count=0 when repo doesn't exist yet, so this data source is never evaluated
  # (and never hits the GitHub API) until the repo actually exists. This avoids
  # a 404 error on first apply before the isolation repo has been created.
  count = local.repo_exists ? 1 : 0

  base_repository = var.name
  owner           = var.github_owner
  base_ref        = "main"
  state           = "closed"
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
    repo      = local.isolation_repo_full_name
    pr_number = tostring(github_repository_pull_request.containment_pr.number)
  }
}

# Compute conclusion state from the external data source.
# pr_is_merged is derived from the github_repository_pull_requests data source
# (strings known a priori) so that count expressions never depend on
# resource-computed attributes.
locals {
  pr_comments_result = data.external.pr_comments.result
  pr_is_merged       = local.repo_exists && length(data.github_repository_pull_requests.isolation_pr_state) > 0 && length(data.github_repository_pull_requests.isolation_pr_state[0].results) > 0
  conclusion_state   = local.pr_comments_result.conclusion_state
  isolation_pr_url   = "https://github.com/${local.isolation_repo_full_name}/pull/${github_repository_pull_request.containment_pr.number}"
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
      REPO                     = local.isolation_repo_full_name
      PR_NUMBER                = tostring(github_repository_pull_request.containment_pr.number)
    }
  }

  depends_on = [data.external.pr_comments]
}

# For each REVISE: comment found on the PR, clone the isolation repo at the HEAD of the
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
        "https://github.com/${local.isolation_repo_full_name}.git", "https://", "https://${var.github_pat}@"
      )
      SLOPSPACES_WORK_DIR = var.slopspaces_working_dir
      DISPATCHER_NAME     = var.dispatcher_name
      UNIX_TIMESTAMP      = local.unix_timestamp
    }
  }

  depends_on = [terraform_data.post_revise_comment]
}

# =============================================================================
# RE-INTEGRATION: When the isolation PR is merged, create a PR in the target repo
# =============================================================================

# Push the branch from isolation repo to target repo when the isolation PR is merged.
# The count uses enable_reintegration (a known value) AND pr_is_merged to avoid
# the "count cannot be determined until apply" error on first apply.
resource "terraform_data" "push_to_target" {
  count = var.enable_reintegration && local.pr_is_merged ? 1 : 0

  provisioner "local-exec" {
    command = "bash ${path.module}/../scripts/push_branch_to_target.sh"

    environment = {
      BRANCH_NAME     = local.branch_name
      SOURCE_REPO_URL = replace(
        "https://github.com/${local.isolation_repo_full_name}.git", "https://", "https://${var.github_pat}@"
      )
      TARGET_REPO_URL = replace(
        "https://github.com/${var.target_repo}.git", "https://", "https://${var.github_pat}@"
      )
      SLOPSPACES_WORK_DIR = var.slopspaces_working_dir
      UNIX_TIMESTAMP      = local.unix_timestamp
    }
  }

  depends_on = [terraform_data.handle_revise_comments]
}

# Create the re-integration PR in the target repo once the branch has been pushed
resource "github_repository_pull_request" "reintegration_pr" {
  count = var.enable_reintegration && local.pr_is_merged ? 1 : 0

  title           = "Re-integration: ${var.dispatcher_name} (${local.unix_timestamp})"
  body            = <<-EOT
    This PR re-integrates work from the isolation repository.

    **Original isolation PR:** ${local.isolation_pr_url}
    **Isolation repo:** ${local.isolation_repo_full_name}

    The work was reviewed and approved in the isolation repo before being merged
    and pushed here for final integration.
  EOT
  head_ref        = local.branch_name
  base_ref        = "main"
  base_repository = data.github_repository.target_repo.name

  depends_on = [terraform_data.push_to_target]
}

# =============================================================================
# REINTEGRATION PR STATE: Track when the reintegration PR is merged/closed
# =============================================================================

# Fetch the reintegration PR state to know when it's safe to destroy resources.
# This only executes when the reintegration PR exists (count > 0).
data "external" "reintegration_pr_state" {
  count = var.enable_reintegration && local.pr_is_merged ? 1 : 0

  program = ["python3", "${path.module}/../scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = var.target_repo
    pr_number = tostring(github_repository_pull_request.reintegration_pr[0].number)
  }

  depends_on = [github_repository_pull_request.reintegration_pr]
}

locals {
  # Reintegration PR state - only valid when reintegration PR exists
  reintegration_pr_result = try(data.external.reintegration_pr_state[0].result, null)
  reintegration_conclusion_state = try(local.reintegration_pr_result.conclusion_state, "none")
}
