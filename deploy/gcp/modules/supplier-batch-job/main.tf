locals {
  runner_service_account_id    = "newapi-supplier-batch-runner"
  scheduler_service_account_id = "newapi-supplier-scheduler"
  promoter_service_account_id  = "newapi-supplier-promoter"
  token_secret_ids = {
    current = "newapi-supplier-batch-token-current"
    next    = "newapi-supplier-batch-token-next"
  }
}

resource "google_service_account" "runner" {
  project      = var.project_id
  account_id   = local.runner_service_account_id
  display_name = "new-api supplier accounting one-shot runner"
}

resource "google_service_account" "scheduler" {
  project      = var.project_id
  account_id   = local.scheduler_service_account_id
  display_name = "new-api supplier accounting Cloud Scheduler trigger"
}

resource "google_service_account" "promoter" {
  project      = var.project_id
  account_id   = local.promoter_service_account_id
  display_name = "new-api supplier runner production promoter"
}

resource "google_project_iam_custom_role" "promoter_job" {
  project     = var.project_id
  role_id     = "newapiSupplierRunnerJobPromoter"
  title       = "new-api Supplier Runner Job Promoter"
  description = "Inspect, update, and execute the fixed supplier Cloud Run Job."
  permissions = [
    "run.jobs.get",
    "run.jobs.run",
    "run.jobs.update",
  ]
}

resource "google_project_iam_custom_role" "promoter_observer" {
  project     = var.project_id
  role_id     = "newapiSupplierRunnerPromotionObserver"
  title       = "new-api Supplier Runner Promotion Observer"
  description = "Read project-level Run and log evidence required by Supplier Runner promotion."
  permissions = [
    "logging.logEntries.list",
    "run.executions.get",
    "run.operations.get",
    "run.revisions.get",
    "run.services.get",
  ]
}

resource "google_project_iam_member" "promoter_observer" {
  project = var.project_id
  role    = google_project_iam_custom_role.promoter_observer.name
  member  = "serviceAccount:${google_service_account.promoter.email}"
}

resource "google_service_account_iam_member" "promoter_runner_act_as" {
  service_account_id = google_service_account.runner.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.promoter.email}"
}

resource "google_artifact_registry_repository_iam_member" "promoter_reader" {
  project    = var.project_id
  location   = var.region
  repository = var.artifact_registry_repository_id
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.promoter.email}"
}

resource "google_project_iam_member" "runner_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.runner.email}"
}

resource "google_secret_manager_secret" "runner_token" {
  for_each = local.token_secret_ids

  project   = var.project_id
  secret_id = each.value

  replication {
    auto {}
  }

  labels = {
    component = "supplier-accounting"
    slot      = each.key
  }
}

resource "google_secret_manager_secret_iam_member" "runner_token_access" {
  for_each = google_secret_manager_secret.runner_token

  project   = var.project_id
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runner.email}"
}

resource "google_cloud_run_v2_job" "runner" {
  count = var.enabled ? 1 : 0

  project  = var.project_id
  name     = var.job_name
  location = var.region

  deletion_protection = true

  template {
    parallelism = 1
    task_count  = 1

    template {
      service_account = google_service_account.runner.email
      max_retries     = 0
      timeout         = "${var.job_timeout_seconds}s"

      containers {
        image   = var.runner_image
        command = ["/app/supplier_batch_runner"]

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        env {
          name  = "SUPPLIER_BATCH_CONSOLE_URL"
          value = trimsuffix(var.console_url, "/")
        }

        env {
          name  = "SUPPLIER_BATCH_POLL_INTERVAL"
          value = var.poll_interval
        }

        env {
          name = "SUPPLIER_BATCH_TOKEN"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.runner_token[var.active_token_slot].secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }

  lifecycle {
    # CI/CD deploys a verified digest after build. Terraform owns the Job shape,
    # identity, timeout, and token slot but must not roll a deployed digest back.
    ignore_changes = [template[0].template[0].containers[0].image]

    precondition {
      condition     = var.runner_image != null && can(regex("^[^[:space:]@]+@sha256:[0-9a-fA-F]{64}$", var.runner_image))
      error_message = "runner_image must be an immutable image@sha256 digest when the supplier batch Job is enabled."
    }
  }

  depends_on = [
    google_project_iam_member.runner_log_writer,
    google_secret_manager_secret_iam_member.runner_token_access,
  ]
}

resource "google_cloud_run_v2_job_iam_member" "scheduler_invoker" {
  count = var.enabled ? 1 : 0

  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.runner[0].name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.scheduler.email}"
}

resource "google_cloud_run_v2_job_iam_member" "promoter_job" {
  count = var.enabled ? 1 : 0

  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.runner[0].name
  role     = google_project_iam_custom_role.promoter_job.name
  member   = "serviceAccount:${google_service_account.promoter.email}"
}

resource "google_cloud_scheduler_job" "daily" {
  count = var.enabled ? 1 : 0

  project   = var.project_id
  region    = var.region
  name      = var.schedule_name
  schedule  = var.schedule
  time_zone = var.time_zone
  paused    = true

  description      = "Start the supplier accounting one-shot Cloud Run Job after the Shanghai close grace."
  attempt_deadline = "30s"

  retry_config {
    retry_count = 0
  }

  http_target {
    http_method = "POST"
    uri         = "https://run.googleapis.com/v2/projects/${var.project_id}/locations/${var.region}/jobs/${google_cloud_run_v2_job.runner[0].name}:run"

    oauth_token {
      service_account_email = google_service_account.scheduler.email
      scope                 = "https://www.googleapis.com/auth/cloud-platform"
    }
  }

  lifecycle {
    # Every create/re-create starts fail closed. Promotion deliberately leaves
    # it paused; a separately authorized operator verifies and resumes it.
    ignore_changes = [paused]
  }

  depends_on = [google_cloud_run_v2_job_iam_member.scheduler_invoker]
}
