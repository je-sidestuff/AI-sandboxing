# ---------------------------------------------------------------------------------------------------------------------
# REQUIRED PARAMETERS
# You must provide a value for each of these parameters.
# ---------------------------------------------------------------------------------------------------------------------

variable "github_pat" {
  description = "The personal access token used to authenticate for GitHub operations."
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) where the target repository will be created."
  type        = string
  default     = "je-sidestuff"
}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL PARAMETERS
# These parameters have reasonable defaults.
# ---------------------------------------------------------------------------------------------------------------------

variable "dispatcher_name" {
  description = "The name of the dispatcher."
  type        = string
  default     = "dispatcher"
}

variable "slopspaces_working_dir" {
  description = "The working directory for slopspaces containment operations."
  type        = string
  default     = "/workspaces/slopspaces/working/"
}

variable "start_time_millis" {
  description = <<-EOT
    The start time in milliseconds for the sequence. On first run, this defaults
    to a far-future date (year 3000) so no steps execute. After the first apply,
    capture the actual_start_time_millis output and pass it back here on
    subsequent applies to enable time-based step activation.
  EOT
  type    = number
  default = 32503680000000 # Year 3000 in milliseconds
}

variable "minutes_between_steps" {
  description = "The number of minutes to wait between each step in the sequence."
  type        = number
  default     = 10
}
