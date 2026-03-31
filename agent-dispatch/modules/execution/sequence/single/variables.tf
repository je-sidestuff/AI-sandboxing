# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "target_pr" {
  description = <<-EOT
    The repository and PR where execution should happen. Both values must be
    known a-priori (not computed from other resources) to enable the count-based
    filtering pattern that avoids "count cannot be determined until apply" errors.

    - repo: The name of the repository (without owner prefix, e.g., "my-repo")
    - pr_number: The PR number to execute against. The module will look up the PR
                 to find its head branch (the branch where commits are pushed).
  EOT
  type = object({
    repo      = string
    pr_number = number
  })
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) where the repository exists. Must be known a priori."
  type        = string
}

variable "github_pat" {
  description = "The personal access token used to authenticate with GitHub."
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

variable "target_branch" {
  description = <<-EOT
    Optional: The target branch to use for execution. When provided along with
    skip_existence_check=true, the module will skip querying GitHub for repo/PR
    existence and use this branch directly. This is useful when the parent module
    has already verified existence and fetched the branch name.
  EOT
  type        = string
  default     = ""
}

variable "skip_existence_check" {
  description = <<-EOT
    When true, skip the data source queries for repo/PR existence and assume
    the repo and PR exist. The target_branch variable must be provided when
    this is true. This avoids the "count cannot be determined until apply"
    error when this module has depends_on referencing modules with pending changes.
  EOT
  type        = bool
  default     = false
}
