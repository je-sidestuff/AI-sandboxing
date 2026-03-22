module "example_helpers" {
  source = "github.com/je-sidestuff/terraform-example-helpers?ref=v0.0.1"
}

module "approval_request" {
  source = "../../../modules/containment/approval"

  dispatcher_name     = "${var.dispatcher_name}-${module.example_helpers.random_value}"
  github_owner        = var.github_owner
  github_pat          = var.github_pat
  approval_repo       = var.approval_repo
  pending_instruction = var.pending_instruction
  pending_mode        = var.pending_mode
  pending_agent       = var.pending_agent
  source_context      = var.source_context
}
