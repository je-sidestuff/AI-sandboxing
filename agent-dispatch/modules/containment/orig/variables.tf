# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "github_pat" {
  description = "The personal access token used to authenticate for the runner-creation interactions."
  type        = string
  sensitive   = true
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "name" {
  description = "The name oh the repo to create."
  type        = string
  default     = "ai-containment"
}

variable "visibility" {
  description = "The visibility for the smart template repo to deploy. (public or private)"
  type        = string
  default     = "private"
  validation {
    condition     = contains(["public", "private"], var.visibility)
    error_message = "The visibility must be 'public' or 'private'."
  }
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
