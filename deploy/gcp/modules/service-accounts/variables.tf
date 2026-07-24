variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "region" {
  type        = string
  description = "GCP region containing the Artifact Registry repository and Cloud Run services."
}

variable "artifact_registry_repository_id" {
  type        = string
  description = "Artifact Registry repository used by CI builds and Cloud Run deployments."
}

variable "runtime_secret_ids" {
  type        = list(string)
  description = "Secret Manager secret IDs the runtime SA must read"
  default     = []
}
