# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "target_repo_name" {
  description = "The name of the new target repository to create."
  type        = string
}

variable "dispatcher_name" {
  description = "The name of the dispatcher."
  type        = string
}

variable "github_pat" {
  description = "The personal access token used to authenticate for GitHub operations."
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) for the new target repository."
  type        = string
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "source_repo" {
  description = "The name of the source repo to clone from (owner/repo format)."
  type        = string
  default     = "je-sidestuff/AI-sandboxing"
}

variable "description" {
  description = "Description of the new target repository."
  type        = string
  default     = "A repository created for AI containment work."
}

variable "visibility" {
  description = "Visibility of the new target repository (public or private)."
  type        = string
  default     = "private"
}

variable "slopspaces_working_dir" {
  description = "The working directory for slopspaces containment operations."
  type        = string
  default     = "/workspaces/slopspaces/working/"
}

variable "instruction" {
  description = "The instruction to pass to the AI agent."
  type        = string
  default     = "You are working on a contained branch for testing AI code generation.\n\nPlease make meaningful improvements to this repository. Some suggestions:\n- Add useful documentation\n- Improve code quality\n- Add helpful comments\n- Fix any obvious issues\n\nWhen you are done, save your changes. The containment system will handle committing and pushing."
}

variable "instruction_mode" {
  description = "The instruction mode for the AI agent (execute or other modes)."
  type        = string
  default     = "execute"
}
