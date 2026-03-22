output "from_module" {
  value = module.ai_containment
}

# Expose key conclusion-related outputs at the top level for easy access
output "conclusion_state" {
  description = "Simplified conclusion state: 'active', 'closed', or 'merged'"
  value       = module.ai_containment.conclusion_state
}

output "pr_url" {
  description = "The URL of the containment PR in the isolation repo"
  value       = module.ai_containment.pr_url
}

output "target_repo" {
  description = "The original target repo (for re-integration when merged)"
  value       = module.ai_containment.target_repo
}

# Re-integration outputs - populated when the isolation PR is merged
output "reintegration_pr_url" {
  description = "The URL of the re-integration PR in the target repo (only set when merged)"
  value       = module.ai_containment.reintegration_pr_url
}

output "reintegration_conclusion_state" {
  description = "Conclusion state of the reintegration PR: 'none', 'active', 'closed', or 'merged'"
  value       = module.ai_containment.reintegration_conclusion_state
}
