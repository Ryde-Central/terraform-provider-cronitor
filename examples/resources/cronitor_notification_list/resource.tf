resource "cronitor_notification_list" "devops" {
  key  = "devops"
  name = "DevOps"

  notifications = {
    emails = ["oncall@example.com"]
    phones = ["+15551234567"]
    # Integrations are referenced by the label shown in Cronitor, and must be connected first.
    slack = ["#alerts"]
  }

  environments = ["production"]
}
