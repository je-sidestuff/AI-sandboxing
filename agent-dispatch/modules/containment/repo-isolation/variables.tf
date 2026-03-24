# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "name" {
  description = "The name of the isolation repository to create."
  type        = string
}

variable "dispatcher_name" {
  description = "The name of the dispatcher."
  type        = string
}

variable "github_pat" {
  description = "The personal access token used to authenticate for the runner-creation interactions."
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) where the isolation repository will be created. This must be known a priori."
  type        = string
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

variable "enable_reintegration" {
  description = "When true (the default), reintegration resources are created once the isolation PR is merged. On the very first apply the count is deferred; Terraform 1.12+ handles this automatically."
  type        = bool
  default     = true
}

variable "instruction" {
  description = "The instruction to pass to the AI agent for processing in this repo-isolation dispatch."
  type        = string
}

variable "instruction_mode" {
  description = "The mode for the instruction ('prompt' or 'execute')."
  type        = string
  default     = "execute"
}

variable "pr_title" {
  description = "The title for the containment PR. Defaults to a generated title based on dispatcher_name and timestamp."
  type        = string
  default     = ""
}

variable "pr_body" {
  description = "The body for the containment PR. Defaults to the instruction text."
  type        = string
  default     = ""
}

