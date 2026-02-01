// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

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
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

// Ensure NayatelProvider satisfies various provider interfaces.
var _ provider.Provider = &NayatelProvider{}

// NayatelProvider defines the provider implementation.
type NayatelProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// NayatelProviderModel describes the provider data model.
type NayatelProviderModel struct {
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
	Token     types.String `tfsdk:"token"`
	ProjectID types.String `tfsdk:"project_id"`
	BaseURL   types.String `tfsdk:"base_url"`
}

func (p *NayatelProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nayatel"
	resp.Version = p.version
}

func (p *NayatelProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Nayatel provider is a community-maintained, unofficial provider for interacting with Nayatel Cloud resources.",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				MarkdownDescription: "Nayatel Cloud username. Can also be set via `NAYATEL_USERNAME` environment variable.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Nayatel Cloud password. Can also be set via `NAYATEL_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Nayatel Cloud JWT token. Can also be set via `NAYATEL_TOKEN` environment variable. If provided, username/password are not required.",
				Optional:            true,
				Sensitive:           true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Default project ID. Can also be set via `NAYATEL_PROJECT_ID` environment variable.",
				Optional:            true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Nayatel Cloud API base URL. Defaults to `https://cloud.nayatel.com/api`.",
				Optional:            true,
			},
		},
	}
}

func (p *NayatelProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Nayatel client")

	var config NayatelProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get values from config or environment
	username := getConfigOrEnv(config.Username, "NAYATEL_USERNAME")
	password := getConfigOrEnv(config.Password, "NAYATEL_PASSWORD")
	token := getConfigOrEnv(config.Token, "NAYATEL_TOKEN")
	projectID := getConfigOrEnv(config.ProjectID, "NAYATEL_PROJECT_ID")
	baseURL := getConfigOrEnv(config.BaseURL, "NAYATEL_BASE_URL")

	// Validate configuration
	if token == "" && (username == "" || password == "") {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing Nayatel API Credentials",
			"The provider requires either a token or username/password combination. "+
				"Set the credentials in the provider configuration or use environment variables "+
				"(NAYATEL_TOKEN or NAYATEL_USERNAME and NAYATEL_PASSWORD).",
		)
		return
	}

	if username == "" && token != "" {
		// Try to extract username from token (JWT payload contains username)
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing Username",
			"Username is required even when using a token. "+
				"Set the username in the provider configuration or use NAYATEL_USERNAME environment variable.",
		)
		return
	}

	// Build client options
	var opts []client.ClientOption
	if baseURL != "" {
		opts = append(opts, client.WithBaseURL(baseURL))
	}
	if projectID != "" {
		opts = append(opts, client.WithProjectID(projectID))
	}

	// Create client
	var nayatelClient *client.Client
	var err error

	if token != "" {
		// Use provided token
		tflog.Debug(ctx, "Using provided JWT token for authentication")
		nayatelClient = client.NewClient(username, token, opts...)
	} else {
		// Login with username/password
		tflog.Debug(ctx, "Authenticating with username/password")
		nayatelClient, err = client.NewClientWithLogin(ctx, username, password, opts...)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Nayatel API Client",
				"An unexpected error occurred when creating the Nayatel API client. "+
					"Error: "+err.Error(),
			)
			return
		}
	}

	tflog.Info(ctx, "Configured Nayatel client", map[string]any{"username": username})

	// Make the client available to resources and data sources
	resp.DataSourceData = nayatelClient
	resp.ResourceData = nayatelClient
}

func (p *NayatelProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewInstanceResource,
		NewNetworkResource,
		NewRouterResource,
		NewFloatingIPResource,
		NewFloatingIPAssociationResource,
		NewSecurityGroupResource,
		NewSecurityGroupAttachmentResource,
		NewVolumeResource,
		NewVolumeAttachmentResource,
		NewSSHKeyResource,
	}
}

func (p *NayatelProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewImagesDataSource,
		NewFlavorsDataSource,
		NewSSHKeysDataSource,
		NewNetworksDataSource,
		NewSecurityGroupsDataSource,
		NewRoutersDataSource,
		NewFloatingIPsDataSource,
		NewVolumesDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NayatelProvider{
			version: version,
		}
	}
}

// getConfigOrEnv returns the config value if set, otherwise the environment variable value.
func getConfigOrEnv(configValue types.String, envVar string) string {
	if !configValue.IsNull() && !configValue.IsUnknown() {
		return configValue.ValueString()
	}
	return os.Getenv(envVar)
}
