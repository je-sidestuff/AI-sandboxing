output "execution_id" {
  description = "The unique ID for this execution step"
  value       = local.execution_id
}

output "unix_timestamp" {
  description = "The unix seconds timestamp used for this execution"
  value       = local.unix_timestamp
}

output "execution_time" {
  description = "The RFC3339 formatted time when this execution was created"
  value       = time_static.execution_time.rfc3339
}

output "branch_name" {
  description = "The branch that was modified (resolved from PR head_ref)"
  value       = local.target_branch
}

output "pr_number" {
  description = "The PR number used to find the target branch"
  value       = var.target_pr.pr_number
}

output "repo_name" {
  description = "The repository name where execution happens"
  value       = var.target_pr.repo
}

output "repo_full_name" {
  description = "The full name (owner/repo) of the target repository"
  value       = local.repo_full_name
}

output "ready_to_execute" {
  description = "Whether the module is ready to execute (repo and PR both exist)"
  value       = local.ready_to_execute
}

output "repo_exists" {
  description = "Whether the target repository exists"
  value       = local.repo_exists
}

output "pr_exists" {
  description = "Whether the target PR exists in the repository"
  value       = local.pr_exists
}
