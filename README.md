# Terraform / OpenTofu provider for Cronitor

Manages [Cronitor](https://cronitor.io) resources through the
[Cronitor REST API](https://cronitor.io/docs/api) (API version `2025-11-28`).

| Resource | API |
| --- | --- |
| `cronitor_monitor` | Jobs, heartbeats, checks (HTTP/TCP/UDP) and RUM site monitors |
| `cronitor_notification_list` | Notification lists (alert destinations) |
| `cronitor_group` | Monitor groups |
| `cronitor_environment` | Environments |
| `cronitor_status_page` | Hosted status pages |
| `cronitor_status_page_component` | Monitors/groups shown on a status page |
| `cronitor_maintenance_window` | Maintenance windows |

Data sources: `cronitor_monitor`, `cronitor_notification_list`, `cronitor_group`,
`cronitor_environment`, `cronitor_status_page`.

All resources can be imported by key, e.g. `tofu import cronitor_monitor.backup nightly-backup`.

## Usage

```hcl
terraform {
  required_providers {
    cronitor = {
      source = "ryde/cronitor"
    }
  }
}

provider "cronitor" {} # reads CRONITOR_API_KEY

resource "cronitor_notification_list" "devops" {
  key  = "devops"
  name = "DevOps"
  notifications = {
    emails = ["oncall@example.com"]
  }
}

resource "cronitor_monitor" "nightly_backup" {
  key        = "nightly-backup"
  type       = "job"
  schedules  = ["0 2 * * *"]
  assertions = ["metric.duration < 30 min"]
  notify     = [cronitor_notification_list.devops.key]
}
```

More examples are in [examples/](examples/).

### Provider configuration

| Argument | Environment variable | Description |
| --- | --- | --- |
| `api_key` | `CRONITOR_API_KEY` | API key. Use an SDK Integration key, or a custom key with the read/write scopes for the resources you manage (`monitor:*` for monitors, groups, environments and notification lists; `statuspage:*`; `issue:*` for maintenance windows). |
| `api_url` | `CRONITOR_API_URL` | Defaults to `https://cronitor.io/api`. |

### Behaviour notes

* Attributes the API defaults (e.g. a monitor's `name`, `notify`, `timezone`,
  `grace_seconds`) are *optional + computed*: if you remove them from
  configuration the last value is kept rather than reset. Plain optional
  attributes (`note`, `group`, `tags`, `assertions`, ...) are cleared when removed.
* Manage group membership either with `cronitor_monitor.group` or with
  `cronitor_group.monitors`, not both. `cronitor_group.monitors` is ignored when unset.
* `notify_events` switches the monitor's `notify` to the hash form
  (`{"alerts": [...], "events": {...}}`).
* Monitors are updated with the bulk `PUT /api/monitors` endpoint, the only
  monitor update operation the API offers. Changing `key` or `type` replaces the monitor.
* Webhook destinations are returned by the API as identifiers rather than URLs;
  the configured values are kept as long as the number of webhooks matches.
* `cronitor_status_page.password` is write-only in the API, so out-of-band changes are not detected.

## Development

Requires Go 1.25+ and `tofu` or `terraform`.

```bash
make test      # runs tofu/terraform plan+apply cycles against an in-memory fake API
make install   # installs into ~/.terraform.d/plugins for local use
make docs      # regenerates docs/ with tfplugindocs
```

To use a local build without installing, add a `dev_overrides` block to
`~/.tofurc` (OpenTofu) or `~/.terraformrc` (Terraform):

```hcl
provider_installation {
  dev_overrides {
    "ryde/cronitor" = "/path/to/terraform-provider-cronitor" # directory containing the binary
  }
  direct {}
}
```

The tests in `internal/provider` use a fake of the API built from the public
documentation and OpenAPI spec (`https://cronitor.io/api/openapi.json`); they
do not touch a real Cronitor account.
