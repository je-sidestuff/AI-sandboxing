output "isolation_repo_ssh_clone_url" {
  value = github_repository.isolation_repo.ssh_clone_url
}

output "pr_url" {
  description = "The URL of the containment PR for monitoring"
  value       = "https://github.com/${github_repository.isolation_repo.full_name}/pull/${github_repository_pull_request.containment_pr.number}"
}

output "pr_number" {
  description = "The PR number for the containment PR"
  value       = github_repository_pull_request.containment_pr.number
}

output "isolation_repo_full_name" {
  description = "The full name (owner/repo) of the isolation repository"
  value       = github_repository.isolation_repo.full_name
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
  value       = jsondecode(data.external.pr_comments.result.comments_json)
}
