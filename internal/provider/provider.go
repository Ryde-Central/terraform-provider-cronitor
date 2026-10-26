// Package provider implements the Cronitor Terraform/OpenTofu provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"git.mamori.io/trip/rydevops/terraform-provider-cronitor/internal/client"
)

const (
	envAPIKey = "CRONITOR_API_KEY"
	envAPIURL = "CRONITOR_API_URL"
)

var _ provider.Provider = (*cronitorProvider)(nil)

type cronitorProvider struct {
	version string
}

type providerModel struct {
	APIKey types.String `tfsdk:"api_key"`
	APIURL types.String `tfsdk:"api_url"`
}

// New returns a provider factory for the given provider version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &cronitorProvider{version: version}
	}
}

func (p *cronitorProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cronitor"
	resp.Version = p.version
}

func (p *cronitorProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Cronitor (https://cronitor.io) monitors, notification lists, groups, environments, status pages and maintenance windows.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "Cronitor API key. Needs the read and write scopes of the resources you manage " +
					"(for example monitor:read and monitor:write). Can also be set with the " + envAPIKey + " environment variable.",
				Optional:  true,
				Sensitive: true,
			},
			"api_url": schema.StringAttribute{
				Description: "Base URL of the Cronitor REST API. Defaults to " + client.DefaultBaseURL +
					". Can also be set with the " + envAPIURL + " environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *cronitorProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Unknown Cronitor API key",
			"The provider cannot be configured with an API key that is unknown until apply.")
	}
	if cfg.APIURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_url"), "Unknown Cronitor API URL",
			"The provider cannot be configured with an API URL that is unknown until apply.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv(envAPIKey)
	if !cfg.APIKey.IsNull() {
		apiKey = cfg.APIKey.ValueString()
	}
	apiURL := os.Getenv(envAPIURL)
	if !cfg.APIURL.IsNull() {
		apiURL = cfg.APIURL.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing Cronitor API key",
			"Set the api_key provider argument or the "+envAPIKey+" environment variable.")
		return
	}

	c, err := client.New(apiURL, apiKey, client.WithUserAgent("terraform-provider-cronitor/"+p.version))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Cronitor client", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *cronitorProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMonitorResource,
		NewNotificationListResource,
		NewGroupResource,
		NewEnvironmentResource,
		NewStatusPageResource,
		NewStatusPageComponentResource,
		NewMaintenanceWindowResource,
	}
}

func (p *cronitorProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewMonitorDataSource,
		NewNotificationListDataSource,
		NewGroupDataSource,
		NewEnvironmentDataSource,
		NewStatusPageDataSource,
	}
}
