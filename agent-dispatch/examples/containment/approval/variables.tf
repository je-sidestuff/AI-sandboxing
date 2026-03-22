# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "github_pat" {
  description = "The personal access token used to authenticate for GitHub API interactions."
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) for the approval repository."
  type        = string
  default     = "je-sidestuff"
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "dispatcher_name" {
  description = "The name of the dispatcher (used for branch and file naming)."
  type        = string
  default     = "dispatcher"
}

variable "approval_repo" {
  description = "The name of the repository where approval PRs will be created."
  type        = string
  default     = "sloppo"
}

variable "pending_instruction" {
  description = "The instruction that will be executed if the approval PR is merged."
  type        = string
  default     = "echo 'Hello from an approved instruction!'"
}

variable "pending_mode" {
  description = "The mode for the pending instruction ('prompt' or 'execute')."
  type        = string
  default     = "execute"
}

variable "pending_agent" {
  description = "Optional agent override for the pending instruction."
  type        = string
  default     = ""
}

variable "source_context" {
  description = "Description of where this approval request originated from."
  type        = string
  default     = "example-manual-test"
}
