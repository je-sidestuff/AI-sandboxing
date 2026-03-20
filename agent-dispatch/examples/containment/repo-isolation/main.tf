resource "random_pet" "isolation_repo_name" {
  length    = 2
  separator = "-"
}

module "ai_containment" {
  source = "../../../modules/containment/repo-isolation"

  name                   = "isolation-${random_pet.isolation_repo_name.id}"
  dispatcher_name        = var.dispatcher_name
  github_pat             = var.github_pat
  slopspaces_working_dir = var.slopspaces_working_dir
}
