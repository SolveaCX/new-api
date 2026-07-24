output "cloud_run_uri" {
  description = "Direct Cloud Run *.run.app URL (used by GHA smoke test)"
  value       = var.enable_legacy_runtime ? module.cloud_run.service_uri : null
}

output "artifact_registry_url" {
  description = "Push docker images to this URL"
  value       = module.artifact_registry.repository_url
}

output "cloudsql_connection_name" {
  value = module.cloud_sql.connection_name
}

output "domain_mappings" {
  description = "Cloudflare DNS targets — create one CNAME per host, pointing to ghs.googlehosted.com"
  value       = var.enable_legacy_runtime ? module.cloud_run.domain_mappings : {}
}

output "github_wif_provider" {
  description = "Build-only WIF provider retained for GCP_WIF_PROVIDER compatibility"
  value       = module.github_wif.provider_resource_name
}

output "github_builder_sa_email" {
  description = "Artifact Registry-only image builder; this identity has no Cloud Run or actAs permission"
  value       = module.service_accounts.deployer_email
}

output "github_privileged_wif_providers" {
  description = "Workflow-pinned WIF providers for deploy, rollback, staging, and Supplier promotion lanes"
  value       = module.github_wif.privileged_provider_resource_names
}

output "github_service_deployer_emails" {
  description = "Target-specific Cloud Run service deploy identities"
  value = {
    production_console = module.service_accounts.production_console_deployer_email
    production_router  = module.service_accounts.production_router_deployer_email
    rollback_console   = module.service_accounts.production_console_rollback_email
    rollback_router    = module.service_accounts.production_router_rollback_email
    production_website = module.service_accounts.production_website_deployer_email
    staging            = module.service_accounts.staging_deployer_email
  }
}

output "github_service_deployer_custom_roles" {
  description = "Read-only observer and exact-resource service mutation roles; neither contains run.jobs permissions"
  value = {
    observer = module.service_accounts.service_deployer_observer_custom_role_name
    mutator  = module.service_accounts.service_deployer_mutator_custom_role_name
  }
}

output "supplier_runner_promoter_sa_email" {
  description = "Fixed production-only Supplier Runner promotion identity; the workflow does not accept an SA input or repository variable"
  value       = module.supplier_batch_job.promoter_service_account_email
}

output "supplier_runner_promoter_wif_subject" {
  description = "Exact GitHub OIDC subject allowed to impersonate the Supplier Runner promoter"
  value       = module.github_wif.supplier_runner_promoter_subject
}

output "supplier_runner_promoter_custom_roles" {
  description = "Fixed-Job mutation and project read-only custom roles used by Supplier Runner promotion; neither can mutate Scheduler"
  value       = module.supplier_batch_job.promoter_custom_role_names
}

output "runtime_sa_email" {
  value = module.service_accounts.runtime_email
}

output "supplier_batch_job_name" {
  value = module.supplier_batch_job.job_name
}

output "supplier_batch_runner_sa_email" {
  value = module.supplier_batch_job.runner_service_account_email
}

output "supplier_batch_scheduler_sa_email" {
  value = module.supplier_batch_job.scheduler_service_account_email
}

output "supplier_batch_token_secret_ids" {
  value = module.supplier_batch_job.token_secret_ids
}

output "lb_ip" {
  description = "Static IPv4 to A-record in Cloudflare for every domain in lb_domains. Null if LB disabled."
  value       = var.enable_load_balancer ? module.cloud_lb[0].ip_address : null
}

output "lb_ssl_certificate_status_hint" {
  value = var.enable_load_balancer ? module.cloud_lb[0].ssl_certificate_status_hint : null
}

output "placeholder_secrets_to_fill" {
  description = "These secrets were created empty — set their values via 'gcloud secrets versions add' before the features that need them are used"
  value = [
    "newapi-github-client-id",
    "newapi-github-client-secret",
    "newapi-stripe-secret-key",
  ]
}
