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

# Parse SEQUENCE: instructions from the containment PR comments
locals {
  sequence_instructions = jsondecode(module.ai_containment.sequence_instructions_json)
  has_sequence          = length(local.sequence_instructions) > 0

  # For now, we only use the first SEQUENCE: instruction (just one instance)
  first_sequence_instruction = local.has_sequence ? local.sequence_instructions[0] : ""
}

# Execute the single execution step when a SEQUENCE: comment is posted
# This module is only created when there is at least one SEQUENCE: instruction
module "single_execution" {
  source = "../../../modules/execution/single"
  count  = local.has_sequence ? 1 : 0

  branch_name = module.ai_containment.branch_name
  source_repo_url = replace(
    module.ai_containment.target_repo_http_clone_url, "https://", "https://${var.github_pat}@"
  )
  dispatcher_name        = "${var.dispatcher_name}-sequence"
  instruction            = local.first_sequence_instruction
  slopspaces_working_dir = var.slopspaces_working_dir
}
