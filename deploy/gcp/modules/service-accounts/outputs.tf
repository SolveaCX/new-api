output "runtime_email" {
  value = google_service_account.runtime.email
}

output "runtime_id" {
  value = google_service_account.runtime.id
}

output "runtime_name" {
  value = google_service_account.runtime.name
}

output "deployer_email" {
  value = google_service_account.deployer.email
}

output "deployer_id" {
  value = google_service_account.deployer.id
}

output "deployer_name" {
  value = google_service_account.deployer.name
}

output "production_console_deployer_email" {
  value = google_service_account.production_console_deployer.email
}

output "production_console_deployer_name" {
  value = google_service_account.production_console_deployer.name
}

output "production_router_deployer_email" {
  value = google_service_account.production_router_deployer.email
}

output "production_router_deployer_name" {
  value = google_service_account.production_router_deployer.name
}

output "production_console_rollback_email" {
  value = google_service_account.production_console_rollback.email
}

output "production_console_rollback_name" {
  value = google_service_account.production_console_rollback.name
}

output "production_router_rollback_email" {
  value = google_service_account.production_router_rollback.email
}

output "production_router_rollback_name" {
  value = google_service_account.production_router_rollback.name
}

output "production_website_deployer_email" {
  value = google_service_account.production_website_deployer.email
}

output "production_website_deployer_name" {
  value = google_service_account.production_website_deployer.name
}

output "staging_deployer_email" {
  value = google_service_account.staging_deployer.email
}

output "staging_deployer_name" {
  value = google_service_account.staging_deployer.name
}

output "service_deployer_observer_custom_role_name" {
  value = google_project_iam_custom_role.service_deployer_observer.name
}

output "service_deployer_mutator_custom_role_name" {
  value = google_project_iam_custom_role.service_deployer_mutator.name
}
