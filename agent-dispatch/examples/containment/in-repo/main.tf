module "ai_containment" {
  source = "../../../modules/containment/in-repo"

  dispatcher_name        = var.dispatcher_name
  github_pat             = var.github_pat
  slopspaces_working_dir = var.slopspaces_working_dir
}
