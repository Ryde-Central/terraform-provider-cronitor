package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestMonitorJob(t *testing.T) {
	srv := newFakeAPI(t)
	cfg := providerConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg + `
resource "cronitor_monitor" "backup" {
  key       = "nightly-backup"
  type      = "job"
  schedules = ["0 0 * * *"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "id", "nightly-backup"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "name", "nightly-backup"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "grace_seconds", "60"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "platform", "linux cron"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "notify.#", "1"),
					resource.TestCheckTypeSetElemAttr("cronitor_monitor.backup", "notify.*", "default"),
					resource.TestCheckTypeSetElemAttr("cronitor_monitor.backup", "environments.*", "production"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "paused", "false"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "note"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "tags"),
					resource.TestCheckResourceAttrSet("cronitor_monitor.backup", "badge_url"),
				),
			},
			{
				Config: cfg + `
resource "cronitor_monitor" "backup" {
  key               = "nightly-backup"
  type              = "job"
  name              = "Nightly backup"
  schedules         = ["0 0 * * *", "0 12 * * 6"]
  assertions        = ["metric.duration < 30min"]
  notify            = ["devops"]
  note              = "Backs up the database"
  tags              = ["db", "backup"]
  group             = "production"
  grace_seconds     = 300
  failure_tolerance = 1
  realert_interval  = "4h"
  paused            = true
  notify_events = {
    complete = true
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "name", "Nightly backup"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "schedules.#", "2"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "realert_interval", "4h"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "notify_events.complete", "true"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "notify_events.run", "false"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "tags.#", "2"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "paused", "true"),
				),
			},
			{
				ResourceName:      "cronitor_monitor.backup",
				ImportState:       true,
				ImportStateVerify: true,
				// The API returns its canonical form of the interval.
				ImportStateVerifyIgnore: []string{"realert_interval"},
			},
			{
				// Removing optional attributes clears them.
				Config: cfg + `
resource "cronitor_monitor" "backup" {
  key       = "nightly-backup"
  type      = "job"
  name      = "Nightly backup"
  schedules = ["0 0 * * *"]
  notify    = ["devops"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "note"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "group"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "tags"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "assertions"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.backup", "notify_events"),
					resource.TestCheckResourceAttr("cronitor_monitor.backup", "paused", "false"),
				),
			},
		},
	})
}

func TestMonitorCheck(t *testing.T) {
	srv := newFakeAPI(t)
	cfg := providerConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg + `
resource "cronitor_monitor" "site" {
  key        = "homepage"
  type       = "check"
  schedules  = ["every 1 min"]
  assertions = ["response.code = 200", "response.time < 2s"]
  request = {
    url = "https://example.com"
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_monitor.site", "request.method", "GET"),
					resource.TestCheckResourceAttr("cronitor_monitor.site", "request.timeout_seconds", "10"),
					resource.TestCheckResourceAttr("cronitor_monitor.site", "request.verify_ssl", "true"),
					resource.TestCheckNoResourceAttr("cronitor_monitor.site", "request.headers"),
				),
			},
			{
				Config: cfg + `
resource "cronitor_monitor" "site" {
  key        = "homepage"
  type       = "check"
  schedules  = ["every 1 min"]
  assertions = ["response.code = 200"]
  request = {
    url             = "https://example.com/health"
    method          = "POST"
    body            = "{}"
    headers         = { Authorization = "Bearer x" }
    timeout_seconds = 5
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_monitor.site", "request.method", "POST"),
					resource.TestCheckResourceAttr("cronitor_monitor.site", "request.headers.Authorization", "Bearer x"),
				),
			},
			{
				ResourceName:      "cronitor_monitor.site",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestMonitorValidation(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig("http://127.0.0.1:1") + `
resource "cronitor_monitor" "bad" {
  key  = "bad"
  type = "check"
}`,
				ExpectError: regexp.MustCompile(`require a request`),
			},
			{
				Config: providerConfig("http://127.0.0.1:1") + `
resource "cronitor_monitor" "bad" {
  key  = "bad"
  type = "cron"
}`,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestNotificationListGroupEnvironment(t *testing.T) {
	srv := newFakeAPI(t)
	cfg := providerConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg + `
resource "cronitor_environment" "staging" {
  key  = "staging"
  name = "Staging"
}

resource "cronitor_notification_list" "devops" {
  name = "DevOps Team"
  notifications = {
    emails   = ["oncall@example.com"]
    webhooks = ["https://example.com/hook"]
  }
  environments = [cronitor_environment.staging.key]
}

resource "cronitor_group" "prod" {
  key  = "production"
  name = "Production"
}

resource "cronitor_monitor" "hb" {
  key    = "heartbeat"
  type   = "heartbeat"
  group  = cronitor_group.prod.key
  notify = [cronitor_notification_list.devops.key]
}

data "cronitor_notification_list" "devops" {
  key = cronitor_notification_list.devops.key
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_notification_list.devops", "key", "devops-team"),
					resource.TestCheckResourceAttr("cronitor_notification_list.devops", "notifications.webhooks.#", "1"),
					resource.TestCheckTypeSetElemAttr("cronitor_notification_list.devops", "notifications.webhooks.*", "https://example.com/hook"),
					resource.TestCheckNoResourceAttr("cronitor_notification_list.devops", "notifications.slack"),
					resource.TestCheckResourceAttr("cronitor_environment.staging", "with_alerts", "false"),
					resource.TestCheckResourceAttr("cronitor_environment.staging", "is_default", "false"),
					resource.TestCheckNoResourceAttr("cronitor_group.prod", "monitors"),
					resource.TestCheckResourceAttr("data.cronitor_notification_list.devops", "name", "DevOps Team"),
					resource.TestCheckTypeSetElemAttr("data.cronitor_notification_list.devops", "emails.*", "oncall@example.com"),
				),
			},
			{
				Config: cfg + `
resource "cronitor_environment" "staging" {
  key         = "staging"
  name        = "Staging"
  with_alerts = true
}

resource "cronitor_notification_list" "devops" {
  name = "DevOps"
  notifications = {
    emails = ["oncall@example.com", "devops@example.com"]
    slack  = ["#alerts"]
  }
}

resource "cronitor_group" "prod" {
  key      = "production"
  name     = "Production"
  monitors = ["a", "b"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_notification_list.devops", "key", "devops-team"),
					resource.TestCheckResourceAttr("cronitor_notification_list.devops", "notifications.emails.#", "2"),
					resource.TestCheckNoResourceAttr("cronitor_notification_list.devops", "notifications.webhooks"),
					resource.TestCheckNoResourceAttr("cronitor_notification_list.devops", "environments"),
					resource.TestCheckResourceAttr("cronitor_environment.staging", "with_alerts", "true"),
					resource.TestCheckResourceAttr("cronitor_group.prod", "monitors.#", "2"),
				),
			},
			{
				ResourceName:      "cronitor_environment.staging",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "cronitor_notification_list.devops",
				ImportState:       true,
				ImportStateId:     "devops-team",
				ImportStateVerify: true,
			},
		},
	})
}

func TestStatusPageAndMaintenanceWindow(t *testing.T) {
	srv := newFakeAPI(t)
	cfg := providerConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg + `
resource "cronitor_monitor" "api" {
  key  = "api"
  type = "heartbeat"
}

resource "cronitor_status_page" "main" {
  name             = "Example Status"
  hosted_subdomain = "example"
  website_url      = "https://example.com"
}

resource "cronitor_status_page_component" "api" {
  statuspage = cronitor_status_page.main.key
  monitor    = cronitor_monitor.api.key
  name       = "API"
}

resource "cronitor_maintenance_window" "upgrade" {
  name        = "Database upgrade"
  start       = "2030-01-01T02:00:00Z"
  end         = "2030-01-01T04:00:00Z"
  monitors    = [cronitor_monitor.api.key]
  statuspages = [cronitor_status_page.main.key]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_status_page.main", "key", "sp_1"),
					resource.TestCheckResourceAttr("cronitor_status_page.main", "access", "public"),
					resource.TestCheckNoResourceAttr("cronitor_status_page.main", "support_url"),
					resource.TestCheckResourceAttr("cronitor_status_page_component.api", "autopublish", "true"),
					resource.TestCheckResourceAttr("cronitor_maintenance_window.upgrade", "start", "2030-01-01T02:00:00Z"),
					resource.TestCheckResourceAttr("cronitor_maintenance_window.upgrade", "duration", "7200"),
					resource.TestCheckResourceAttr("cronitor_maintenance_window.upgrade", "state", "upcoming"),
				),
			},
			{
				Config: cfg + `
resource "cronitor_monitor" "api" {
  key  = "api"
  type = "heartbeat"
}

resource "cronitor_status_page" "main" {
  name             = "Example Status"
  hosted_subdomain = "example"
  access           = "password"
  password         = "hunter2"
}

resource "cronitor_status_page_component" "api" {
  statuspage  = cronitor_status_page.main.key
  monitor     = cronitor_monitor.api.key
  name        = "Public API"
  description = "The REST API"
  autopublish = false
}

resource "cronitor_maintenance_window" "upgrade" {
  name                 = "Database upgrade"
  start                = "2030-01-01T02:00:00Z"
  end                  = "2030-01-01T06:00:00Z"
  affects_all_monitors = true
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_status_page.main", "access", "password"),
					resource.TestCheckNoResourceAttr("cronitor_status_page.main", "website_url"),
					resource.TestCheckResourceAttr("cronitor_status_page_component.api", "autopublish", "false"),
					resource.TestCheckResourceAttr("cronitor_maintenance_window.upgrade", "duration", "14400"),
					resource.TestCheckNoResourceAttr("cronitor_maintenance_window.upgrade", "monitors"),
				),
			},
			{
				ResourceName:            "cronitor_status_page.main",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
			{
				ResourceName:      "cronitor_status_page_component.api",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestStatusPageComponentRequiresTarget(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig("http://127.0.0.1:1") + `
resource "cronitor_status_page_component" "bad" {
  statuspage = "sp_1"
  monitor    = "a"
  group      = "b"
}`,
			ExpectError: regexp.MustCompile(`(?s)These attributes cannot be configured together|Invalid Attribute Combination`),
		}},
	})
}
