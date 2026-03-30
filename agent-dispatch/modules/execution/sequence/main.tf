# =============================================================================
# SEQUENCE EXECUTION MODULE
#
# This module orchestrates up to 100 single execution steps in a linear
# dependency chain. Steps are activated based on:
#   1. Time elapsed since start (minutes_between_steps intervals)
#   2. Presence of the command in the commands map
#   3. Repo and PR existence (inherited from single execution module)
# =============================================================================

locals {
  # The full name of the repo, computed a priori from input variables
  repo_full_name = "${var.github_owner}/${var.target_pr.repo}"

  # Milliseconds between each step
  millis_between_steps = var.minutes_between_steps * 60 * 1000
}

# =============================================================================
# TIME MEASUREMENT
# Capture the actual start time on first execution and measure current time.
# =============================================================================

# This time_static creates the actual start time output that callers should
# feed back into start_time_millis on subsequent applies.
resource "time_static" "sequence_start" {}

locals {
  # The actual start time in milliseconds - this is the value callers should
  # capture and feed back into the start_time_millis variable.
  actual_start_time_millis = time_static.sequence_start.unix * 1000
}

# Fetch the current time in milliseconds using external data source.
# This is evaluated on each apply to determine which steps are ready.
data "external" "current_time" {
  program = ["bash", "-c", <<-EOT
    echo "{\"millis\": \"$(date +%s%3N)\"}"
  EOT
  ]
}

locals {
  # Current time in milliseconds
  current_time_millis = tonumber(data.external.current_time.result.millis)

  # Calculate time elapsed since the configured start time
  elapsed_millis = local.current_time_millis - var.start_time_millis
}

# =============================================================================
# EXISTENCE CHECK: Use data sources with a-priori-known strings to determine
# if the target repo and PR exist. This avoids "count cannot be determined
# until apply" errors by never depending on resource-computed attributes.
# =============================================================================

# Look up repos broadly by owner so the query doesn't depend on computed values.
data "github_repositories" "target" {
  query = "user:${var.github_owner} ${var.target_pr.repo} in:name"
}

locals {
  # Filter the broad search results to find exactly our repo
  repo_match  = [for n in data.github_repositories.target.names : n if n == var.target_pr.repo]
  repo_exists = length(local.repo_match) > 0
}

# Look up the PR details using the external script.
data "external" "pr_details" {
  count = local.repo_exists ? 1 : 0

  program = ["python3", "${path.module}/../../containment/scripts/fetch_pr_comments.py"]

  query = {
    pat       = var.github_pat
    repo      = local.repo_full_name
    pr_number = tostring(var.target_pr.pr_number)
  }
}

locals {
  # Extract PR details from the external data source
  pr_result     = local.repo_exists ? data.external.pr_details[0].result : null
  pr_exists     = local.pr_result != null && try(local.pr_result.head_ref, "") != ""
  target_branch = local.pr_exists ? local.pr_result.head_ref : ""

  # Base readiness: both repo and PR must exist
  base_ready = local.repo_exists && local.pr_exists
}

# =============================================================================
# STEP READINESS CALCULATION
# For each potential step (1-100), determine if it should execute based on:
#   1. The command exists in the commands map
#   2. Enough time has elapsed since start_time_millis
#   3. Base readiness (repo + PR exist)
# =============================================================================

locals {
  # Calculate readiness for each step (1-100)
  # A step is ready when:
  #   - base_ready is true (repo and PR exist)
  #   - The step number exists as a key in commands map
  #   - Enough time has elapsed: elapsed_millis >= (step_number - 1) * millis_between_steps
  step_ready = {
    for i in range(1, 101) : i => (
      local.base_ready &&
      contains(keys(var.commands), tostring(i)) &&
      local.elapsed_millis >= (i - 1) * local.millis_between_steps
    )
  }

  # Get the instruction for each step (empty string if not in map)
  step_instruction = {
    for i in range(1, 101) : i => lookup(var.commands, tostring(i), "")
  }
}
