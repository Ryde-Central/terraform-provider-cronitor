package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

// Acceptance tests create, update, import and destroy real objects in the
// Cronitor account behind CRONITOR_API_KEY. They only run with TF_ACC=1
// (`make testacc`). Every key and name starts with "tfacc-" so leftovers from
// an interrupted run are easy to find. Monitors are paused so they never alert.

func testAccPreCheck(t *testing.T) {
	if os.Getenv("CRONITOR_API_KEY") == "" {
		t.Fatal("CRONITOR_API_KEY must be set for acceptance tests")
	}
}

const testAccProviderConfig = `provider "cronitor" {}
`

// testAccCheckDestroyed verifies that every resource of the managed types has
// been deleted from the API.
func testAccCheckDestroyed(s *terraform.State) error {
	c, err := client.New(os.Getenv("CRONITOR_API_URL"), os.Getenv("CRONITOR_API_KEY"))
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, rs := range s.RootModule().Resources {
		key := rs.Primary.ID
		switch rs.Type {
		case "cronitor_monitor":
			_, err = c.GetMonitor(ctx, key)
		case "cronitor_notification_list":
			_, err = c.GetNotificationList(ctx, key)
		case "cronitor_group":
			_, err = c.GetGroup(ctx, key)
		case "cronitor_environment":
			_, err = c.GetEnvironment(ctx, key)
		case "cronitor_status_page":
			_, err = c.GetStatusPage(ctx, key)
		case "cronitor_status_page_component":
			_, err = c.GetStatusPageComponent(ctx, key)
		case "cronitor_maintenance_window":
			_, err = c.GetMaintenanceWindow(ctx, key)
		default:
			continue
		}
		if err == nil {
			return fmt.Errorf("%s %q still exists", rs.Type, key)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking %s %q: %w", rs.Type, key, err)
		}
	}
	return nil
}

func testAccName() string {
	return "tfacc-" + strings.ToLower(acctest.RandString(8))
}

func TestAccMonitorJob(t *testing.T) {
	key := testAccName()
	res := "cronitor_monitor.job"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "job" {
  key       = %[1]q
  type      = "job"
  schedules = ["0 0 * * *"]
  paused    = true
}`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "id", key),
					resource.TestCheckResourceAttrSet(res, "name"),
					resource.TestCheckResourceAttrSet(res, "timezone"),
					resource.TestCheckResourceAttrSet(res, "grace_seconds"),
					resource.TestCheckResourceAttrSet(res, "created"),
					resource.TestCheckResourceAttr(res, "paused", "true"),
				),
			},
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "job" {
  key               = %[1]q
  type              = "job"
  name              = "%[1]s nightly"
  schedules         = ["0 0 * * *", "0 12 * * 6"]
  assertions        = ["metric.duration < 30min"]
  note              = "Created by the provider acceptance tests"
  tags              = ["tfacc", "backup"]
  timezone          = "Australia/Sydney"
  grace_seconds     = 300
  failure_tolerance = 1
  realert_interval  = "4h"
  paused            = true
  notify_events = {
    complete = true
  }
}`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "name", key+" nightly"),
					resource.TestCheckResourceAttr(res, "schedules.#", "2"),
					resource.TestCheckResourceAttr(res, "tags.#", "2"),
					resource.TestCheckResourceAttr(res, "grace_seconds", "300"),
					resource.TestCheckResourceAttr(res, "notify_events.complete", "true"),
				),
			},
			{
				ResourceName:            res,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"realert_interval"},
			},
			{
				// Removing optional attributes clears them.
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "job" {
  key       = %[1]q
  type      = "job"
  name      = "%[1]s nightly"
  schedules = ["0 0 * * *"]
  paused    = true
}`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "note"),
					resource.TestCheckNoResourceAttr(res, "tags"),
					resource.TestCheckNoResourceAttr(res, "assertions"),
				),
			},
		},
	})
}

func TestAccMonitorCheck(t *testing.T) {
	key := testAccName()
	res := "cronitor_monitor.check"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "check" {
  key        = %[1]q
  type       = "check"
  schedules  = ["every 5 min"]
  assertions = ["response.code = 200"]
  paused     = true
  request = {
    url = "https://example.com"
  }
}`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(res, "request.method"),
					resource.TestCheckResourceAttrSet(res, "request.timeout_seconds"),
				),
			},
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "check" {
  key        = %[1]q
  type       = "check"
  schedules  = ["every 5 min"]
  assertions = ["response.code = 200", "response.time < 5s"]
  paused     = true
  request = {
    url             = "https://example.com/health"
    method          = "POST"
    body            = "{}"
    headers         = { X-Test = "tfacc" }
    timeout_seconds = 5
  }
}`, key),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "request.method", "POST"),
					resource.TestCheckResourceAttr(res, "request.headers.X-Test", "tfacc"),
				),
			},
			{
				ResourceName:      res,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccNotificationListGroupEnvironment(t *testing.T) {
	name := testAccName()

	config := func(step int) string {
		envAlerts, listName, channels, members := "false", name+" list", `
    webhooks = ["https://example.com/tfacc-hook"]`, ""
		if step == 2 {
			envAlerts, listName, channels, members = "true", name+" renamed", `
    webhooks = ["https://example.com/tfacc-hook", "https://example.com/tfacc-hook-2"]`, `
  monitors = [cronitor_monitor.hb.key]`
		}
		return testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_environment" "env" {
  key         = %[1]q
  name        = "%[1]s env"
  with_alerts = %[2]s
}

resource "cronitor_notification_list" "list" {
  key  = %[1]q
  name = %[3]q
  notifications = {%[4]s
  }
}

resource "cronitor_group" "group" {
  key  = %[1]q
  name = "%[1]s group"%[5]s
}

resource "cronitor_monitor" "hb" {
  key    = %[1]q
  type   = "heartbeat"
  notify = [cronitor_notification_list.list.key]
  paused = true
}

data "cronitor_notification_list" "list" {
  key = cronitor_notification_list.list.key
}

data "cronitor_environment" "env" {
  key = cronitor_environment.env.key
}`, name, envAlerts, listName, channels, members)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckDestroyed,
		Steps: []resource.TestStep{
			{
				Config: config(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_notification_list.list", "key", name),
					resource.TestCheckResourceAttr("cronitor_notification_list.list", "notifications.webhooks.#", "1"),
					resource.TestCheckResourceAttr("cronitor_environment.env", "with_alerts", "false"),
					resource.TestCheckResourceAttr("data.cronitor_notification_list.list", "name", name+" list"),
					resource.TestCheckResourceAttr("data.cronitor_environment.env", "name", name+" env"),
					resource.TestCheckTypeSetElemAttr("cronitor_monitor.hb", "notify.*", name),
				),
			},
			{
				Config: config(2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_notification_list.list", "name", name+" renamed"),
					resource.TestCheckResourceAttr("cronitor_notification_list.list", "notifications.webhooks.#", "2"),
					resource.TestCheckResourceAttr("cronitor_environment.env", "with_alerts", "true"),
					resource.TestCheckResourceAttr("cronitor_group.group", "monitors.#", "1"),
				),
			},
			{
				ResourceName:      "cronitor_environment.env",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "cronitor_group.group",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Webhook URLs come back as identifiers, so they can't be
				// verified after a fresh import.
				ResourceName:            "cronitor_notification_list.list",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"notifications.webhooks"},
			},
		},
	})
}

func TestAccStatusPageAndMaintenanceWindow(t *testing.T) {
	name := testAccName()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testProviderFactories,
		CheckDestroy:             testAccCheckDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "api" {
  key    = %[1]q
  type   = "heartbeat"
  paused = true
}

resource "cronitor_status_page" "page" {
  name             = "%[1]s status"
  hosted_subdomain = %[1]q
  website_url      = "https://example.com"
}

resource "cronitor_status_page_component" "api" {
  statuspage = cronitor_status_page.page.key
  monitor    = cronitor_monitor.api.key
  name       = "API"
}

resource "cronitor_maintenance_window" "mw" {
  name        = "%[1]s upgrade"
  start       = "2035-01-01T02:00:00Z"
  end         = "2035-01-01T04:00:00Z"
  monitors    = [cronitor_monitor.api.key]
  statuspages = [cronitor_status_page.page.key]
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("cronitor_status_page.page", "key"),
					resource.TestCheckResourceAttr("cronitor_status_page_component.api", "name", "API"),
					resource.TestCheckResourceAttr("cronitor_maintenance_window.mw", "duration", "7200"),
				),
			},
			{
				Config: testAccProviderConfig + fmt.Sprintf(`
resource "cronitor_monitor" "api" {
  key    = %[1]q
  type   = "heartbeat"
  paused = true
}

resource "cronitor_status_page" "page" {
  name             = "%[1]s status page"
  hosted_subdomain = %[1]q
  support_url      = "https://example.com/support"
}

resource "cronitor_status_page_component" "api" {
  statuspage  = cronitor_status_page.page.key
  monitor     = cronitor_monitor.api.key
  name        = "Public API"
  description = "The REST API"
  autopublish = false
}

resource "cronitor_maintenance_window" "mw" {
  name                 = "%[1]s upgrade"
  description          = "Planned upgrade"
  start                = "2035-01-01T02:00:00Z"
  end                  = "2035-01-01T06:00:00Z"
  affects_all_monitors = true
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cronitor_status_page.page", "name", name+" status page"),
					resource.TestCheckNoResourceAttr("cronitor_status_page.page", "website_url"),
					resource.TestCheckResourceAttr("cronitor_status_page_component.api", "autopublish", "false"),
					resource.TestCheckResourceAttr("cronitor_maintenance_window.mw", "duration", "14400"),
				),
			},
			{
				ResourceName:      "cronitor_status_page.page",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "cronitor_status_page_component.api",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "cronitor_maintenance_window.mw",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
