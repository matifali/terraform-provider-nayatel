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
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
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
				MarkdownDescription: "Nayatel Cloud username. Required with `password`; can also be set via the `NAYATEL_USERNAME` environment variable.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Nayatel Cloud password for non-interactive CSRF-protected form login. Can also be set via the `NAYATEL_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

type authValidationDiagnostic struct {
	attribute path.Path
	summary   string
	detail    string
}

func validateAuthenticationConfig(username, password string) *authValidationDiagnostic {
	missingCredentialsDetail := "The provider requires `username` and `password`. " +
		"Set the credentials in the provider configuration or use environment variables " +
		"(`NAYATEL_USERNAME` and `NAYATEL_PASSWORD`)."

	if username == "" {
		return &authValidationDiagnostic{
			attribute: path.Root("username"),
			summary:   "Missing Nayatel API Credentials",
			detail:    missingCredentialsDetail,
		}
	}

	if password == "" {
		return &authValidationDiagnostic{
			attribute: path.Root("password"),
			summary:   "Missing Nayatel API Credentials",
			detail:    missingCredentialsDetail,
		}
	}

	return nil
}

func (p *NayatelProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Nayatel client")

	var config NayatelProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Credentials must be known at plan time; silently falling back to the
	// environment would let plan and apply authenticate as different accounts.
	if config.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("username"), "Unknown Provider Configuration Value",
			"`username` depends on a value that is not known until apply. Use a static value or the `NAYATEL_USERNAME` environment variable.")
	}
	if config.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("password"), "Unknown Provider Configuration Value",
			"`password` depends on a value that is not known until apply. Use a static value or the `NAYATEL_PASSWORD` environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Get values from config or environment
	username := getConfigOrEnv(config.Username, "NAYATEL_USERNAME")
	password := getConfigOrEnv(config.Password, "NAYATEL_PASSWORD")

	// Validate configuration
	if authDiag := validateAuthenticationConfig(username, password); authDiag != nil {
		resp.Diagnostics.AddAttributeError(
			authDiag.attribute,
			authDiag.summary,
			authDiag.detail,
		)
		return
	}

	// Login with username/password
	tflog.Debug(ctx, "Authenticating with username/password")
	nayatelClient, err := client.NewClientWithLogin(ctx, username, password)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Nayatel API Client",
			"An unexpected error occurred when creating the Nayatel API client. "+
				"Error: "+err.Error(),
		)
		return
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
		NewCubeResource,
	}
}

func (p *NayatelProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewImagesDataSource,
		NewImageDataSource,
		NewSSHKeyDataSource,
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
