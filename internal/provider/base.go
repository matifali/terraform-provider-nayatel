// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

// withClient holds the shared API client and implements the common
// ProviderData handling for both resources and data sources.
type withClient struct {
	client *client.Client
}

func (w *withClient) configure(providerData any, kind string, diags *diag.Diagnostics) {
	if providerData == nil {
		return
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			fmt.Sprintf("Unexpected %s Configure Type", kind),
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return
	}
	w.client = c
}

// resourceWithClient is embedded by every resource to provide Configure.
type resourceWithClient struct{ withClient }

func (w *resourceWithClient) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	w.configure(req.ProviderData, "Resource", &resp.Diagnostics)
}

// datasourceWithClient is embedded by every data source to provide Configure.
type datasourceWithClient struct{ withClient }

func (w *datasourceWithClient) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	w.configure(req.ProviderData, "Data Source", &resp.Diagnostics)
}
