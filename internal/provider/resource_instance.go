// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &InstanceResource{}
var _ resource.ResourceWithImportState = &InstanceResource{}
var _ resource.ResourceWithModifyPlan = &InstanceResource{}

func NewInstanceResource() resource.Resource {
	return &InstanceResource{}
}

// InstanceResource defines the resource implementation.
type InstanceResource struct {
	client *client.Client
}

// InstanceResourceModel describes the resource data model.
type InstanceResourceModel struct {
	ID             types.String  `tfsdk:"id"`
	Name           types.String  `tfsdk:"name"`
	Description    types.String  `tfsdk:"description"`
	ImageID        types.String  `tfsdk:"image_id"`
	CPU            types.Int64   `tfsdk:"cpu"`
	RAM            types.Int64   `tfsdk:"ram"`
	Disk           types.Int64   `tfsdk:"disk"`
	NetworkID      types.String  `tfsdk:"network_id"`
	SSHFingerprint types.String  `tfsdk:"ssh_fingerprint"`
	Status         types.String  `tfsdk:"status"`
	PublicIP       types.String  `tfsdk:"public_ip"`
	PrivateIP      types.String  `tfsdk:"private_ip"`
	MonthlyCost    types.Float64 `tfsdk:"monthly_cost"`
}

func (r *InstanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance"
}

func (r *InstanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a Nayatel Cloud instance (virtual machine).

## Import

Instances can be imported using the instance ID. The configuration must match the actual instance settings:

` + "```" + `
terraform import nayatel_instance.example <instance_id>
` + "```" + `

**Note:** After import, ensure your configuration matches the instance's actual settings (cpu, ram, disk, image_id, network_id, ssh_fingerprint) to avoid unexpected replacements.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Instance identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the instance. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the instance",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"image_id": schema.StringAttribute{
				MarkdownDescription: "ID of the OS image to use. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpu": schema.Int64Attribute{
				MarkdownDescription: "Number of vCPUs. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"ram": schema.Int64Attribute{
				MarkdownDescription: "RAM in GB. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"disk": schema.Int64Attribute{
				MarkdownDescription: "Root disk size in GB. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "ID of the network to attach the instance to. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ssh_fingerprint": schema.StringAttribute{
				MarkdownDescription: "SSH key fingerprint for authentication. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the instance (ACTIVE, BUILD, SHUTOFF, ERROR)",
				Computed:            true,
			},
			"public_ip": schema.StringAttribute{
				MarkdownDescription: "Public IP address (if floating IP attached)",
				Computed:            true,
			},
			"private_ip": schema.StringAttribute{
				MarkdownDescription: "Private IP address",
				Computed:            true,
			},
			"monthly_cost": schema.Float64Attribute{
				MarkdownDescription: "Estimated monthly cost in Rs. for the current billing cycle",
				Computed:            true,
			},
		},
	}
}

func (r *InstanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data InstanceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating instance", map[string]any{"name": data.Name.ValueString()})

	// Create instance request
	createReq := &client.InstanceCreateRequest{
		Name:           data.Name.ValueString(),
		Description:    data.Description.ValueString(),
		ImageID:        data.ImageID.ValueString(),
		CPU:            int(data.CPU.ValueInt64()),
		RAM:            int(data.RAM.ValueInt64()),
		Disk:           int(data.Disk.ValueInt64()),
		NetworkID:      data.NetworkID.ValueString(),
		SSHFingerprint: data.SSHFingerprint.ValueString(),
	}

	if createReq.Description == "" {
		createReq.Description = "Nayatel Cloud VPS"
	}

	// SafeCreate does preview check, balance verification (with retry for 0 balance glitch),
	// and creation with retries - all with safety checks to avoid unwanted charges
	_, err := r.client.Instances.SafeCreate(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create instance: %s", err))
		return
	}

	// Wait for instance to be created and find it by name
	tflog.Debug(ctx, "Waiting for instance to be created")
	time.Sleep(5 * time.Second)

	// Find instance by name
	instance, err := r.client.Instances.FindByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find created instance: %s", err))
		return
	}
	if instance == nil {
		resp.Diagnostics.AddError("Client Error", "Instance not found after creation")
		return
	}

	// Wait for instance to become active
	tflog.Debug(ctx, "Waiting for instance to become active")
	instance, err = r.client.Instances.WaitForStatus(ctx, instance.GetID(), client.InstanceStatusActive, 10*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Instance did not become active: %s", err))
		return
	}

	// Update state
	data.ID = types.StringValue(instance.GetID())
	data.Status = types.StringValue(string(instance.GetStatus()))
	data.Description = types.StringValue(createReq.Description)

	if publicIP := instance.GetPublicIP(); publicIP != "" {
		data.PublicIP = types.StringValue(publicIP)
	} else {
		data.PublicIP = types.StringNull()
	}

	if privateIP := instance.GetPrivateIP(); privateIP != "" {
		data.PrivateIP = types.StringValue(privateIP)
	} else {
		data.PrivateIP = types.StringNull()
	}

	tflog.Trace(ctx, "Created instance", map[string]any{"id": instance.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data InstanceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use FindByID which uses List - more reliable than Get during BUILD
	instance, err := r.client.Instances.FindByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance: %s", err))
		return
	}
	if instance == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update attributes that can be read from API
	data.Name = types.StringValue(instance.GetName())
	data.Status = types.StringValue(string(instance.GetStatus()))

	// Populate CPU, RAM from instance data
	if instance.CPU > 0 {
		data.CPU = types.Int64Value(int64(instance.CPU))
	}
	if instance.RAM > 0 {
		data.RAM = types.Int64Value(int64(instance.RAM))
	}

	// Note: disk, image_id, network_id, ssh_fingerprint are not returned by the API.
	// These Required attributes are preserved from state/config.
	// After import, terraform will compare against user's config values.

	if publicIP := instance.GetPublicIP(); publicIP != "" {
		data.PublicIP = types.StringValue(publicIP)
	} else {
		data.PublicIP = types.StringNull()
	}

	if privateIP := instance.GetPrivateIP(); privateIP != "" {
		data.PrivateIP = types.StringValue(privateIP)
	} else {
		data.PrivateIP = types.StringNull()
	}

	// Preserve monthly_cost from state (it doesn't change after creation)
	// If not set, leave it null
	if data.MonthlyCost.IsNull() || data.MonthlyCost.IsUnknown() {
		data.MonthlyCost = types.Float64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data InstanceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Instance updates are mostly handled by RequiresReplace
	// This is a placeholder for any in-place updates

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data InstanceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Stopping instance before deletion", map[string]any{"id": data.ID.ValueString()})

	// Stop instance first
	_, err := r.client.Instances.Stop(ctx, data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		tflog.Warn(ctx, "Failed to stop instance", map[string]any{"error": err.Error()})
	}

	// Wait a bit for instance to stop
	time.Sleep(10 * time.Second)

	tflog.Debug(ctx, "Deleting instance", map[string]any{"id": data.ID.ValueString()})

	_, err = r.client.Instances.Delete(ctx, data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete instance: %s", err))
		return
	}

	tflog.Trace(ctx, "Deleted instance", map[string]any{"id": data.ID.ValueString()})
}

func (r *InstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *InstanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip if destroying or no client configured
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}

	var plan InstanceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only calculate cost for new resources (no state ID)
	var state InstanceResourceModel
	req.State.Get(ctx, &state)
	if !state.ID.IsNull() {
		return
	}

	// Build preview request from plan values
	previewReq := &client.InstanceCreateRequest{
		CPU:  int(plan.CPU.ValueInt64()),
		RAM:  int(plan.RAM.ValueInt64()),
		Disk: int(plan.Disk.ValueInt64()),
	}

	preview, err := r.client.Instances.Preview(ctx, previewReq)
	if err != nil {
		tflog.Warn(ctx, "Unable to get cost preview during plan", map[string]any{"error": err.Error()})
		return
	}

	if preview != nil {
		var cost float64
		// Check nested data.charge (Nayatel API format)
		if data, ok := preview["data"].(map[string]interface{}); ok {
			if c, ok := data["charge"].(float64); ok {
				cost = c
			}
		}
		// Fallback to top-level fields
		if cost == 0 {
			if c, ok := preview["charge"].(float64); ok {
				cost = c
			} else if c, ok := preview["monthly_cost"].(float64); ok {
				cost = c
			}
		}
		if cost > 0 {
			plan.MonthlyCost = types.Float64Value(cost)
			resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
			tflog.Info(ctx, "Instance estimated monthly cost", map[string]any{"cost_rs": cost})
		}
	}
}
