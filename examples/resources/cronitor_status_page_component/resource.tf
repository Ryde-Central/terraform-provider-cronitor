resource "cronitor_status_page_component" "homepage" {
  statuspage  = cronitor_status_page.main.key
  monitor     = cronitor_monitor.homepage.key
  name        = "Website"
  description = "example.com"
  autopublish = true
}

resource "cronitor_status_page_component" "batch" {
  statuspage = cronitor_status_page.main.key
  group      = cronitor_group.batch.key
}
