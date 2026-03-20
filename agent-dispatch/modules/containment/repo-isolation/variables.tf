# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "dispatcher_name" {
  description = "The name of the dispatcher."
  type        = string
}

variable "github_pat" {
  description = "The personal access token used to authenticate for the runner-creation interactions."
  type        = string
  sensitive   = true
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "target_repo" {
  description = "The name of the repo to operate on."
  type        = string
  default     = "je-sidestuff/AI-sandboxing"
}

variable "description" {
  description = "Description of the repo."
  type        = string
  default     = "A repository for AI containment."
}

variable "slopspaces_working_dir" {
  description = "The working directory for slopspaces containment operations."
  type        = string
  default     = "/workspaces/slopspaces/working/"
}
