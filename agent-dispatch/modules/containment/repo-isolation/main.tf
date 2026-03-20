locals {
  # Unix timestamp for unique branch naming
  # Note: time_static only has second-level precision, so we use unix seconds
  unix_timestamp = time_static.dispatch_time.unix

  # We name the branch after the dispatcher and the unix timestamp
  branch_name = "dispatch-${var.dispatcher_name}-${local.unix_timestamp}"
}

# Capture the current time at resource creation for consistent naming
resource "time_static" "dispatch_time" {}

data "github_repository" "target_repo" {
  full_name = var.target_repo
}

# Create the isolation repository where the AI will work
resource "github_repository" "isolation_repo" {
  name       = var.name
  visibility = "private"
  auto_init  = false
}

# Look up the isolation repo via data source
data "github_repositories" "isolation" {
  query      = "repo:${github_repository.isolation_repo.full_name}"
  depends_on = [github_repository.isolation_repo]
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

# Read back the PR via data source
data "github_repository_pull_requests" "containment" {
  base_repository = github_repository.isolation_repo.name
  base_ref        = "main"
  depends_on      = [github_repository_pull_request.containment_pr]
}

# Fetch all comments on the containment PR using the shared Python script
data "external" "pr_comments" {
  program = ["python3", "${path.module}/../scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = github_repository.isolation_repo.full_name
    pr_number = tostring(github_repository_pull_request.containment_pr.number)
  }

  depends_on = [github_repository_pull_request.containment_pr]
}
