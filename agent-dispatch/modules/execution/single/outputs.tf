output "execution_id" {
  description = "The unique ID for this execution step"
  value       = local.execution_id
}

output "unix_timestamp" {
  description = "The unix seconds timestamp used for this execution"
  value       = local.unix_timestamp
}

output "execution_time" {
  description = "The RFC3339 formatted time when this execution was created"
  value       = time_static.execution_time.rfc3339
}

output "branch_name" {
  description = "The branch that was modified"
  value       = var.branch_name
}
