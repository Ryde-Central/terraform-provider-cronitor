package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

var (
	_ resource.Resource                = (*environmentResource)(nil)
	_ resource.ResourceWithImportState = (*environmentResource)(nil)
)

func NewEnvironmentResource() resource.Resource { return &environmentResource{} }

type environmentResource struct{ baseResource }

type environmentModel struct {
	ID         types.String `tfsdk:"id"`
	Key        types.String `tfsdk:"key"`
	Name       types.String `tfsdk:"name"`
	WithAlerts types.Bool   `tfsdk:"with_alerts"`
	IsDefault  types.Bool   `tfsdk:"is_default"`
	Created    types.String `tfsdk:"created"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := stringplanmodifier.UseStateForUnknown()
	resp.Schema = schema.Schema{
		Description: "An environment (e.g. production, staging) that monitor status is tracked separately in.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Same as key.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"key": schema.StringAttribute{
				Description:   "Unique identifier, sent as the env parameter of telemetry events. Changing it creates a new environment.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"name": schema.StringAttribute{
				Description: "Display name.",
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"with_alerts": schema.BoolAttribute{
				Description: "Whether alerts are sent for monitors in this environment. Defaults to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"is_default": schema.BoolAttribute{
				Description:   "Whether this is the account's default environment.",
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"created": schema.StringAttribute{
				Description:   "When the environment was created.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
		},
	}
}

func (m *environmentModel) toInput() client.EnvironmentInput {
	return client.EnvironmentInput{
		Key:        m.Key.ValueString(),
		Name:       m.Name.ValueString(),
		WithAlerts: m.WithAlerts.ValueBool(),
	}
}

func (m *environmentModel) fromAPI(api *client.Environment) {
	m.ID = types.StringValue(api.Key)
	m.Key = types.StringValue(api.Key)
	m.Name = types.StringValue(api.Name)
	m.WithAlerts = types.BoolValue(api.WithAlerts)
	m.IsDefault = types.BoolValue(api.Default)
	m.Created = stringOrNull(api.Created)
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.CreateEnvironment(ctx, plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error creating environment", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.GetEnvironment(ctx, state.Key.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading environment", err.Error())
		return
	}
	state.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.UpdateEnvironment(ctx, plan.Key.ValueString(), plan.toInput())
	if err != nil {
		resp.Diagnostics.AddError("Error updating environment", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEnvironment(ctx, state.Key.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting environment", err.Error())
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
