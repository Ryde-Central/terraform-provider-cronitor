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
	_ resource.Resource                = (*groupResource)(nil)
	_ resource.ResourceWithImportState = (*groupResource)(nil)
)

func NewGroupResource() resource.Resource { return &groupResource{} }

type groupResource struct{ baseResource }

type groupModel struct {
	ID       types.String `tfsdk:"id"`
	Key      types.String `tfsdk:"key"`
	Name     types.String `tfsdk:"name"`
	Monitors types.Set    `tfsdk:"monitors"`
	Created  types.String `tfsdk:"created"`
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := stringplanmodifier.UseStateForUnknown()
	resp.Schema = schema.Schema{
		Description: "A group of monitors.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Same as key.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
			"key": schema.StringAttribute{
				Description:   "Unique identifier, used in a monitor's group attribute. Generated from the name if not set. Changing it creates a new group.",
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
			"monitors": schema.SetAttribute{
				Description: "Keys of the monitors in this group. When set, this is the complete membership: monitors not listed are removed. " +
					"Leave unset to manage membership with the group attribute of cronitor_monitor instead; do not use both for the same group.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"created": schema.StringAttribute{
				Description:   "When the group was created.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{keep},
			},
		},
	}
}

func (m *groupModel) toInput(ctx context.Context, diags *diag.Diagnostics) client.GroupInput {
	in := client.GroupInput{Name: m.Name.ValueString()}
	if known(m.Key) {
		in.Key = m.Key.ValueString()
	}
	if known(m.Monitors) {
		monitors := stringsFrom(ctx, m.Monitors, diags)
		in.Monitors = &monitors
	}
	return in
}

func (m *groupModel) fromAPI(api *client.Group) {
	m.ID = types.StringValue(api.Key)
	m.Key = types.StringValue(api.Key)
	m.Name = types.StringValue(api.Name)
	// Membership is only tracked when managed by this resource.
	if !m.Monitors.IsNull() {
		m.Monitors = setValue(api.Monitors, m.Monitors)
	}
	m.Created = stringOrNull(api.Created)
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toInput(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.CreateGroup(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating group", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.GetGroup(ctx, state.Key.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading group", err.Error())
		return
	}
	state.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toInput(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	api, err := r.client.UpdateGroup(ctx, plan.Key.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating group", err.Error())
		return
	}
	plan.fromAPI(api)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteGroup(ctx, state.Key.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting group", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
