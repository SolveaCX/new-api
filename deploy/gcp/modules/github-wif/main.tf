// Workload Identity Federation: lets GitHub Actions exchange OIDC tokens for
// short-lived GCP credentials — no service account keys stored anywhere.

locals {
  allowed_workflow_conditions = [
    for rule in var.allowed_workflows :
    "(assertion.ref == '${rule.ref}' && assertion.workflow_ref in [${join(",", [
      for workflow_path in rule.workflow_paths :
      "'${var.github_repository}/${workflow_path}@${rule.ref}'"
    ])}])"
  ]
}

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = "github-actions"
  display_name              = "GitHub Actions"
  description               = "OIDC trust for ${var.github_repository}"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub OIDC"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"                = "assertion.sub"
    "attribute.repository"          = "assertion.repository"
    "attribute.repository_id"       = "assertion.repository_id"
    "attribute.repository_owner_id" = "assertion.repository_owner_id"
    "attribute.workflow_ref"        = "assertion.workflow_ref"
    "attribute.ref"                 = "assertion.ref"
    "attribute.actor"               = "assertion.actor"
  }

  // Require immutable repository identity and an explicitly approved workflow/ref pair.
  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_repository_owner_id}' && (${join(" || ", local.allowed_workflow_conditions)})"
}

// Bind the deployer SA so principals from the target repo can impersonate it.
resource "google_service_account_iam_member" "wif_deploy_binding" {
  service_account_id = var.deployer_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"
}
