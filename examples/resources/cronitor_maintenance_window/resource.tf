resource "cronitor_maintenance_window" "db_upgrade" {
  name        = "Database upgrade"
  description = "Upgrading to PostgreSQL 18"
  start       = "2026-11-01T02:00:00Z"
  end         = "2026-11-01T04:00:00Z"

  monitors                      = [cronitor_monitor.nightly_backup.key]
  groups                        = [cronitor_group.production.key]
  statuspages                   = [cronitor_status_page.main.key]
  affects_statuspage_components = true
}
