resource "cronitor_status_page" "main" {
  name                = "Example Status"
  hosted_subdomain    = "example"
  website_url         = "https://example.com"
  support_url         = "https://example.com/support"
  default_time_period = "30d"
}
