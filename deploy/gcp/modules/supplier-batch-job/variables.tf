variable "project_id" {
  type        = string
  description = "GCP project ID."
}

variable "region" {
  type        = string
  description = "GCP region for the Cloud Run Job and Cloud Scheduler job."
}

variable "artifact_registry_repository_id" {
  type        = string
  description = "Artifact Registry repository containing the promoted server image."
}

variable "enabled" {
  type        = bool
  description = "Create the Cloud Run Job and Scheduler after both secret slots have enabled versions and Console verifier hashes are ready. Service accounts, secret containers, and IAM are always created."
  default     = false
}

variable "job_name" {
  type        = string
  description = "Cloud Run Job name."
  default     = "newapi-supplier-batch"
}

variable "schedule_name" {
  type        = string
  description = "Cloud Scheduler job name."
  default     = "newapi-supplier-batch-daily"
}

variable "runner_image" {
  type        = string
  description = "Immutable supplier batch runner image reference. Tags are rejected."
  default     = null
  nullable    = true

  validation {
    condition     = var.runner_image == null || trimspace(var.runner_image) == "" || can(regex("^[^[:space:]@]+@sha256:[0-9a-fA-F]{64}$", var.runner_image))
    error_message = "runner_image must be null/empty while disabled or an immutable image@sha256:<64 hexadecimal characters> reference."
  }
}

variable "console_url" {
  type        = string
  description = "HTTPS origin of the Console service used by the one-shot runner."

  validation {
    condition     = can(regex("^https://[^/[:space:]]+/?$", var.console_url))
    error_message = "console_url must be an HTTPS origin without a path."
  }
}

variable "active_token_slot" {
  type        = string
  description = "Secret container mounted as SUPPLIER_BATCH_TOKEN. Rotate by preparing the inactive slot, updating Console verifier hashes, and switching this value."
  default     = "current"

  validation {
    condition     = contains(["current", "next"], var.active_token_slot)
    error_message = "active_token_slot must be current or next."
  }
}

variable "poll_interval" {
  type        = string
  description = "Optional Go duration used while polling ambiguous scheduler command results."
  default     = "5s"

  validation {
    condition     = can(regex("^[1-9][0-9]*(ms|s|m)$", var.poll_interval))
    error_message = "poll_interval must be a positive Go duration using ms, s, or m."
  }
}

variable "schedule" {
  type        = string
  description = "Cloud Scheduler cron expression. 02:05 leaves the recovery window before the 08:00 SLO."
  default     = "5 2 * * *"
}

variable "time_zone" {
  type        = string
  description = "IANA timezone used by Cloud Scheduler."
  default     = "Asia/Shanghai"
}

variable "job_timeout_seconds" {
  type        = number
  description = "Cloud Run Job task timeout. Must be at least 60 minutes."
  default     = 3600

  validation {
    condition     = var.job_timeout_seconds >= 3600
    error_message = "job_timeout_seconds must be at least 3600."
  }
}
