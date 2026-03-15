output "repo_ssh_clone_url" {
  value = data.github_repository.target_repo.ssh_clone_url
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
