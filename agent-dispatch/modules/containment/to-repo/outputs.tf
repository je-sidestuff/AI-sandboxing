output "target_repo_ssh_clone_url" {
  description = "The SSH clone URL of the created target repository"
  value       = github_repository.target_repo.ssh_clone_url
}

output "target_repo_http_clone_url" {
  description = "The HTTP clone URL of the created target repository"
  value       = github_repository.target_repo.http_clone_url
}

output "target_repo_full_name" {
  description = "The full name (owner/repo) of the created target repository"
  value       = local.target_repo_full_name
}

output "source_repo" {
  description = "The full name (owner/repo) of the source repository"
  value       = var.source_repo
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

output "pr_url" {
  description = "The URL of the containment PR for monitoring"
  value       = "https://github.com/${local.target_repo_full_name}/pull/${github_repository_pull_request.containment_pr.number}"
}

output "pr_number" {
  description = "The PR number for the containment PR"
  value       = github_repository_pull_request.containment_pr.number
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

output "sequence_instructions_json" {
  description = "JSON-encoded list of SEQUENCE: instruction strings from PR comments"
  value       = local.pr_comments_result.sequence_instructions_json
}
