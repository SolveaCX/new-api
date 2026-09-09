variable "project_id" {
  type        = string
  description = "GCP project ID"
}

variable "github_repository" {
  type        = string
  description = "GitHub repository in OWNER/REPO form"

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repository))
    error_message = "github_repository must use the OWNER/REPO form."
  }
}

variable "github_repository_id" {
  type        = string
  description = "Immutable numeric GitHub repository ID"

  validation {
    condition     = can(regex("^[0-9]+$", var.github_repository_id))
    error_message = "github_repository_id must be a numeric GitHub repository ID."
  }
}

variable "github_repository_owner_id" {
  type        = string
  description = "Immutable numeric GitHub repository owner ID"

  validation {
    condition     = can(regex("^[0-9]+$", var.github_repository_owner_id))
    error_message = "github_repository_owner_id must be a numeric GitHub repository owner ID."
  }
}

variable "allowed_workflows" {
  type = list(object({
    ref            = string
    workflow_paths = list(string)
  }))
  description = "Allowed branch refs and repository-relative GitHub Actions workflow paths"

  validation {
    condition = (
      length(var.allowed_workflows) > 0 &&
      length(distinct([for rule in var.allowed_workflows : rule.ref])) == length(var.allowed_workflows) &&
      alltrue([
        for rule in var.allowed_workflows :
        can(regex("^refs/heads/[A-Za-z0-9._/-]+$", rule.ref)) &&
        length(rule.workflow_paths) > 0 &&
        length(distinct(rule.workflow_paths)) == length(rule.workflow_paths) &&
        alltrue([
          for path in rule.workflow_paths :
          can(regex("^\\.github/workflows/[A-Za-z0-9._/-]+\\.ya?ml$", path))
        ])
      ])
    )
    error_message = "allowed_workflows must contain unique branch refs and at least one valid .github/workflows/*.yml path per ref."
  }
}

variable "deployer_sa_name" {
  type        = string
  description = "Full resource name of the deployer service account (projects/.../serviceAccounts/...)"
}
