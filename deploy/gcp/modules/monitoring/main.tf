locals {
  alert_emails   = distinct(compact(concat(var.alert_email == "" ? [] : [var.alert_email], var.alert_emails)))
  alerts_enabled = length(local.alert_emails) > 0
}

// Uptime check against the public Cloud Run URL — independent of Cloudflare/DNS.

resource "google_monitoring_uptime_check_config" "api_status" {
  project      = var.project_id
  display_name = "new-api /api/status"
  timeout      = "10s"
  period       = "60s"

  http_check {
    path           = "/api/status"
    port           = 443
    use_ssl        = true
    validate_ssl   = true
    request_method = "GET"
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      host       = var.uptime_host
      project_id = var.project_id
    }
  }
}

// Email alert channel — operator can register more channels manually.
resource "google_monitoring_notification_channel" "email" {
  for_each = toset(local.alert_emails)

  project      = var.project_id
  display_name = "new-api email alerts (${each.value})"
  type         = "email"
  labels = {
    email_address = each.value
  }
}

resource "google_monitoring_alert_policy" "uptime_failed" {
  count = local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api uptime failed"
  combiner     = "OR"

  conditions {
    display_name = "Uptime check failed"
    condition_threshold {
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.api_status.uptime_check_id}\""
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.host"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "router_instances_near_max" {
  count = local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api router instances near max"
  combiner     = "OR"

  conditions {
    display_name = "Router instance count is near maxScale"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/container/instance_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${var.router_service_name}\"",
        "resource.label.location=\"${var.region}\"",
      ])
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = floor(var.router_max_instances * var.router_instance_saturation_ratio)
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.label.service_name"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "router_pending_requests" {
  count = local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api router pending requests"
  combiner     = "OR"

  conditions {
    display_name = "Router requests are waiting in Cloud Run pending queue"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/pending_queue/pending_requests\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${var.router_service_name}\"",
        "resource.label.location=\"${var.region}\"",
      ])
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = var.router_pending_requests_threshold
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.label.service_name"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "router_5xx" {
  count = local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api router 5xx spike"
  combiner     = "OR"

  conditions {
    display_name = "Router 5xx responses exceed threshold"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${var.router_service_name}\"",
        "resource.label.location=\"${var.region}\"",
        "metric.label.response_code_class=\"5xx\"",
      ])
      duration        = "0s"
      comparison      = "COMPARISON_GT"
      threshold_value = var.router_5xx_per_5m_threshold
      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "redis_cpu_high" {
  count = local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api Redis CPU high"
  combiner     = "OR"
  severity     = "WARNING"

  conditions {
    display_name = "Redis CPU utilization is high"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"redis.googleapis.com/stats/cpu_utilization\"",
        "resource.type=\"redis_instance\"",
        "resource.label.instance_id=\"${var.redis_instance_id}\"",
        "resource.label.region=\"${var.region}\"",
        "metric.label.role=\"primary\"",
        "metric.label.relationship=\"parent\"",
      ])
      duration        = "300s"
      comparison      = "COMPARISON_GT"
      threshold_value = var.redis_cpu_threshold
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_RATE"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.instance_id"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "supplier_batch_never_published_days" {
  count = var.supplier_batch_monitoring_enabled && local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api supplier accounting never-published backlog is at least two days"
  combiner     = "OR"
  severity     = "WARNING"

  conditions {
    display_name = "Current DB-derived never-published day count is at least two"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"prometheus.googleapis.com/newapi_supplier_accounting_never_published_days/gauge\"",
        "resource.type=\"prometheus_target\"",
        "metric.label.service=\"${var.router_service_name}\"",
      ])
      duration        = "60s"
      comparison      = "COMPARISON_GT"
      threshold_value = 1
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["metric.label.service"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "supplier_batch_oldest_never_published_age" {
  count = var.supplier_batch_monitoring_enabled && local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api supplier accounting oldest never-published day exceeds 24 hours"
  combiner     = "OR"
  severity     = "WARNING"

  conditions {
    display_name = "Current DB-derived oldest never-published age is strictly above 24 hours"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"prometheus.googleapis.com/newapi_supplier_accounting_oldest_never_published_age_seconds/gauge\"",
        "resource.type=\"prometheus_target\"",
        "metric.label.service=\"${var.router_service_name}\"",
      ])
      duration        = "60s"
      comparison      = "COMPARISON_GT"
      threshold_value = 86400
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["metric.label.service"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "supplier_batch_prior_day_unpublished_after_0800" {
  count = var.supplier_batch_monitoring_enabled && local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api supplier accounting prior day unpublished after 08:00 Asia/Shanghai"
  combiner     = "OR"
  severity     = "WARNING"

  conditions {
    display_name = "Current DB-derived prior-day 08:00 publication SLO is missed"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"prometheus.googleapis.com/newapi_supplier_accounting_prior_day_unpublished_after_0800/gauge\"",
        "resource.type=\"prometheus_target\"",
        "metric.label.service=\"${var.router_service_name}\"",
      ])
      duration        = "60s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["metric.label.service"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}

resource "google_monitoring_alert_policy" "supplier_batch_backlog_observer_unhealthy" {
  count = var.supplier_batch_monitoring_enabled && local.alerts_enabled ? 1 : 0

  project      = var.project_id
  display_name = "new-api supplier accounting backlog observer is unhealthy"
  combiner     = "OR"
  severity     = "WARNING"

  conditions {
    display_name = "Every reporting Router observer is down"
    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"prometheus.googleapis.com/newapi_supplier_accounting_backlog_observer_up/gauge\"",
        "resource.type=\"prometheus_target\"",
        "metric.label.service=\"${var.router_service_name}\"",
      ])
      duration        = "120s"
      comparison      = "COMPARISON_LT"
      threshold_value = 1
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["metric.label.service"]
      }
      trigger {
        count = 1
      }
    }
  }

  conditions {
    display_name = "Router backlog observer metric is absent for 120 seconds"
    condition_absent {
      filter = join(" AND ", [
        "metric.type=\"prometheus.googleapis.com/newapi_supplier_accounting_backlog_observer_up/gauge\"",
        "resource.type=\"prometheus_target\"",
        "metric.label.service=\"${var.router_service_name}\"",
      ])
      duration = "120s"
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_MAX"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["metric.label.service"]
      }
      trigger {
        count = 1
      }
    }
  }

  notification_channels = values(google_monitoring_notification_channel.email)[*].id

  alert_strategy {
    auto_close = "3600s"
  }
}
