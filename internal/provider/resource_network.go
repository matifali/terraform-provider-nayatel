// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

var _ resource.Resource = &NetworkResource{}
var _ resource.ResourceWithImportState = &NetworkResource{}
var _ resource.ResourceWithModifyPlan = &NetworkResource{}

func NewNetworkResource() resource.Resource {
	return &NetworkResource{}
}

type NetworkResource struct {
	resourceWithClient
}

type NetworkResourceModel struct {
	ID             types.String  `tfsdk:"id"`
	Name           types.String  `tfsdk:"name"`
	Status         types.String  `tfsdk:"status"`
	BandwidthLimit types.Int64   `tfsdk:"bandwidth_limit"`
	SubnetID       types.String  `tfsdk:"subnet_id"`
	SubnetCIDR     types.String  `tfsdk:"subnet_cidr"`
	MonthlyCost    types.Float64 `tfsdk:"monthly_cost"`
}

func (r *NetworkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *NetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nayatel Cloud network.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Network identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the network",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Network status",
			},
			"bandwidth_limit": schema.Int64Attribute{
				MarkdownDescription: "Bandwidth limit (1 = 25-250 Mbps). Default is 1. Changing this forces a new network.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"subnet_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the created subnet",
			},
			"subnet_cidr": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "CIDR of the created subnet",
			},
			"monthly_cost": schema.Float64Attribute{
				Computed:            true,
				MarkdownDescription: "Estimated monthly cost in Rs. for the current billing cycle",
			},
		},
	}
}

func (r *NetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NetworkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating network")

	createReq := &client.NetworkCreateRequest{
		BandwidthLimit: int(data.BandwidthLimit.ValueInt64()),
	}

	// Snapshot existing networks first: the create API returns only a
	// status message, so the new network is identified by diffing the
	// list. Taking the list's last entry would misidentify the created
	// network if another network is created concurrently or the API
	// doesn't return items in creation order.
	before, err := r.client.Networks.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list networks before creation: %s", err))
		return
	}
	existing := snapshotIDs(before, func(n client.Network) string { return n.ID })

	// SafeCreate does preview check, balance verification (with retry for 0 balance glitch),
	// and creation with retries - all with safety checks to avoid unwanted charges
	_, err = r.client.Networks.SafeCreate(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create network: %s", err))
		return
	}

	network, err := identifyCreated(ctx, existing, "", r.client.Networks.List,
		func(n client.Network) string { return n.ID },
		func(n client.Network) string { return n.Name },
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list networks after creation: %s", err))
		return
	}
	if network == nil {
		resp.Diagnostics.AddError("Client Error", "Unable to identify the created network: no new network appeared in the network list")
		return
	}

	data.ID = types.StringValue(network.ID)
	data.Name = types.StringValue(network.Name)
	data.Status = types.StringValue(network.Status)

	if network.SubnetID != "" {
		data.SubnetID = types.StringValue(network.SubnetID)
	} else {
		data.SubnetID = types.StringValue("")
	}

	if network.SubnetCIDR != "" {
		data.SubnetCIDR = types.StringValue(network.SubnetCIDR)
	} else {
		data.SubnetCIDR = types.StringValue("")
	}

	// monthly_cost comes out of the plan as unknown (ModifyPlan only warns
	// with an estimate, it never commits a plan value - see
	// applyCostPreview), so it must be resolved to a concrete number here:
	// State, unlike Plan, can never contain unknown values.
	data.MonthlyCost = types.Float64Null()
	if previewResp, err := r.client.Networks.Preview(ctx, createReq); err == nil {
		if cost := client.ExtractCostFromPreview(previewResp); cost > 0 {
			data.MonthlyCost = types.Float64Value(cost)
		}
	}

	tflog.Trace(ctx, "Created network", map[string]any{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NetworkResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	network, err := r.client.Networks.FindByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read network: %s", err))
		return
	}

	if network == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Name = types.StringValue(network.Name)
	data.Status = types.StringValue(network.Status)

	if network.SubnetID != "" {
		data.SubnetID = types.StringValue(network.SubnetID)
	}
	if network.SubnetCIDR != "" {
		data.SubnetCIDR = types.StringValue(network.SubnetCIDR)
	}

	// Preserve monthly_cost from state
	if data.MonthlyCost.IsNull() || data.MonthlyCost.IsUnknown() {
		data.MonthlyCost = types.Float64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NetworkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Networks cannot be updated in-place, but we need to refresh computed values
	// Read current state to get the ID
	var state NetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Keep the ID from state
	data.ID = state.ID

	// Fetch the network from API to get actual computed values
	network, err := r.client.Networks.FindByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read network: %s", err))
		return
	}

	if network != nil {
		data.Name = types.StringValue(network.Name)
		data.Status = types.StringValue(network.Status)
		if network.SubnetID != "" {
			data.SubnetID = types.StringValue(network.SubnetID)
		}
		if network.SubnetCIDR != "" {
			data.SubnetCIDR = types.StringValue(network.SubnetCIDR)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NetworkResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting network", map[string]any{"id": data.ID.ValueString()})

	_, err := r.client.Networks.Delete(ctx, data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete network: %s", err))
		return
	}

	tflog.Trace(ctx, "Deleted network", map[string]any{"id": data.ID.ValueString()})
}

func (r *NetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *NetworkResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	applyCostPreview(ctx, r.client, req, resp,
		func(m *NetworkResourceModel) types.String { return m.ID },
		func(ctx context.Context, plan *NetworkResourceModel) (map[string]interface{}, error) {
			previewReq := &client.NetworkCreateRequest{
				BandwidthLimit: int(plan.BandwidthLimit.ValueInt64()),
			}
			return r.client.Networks.Preview(ctx, previewReq)
		},
		"network",
	)
}
