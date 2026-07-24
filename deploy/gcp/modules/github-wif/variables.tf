variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "github_repository" {
  type        = string
  description = "GitHub repository in OWNER/REPO form"
}

variable "github_repository_id" {
  type        = string
  description = "Immutable numeric GitHub repository ID used by WIF claim guards."
}

variable "github_repository_owner_id" {
  type        = string
  description = "Immutable numeric GitHub repository owner ID used by WIF claim guards."
}

variable "builder_sa_name" {
  type        = string
  description = "Full resource name of the Artifact Registry-only builder service account."
}

variable "production_console_deployer_sa_name" {
  type        = string
  description = "Full resource name of the production Console deployer service account."
}

variable "production_router_deployer_sa_name" {
  type        = string
  description = "Full resource name of the production Router deployer service account."
}

variable "production_console_rollback_sa_name" {
  type        = string
  description = "Full resource name of the production Console rollback service account."
}

variable "production_router_rollback_sa_name" {
  type        = string
  description = "Full resource name of the production Router rollback service account."
}

variable "production_website_deployer_sa_name" {
  type        = string
  description = "Full resource name of the production website deployer service account."
}

variable "staging_deployer_sa_name" {
  type        = string
  description = "Full resource name of the staging service deployer service account."
}

variable "supplier_runner_promoter_sa_name" {
  type        = string
  description = "Full resource name of the fixed supplier runner promoter service account."
}
