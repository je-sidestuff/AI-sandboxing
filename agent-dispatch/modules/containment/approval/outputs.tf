output "pr_url" {
  description = "The URL of the approval PR for monitoring"
  value       = local.approval_pr_url
}

output "pr_number" {
  description = "The PR number for the approval PR"
  value       = github_repository_pull_request.approval_pr.number
}

output "approval_repo_full_name" {
  description = "The full name (owner/repo) of the approval repository"
  value       = local.approval_repo_full_name
}

output "branch_name" {
  description = "The name of the approval branch created for this request"
  value       = local.branch_name
}

output "unix_timestamp" {
  description = "The unix seconds timestamp used for this approval request"
  value       = local.unix_timestamp
}

output "dispatch_time" {
  description = "The RFC3339 formatted time when this approval request was created"
  value       = time_static.dispatch_time.rfc3339
}

output "approval_file_path" {
  description = "The path to the approval request file in the repository"
  value       = github_repository_file.approval_request.file
}

# Conclusion state outputs - these are the key outputs for the outer orchestration
output "conclusion_state" {
  description = "Simplified conclusion state: 'active' (PR open), 'closed' (PR closed without merge), or 'merged' (PR merged/approved)"
  value       = local.conclusion_state
}

output "pr_state" {
  description = "Raw PR state from GitHub API: 'open' or 'closed'"
  value       = local.pr_comments_result.pr_state
}

output "pr_merged" {
  description = "Whether the PR was merged (approved): 'true' or 'false'"
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

# Pending instruction outputs for use by the Go watch loop when approved
output "pending_instruction" {
  description = "The instruction that will be executed if approved"
  value       = var.pending_instruction
}

output "pending_mode" {
  description = "The mode for executing the pending instruction"
  value       = var.pending_mode
}

output "pending_agent" {
  description = "The agent override for executing the pending instruction (may be empty)"
  value       = var.pending_agent
}

# ---------------------------------------------------------------------------------------------------------------------
# SEQUENCE OUTPUTS
# These outputs are used for sequence-to-new-repo dispatches.
# ---------------------------------------------------------------------------------------------------------------------

output "sequence_commands" {
  description = "The list of sequence commands to execute (empty if not a sequence dispatch)"
  value       = var.sequence_commands
}

output "sequence_minutes_between" {
  description = "Minutes between sequence steps (default 20)"
  value       = var.sequence_minutes_between
}

output "is_sequence" {
  description = "Whether this is a sequence dispatch (has sequence_commands)"
  value       = local.is_sequence
}
