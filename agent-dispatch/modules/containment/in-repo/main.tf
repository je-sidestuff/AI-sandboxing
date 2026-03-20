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

# Create the isolation branch in terraform before executing the bash script
resource "github_branch" "containment_branch" {
  repository    = data.github_repository.target_repo.name
  branch        = local.branch_name
  source_branch = "main"
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
      DISPATCHER_NAME      = var.dispatcher_name
    }
  }

  depends_on = [github_branch.containment_branch]
}

# Create a pull request from the containment branch to main so that we can have a PR to work with in the next steps
resource "github_repository_pull_request" "containment_pr" {
  title           = "Dispatch: ${var.dispatcher_name} (${local.unix_timestamp})"
  body            = "This is a PR to let us test out some AI containment strategies."
  head_ref        = local.branch_name
  base_ref        = "main"
  base_repository = data.github_repository.target_repo.name

  depends_on = [terraform_data.dispatch_first_work]
}

# Fetch all comments on the containment PR using the shared Python script
data "external" "pr_comments" {
  program = ["python3", "${path.module}/../scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = var.target_repo
    pr_number = tostring(github_repository_pull_request.containment_pr.number)
  }

  depends_on = [github_repository_pull_request.containment_pr]
}
