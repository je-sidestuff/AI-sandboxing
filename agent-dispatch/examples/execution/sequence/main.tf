module "example_helpers" {
  source = "github.com/je-sidestuff/terraform-example-helpers?ref=v0.0.1"
}

module "ai_containment" {
  source = "../../../modules/containment/to-repo"

  target_repo_name       = "to-repo-${module.example_helpers.random_value}"
  dispatcher_name        = var.dispatcher_name
  github_pat             = var.github_pat
  github_owner           = var.github_owner
  slopspaces_working_dir = var.slopspaces_working_dir
}
