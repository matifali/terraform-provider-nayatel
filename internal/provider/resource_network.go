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
	client *client.Client
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
				MarkdownDescription: "Bandwidth limit (1 = 25-250 Mbps). Default is 1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
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

func (r *NetworkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
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

	// SafeCreate does preview check, balance verification (with retry for 0 balance glitch),
	// and creation with retries - all with safety checks to avoid unwanted charges
	_, err := r.client.Networks.SafeCreate(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create network: %s", err))
		return
	}

	// The API doesn't return network details in the create response,
	// so we need to fetch the network list to find the newly created network.
	networks, err := r.client.Networks.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list networks after creation: %s", err))
		return
	}

	// Find the most recently created network (the one we just created)
	if len(networks) == 0 {
		resp.Diagnostics.AddError("Client Error", "No networks found after creation")
		return
	}

	// Take the last network in the list (most recent)
	network := networks[len(networks)-1]
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
	// Skip if destroying or no client configured
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}

	var plan NetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only calculate cost for new resources
	var state NetworkResourceModel
	req.State.Get(ctx, &state)
	if !state.ID.IsNull() {
		return
	}

	previewReq := &client.NetworkCreateRequest{
		BandwidthLimit: int(plan.BandwidthLimit.ValueInt64()),
	}

	preview, err := r.client.Networks.Preview(ctx, previewReq)
	if err != nil {
		tflog.Warn(ctx, "Unable to get network cost preview during plan", map[string]any{"error": err.Error()})
		return
	}

	if preview != nil {
		var cost float64
		// Check nested data.charges.total_amount (Nayatel network API format)
		if data, ok := preview["data"].(map[string]interface{}); ok {
			if charges, ok := data["charges"].(map[string]interface{}); ok {
				if c, ok := charges["total_amount"].(float64); ok {
					cost = c
				}
			}
		}
		if cost > 0 {
			plan.MonthlyCost = types.Float64Value(cost)
			resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
			tflog.Info(ctx, "Network estimated monthly cost", map[string]any{"cost_rs": cost})
		}
	}
}
