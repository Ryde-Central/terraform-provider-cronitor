# A cron job. Ping https://cronitor.link/p/<api key>/nightly-backup from the job.
resource "cronitor_monitor" "nightly_backup" {
  key        = "nightly-backup"
  type       = "job"
  name       = "Nightly database backup"
  schedules  = ["0 2 * * *"]
  timezone   = "Australia/Sydney"
  assertions = ["metric.duration < 30 min"]
  notify     = [cronitor_notification_list.devops.key]
  group      = cronitor_group.production.key
  note       = "Backs up the production database to S3."
  tags       = ["database"]

  grace_seconds     = 300
  failure_tolerance = 1

  # Also notify when the job completes, not only when it fails.
  notify_events = {
    complete = true
  }
}

# An uptime check.
resource "cronitor_monitor" "homepage" {
  key        = "homepage"
  type       = "check"
  schedules  = ["every 1 minute"]
  assertions = ["response.code = 200", "response.time < 2s", "ssl_certificate.expires_in > 14 days"]

  request = {
    url             = "https://example.com"
    method          = "GET"
    timeout_seconds = 10
    headers = {
      "User-Agent" = "Cronitor"
    }
  }
}

# A heartbeat, expected at least every 5 minutes.
resource "cronitor_monitor" "worker" {
  key       = "queue-worker"
  type      = "heartbeat"
  schedules = ["every 5 minutes"]
}
