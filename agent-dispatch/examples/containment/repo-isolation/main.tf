module "example_helpers" {
  source = "github.com/je-sidestuff/terraform-example-helpers?ref=v0.0.1"
}

module "ai_containment" {
  source = "../../../modules/containment/repo-isolation"

  name                   = "isolation-${module.example_helpers.random_value}"
  dispatcher_name        = var.dispatcher_name
  github_pat             = var.github_pat
  github_owner           = var.github_owner
  slopspaces_working_dir = var.slopspaces_working_dir
  enable_reintegration   = var.enable_reintegration
}
