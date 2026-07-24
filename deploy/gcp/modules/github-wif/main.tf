// Each privileged workflow lane gets its own Workload Identity Pool. IAM
// principals are pool-scoped, so separate providers in one shared pool would
// not isolate workflows that share the same GitHub Environment subject.

locals {
  main_ref_subject           = "repo:${var.github_repository}:ref:refs/heads/main"
  staging_ref_subject        = "repo:${var.github_repository}:ref:refs/heads/staging"
  production_subject         = "repo:${var.github_repository}:environment:production"
  production_console_subject = "repo:${var.github_repository}:environment:production-console"
  production_router_subject  = "repo:${var.github_repository}:environment:production-router"
  repository_claim_guard     = "assertion.repository == '${var.github_repository}' && assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_repository_owner_id}'"
  privileged_workload_lanes = {
    production_app = {
      pool_id      = "github-prod-app-deploy"
      display_name = "GitHub production app deploy"
      condition    = "${local.repository_claim_guard} && assertion.ref == 'refs/heads/main' && (assertion.event_name == 'push' || assertion.event_name == 'workflow_dispatch') && assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy.yml@refs/heads/main' && (assertion.sub == '${local.production_console_subject}' || assertion.sub == '${local.production_router_subject}')"
    }
    production_rollback = {
      pool_id      = "github-prod-rollback"
      display_name = "GitHub production rollback"
      condition    = "${local.repository_claim_guard} && assertion.ref == 'refs/heads/main' && assertion.event_name == 'workflow_dispatch' && assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-rollback.yml@refs/heads/main' && (assertion.sub == '${local.production_console_subject}' || assertion.sub == '${local.production_router_subject}')"
    }
    production_website = {
      pool_id      = "github-prod-web-deploy"
      display_name = "GitHub production web deploy"
      condition    = "${local.repository_claim_guard} && assertion.ref == 'refs/heads/main' && (assertion.event_name == 'push' || assertion.event_name == 'workflow_dispatch') && (assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-website.yml@refs/heads/main' || assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-website-static.yml@refs/heads/main') && assertion.sub == '${local.production_subject}'"
    }
    staging = {
      pool_id      = "github-staging-deploy"
      display_name = "GitHub staging deploy"
      condition    = "${local.repository_claim_guard} && assertion.ref == 'refs/heads/staging' && (assertion.event_name == 'push' || assertion.event_name == 'workflow_dispatch') && (assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-staging.yml@refs/heads/staging' || assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-website-staging.yml@refs/heads/staging') && assertion.sub == '${local.staging_ref_subject}'"
    }
    supplier_promotion = {
      pool_id      = "github-supplier-promote"
      display_name = "GitHub supplier promotion"
      condition    = "${local.repository_claim_guard} && assertion.ref == 'refs/heads/main' && assertion.event_name == 'workflow_dispatch' && assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-promote-supplier-runner.yml@refs/heads/main' && assertion.sub == '${local.production_subject}'"
    }
  }
  common_attribute_mapping = {
    "google.subject"                = "assertion.sub"
    "attribute.repository"          = "assertion.repository"
    "attribute.repository_id"       = "assertion.repository_id"
    "attribute.repository_owner_id" = "assertion.repository_owner_id"
    "attribute.ref"                 = "assertion.ref"
    "attribute.event_name"          = "assertion.event_name"
    "attribute.workflow_ref"        = "assertion.workflow_ref"
  }
}

// The historical pool is retained only for image builds. Its impersonated SA
// has repository-scoped Artifact Registry writer and no Run/Job/actAs access.
resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = "github-actions"
  display_name              = "GitHub image builds"
  description               = "Build-only OIDC trust for ${var.github_repository}"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub image builds"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping   = local.common_attribute_mapping
  attribute_condition = "${local.repository_claim_guard} && (assertion.event_name == 'push' || assertion.event_name == 'workflow_dispatch') && ((assertion.ref == 'refs/heads/main' && assertion.sub == '${local.main_ref_subject}' && (assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy.yml@refs/heads/main' || assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-website.yml@refs/heads/main' || assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-website-static.yml@refs/heads/main')) || (assertion.ref == 'refs/heads/staging' && assertion.sub == '${local.staging_ref_subject}' && (assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-staging.yml@refs/heads/staging' || assertion.workflow_ref == '${var.github_repository}/.github/workflows/gcp-deploy-website-staging.yml@refs/heads/staging')))"
}

resource "google_iam_workload_identity_pool" "privileged" {
  for_each = local.privileged_workload_lanes

  project                   = var.project_id
  workload_identity_pool_id = each.value.pool_id
  display_name              = each.value.display_name
  description               = "Workflow-pinned OIDC trust for ${var.github_repository}"
}

resource "google_iam_workload_identity_pool_provider" "privileged" {
  for_each = local.privileged_workload_lanes

  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.privileged[each.key].workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = each.value.display_name

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping   = local.common_attribute_mapping
  attribute_condition = each.value.condition
}

resource "google_service_account_iam_member" "wif_builder_main" {
  service_account_id = var.builder_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/subject/${local.main_ref_subject}"
}

resource "google_service_account_iam_member" "wif_builder_staging" {
  service_account_id = var.builder_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/subject/${local.staging_ref_subject}"
}

resource "google_service_account_iam_member" "wif_production_console_deploy" {
  service_account_id = var.production_console_deployer_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["production_app"].name}/subject/${local.production_console_subject}"
}

resource "google_service_account_iam_member" "wif_production_router_deploy" {
  service_account_id = var.production_router_deployer_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["production_app"].name}/subject/${local.production_router_subject}"
}

resource "google_service_account_iam_member" "wif_production_console_rollback" {
  service_account_id = var.production_console_rollback_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["production_rollback"].name}/subject/${local.production_console_subject}"
}

resource "google_service_account_iam_member" "wif_production_router_rollback" {
  service_account_id = var.production_router_rollback_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["production_rollback"].name}/subject/${local.production_router_subject}"
}

resource "google_service_account_iam_member" "wif_production_website_deploy" {
  service_account_id = var.production_website_deployer_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["production_website"].name}/subject/${local.production_subject}"
}

resource "google_service_account_iam_member" "wif_staging_deploy" {
  service_account_id = var.staging_deployer_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["staging"].name}/subject/${local.staging_ref_subject}"
}

resource "google_service_account_iam_member" "wif_supplier_runner_promoter" {
  service_account_id = var.supplier_runner_promoter_sa_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principal://iam.googleapis.com/${google_iam_workload_identity_pool.privileged["supplier_promotion"].name}/subject/${local.production_subject}"
}
