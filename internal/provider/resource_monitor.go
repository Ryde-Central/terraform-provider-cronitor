package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

var (
	_ resource.Resource                   = (*monitorResource)(nil)
	_ resource.ResourceWithImportState    = (*monitorResource)(nil)
	_ resource.ResourceWithValidateConfig = (*monitorResource)(nil)
)

func NewMonitorResource() resource.Resource { return &monitorResource{} }

type monitorResource struct{ baseResource }

type monitorModel struct {
	ID                        types.String         `tfsdk:"id"`
	Key                       types.String         `tfsdk:"key"`
	Type                      types.String         `tfsdk:"type"`
	Name                      types.String         `tfsdk:"name"`
	Schedules                 types.List           `tfsdk:"schedules"`
	Assertions                types.List           `tfsdk:"assertions"`
	Notify                    types.Set            `tfsdk:"notify"`
	NotifyEvents              *notifyEventsModel   `tfsdk:"notify_events"`
	Group                     types.String         `tfsdk:"group"`
	Note                      types.String         `tfsdk:"note"`
	Platform                  types.String         `tfsdk:"platform"`
	Timezone                  types.String         `tfsdk:"timezone"`
	Paused                    types.Bool           `tfsdk:"paused"`
	GraceSeconds              types.Int64          `tfsdk:"grace_seconds"`
	FailureTolerance          types.Int64          `tfsdk:"failure_tolerance"`
	ScheduleTolerance         types.Int64          `tfsdk:"schedule_tolerance"`
	ConsecutiveAlertThreshold types.Int64          `tfsdk:"consecutive_alert_threshold"`
	RealertInterval           types.String         `tfsdk:"realert_interval"`
	Tags                      types.Set            `tfsdk:"tags"`
	Environments              types.Set            `tfsdk:"environments"`
	Statuspages               types.Set            `tfsdk:"statuspages"`
	Site                      types.String         `tfsdk:"site"`
	Request                   *monitorRequestModel `tfsdk:"request"`
	Created                   types.String         `tfsdk:"created"`
	BadgeURL                  types.String         `tfsdk:"badge_url"`
	DetailedBadgeURL          types.String         `tfsdk:"detailed_badge_url"`
}

type notifyEventsModel struct {
	Run      types.Bool `tfsdk:"run"`
	Complete types.Bool `tfsdk:"complete"`
}

type monitorRequestModel struct {
	URL             types.String `tfsdk:"url"`
	Method          types.String `tfsdk:"method"`
	Body            types.String `tfsdk:"body"`
	Headers         types.Map    `tfsdk:"headers"`
	Cookies         types.Map    `tfsdk:"cookies"`
	TimeoutSeconds  types.Int64  `tfsdk:"timeout_seconds"`
	Regions         types.Set    `tfsdk:"regions"`
	FollowRedirects types.Bool   `tfsdk:"follow_redirects"`
	VerifySSL       types.Bool   `tfsdk:"verify_ssl"`
	UseIPv6         types.Bool   `tfsdk:"use_ipv6"`
}

var monitorTypes = []string{"job", "heartbeat", "check", "site"}

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := stringplanmodifier.UseStateForUnknown()
	keepInt := int64planmodifier.UseStateForUnknown()
	keepBool := boolplanmodifier.UseStateForUnknown()
	keepSet := setplanmodifier.UseStateForUnknown()

	resp.Schema = schema.Schema{
		Description: "A Cronitor monitor: a job (cron job / scheduled task), heartbeat, check (uptime / HTTP, TCP or UDP request) or site (RUM).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Same as key.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"key": schema.StringAttribute{
				Description:   "Unique identifier of the monitor, used in API requests and telemetry URLs. Changing it creates a new monitor.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"type": schema.StringAttribute{
				Description:   "Monitor type: job, heartbeat, check or site. Changing it creates a new monitor.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.OneOf(monitorTypes...)},
			},
			"name": schema.StringAttribute{
				Description:   "Display name. Defaults to the key.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"schedules": schema.ListAttribute{
				Description: "Schedule expressions. For jobs and heartbeats, when telemetry is expected: cron expressions " +
					"(\"0 0 * * *\"), intervals (\"every 5 minutes\") or times of day (\"at 14:30\"). " +
					"For checks, how often the request is made (an interval between 30 seconds and 24 hours).",
				ElementType: types.StringType,
				Optional:    true,
				Validators:  []validator.List{listvalidator.SizeAtLeast(1), listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
			"assertions": schema.ListAttribute{
				Description: "Assertions such as \"metric.duration < 5 min\", \"response.code = 200\" or \"response.time < 2s\".",
				ElementType: types.StringType,
				Optional:    true,
				Validators:  []validator.List{listvalidator.SizeAtLeast(1), listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
			"notify": schema.SetAttribute{
				Description: "Notification list keys, or inline destinations such as \"email:ops@example.com\" or \"slack:#alerts\". " +
					"Defaults to the account's \"default\" notification list.",
				ElementType:   types.StringType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Set{keepSet},
			},
			"notify_events": schema.SingleNestedAttribute{
				Description: "Also send a notification when these telemetry events occur.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"run": schema.BoolAttribute{
						Description: "Notify when a job starts running.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"complete": schema.BoolAttribute{
						Description: "Notify when a job completes.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
				},
			},
			"group": schema.StringAttribute{
				Description: "Key of the group this monitor belongs to. Do not combine with the monitors attribute of cronitor_group for the same monitor.",
				Optional:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"note": schema.StringAttribute{
				Description: "Context or troubleshooting notes, included in alerts.",
				Optional:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"platform": schema.StringAttribute{
				Description:   "Where the monitor runs, e.g. \"linux cron\", \"kubernetes\", \"sidekiq\" for jobs, or \"http\", \"browser\", \"tcp\", \"udp\" for checks.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"timezone": schema.StringAttribute{
				Description:   "IANA timezone used to evaluate the schedule. Defaults to the account timezone.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"paused": schema.BoolAttribute{
				Description: "Whether alerting (and, for checks, outbound requests) is paused.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"grace_seconds": schema.Int64Attribute{
				Description:   "Seconds to wait for a late job or heartbeat before alerting. Defaults to 60 for jobs, 0 otherwise.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{keepInt},
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
			},
			"failure_tolerance": schema.Int64Attribute{
				Description:   "Number of failures tolerated before alerting. Defaults to 0.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{keepInt},
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
			},
			"schedule_tolerance": schema.Int64Attribute{
				Description:   "Number of missed scheduled executions tolerated before alerting. Defaults to 0.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{keepInt},
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
			},
			"consecutive_alert_threshold": schema.Int64Attribute{
				Description:   "Maximum consecutive alerts without recovery before further alerts are suppressed (1-1000).",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{keepInt},
				Validators:    []validator.Int64{int64validator.Between(1, 1000)},
			},
			"realert_interval": schema.StringAttribute{
				Description:   "How long to wait before re-sending an alert for a monitor that has not recovered, e.g. \"8 hours\".",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"tags": schema.SetAttribute{
				Description: "Tags.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"environments": schema.SetAttribute{
				Description:   "Keys of the environments this monitor is associated with.",
				ElementType:   types.StringType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Set{keepSet},
			},
			"statuspages": schema.SetAttribute{
				Description: "Keys of the status pages this monitor is shown on.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"site": schema.StringAttribute{
				Description: "Key of the RUM site (site monitors).",
				Optional:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"request": schema.SingleNestedAttribute{
				Description: "The request a check monitor makes. Required when type is \"check\".",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Description: "URL for HTTP checks, or host:port for TCP and UDP checks.",
						Required:    true,
					},
					"method": schema.StringAttribute{
						Description:   "GET, POST, PUT, PATCH, HEAD, TCP or UDP. Defaults to GET.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.String{keep},
						Validators:    []validator.String{stringvalidator.OneOf("GET", "POST", "PUT", "PATCH", "HEAD", "TCP", "UDP")},
					},
					"body": schema.StringAttribute{
						Description: "Request body.",
						Optional:    true,
					},
					"headers": schema.MapAttribute{
						Description: "Request headers.",
						ElementType: types.StringType,
						Optional:    true,
						Sensitive:   true,
					},
					"cookies": schema.MapAttribute{
						Description: "Cookies to set on the request.",
						ElementType: types.StringType,
						Optional:    true,
						Sensitive:   true,
					},
					"timeout_seconds": schema.Int64Attribute{
						Description:   "Request timeout in seconds. Defaults to 10.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int64{keepInt},
						Validators:    []validator.Int64{int64validator.Between(1, 60)},
					},
					"regions": schema.SetAttribute{
						Description:   "Regions the check is made from.",
						ElementType:   types.StringType,
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Set{keepSet},
					},
					"follow_redirects": schema.BoolAttribute{
						Description:   "Follow redirects (HTTP only). Defaults to true.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{keepBool},
					},
					"verify_ssl": schema.BoolAttribute{
						Description:   "Verify the TLS certificate (HTTP only). Defaults to true.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{keepBool},
					},
					"use_ipv6": schema.BoolAttribute{
						Description:   "Make the request over IPv6.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{keepBool},
					},
				},
			},
			"created": schema.StringAttribute{
				Description:   "When the monitor was created.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"badge_url": schema.StringAttribute{
				Description:   "URL of the simple status badge.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"detailed_badge_url": schema.StringAttribute{
				Description:   "URL of the detailed status badge.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
		},
	}
}

func (r *monitorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg monitorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || !known(cfg.Type) {
		return
	}
	isCheck := cfg.Type.ValueString() == "check"
	if isCheck && cfg.Request == nil {
		resp.Diagnostics.AddAttributeError(path.Root("request"), "Missing request",
			"Monitors of type \"check\" require a request block.")
	}
	if !isCheck && cfg.Request != nil {
		resp.Diagnostics.AddAttributeError(path.Root("request"), "Unexpected request",
			"The request attribute is only valid for monitors of type \"check\".")
	}
}

// toInput builds the API payload from a plan. Attributes that are only
// Optional are sent as explicit empties/nulls on update so that removing them
// from configuration clears them; Optional+Computed attributes are only sent
// when known, leaving the API default in place otherwise.
func (m *monitorModel) toInput(ctx context.Context, update bool, diags *diag.Diagnostics) client.MonitorInput {
	in := client.MonitorInput{
		"key":    m.Key.ValueString(),
		"type":   m.Type.ValueString(),
		"paused": m.Paused.ValueBool(),
	}
	// clearable sets an optional attribute, or clears it on update.
	clearable := func(name string, v any, isSet bool) {
		if isSet {
			in[name] = v
		} else if update {
			in[name] = nil
		}
	}
	clearableList := func(name string, v []string, isSet bool) {
		if isSet || update {
			in[name] = v
		}
	}

	if known(m.Name) {
		in["name"] = m.Name.ValueString()
	}
	clearableList("schedules", stringsFrom(ctx, m.Schedules, diags), known(m.Schedules))
	clearableList("assertions", stringsFrom(ctx, m.Assertions, diags), known(m.Assertions))
	clearableList("tags", stringsFrom(ctx, m.Tags, diags), known(m.Tags))
	clearableList("statuspages", stringsFrom(ctx, m.Statuspages, diags), known(m.Statuspages))
	clearable("group", nullableString(m.Group), known(m.Group))
	clearable("note", nullableString(m.Note), known(m.Note))
	clearable("site", nullableString(m.Site), known(m.Site))

	if known(m.Notify) || m.NotifyEvents != nil {
		notify := client.Notify{Alerts: []string{"default"}}
		if known(m.Notify) {
			notify.Alerts = stringsFrom(ctx, m.Notify, diags)
		}
		if ev := m.NotifyEvents; ev != nil && (ev.Run.ValueBool() || ev.Complete.ValueBool()) {
			notify.Events = &client.NotifyEvents{Run: ev.Run.ValueBool(), Complete: ev.Complete.ValueBool()}
		} else if update {
			// Switch back from the hash form, which disables event notifications.
			notify.Events = &client.NotifyEvents{}
		}
		in["notify"] = notify
	}
	if known(m.Environments) {
		in["environments"] = stringsFrom(ctx, m.Environments, diags)
	}
	for name, v := range map[string]types.String{
		"platform":         m.Platform,
		"timezone":         m.Timezone,
		"realert_interval": m.RealertInterval,
	} {
		if known(v) {
			in[name] = v.ValueString()
		}
	}
	for name, v := range map[string]types.Int64{
		"grace_seconds":               m.GraceSeconds,
		"failure_tolerance":           m.FailureTolerance,
		"schedule_tolerance":          m.ScheduleTolerance,
		"consecutive_alert_threshold": m.ConsecutiveAlertThreshold,
	} {
		if known(v) {
			in[name] = v.ValueInt64()
		}
	}

	if rq := m.Request; rq != nil {
		req := map[string]any{"url": rq.URL.ValueString()}
		if known(rq.Method) {
			req["method"] = rq.Method.ValueString()
		}
		if known(rq.Body) {
			req["body"] = rq.Body.ValueString()
		} else if update {
			req["body"] = nil
		}
		if known(rq.Headers) || update {
			req["headers"] = stringMapFrom(ctx, rq.Headers, diags)
		}
		if known(rq.Cookies) || update {
			req["cookies"] = stringMapFrom(ctx, rq.Cookies, diags)
		}
		if known(rq.TimeoutSeconds) {
			req["timeout_seconds"] = rq.TimeoutSeconds.ValueInt64()
		}
		if known(rq.Regions) {
			req["regions"] = stringsFrom(ctx, rq.Regions, diags)
		}
		for name, v := range map[string]types.Bool{
			"follow_redirects": rq.FollowRedirects,
			"verify_ssl":       rq.VerifySSL,
			"use_ipv6":         rq.UseIPv6,
		} {
			if known(v) {
				req[name] = v.ValueBool()
			}
		}
		in["request"] = req
	} else if update {
		in["request"] = nil
	}
	return in
}

// fromAPI sets the model from an API monitor. prior is the previous model
// (plan or state), used to keep null-vs-empty and equivalent-expression
// distinctions stable.
func (m *monitorModel) fromAPI(api *client.Monitor) {
	prior := *m

	m.ID = types.StringValue(api.Key)
	m.Key = types.StringValue(api.Key)
	m.Type = types.StringValue(api.Type)
	m.Name = stringOrNull(api.Name)
	if m.Name.IsNull() {
		m.Name = types.StringValue(api.Key)
	}
	m.Schedules = listValue(api.Schedules, prior.Schedules)
	m.Assertions = listValue(api.Assertions, prior.Assertions)
	m.Notify = setValue(api.Notify.Alerts, types.SetValueMust(types.StringType, nil))
	if ev := api.Notify.Events; ev != nil && (ev.Run || ev.Complete) {
		m.NotifyEvents = &notifyEventsModel{Run: types.BoolValue(ev.Run), Complete: types.BoolValue(ev.Complete)}
	} else if prior.NotifyEvents != nil && !prior.NotifyEvents.Run.ValueBool() && !prior.NotifyEvents.Complete.ValueBool() {
		// An all-false block in configuration is equivalent to none.
		m.NotifyEvents = prior.NotifyEvents
	} else {
		m.NotifyEvents = nil
	}
	m.Group = stringOrNull(string(api.Group))
	m.Note = stringPtrOrNull(api.Note)
	m.Platform = stringPtrOrNull(api.Platform)
	m.Timezone = stringPtrOrNull(api.Timezone)
	m.Paused = types.BoolValue(api.Paused)
	m.GraceSeconds = int64PtrOrNull(api.GraceSeconds)
	m.FailureTolerance = int64PtrOrNull(api.FailureTolerance)
	m.ScheduleTolerance = int64PtrOrNull(api.ScheduleTolerance)
	m.ConsecutiveAlertThreshold = int64PtrOrNull(api.ConsecutiveAlertThreshold)
	m.RealertInterval = types.StringNull()
	if api.RealertInterval != nil {
		m.RealertInterval = sameInterval(string(*api.RealertInterval), prior.RealertInterval)
	}
	m.Tags = setValue(api.Tags, prior.Tags)
	m.Environments = setValue(api.Environments, types.SetValueMust(types.StringType, nil))
	m.Statuspages = setValue(api.Statuspages, prior.Statuspages)
	m.Site = stringOrNull(string(api.Site))

	m.Request = nil
	if rq := api.Request; rq != nil && rq.URL != "" {
		var p monitorRequestModel
		if prior.Request != nil {
			p = *prior.Request
		}
		out := &monitorRequestModel{
			URL:             types.StringValue(rq.URL),
			Method:          stringOrNull(rq.Method),
			Body:            stringPtrOrNull(rq.Body),
			Headers:         mapValue(rq.Headers, p.Headers),
			Cookies:         mapValue(rq.Cookies, p.Cookies),
			TimeoutSeconds:  int64PtrOrNull(rq.TimeoutSeconds),
			Regions:         setValue(rq.Regions, types.SetValueMust(types.StringType, nil)),
			FollowRedirects: types.BoolPointerValue(rq.FollowRedirects),
			VerifySSL:       types.BoolPointerValue(rq.VerifySSL),
			UseIPv6:         types.BoolPointerValue(rq.UseIPv6),
		}
		m.Request = out
	}

	m.Created = stringOrNull(api.Created)
	m.BadgeURL, m.DetailedBadgeURL = types.StringNull(), types.StringNull()
	if api.Badges != nil {
		m.BadgeURL = stringOrNull(api.Badges.Simple)
		m.DetailedBadgeURL = stringOrNull(api.Badges.Detailed)
	}
}

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan monitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toInput(ctx, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.CreateMonitor(ctx, in); err != nil {
		resp.Diagnostics.AddError("Error creating monitor", err.Error())
		return
	}
	// Re-read so that the state reflects a full representation of the monitor.
	api, err := r.client.GetMonitor(ctx, plan.Key.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading monitor after create", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state monitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.GetMonitor(ctx, state.Key.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading monitor", err.Error())
		return
	}
	state.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan monitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toInput(ctx, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.UpdateMonitor(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating monitor", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state monitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMonitor(ctx, state.Key.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting monitor", err.Error())
	}
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
