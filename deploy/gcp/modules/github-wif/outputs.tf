output "pool_id" {
  value = google_iam_workload_identity_pool.github.workload_identity_pool_id
}

output "provider_resource_name" {
  description = "Build-only WIF provider retained as the GCP_WIF_PROVIDER compatibility output."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "privileged_provider_resource_names" {
  description = "Workflow-pinned WIF providers for privileged deploy and promotion lanes."
  value = {
    for lane, provider in google_iam_workload_identity_pool_provider.privileged :
    lane => provider.name
  }
}

output "supplier_runner_promoter_subject" {
  value = local.production_subject
}
