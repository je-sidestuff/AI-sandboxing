locals {
  random_result = random_string.random.result
}

resource "random_string" "random" {
  length  = 8
  special = false
}

resource "github_repository" "containment_repo" {
  name        = "${var.name}-${local.random_result}"
  description = var.description
  visibility  = var.visibility

  is_template = false

  auto_init = true
}

# After we have created the repo through TF we obtain a local copy to work with
resource "terraform_data" "prepare_repo" {
  provisioner "local-exec" {
    command = "${path.module}/init_containment_repo.sh > /tmp/loglog.txt 2>&1"

    environment = {
      ACTION               = "create"
      RANDOM_SUFFIX        = local.random_result
      SOURCE_REPO_URL      = replace(
      "https://github.com/forjor/hello-copilot-cli.git", "https://", "https://${var.github_pat}@"
      )
      REPO_HTTPS_CLONE_URL = replace(
      github_repository.containment_repo.http_clone_url, "https://", "https://${var.github_pat}@"
      )
    }
  }

  depends_on = [ github_repository.containment_repo ]
}

# Bring in the created repo (as a second copy on the FLAZZERWOOZLE-WAS-HERE branch) so that we can work with it in TF
data "github_repository" "containment_repo" {
  full_name = github_repository.containment_repo.full_name
  depends_on = [ terraform_data.prepare_repo ]
}

# Create a pull request from the FLAZZERWOOZLE-WAS-HERE branch to main so that we can have a PR to work with in the next steps
resource "github_repository_pull_request" "containment_pr" {
  title     = "FLAZZERWOOZLE was here"
  body      = "This is a PR to let us test out some AI containment strategies."
  head_ref      = "FLAZZERWOOZLE-WAS-HERE"
  base_ref      = "main"
  base_repository = data.github_repository.containment_repo.name
}
