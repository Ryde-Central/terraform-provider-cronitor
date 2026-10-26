package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

var keyPath = path.Root("key")

// keyedDataSource is a data source that looks up a single object by key.
type keyedDataSource[M any] struct {
	baseDataSource
	typeName    string
	description string
	attributes  map[string]schema.Attribute
	read        func(ctx context.Context, c *client.Client, key string) (*M, error)
}

func (d *keyedDataSource[M]) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.typeName
}

func (d *keyedDataSource[M]) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"key": schema.StringAttribute{Description: "Key to look up.", Required: true},
		"id":  schema.StringAttribute{Description: "Same as key.", Computed: true},
	}
	for k, v := range d.attributes {
		attrs[k] = v
	}
	resp.Schema = schema.Schema{Description: d.description, Attributes: attrs}
}

func (d *keyedDataSource[M]) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var key types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, keyPath, &key)...)
	if resp.Diagnostics.HasError() {
		return
	}
	model, err := d.read(ctx, d.client, key.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading "+d.typeName, err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func computedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Description: desc, Computed: true}
}

func computedBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{Description: desc, Computed: true}
}

func computedStringSet(desc string) schema.SetAttribute {
	return schema.SetAttribute{Description: desc, Computed: true, ElementType: types.StringType}
}

func computedStringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{Description: desc, Computed: true, ElementType: types.StringType}
}

type monitorDataModel struct {
	ID           types.String `tfsdk:"id"`
	Key          types.String `tfsdk:"key"`
	Type         types.String `tfsdk:"type"`
	Name         types.String `tfsdk:"name"`
	Schedules    types.List   `tfsdk:"schedules"`
	Assertions   types.List   `tfsdk:"assertions"`
	Notify       types.Set    `tfsdk:"notify"`
	Group        types.String `tfsdk:"group"`
	Note         types.String `tfsdk:"note"`
	Platform     types.String `tfsdk:"platform"`
	Timezone     types.String `tfsdk:"timezone"`
	Tags         types.Set    `tfsdk:"tags"`
	Environments types.Set    `tfsdk:"environments"`
	Paused       types.Bool   `tfsdk:"paused"`
	Disabled     types.Bool   `tfsdk:"disabled"`
	Passing      types.Bool   `tfsdk:"passing"`
	Initialized  types.Bool   `tfsdk:"initialized"`
	Created      types.String `tfsdk:"created"`
	BadgeURL     types.String `tfsdk:"badge_url"`
}

func NewMonitorDataSource() datasource.DataSource {
	empty := types.SetValueMust(types.StringType, nil)
	return &keyedDataSource[monitorDataModel]{
		typeName:    "monitor",
		description: "Looks up a Cronitor monitor by key.",
		attributes: map[string]schema.Attribute{
			"type":         computedString("job, heartbeat, check or site."),
			"name":         computedString("Display name."),
			"schedules":    computedStringList("Schedule expressions."),
			"assertions":   computedStringList("Assertions."),
			"notify":       computedStringSet("Notification list keys or inline destinations."),
			"group":        computedString("Group key."),
			"note":         computedString("Note."),
			"platform":     computedString("Platform."),
			"timezone":     computedString("Timezone."),
			"tags":         computedStringSet("Tags."),
			"environments": computedStringSet("Environment keys."),
			"paused":       computedBool("Whether the monitor is paused."),
			"disabled":     computedBool("Whether the monitor is disabled."),
			"passing":      computedBool("Whether the monitor is currently passing."),
			"initialized":  computedBool("Whether the monitor has received telemetry."),
			"created":      computedString("When the monitor was created."),
			"badge_url":    computedString("URL of the simple status badge."),
		},
		read: func(ctx context.Context, c *client.Client, key string) (*monitorDataModel, error) {
			m, err := c.GetMonitorWithStatus(ctx, key)
			if err != nil {
				return nil, err
			}
			out := &monitorDataModel{
				ID:           types.StringValue(m.Key),
				Key:          types.StringValue(m.Key),
				Type:         types.StringValue(m.Type),
				Name:         types.StringValue(m.Name),
				Schedules:    listValue(m.Schedules, types.ListValueMust(types.StringType, nil)),
				Assertions:   listValue(m.Assertions, types.ListValueMust(types.StringType, nil)),
				Notify:       setValue(m.Notify.Alerts, empty),
				Group:        stringOrNull(string(m.Group)),
				Note:         stringPtrOrNull(m.Note),
				Platform:     stringPtrOrNull(m.Platform),
				Timezone:     stringPtrOrNull(m.Timezone),
				Tags:         setValue(m.Tags, empty),
				Environments: setValue(m.Environments, empty),
				Paused:       types.BoolValue(m.Paused),
				Disabled:     types.BoolValue(m.Disabled),
				Passing:      types.BoolPointerValue(m.Passing),
				Initialized:  types.BoolPointerValue(m.Initialized),
				Created:      stringOrNull(m.Created),
				BadgeURL:     types.StringNull(),
			}
			if m.Badges != nil {
				out.BadgeURL = stringOrNull(m.Badges.Simple)
			}
			return out, nil
		},
	}
}

type notificationListDataModel struct {
	ID           types.String `tfsdk:"id"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Emails       types.Set    `tfsdk:"emails"`
	Phones       types.Set    `tfsdk:"phones"`
	Environments types.Set    `tfsdk:"environments"`
	Created      types.String `tfsdk:"created"`
}

func NewNotificationListDataSource() datasource.DataSource {
	empty := types.SetValueMust(types.StringType, nil)
	return &keyedDataSource[notificationListDataModel]{
		typeName:    "notification_list",
		description: "Looks up a Cronitor notification list by key, e.g. \"default\".",
		attributes: map[string]schema.Attribute{
			"name":         computedString("Display name."),
			"emails":       computedStringSet("Email addresses."),
			"phones":       computedStringSet("Phone numbers."),
			"environments": computedStringSet("Environment keys."),
			"created":      computedString("When the list was created."),
		},
		read: func(ctx context.Context, c *client.Client, key string) (*notificationListDataModel, error) {
			n, err := c.GetNotificationList(ctx, key)
			if err != nil {
				return nil, err
			}
			return &notificationListDataModel{
				ID:           types.StringValue(n.Key),
				Key:          types.StringValue(n.Key),
				Name:         types.StringValue(n.Name),
				Emails:       setValue(n.Notifications.Emails, empty),
				Phones:       setValue(n.Notifications.Phones, empty),
				Environments: setValue(n.Environments, empty),
				Created:      stringOrNull(n.Created),
			}, nil
		},
	}
}

type groupDataModel struct {
	ID       types.String `tfsdk:"id"`
	Key      types.String `tfsdk:"key"`
	Name     types.String `tfsdk:"name"`
	Monitors types.Set    `tfsdk:"monitors"`
	Created  types.String `tfsdk:"created"`
}

func NewGroupDataSource() datasource.DataSource {
	return &keyedDataSource[groupDataModel]{
		typeName:    "group",
		description: "Looks up a Cronitor monitor group by key.",
		attributes: map[string]schema.Attribute{
			"name":     computedString("Display name."),
			"monitors": computedStringSet("Keys of the monitors in the group."),
			"created":  computedString("When the group was created."),
		},
		read: func(ctx context.Context, c *client.Client, key string) (*groupDataModel, error) {
			g, err := c.GetGroup(ctx, key)
			if err != nil {
				return nil, err
			}
			return &groupDataModel{
				ID:       types.StringValue(g.Key),
				Key:      types.StringValue(g.Key),
				Name:     types.StringValue(g.Name),
				Monitors: setValue(g.Monitors, types.SetValueMust(types.StringType, nil)),
				Created:  stringOrNull(g.Created),
			}, nil
		},
	}
}

func NewEnvironmentDataSource() datasource.DataSource {
	return &keyedDataSource[environmentModel]{
		typeName:    "environment",
		description: "Looks up a Cronitor environment by key.",
		attributes: map[string]schema.Attribute{
			"name":        computedString("Display name."),
			"with_alerts": computedBool("Whether alerts are sent for this environment."),
			"is_default":  computedBool("Whether this is the default environment."),
			"created":     computedString("When the environment was created."),
		},
		read: func(ctx context.Context, c *client.Client, key string) (*environmentModel, error) {
			e, err := c.GetEnvironment(ctx, key)
			if err != nil {
				return nil, err
			}
			var m environmentModel
			m.fromAPI(e)
			return &m, nil
		},
	}
}

type statusPageDataModel struct {
	ID              types.String `tfsdk:"id"`
	Key             types.String `tfsdk:"key"`
	Name            types.String `tfsdk:"name"`
	HostedSubdomain types.String `tfsdk:"hosted_subdomain"`
	CustomDomain    types.String `tfsdk:"custom_domain"`
	Access          types.String `tfsdk:"access"`
	Environment     types.String `tfsdk:"environment"`
	BadgeURLLight   types.String `tfsdk:"badge_url_light"`
	BadgeURLDark    types.String `tfsdk:"badge_url_dark"`
}

func NewStatusPageDataSource() datasource.DataSource {
	return &keyedDataSource[statusPageDataModel]{
		typeName:    "status_page",
		description: "Looks up a Cronitor status page by key.",
		attributes: map[string]schema.Attribute{
			"name":             computedString("Display name."),
			"hosted_subdomain": computedString("Hosted subdomain."),
			"custom_domain":    computedString("Custom domain."),
			"access":           computedString("public or password."),
			"environment":      computedString("Environment key."),
			"badge_url_light":  computedString("URL of the light status badge."),
			"badge_url_dark":   computedString("URL of the dark status badge."),
		},
		read: func(ctx context.Context, c *client.Client, key string) (*statusPageDataModel, error) {
			s, err := c.GetStatusPage(ctx, key)
			if err != nil {
				return nil, err
			}
			return &statusPageDataModel{
				ID:              types.StringValue(s.Key),
				Key:             types.StringValue(s.Key),
				Name:            types.StringValue(s.Name),
				HostedSubdomain: types.StringValue(s.HostedSubdomain),
				CustomDomain:    stringPtrOrNull(s.CustomDomain),
				Access:          stringOrNull(s.Access),
				Environment:     stringOrNull(string(s.Environment)),
				BadgeURLLight:   stringOrNull(s.BadgeURLLight),
				BadgeURLDark:    stringOrNull(s.BadgeURLDark),
			}, nil
		},
	}
}
