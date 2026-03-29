locals {
  # Unix timestamp for unique execution naming
  unix_timestamp = time_static.execution_time.unix

  # Unique execution ID for this single execution step
  execution_id = "single_${var.dispatcher_name}_${local.unix_timestamp}"
}

# Capture the current time at resource creation for consistent naming
resource "time_static" "execution_time" {}

# Execute the single AI execution step
# This clones the repo at the branch HEAD, dispatches an AI work unit
# with the instruction, waits for completion, and pushes results back
# to the same branch.
resource "terraform_data" "execute_single_step" {
  provisioner "local-exec" {
    command = "${path.module}/execute_single_step.sh > /tmp/single_step_${local.unix_timestamp}.log 2>&1"

    environment = {
      UNIX_TIMESTAMP      = local.unix_timestamp
      EXECUTION_ID        = local.execution_id
      BRANCH_NAME         = var.branch_name
      SOURCE_REPO_URL     = var.source_repo_url
      SLOPSPACES_WORK_DIR = var.slopspaces_working_dir
      DISPATCHER_NAME     = var.dispatcher_name
      INSTRUCTION         = var.instruction
      INSTRUCTION_MODE    = var.instruction_mode
    }
  }
}
