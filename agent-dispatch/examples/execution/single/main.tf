module "example_helpers" {
  source = "github.com/je-sidestuff/terraform-example-helpers?ref=v0.0.1"
}

# Compute the target repo name a-priori so it can be used in both modules
locals {
  target_repo_name = "to-repo-${module.example_helpers.random_value}"

  # The containment module always creates PR #1 in the new target repository.
  # We declare this a-priori so the execution module can check for its existence
  # without depending on a computed value from the containment module.
  expected_pr_number = 1
}

module "ai_containment" {
  source = "../../../modules/containment/to-repo"

  target_repo_name       = local.target_repo_name
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

# Execute the single execution step when a SEQUENCE: comment is posted.
# This module uses the count-based filtering pattern internally - it checks
# if the target repo and PR exist, and only executes when both conditions
# are met. The PR number is known a-priori (always 1 for new repos).
#
# On first apply: repo and PR don't exist yet, module.single_execution.ready_to_execute = false
# On second apply: repo and PR exist, module checks and executes when ready
module "single_execution" {
  source = "../../../modules/execution/single"

  target_pr = {
    repo      = local.target_repo_name
    pr_number = local.expected_pr_number
  }

  github_owner           = var.github_owner
  github_pat             = var.github_pat
  dispatcher_name        = "${var.dispatcher_name}-sequence"
  instruction            = local.first_sequence_instruction
  slopspaces_working_dir = var.slopspaces_working_dir
}
