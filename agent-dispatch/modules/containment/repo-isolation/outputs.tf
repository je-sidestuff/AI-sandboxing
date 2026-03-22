output "isolation_repo_ssh_clone_url" {
  value = github_repository.isolation_repo.ssh_clone_url
}

output "pr_url" {
  description = "The URL of the containment PR for monitoring"
  value       = "https://github.com/${local.isolation_repo_full_name}/pull/${github_repository_pull_request.containment_pr.number}"
}

output "pr_number" {
  description = "The PR number for the containment PR"
  value       = github_repository_pull_request.containment_pr.number
}

output "isolation_repo_full_name" {
  description = "The full name (owner/repo) of the isolation repository"
  value       = local.isolation_repo_full_name
}

output "target_repo" {
  description = "The full name (owner/repo) of the original target repository (for re-integration)"
  value       = var.target_repo
}

output "branch_name" {
  description = "The name of the containment branch created for this dispatch"
  value       = local.branch_name
}

output "unix_timestamp" {
  description = "The unix seconds timestamp used for this dispatch"
  value       = local.unix_timestamp
}

output "dispatch_time" {
  description = "The RFC3339 formatted time when this dispatch was created"
  value       = time_static.dispatch_time.rfc3339
}

output "pr_comments" {
  description = "List of all comment bodies on the containment PR at time of apply"
  value       = jsondecode(local.pr_comments_result.comments_json)
}

# Conclusion state outputs - these are the key outputs for the outer orchestration
# Using the external Python script to fetch PR state since the native Terraform
# GitHub provider data source does not expose merged_at or closed_at attributes.
output "conclusion_state" {
  description = "Simplified conclusion state: 'active' (PR open), 'closed' (PR closed without merge), or 'merged' (PR merged)"
  value       = local.conclusion_state
}

output "pr_state" {
  description = "Raw PR state from GitHub API: 'open' or 'closed'"
  value       = local.pr_comments_result.pr_state
}

output "pr_merged" {
  description = "Whether the PR was merged: 'true' or 'false'"
  value       = local.pr_comments_result.pr_merged
}

output "pr_merged_at" {
  description = "ISO-8601 timestamp when the PR was merged, or empty string if not merged"
  value       = local.pr_comments_result.pr_merged_at
}

output "pr_closed_at" {
  description = "ISO-8601 timestamp when the PR was closed, or empty string if still open"
  value       = local.pr_comments_result.pr_closed_at
}

# Re-integration outputs - only populated when the isolation PR has been merged
output "reintegration_pr_url" {
  description = "The URL of the re-integration PR in the target repo (only set when isolation PR is merged)"
  value       = try("https://github.com/${var.target_repo}/pull/${github_repository_pull_request.reintegration_pr[0].number}", "")
}

output "reintegration_pr_number" {
  description = "The PR number for the re-integration PR (only set when isolation PR is merged)"
  value       = try(github_repository_pull_request.reintegration_pr[0].number, null)
}

output "pr_is_merged" {
  description = "Whether the isolation PR was merged: 'true' or 'false'"
  value       = local.pr_is_merged
}
