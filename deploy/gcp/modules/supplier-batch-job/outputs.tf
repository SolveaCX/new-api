output "job_name" {
  value = var.enabled ? google_cloud_run_v2_job.runner[0].name : null
}

output "schedule_name" {
  value = var.enabled ? google_cloud_scheduler_job.daily[0].name : null
}

output "runner_service_account_email" {
  value = google_service_account.runner.email
}

output "scheduler_service_account_email" {
  value = google_service_account.scheduler.email
}

output "promoter_service_account_email" {
  value = google_service_account.promoter.email
}

output "promoter_service_account_name" {
  value = google_service_account.promoter.name
}

output "promoter_custom_role_names" {
  value = {
    job      = google_project_iam_custom_role.promoter_job.name
    observer = google_project_iam_custom_role.promoter_observer.name
  }
}

output "token_secret_ids" {
  value = {
    for slot, secret in google_secret_manager_secret.runner_token : slot => secret.secret_id
  }
}
