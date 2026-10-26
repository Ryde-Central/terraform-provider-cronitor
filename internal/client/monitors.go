package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Monitor is a Cronitor monitor as returned by the API.
type Monitor struct {
	Key                       string          `json:"key"`
	Type                      string          `json:"type"`
	Name                      string          `json:"name"`
	Schedules                 []string        `json:"schedules"`
	Assertions                []string        `json:"assertions"`
	Notify                    Notify          `json:"notify"`
	Group                     FlexKey         `json:"group"`
	Note                      *string         `json:"note"`
	Platform                  *string         `json:"platform"`
	Timezone                  *string         `json:"timezone"`
	Paused                    bool            `json:"paused"`
	GraceSeconds              *FlexInt        `json:"grace_seconds"`
	FailureTolerance          *FlexInt        `json:"failure_tolerance"`
	ScheduleTolerance         *FlexInt        `json:"schedule_tolerance"`
	ConsecutiveAlertThreshold *FlexInt        `json:"consecutive_alert_threshold"`
	RealertInterval           *FlexString     `json:"realert_interval"`
	Tags                      []string        `json:"tags"`
	Environments              KeyList         `json:"environments"`
	Statuspages               KeyList         `json:"statuspages"`
	Request                   *MonitorRequest `json:"request"`
	Site                      FlexKey         `json:"site"`
	Metadata                  *string         `json:"metadata"`

	// Read-only.
	Created        string   `json:"created"`
	Disabled       bool     `json:"disabled"`
	Initialized    *bool    `json:"initialized"`
	Passing        *bool    `json:"passing"`
	Running        *bool    `json:"running"`
	Badges         *Badges  `json:"badges"`
	NextExpectedAt *float64 `json:"next_expected_at"`
}

// MonitorRequest is the outbound request made by a check monitor.
type MonitorRequest struct {
	URL             string            `json:"url"`
	Method          string            `json:"method,omitempty"`
	Body            *string           `json:"body"`
	Headers         map[string]string `json:"headers,omitempty"`
	Cookies         map[string]string `json:"cookies,omitempty"`
	TimeoutSeconds  *FlexInt          `json:"timeout_seconds,omitempty"`
	Regions         []string          `json:"regions,omitempty"`
	FollowRedirects *bool             `json:"follow_redirects,omitempty"`
	VerifySSL       *bool             `json:"verify_ssl,omitempty"`
	UseIPv6         *bool             `json:"use_ipv6,omitempty"`
}

// Badges holds a monitor's embeddable status badge URLs.
type Badges struct {
	Simple   string `json:"simple"`
	Detailed string `json:"detailed"`
}

// Notify is where a monitor's alerts go. The API accepts and returns either a
// plain array of notification list keys or a hash of {"alerts": [...],
// "events": {...}}.
type Notify struct {
	Alerts []string
	Events *NotifyEvents
}

// NotifyEvents turns on notifications for telemetry lifecycle events.
type NotifyEvents struct {
	Run      bool `json:"run"`
	Complete bool `json:"complete"`
}

func (n *Notify) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*n = Notify{}
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '[' {
		return json.Unmarshal(b, &n.Alerts)
	}
	var obj struct {
		Alerts []string        `json:"alerts"`
		Events json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	n.Alerts = obj.Alerts
	events := bytes.TrimSpace(obj.Events)
	switch {
	case len(events) == 0, bytes.Equal(events, []byte("null")), bytes.Equal(events, []byte("false")):
	case bytes.Equal(events, []byte("true")):
		n.Events = &NotifyEvents{Run: true, Complete: true}
	default:
		var ev NotifyEvents
		if err := json.Unmarshal(events, &ev); err != nil {
			return fmt.Errorf("notify.events: %w", err)
		}
		if ev.Run || ev.Complete {
			n.Events = &ev
		}
	}
	return nil
}

func (n Notify) MarshalJSON() ([]byte, error) {
	alerts := n.Alerts
	if alerts == nil {
		alerts = []string{}
	}
	if n.Events == nil {
		return json.Marshal(alerts)
	}
	return json.Marshal(struct {
		Alerts []string     `json:"alerts"`
		Events NotifyEvents `json:"events"`
	}{alerts, *n.Events})
}

// MonitorInput is the body of a create or update. It is a map rather than a
// struct so that callers control exactly which attributes are sent, including
// explicit nulls used to clear a previously-set value.
type MonitorInput map[string]any

// GetMonitor fetches a monitor by key.
func (c *Client) GetMonitor(ctx context.Context, key string) (*Monitor, error) {
	var m Monitor
	q := url.Values{"withStatus": {"false"}}
	if err := c.do(ctx, http.MethodGet, "/monitors/"+escape(key), q, nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// GetMonitorWithStatus fetches a monitor by key, including its current status
// (passing, latest_event, ...).
func (c *Client) GetMonitorWithStatus(ctx context.Context, key string) (*Monitor, error) {
	var m Monitor
	if err := c.do(ctx, http.MethodGet, "/monitors/"+escape(key), nil, nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateMonitor creates a single monitor. It fails if the key already exists.
func (c *Client) CreateMonitor(ctx context.Context, in MonitorInput) (*Monitor, error) {
	var m Monitor
	if err := c.do(ctx, http.MethodPost, "/monitors", nil, in, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMonitor creates or updates a monitor via the bulk PUT endpoint, which
// is the API's only update operation for monitors. The monitor is identified by
// in["key"]. The updated monitor is re-read and returned.
func (c *Client) UpdateMonitor(ctx context.Context, in MonitorInput) (*Monitor, error) {
	key, _ := in["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("monitor key is required for update")
	}
	body := map[string]any{"monitors": []MonitorInput{in}}
	if err := c.do(ctx, http.MethodPut, "/monitors", nil, body, nil); err != nil {
		return nil, err
	}
	return c.GetMonitor(ctx, key)
}

// DeleteMonitor deletes a monitor by key.
func (c *Client) DeleteMonitor(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/monitors/"+escape(key), nil, nil, nil)
}
