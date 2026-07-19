// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

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

// snapshotIDs captures the IDs present in items, for later diffing against
// a post-create list to find newly appeared resources.
func snapshotIDs[T any](items []T, getID func(T) string) map[string]struct{} {
	existing := make(map[string]struct{}, len(items))
	for _, item := range items {
		existing[getID(item)] = struct{}{}
	}
	return existing
}

// identifyCreated finds a resource created after existing was captured by
// snapshotIDs. Several Nayatel create APIs return only a status message, not
// the created object, so the new resource must be found by diffing a
// subsequent list; that list can also lag briefly after create, so this
// retries. If several new items appear at once (concurrent creates), it
// prefers one named wantName among them, else the most recently listed one.
// Returns nil, nil if no new item ever appears.
func identifyCreated[T any](ctx context.Context, existing map[string]struct{}, wantName string, list func(context.Context) ([]T, error), getID, getName func(T) string) (*T, error) {
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}

		items, err := list(ctx)
		if err != nil {
			return nil, err
		}

		var created []*T
		for i := range items {
			if _, ok := existing[getID(items[i])]; !ok {
				created = append(created, &items[i])
			}
		}

		if len(created) == 1 {
			return created[0], nil
		}
		if len(created) > 1 {
			for _, cand := range created {
				if getName(*cand) == wantName {
					return cand, nil
				}
			}
			return created[len(created)-1], nil
		}
	}

	return nil, nil
}

// applyCostPreview implements the ModifyPlan cost-preview logic shared by
// every billable resource: skip if destroying, no client, already created,
// or already planned (Terraform may re-run ModifyPlan during apply as
// unknown values resolve; a time-sensitive prorated preview must not change
// an already-planned cost and fail the apply) — otherwise call preview and
// set monthly_cost on the plan. getID/getMonthlyCost/setMonthlyCost let this
// work across each resource's distinct Model type without reflection.
func applyCostPreview[T any](
	ctx context.Context,
	c *client.Client,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
	getID func(*T) types.String,
	getMonthlyCost func(*T) types.Float64,
	setMonthlyCost func(*T, types.Float64),
	preview func(context.Context, *T) (map[string]interface{}, error),
	logName string,
) {
	if req.Plan.Raw.IsNull() || c == nil {
		return
	}

	var plan T
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state T
	req.State.Get(ctx, &state)
	if !getID(&state).IsNull() {
		return
	}

	if cost := getMonthlyCost(&plan); !cost.IsNull() && !cost.IsUnknown() {
		return
	}

	previewResp, err := preview(ctx, &plan)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("Unable to get %s cost preview during plan", logName), map[string]any{"error": err.Error()})
		return
	}

	if previewResp == nil {
		return
	}

	cost := client.ExtractCostFromPreview(previewResp)
	if cost > 0 {
		setMonthlyCost(&plan, types.Float64Value(cost))
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
		tflog.Info(ctx, fmt.Sprintf("%s estimated monthly cost", logName), map[string]any{"cost_rs": cost})
	}
}
