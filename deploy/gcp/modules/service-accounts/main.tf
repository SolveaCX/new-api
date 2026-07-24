// Runtime and CI identities are deliberately split. The historical
// newapi-ci-deployer account is now a build-only Artifact Registry writer; it
// has no Cloud Run or service-account impersonation permission.

resource "google_service_account" "runtime" {
  project      = var.project_id
  account_id   = "newapi-runtime"
  display_name = "new-api Cloud Run runtime"
}

resource "google_service_account" "deployer" {
  project      = var.project_id
  account_id   = "newapi-ci-deployer"
  display_name = "new-api CI image builder (GitHub Actions via WIF)"
}

resource "google_service_account" "production_console_deployer" {
  project      = var.project_id
  account_id   = "newapi-prod-console-deployer"
  display_name = "new-api production Console deployer"
}

resource "google_service_account" "production_router_deployer" {
  project      = var.project_id
  account_id   = "newapi-prod-router-deployer"
  display_name = "new-api production Router deployer"
}

resource "google_service_account" "production_console_rollback" {
  project      = var.project_id
  account_id   = "newapi-prod-console-rollback"
  display_name = "new-api production Console rollback"
}

resource "google_service_account" "production_router_rollback" {
  project      = var.project_id
  account_id   = "newapi-prod-router-rollback"
  display_name = "new-api production Router rollback"
}

resource "google_service_account" "production_website_deployer" {
  project      = var.project_id
  account_id   = "newapi-prod-web-deployer"
  display_name = "new-api production website deployer"
}

resource "google_service_account" "staging_deployer" {
  project      = var.project_id
  account_id   = "newapi-staging-deployer"
  display_name = "new-api staging service deployer"
}

// Runtime SA permissions
resource "google_project_iam_member" "runtime_cloudsql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.runtime.email}"
}

// Per-secret accessor grant for runtime SA
resource "google_secret_manager_secret_iam_member" "runtime_secret_access" {
  for_each = toset(var.runtime_secret_ids)

  project   = var.project_id
  secret_id = each.value
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_project_iam_member" "runtime_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_project_iam_member" "runtime_metrics" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_project_iam_member" "runtime_trace" {
  project = var.project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.runtime.email}"
}

// Read-only Cloud Run evidence needed by gcloud describe/update polling. This
// role intentionally omits every run.jobs.* permission.
resource "google_project_iam_custom_role" "service_deployer_observer" {
  project     = var.project_id
  role_id     = "newapiCloudRunServiceObserver"
  title       = "new-api Cloud Run Service Observer"
  description = "Read Cloud Run service, revision, location, and operation evidence without Job access."
  permissions = [
    "run.locations.list",
    "run.operations.get",
    "run.revisions.get",
    "run.services.get",
  ]
}

// The only mutation permission granted to application deploy identities. The
// composition layer binds it directly on each exact Cloud Run service.
resource "google_project_iam_custom_role" "service_deployer_mutator" {
  project     = var.project_id
  role_id     = "newapiCloudRunServiceMutator"
  title       = "new-api Cloud Run Service Mutator"
  description = "Update an explicitly conditioned Cloud Run service."
  permissions = [
    "run.services.get",
    "run.services.update",
  ]
}

locals {
  service_observers = {
    production_console = google_service_account.production_console_deployer.email
    production_router  = google_service_account.production_router_deployer.email
    rollback_console   = google_service_account.production_console_rollback.email
    rollback_router    = google_service_account.production_router_rollback.email
    production_website = google_service_account.production_website_deployer.email
    staging            = google_service_account.staging_deployer.email
  }
  image_deployers = {
    production_console = google_service_account.production_console_deployer.email
    production_router  = google_service_account.production_router_deployer.email
    production_website = google_service_account.production_website_deployer.email
    staging            = google_service_account.staging_deployer.email
  }
}

resource "google_project_iam_member" "service_deployer_observer" {
  for_each = local.service_observers

  project = var.project_id
  role    = google_project_iam_custom_role.service_deployer_observer.name
  member  = "serviceAccount:${each.value}"
}

resource "google_artifact_registry_repository_iam_member" "builder_writer" {
  project    = var.project_id
  location   = var.region
  repository = var.artifact_registry_repository_id
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_artifact_registry_repository_iam_member" "service_deployer_reader" {
  for_each = local.image_deployers

  project    = var.project_id
  location   = var.region
  repository = var.artifact_registry_repository_id
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${each.value}"
}

resource "google_service_account_iam_member" "production_console_runtime_sa_user" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.production_console_deployer.email}"
}

resource "google_service_account_iam_member" "production_router_runtime_sa_user" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.production_router_deployer.email}"
}
