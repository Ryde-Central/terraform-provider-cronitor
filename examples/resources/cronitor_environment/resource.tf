resource "cronitor_environment" "staging" {
  key         = "staging"
  name        = "Staging"
  with_alerts = false
}
