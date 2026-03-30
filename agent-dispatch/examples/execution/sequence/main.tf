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
  sequence_instructions = [
    "We're writing a story about cats - write chapter 1 in docs/CHAPTER1.md",
    "We're writing a story about cats - write chapter 2 in docs/CHAPTER2.md",
    "We're writing a story about cats - write chapter 3 in docs/CHAPTER3.md"
  ]
  has_sequence          = length(local.sequence_instructions) > 0

  # Build a commands map from the first 3 SEQUENCE: instructions
  # The sequence module expects a map with string keys "1", "2", "3", etc.
  sequence_commands = {
    for i in range(min(3, length(local.sequence_instructions))) :
    tostring(i + 1) => local.sequence_instructions[i]
  }
}

# Execute a 3-command sequence when SEQUENCE: comments are posted.
# This module uses the count-based filtering pattern internally - it checks
# if the target repo and PR exist, and only executes when both conditions
# are met. The PR number is known a-priori (always 1 for new repos).
#
# On first apply: repo and PR don't exist yet, module won't execute any steps
# On subsequent applies: steps activate based on time elapsed since start_time_millis
module "sequence_execution" {
  source = "../../../modules/execution/sequence"

  target_pr = {
    repo      = local.target_repo_name
    pr_number = local.expected_pr_number
  }

  github_owner           = var.github_owner
  github_pat             = var.github_pat
  dispatcher_name        = "${var.dispatcher_name}-sequence"
  commands               = local.sequence_commands
  start_time_millis      = var.start_time_millis
  minutes_between_steps  = var.minutes_between_steps
  slopspaces_working_dir = var.slopspaces_working_dir
}
