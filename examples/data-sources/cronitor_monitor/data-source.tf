data "cronitor_monitor" "backup" {
  key = "nightly-backup"
}

output "backup_passing" {
  value = data.cronitor_monitor.backup.passing
}
