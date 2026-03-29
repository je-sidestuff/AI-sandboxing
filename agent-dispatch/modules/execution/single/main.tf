locals {
  # Unix timestamp for unique execution naming
  unix_timestamp = time_static.execution_time.unix

  # Unique execution ID for this single execution step
  execution_id = "single_${var.dispatcher_name}_${local.unix_timestamp}"

  # The full name of the repo, computed a priori from input variables
  repo_full_name = "${var.github_owner}/${var.target_pr.repo}"
}

# Capture the current time at resource creation for consistent naming
resource "time_static" "execution_time" {}

# =============================================================================
# EXISTENCE CHECK: Use data sources with a-priori-known strings to determine
# if the target repo and PR exist. This avoids "count cannot be determined
# until apply" errors by never depending on resource-computed attributes.
# =============================================================================

# Look up repos broadly by owner so the query doesn't depend on computed values.
# We filter the results below to check if our specific repo exists.
data "github_repositories" "target" {
  query = "user:${var.github_owner} ${var.target_pr.repo} in:name"
}

locals {
  # Filter the broad search results to find exactly our repo (empty list = not created yet)
  repo_match  = [for n in data.github_repositories.target.names : n if n == var.target_pr.repo]
  repo_exists = length(local.repo_match) > 0
}

# Look up the PR details using the external script. This fetches the PR's head_ref
# (branch name) along with state information. count=0 when repo doesn't exist yet,
# so this data source is never evaluated until the repo actually exists.
data "external" "pr_details" {
  count = local.repo_exists ? 1 : 0

  program = ["python3", "${path.module}/../../containment/scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = local.repo_full_name
    pr_number = tostring(var.target_pr.pr_number)
  }
}

locals {
  # Extract PR details from the external data source
  pr_result   = local.repo_exists ? data.external.pr_details[0].result : null
  pr_exists   = local.pr_result != null && try(local.pr_result.head_ref, "") != ""
  target_branch = local.pr_exists ? local.pr_result.head_ref : ""

  # The module is ready to execute only when both repo and PR exist
  ready_to_execute = local.repo_exists && local.pr_exists

  # Build the authenticated repo URL
  source_repo_url = "https://${var.github_pat}@github.com/${local.repo_full_name}.git"
}

# Execute the single AI execution step
# This clones the repo at the branch HEAD, dispatches an AI work unit
# with the instruction, waits for completion, and pushes results back
# to the same branch.
#
# count=0 until both the repo and PR exist, avoiding the "count cannot
# be determined until apply" error on first apply.
resource "terraform_data" "execute_single_step" {
  count = local.ready_to_execute ? 1 : 0

  provisioner "local-exec" {
    command = "${path.module}/execute_single_step.sh > /tmp/single_step_${local.unix_timestamp}.log 2>&1"

    environment = {
      UNIX_TIMESTAMP      = local.unix_timestamp
      EXECUTION_ID        = local.execution_id
      BRANCH_NAME         = local.target_branch
      SOURCE_REPO_URL     = local.source_repo_url
      SLOPSPACES_WORK_DIR = var.slopspaces_working_dir
      DISPATCHER_NAME     = var.dispatcher_name
      INSTRUCTION         = var.instruction
      INSTRUCTION_MODE    = var.instruction_mode
    }
  }
}
