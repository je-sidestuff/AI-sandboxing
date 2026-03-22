output "pr_url" {
  description = "The URL of the approval PR"
  value       = module.approval_request.pr_url
}

output "pr_number" {
  description = "The PR number for the approval PR"
  value       = module.approval_request.pr_number
}

output "approval_repo_full_name" {
  description = "The full name (owner/repo) of the approval repository"
  value       = module.approval_request.approval_repo_full_name
}

output "branch_name" {
  description = "The name of the approval branch"
  value       = module.approval_request.branch_name
}

output "conclusion_state" {
  description = "Current conclusion state: 'active', 'closed', or 'merged'"
  value       = module.approval_request.conclusion_state
}

output "pending_instruction" {
  description = "The instruction that will be executed if approved"
  value       = module.approval_request.pending_instruction
}

output "pending_mode" {
  description = "The mode for executing the pending instruction"
  value       = module.approval_request.pending_mode
}
