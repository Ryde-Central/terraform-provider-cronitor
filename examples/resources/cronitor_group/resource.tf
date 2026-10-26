# Membership managed from the monitors (cronitor_monitor.group).
resource "cronitor_group" "production" {
  key  = "production"
  name = "Production"
}

# Membership managed from the group.
resource "cronitor_group" "batch" {
  key      = "batch-jobs"
  name     = "Batch jobs"
  monitors = ["nightly-backup", "weekly-report"]
}
