package client

import (
	"context"
	"net/http"
)

// NotificationList is a named set of alert destinations ("template" in the API).
type NotificationList struct {
	Key           string        `json:"key"`
	Name          string        `json:"name"`
	Notifications Notifications `json:"notifications"`
	Environments  KeyList       `json:"environments"`
	Status        string        `json:"status"`
	Created       string        `json:"created"`
}

// Notifications are the destinations of a notification list. Integration
// entries (slack, pagerduty, ...) are integration labels.
type Notifications struct {
	Emails         []string `json:"emails"`
	Phones         []string `json:"phones"`
	Slack          []string `json:"slack"`
	PagerDuty      []string `json:"pagerduty"`
	DatadogOnCall  []string `json:"datadog-on-call"`
	Opsgenie       []string `json:"opsgenie"`
	VictorOps      []string `json:"victorops"`
	MicrosoftTeams []string `json:"microsoft-teams"`
	Discord        []string `json:"discord"`
	Telegram       []string `json:"telegram"`
	GChat          []string `json:"gchat"`
	LarkSuite      []string `json:"larksuite"`
	Webhooks       []string `json:"webhooks"`
}

// NotificationListInput is the body of a notification list create or update.
type NotificationListInput struct {
	Key           string        `json:"key,omitempty"`
	Name          string        `json:"name"`
	Notifications Notifications `json:"notifications"`
	Environments  []string      `json:"environments"`
}

func (c *Client) GetNotificationList(ctx context.Context, key string) (*NotificationList, error) {
	var out NotificationList
	if err := c.do(ctx, http.MethodGet, "/notifications/"+escape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateNotificationList(ctx context.Context, in NotificationListInput) (*NotificationList, error) {
	var out NotificationList
	if err := c.do(ctx, http.MethodPost, "/notifications", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateNotificationList(ctx context.Context, key string, in NotificationListInput) (*NotificationList, error) {
	in.Key = key
	var out NotificationList
	if err := c.do(ctx, http.MethodPut, "/notifications/"+escape(key), nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteNotificationList(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/notifications/"+escape(key), nil, nil, nil)
}

// Group is a named collection of monitors.
type Group struct {
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Monitors KeyList `json:"monitors"`
	Created  string  `json:"created"`
}

// GroupInput is the body of a group create or update. A nil Monitors leaves
// group membership untouched.
type GroupInput struct {
	Key      string    `json:"key,omitempty"`
	Name     string    `json:"name"`
	Monitors *[]string `json:"monitors,omitempty"`
}

func (c *Client) GetGroup(ctx context.Context, key string) (*Group, error) {
	var out Group
	if err := c.do(ctx, http.MethodGet, "/groups/"+escape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateGroup(ctx context.Context, in GroupInput) (*Group, error) {
	var out Group
	if err := c.do(ctx, http.MethodPost, "/groups", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateGroup(ctx context.Context, key string, in GroupInput) (*Group, error) {
	in.Key = ""
	var out Group
	if err := c.do(ctx, http.MethodPut, "/groups/"+escape(key), nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteGroup(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/groups/"+escape(key), nil, nil, nil)
}

// Environment is a deployment environment such as production or staging.
type Environment struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	WithAlerts bool   `json:"with_alerts"`
	Default    bool   `json:"default"`
	Created    string `json:"created"`
}

// EnvironmentInput is the body of an environment create or update.
type EnvironmentInput struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	WithAlerts bool   `json:"with_alerts"`
}

func (c *Client) GetEnvironment(ctx context.Context, key string) (*Environment, error) {
	var out Environment
	if err := c.do(ctx, http.MethodGet, "/environments/"+escape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateEnvironment(ctx context.Context, in EnvironmentInput) (*Environment, error) {
	var out Environment
	if err := c.do(ctx, http.MethodPost, "/environments", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, key string, in EnvironmentInput) (*Environment, error) {
	in.Key = key
	var out Environment
	if err := c.do(ctx, http.MethodPut, "/environments/"+escape(key), nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/environments/"+escape(key), nil, nil, nil)
}

// StatusPage is a hosted status page.
type StatusPage struct {
	Key                      string   `json:"key"`
	Name                     string   `json:"name"`
	LogoURL                  *string  `json:"logo_url"`
	WebsiteURL               *string  `json:"website_url"`
	SupportURL               *string  `json:"support_url"`
	AllowSearchIndex         bool     `json:"allow_search_index"`
	EnablePerformanceMetrics bool     `json:"enable_performance_metrics"`
	DefaultTimePeriod        string   `json:"default_time_period"`
	EnableDarkMode           bool     `json:"enable_dark_mode"`
	HostedSubdomain          string   `json:"hosted_subdomain"`
	CustomDomain             *string  `json:"custom_domain"`
	Access                   string   `json:"access"`
	IPAllowlist              FlexText `json:"ip_allowlist"`
	Environment              FlexKey  `json:"environment"`
	BadgeURLLight            string   `json:"badge_url_light"`
	BadgeURLDark             string   `json:"badge_url_dark"`
	Created                  string   `json:"created"`
	Updated                  string   `json:"updated"`
}

// StatusPageInput is the body of a status page create or update.
type StatusPageInput map[string]any

func (c *Client) GetStatusPage(ctx context.Context, key string) (*StatusPage, error) {
	var out StatusPage
	if err := c.do(ctx, http.MethodGet, "/statuspages/"+escape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateStatusPage(ctx context.Context, in StatusPageInput) (*StatusPage, error) {
	var out StatusPage
	if err := c.do(ctx, http.MethodPost, "/statuspages", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateStatusPage(ctx context.Context, key string, in StatusPageInput) (*StatusPage, error) {
	var out StatusPage
	if err := c.do(ctx, http.MethodPut, "/statuspages/"+escape(key), nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteStatusPage(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/statuspages/"+escape(key), nil, nil, nil)
}

// StatusPageComponent places a monitor or group on a status page.
type StatusPageComponent struct {
	Key         string     `json:"key"`
	Type        string     `json:"type"`
	Position    FlexString `json:"position"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Monitor     FlexKey    `json:"monitor"`
	Group       FlexKey    `json:"group"`
	Statuspage  FlexKey    `json:"statuspage"`
	Autopublish bool       `json:"autopublish"`
	Created     string     `json:"created"`
	Updated     string     `json:"updated"`
}

// StatusPageComponentInput is the body of a component create or update.
type StatusPageComponentInput map[string]any

func (c *Client) GetStatusPageComponent(ctx context.Context, key string) (*StatusPageComponent, error) {
	var out StatusPageComponent
	if err := c.do(ctx, http.MethodGet, "/statuspage_components/"+escape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateStatusPageComponent(ctx context.Context, in StatusPageComponentInput) (*StatusPageComponent, error) {
	var out StatusPageComponent
	if err := c.do(ctx, http.MethodPost, "/statuspage_components", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateStatusPageComponent(ctx context.Context, key string, in StatusPageComponentInput) (*StatusPageComponent, error) {
	var out StatusPageComponent
	if err := c.do(ctx, http.MethodPut, "/statuspage_components/"+escape(key), nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteStatusPageComponent(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/statuspage_components/"+escape(key), nil, nil, nil)
}

// MaintenanceWindow suppresses alerts for a period of planned downtime.
type MaintenanceWindow struct {
	Key                         string  `json:"key"`
	Name                        string  `json:"name"`
	Description                 *string `json:"description"`
	Start                       string  `json:"start"`
	End                         string  `json:"end"`
	AffectsAllMonitors          bool    `json:"affects_all_monitors"`
	AffectsStatuspageComponents bool    `json:"affects_statuspage_components"`
	Environment                 FlexKey `json:"environment"`
	Statuspages                 KeyList `json:"statuspages"`
	Monitors                    KeyList `json:"monitors"`
	Groups                      KeyList `json:"groups"`
	Duration                    FlexInt `json:"duration"`
	State                       string  `json:"state"`
	Created                     string  `json:"created"`
	Updated                     string  `json:"updated"`
}

// MaintenanceWindowInput is the body of a maintenance window create or update.
type MaintenanceWindowInput map[string]any

func (c *Client) GetMaintenanceWindow(ctx context.Context, key string) (*MaintenanceWindow, error) {
	var out MaintenanceWindow
	if err := c.do(ctx, http.MethodGet, "/maintenance_windows/"+escape(key), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateMaintenanceWindow(ctx context.Context, in MaintenanceWindowInput) (*MaintenanceWindow, error) {
	var out MaintenanceWindow
	if err := c.do(ctx, http.MethodPost, "/maintenance_windows", nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMaintenanceWindow(ctx context.Context, key string, in MaintenanceWindowInput) (*MaintenanceWindow, error) {
	var out MaintenanceWindow
	if err := c.do(ctx, http.MethodPut, "/maintenance_windows/"+escape(key), nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteMaintenanceWindow(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, "/maintenance_windows/"+escape(key), nil, nil, nil)
}
