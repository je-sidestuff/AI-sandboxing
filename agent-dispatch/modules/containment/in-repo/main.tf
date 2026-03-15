locals {
  random_result = random_string.random.result

  # We name the branch after the dispatcher and the unix time TODO
  branch_name = "dispatch-${var.dispatcher_name}-${local.random_result}"
}

resource "random_string" "random" {
  length  = 4
  special = false
}

data "github_repository" "target_repo" {
  full_name = github_repository.containment_repo.full_name
}

resource "terraform_data" "dispatch_first_work" {
  provisioner "local-exec" {
    command = "${path.module}/init_containment_branch.sh > /tmp/loglog.txt 2>&1"

    environment = {
      RANDOM_SUFFIX        = local.random_result
      SLOPSPACES_WORK_DIR  = var.slopspaces_working_dir
      SOURCE_REPO_URL      = replace(
      "https://github.com/je-sidestuff/AI-sandboxing.git", "https://", "https://${var.github_pat}@"
      )
      DISPATCHER_NAME = "${var.dispatcher_name}"
    }
  }
}

# Create a pull request from the FLAZZERWOOZLE-WAS-HERE branch to main so that we can have a PR to work with in the next steps
resource "github_repository_pull_request" "containment_pr" {
  title     = "FLAZZERWOOZLE was here"
  body      = "This is a PR to let us test out some AI containment strategies."
  head_ref      = "FLAZZERWOOZLE-WAS-HERE"
  base_ref      = "main"
  base_repository = data.github_repository.containment_repo.name

  depends_on = [ terraform_data.dispatch_first_work ]
}
