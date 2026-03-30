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

variable "commands" {
  description = <<-EOT
    A map of step numbers to command strings. The keys must be continuous integers
    starting from 1 (e.g., {1 = "cmd1", 2 = "cmd2", 3 = "cmd3"}). Each command
    will be passed to a single execution module in sequence.
  EOT
  type = map(string)

  validation {
    condition = alltrue([
      for k, v in var.commands : can(tonumber(k)) && tonumber(k) >= 1
    ])
    error_message = "All command keys must be positive integers starting from 1."
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "start_time_millis" {
  description = <<-EOT
    The start time in milliseconds for the sequence. On first run, this defaults
    to a far-future date (year 3000). The module outputs an actual_start_time_millis
    that should be fed back into this variable on subsequent applies to enable
    time-based step activation.
  EOT
  type    = number
  default = 32503680000000 # Year 3000 in milliseconds
}

variable "minutes_between_steps" {
  description = "The number of minutes to wait between each step in the sequence."
  type        = number
  default     = 20
}

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
