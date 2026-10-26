package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

var (
	_ resource.Resource                = (*notificationListResource)(nil)
	_ resource.ResourceWithImportState = (*notificationListResource)(nil)
)

func NewNotificationListResource() resource.Resource { return &notificationListResource{} }

type notificationListResource struct{ baseResource }

type notificationListModel struct {
	ID            types.String        `tfsdk:"id"`
	Key           types.String        `tfsdk:"key"`
	Name          types.String        `tfsdk:"name"`
	Notifications *notificationsModel `tfsdk:"notifications"`
	Environments  types.Set           `tfsdk:"environments"`
	Created       types.String        `tfsdk:"created"`
}

type notificationsModel struct {
	Emails         types.Set `tfsdk:"emails"`
	Phones         types.Set `tfsdk:"phones"`
	Slack          types.Set `tfsdk:"slack"`
	PagerDuty      types.Set `tfsdk:"pagerduty"`
	DatadogOnCall  types.Set `tfsdk:"datadog_on_call"`
	Opsgenie       types.Set `tfsdk:"opsgenie"`
	VictorOps      types.Set `tfsdk:"victorops"`
	MicrosoftTeams types.Set `tfsdk:"microsoft_teams"`
	Discord        types.Set `tfsdk:"discord"`
	Telegram       types.Set `tfsdk:"telegram"`
	GChat          types.Set `tfsdk:"gchat"`
	LarkSuite      types.Set `tfsdk:"larksuite"`
	Webhooks       types.Set `tfsdk:"webhooks"`
}

// notificationChannels describes each destination attribute: its schema name,
// description, and accessors on the model and API types.
var notificationChannels = []struct {
	name        string
	description string
	model       func(*notificationsModel) *types.Set
	api         func(*client.Notifications) *[]string
}{
	{"emails", "Email addresses.", func(m *notificationsModel) *types.Set { return &m.Emails }, func(n *client.Notifications) *[]string { return &n.Emails }},
	{"phones", "Phone numbers for SMS, e.g. +15551234567.", func(m *notificationsModel) *types.Set { return &m.Phones }, func(n *client.Notifications) *[]string { return &n.Phones }},
	{"slack", "Slack integration labels.", func(m *notificationsModel) *types.Set { return &m.Slack }, func(n *client.Notifications) *[]string { return &n.Slack }},
	{"pagerduty", "PagerDuty integration labels.", func(m *notificationsModel) *types.Set { return &m.PagerDuty }, func(n *client.Notifications) *[]string { return &n.PagerDuty }},
	{"datadog_on_call", "Datadog On-Call integration labels.", func(m *notificationsModel) *types.Set { return &m.DatadogOnCall }, func(n *client.Notifications) *[]string { return &n.DatadogOnCall }},
	{"opsgenie", "Opsgenie integration labels.", func(m *notificationsModel) *types.Set { return &m.Opsgenie }, func(n *client.Notifications) *[]string { return &n.Opsgenie }},
	{"victorops", "Splunk On-Call (VictorOps) integration labels.", func(m *notificationsModel) *types.Set { return &m.VictorOps }, func(n *client.Notifications) *[]string { return &n.VictorOps }},
	{"microsoft_teams", "Microsoft Teams integration labels.", func(m *notificationsModel) *types.Set { return &m.MicrosoftTeams }, func(n *client.Notifications) *[]string { return &n.MicrosoftTeams }},
	{"discord", "Discord integration labels.", func(m *notificationsModel) *types.Set { return &m.Discord }, func(n *client.Notifications) *[]string { return &n.Discord }},
	{"telegram", "Telegram integration labels.", func(m *notificationsModel) *types.Set { return &m.Telegram }, func(n *client.Notifications) *[]string { return &n.Telegram }},
	{"gchat", "Google Chat integration labels.", func(m *notificationsModel) *types.Set { return &m.GChat }, func(n *client.Notifications) *[]string { return &n.GChat }},
	{"larksuite", "Lark integration labels.", func(m *notificationsModel) *types.Set { return &m.LarkSuite }, func(n *client.Notifications) *[]string { return &n.LarkSuite }},
	{"webhooks", "Webhook URLs or webhook integration labels. The API returns webhook identifiers rather than URLs, so " +
		"configured values are kept in state as long as the number of webhooks is unchanged.",
		func(m *notificationsModel) *types.Set { return &m.Webhooks }, func(n *client.Notifications) *[]string { return &n.Webhooks }},
}

func (r *notificationListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_list"
}

func (r *notificationListResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	channels := map[string]schema.Attribute{}
	for _, ch := range notificationChannels {
		channels[ch.name] = schema.SetAttribute{
			Description: ch.description,
			ElementType: types.StringType,
			Optional:    true,
		}
	}
	keep := stringplanmodifier.UseStateForUnknown()

	resp.Schema = schema.Schema{
		Description: "A notification list: a named set of destinations that monitors send alerts to. " +
			"Integrations (Slack, PagerDuty, ...) must be connected in Cronitor before they can be referenced by label.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Same as key.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"key": schema.StringAttribute{
				Description:   "Unique identifier, used in a monitor's notify attribute. Generated from the name if not set. Changing it creates a new list.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep, stringplanmodifier.RequiresReplaceIfConfigured()},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"name": schema.StringAttribute{
				Description: "Display name.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"notifications": schema.SingleNestedAttribute{
				Description: "Alert destinations.",
				Required:    true,
				Attributes:  channels,
			},
			"environments": schema.SetAttribute{
				Description: "Limit this list to the given environment keys.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"created": schema.StringAttribute{
				Description:   "When the list was created.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
		},
	}
}

func (m *notificationListModel) toInput(ctx context.Context, diags *diag.Diagnostics) client.NotificationListInput {
	in := client.NotificationListInput{
		Name:         m.Name.ValueString(),
		Environments: stringsFrom(ctx, m.Environments, diags),
	}
	if known(m.Key) {
		in.Key = m.Key.ValueString()
	}
	if m.Notifications != nil {
		for _, ch := range notificationChannels {
			*ch.api(&in.Notifications) = stringsFrom(ctx, *ch.model(m.Notifications), diags)
		}
	}
	return in
}

func (m *notificationListModel) fromAPI(api *client.NotificationList) {
	prior := m.Notifications
	if prior == nil {
		prior = &notificationsModel{}
	}
	out := &notificationsModel{}
	for _, ch := range notificationChannels {
		priorVal := *ch.model(prior)
		if priorVal.IsNull() {
			priorVal = types.SetNull(types.StringType)
		}
		vals := *ch.api(&api.Notifications)
		v := setValue(vals, priorVal)
		if ch.name == "webhooks" && known(priorVal) && len(priorVal.Elements()) == len(vals) {
			v = priorVal
		}
		*ch.model(out) = v
	}

	m.ID = types.StringValue(api.Key)
	m.Key = types.StringValue(api.Key)
	m.Name = types.StringValue(api.Name)
	m.Notifications = out
	m.Environments = setValue(api.Environments, m.Environments)
	m.Created = stringOrNull(api.Created)
}

func (r *notificationListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan notificationListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toInput(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.CreateNotificationList(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating notification list", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *notificationListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state notificationListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.GetNotificationList(ctx, state.Key.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading notification list", err.Error())
		return
	}
	state.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *notificationListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan notificationListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toInput(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.UpdateNotificationList(ctx, plan.Key.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating notification list", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *notificationListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state notificationListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotificationList(ctx, state.Key.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting notification list", err.Error())
	}
}

func (r *notificationListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
