# =============================================================================
# SEQUENCE MODULE OUTPUTS
# =============================================================================

output "actual_start_time_millis" {
  description = <<-EOT
    The actual start time in milliseconds captured when this module was first applied.
    Feed this value back into the start_time_millis variable on subsequent applies
    to enable time-based step activation.
  EOT
  value = local.actual_start_time_millis
}

output "sequence_start_time" {
  description = "The RFC3339 formatted time when this sequence was started"
  value       = time_static.sequence_start.rfc3339
}

output "current_time_millis" {
  description = "The current time in milliseconds (at time of last apply)"
  value       = local.current_time_millis
}

output "elapsed_millis" {
  description = "Milliseconds elapsed since the configured start_time_millis"
  value       = local.elapsed_millis
}

output "millis_between_steps" {
  description = "Milliseconds between each step (minutes_between_steps * 60 * 1000)"
  value       = local.millis_between_steps
}

output "repo_full_name" {
  description = "The full name (owner/repo) of the target repository"
  value       = local.repo_full_name
}

output "repo_exists" {
  description = "Whether the target repository exists"
  value       = local.repo_exists
}

output "pr_exists" {
  description = "Whether the target PR exists in the repository"
  value       = local.pr_exists
}

output "base_ready" {
  description = "Whether the base conditions are met (repo and PR both exist)"
  value       = local.base_ready
}

output "target_branch" {
  description = "The branch that will be modified (resolved from PR head_ref)"
  value       = local.target_branch
}

output "commands_count" {
  description = "The number of commands provided in the commands map"
  value       = length(var.commands)
}

output "step_readiness" {
  description = "Map of step numbers to their readiness status (for debugging)"
  value       = local.step_ready
}
