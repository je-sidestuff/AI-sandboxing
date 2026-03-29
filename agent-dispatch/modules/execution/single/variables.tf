# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "branch_name" {
  description = "The branch to clone from and push changes back to."
  type        = string
}

variable "source_repo_url" {
  description = "The authenticated HTTPS URL of the repository (e.g., https://PAT@github.com/owner/repo.git)."
  type        = string
  sensitive   = true
}

variable "dispatcher_name" {
  description = "The name of the dispatcher (used for directory and commit naming)."
  type        = string
}

variable "instruction" {
  description = "The instruction to pass to the AI agent for this execution step."
  type        = string
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "slopspaces_working_dir" {
  description = "The working directory for slopspaces containment operations."
  type        = string
  default     = "/workspaces/slopspaces/working/"
}

variable "instruction_mode" {
  description = "The instruction mode for the AI agent (execute or other modes)."
  type        = string
  default     = "execute"
}
