module "ai_containment" {
  source = "../../../modules/containment/in-repo"

  github_pat            = var.github_pat
  slopspaces_working_dir = var.slopspaces_working_dir
}
