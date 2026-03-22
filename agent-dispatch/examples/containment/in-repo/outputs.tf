output "from_module" {
  value = module.ai_containment
}

# Expose key conclusion-related outputs at the top level for easy access
output "conclusion_state" {
  description = "Simplified conclusion state: 'active', 'closed', or 'merged'"
  value       = module.ai_containment.conclusion_state
}

output "pr_url" {
  description = "The URL of the containment PR"
  value       = module.ai_containment.pr_url
}
