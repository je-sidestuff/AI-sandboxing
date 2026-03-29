output "from_module" {
  value = module.ai_containment
}

# Expose key conclusion-related outputs at the top level for easy access
output "conclusion_state" {
  description = "Simplified conclusion state: 'active', 'closed', or 'merged'"
  value       = module.ai_containment.conclusion_state
}

output "pr_url" {
  description = "The URL of the containment PR in the target repo"
  value       = module.ai_containment.pr_url
}

output "target_repo_full_name" {
  description = "The full name (owner/repo) of the created target repository"
  value       = module.ai_containment.target_repo_full_name
}

output "source_repo" {
  description = "The source repo that was cloned from"
  value       = module.ai_containment.source_repo
}

# Sequence execution outputs
output "sequence_instructions" {
  description = "The list of SEQUENCE: instructions found in PR comments"
  value       = local.sequence_instructions
}

output "sequence_execution" {
  description = "The single execution module output"
  value       = module.single_execution
}
