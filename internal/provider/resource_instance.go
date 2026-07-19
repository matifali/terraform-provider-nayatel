// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

var _ resource.Resource = &InstanceResource{}
var _ resource.ResourceWithImportState = &InstanceResource{}
var _ resource.ResourceWithModifyPlan = &InstanceResource{}

func NewInstanceResource() resource.Resource {
	return &InstanceResource{}
}

type InstanceResource struct {
	resourceWithClient
}

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

	DeleteRootVolumeOnDestroy types.Bool `tfsdk:"delete_root_volume_on_destroy"`
}

func (r *InstanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance"
}

func (r *InstanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a Nayatel Cloud instance (virtual machine).

~> After import, ensure your configuration matches the instance's actual settings (` + "`cpu`, `ram`, `disk`, `image_id`, `network_id`, `ssh_fingerprint`" + `) to avoid unexpected replacements.

!> Creating an instance incurs charges on your Nayatel Cloud account. The provider previews the cost and verifies your balance before provisioning, and aborts if either check fails.`,

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
			"delete_root_volume_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Whether to delete the instance's root volume when the instance is destroyed. Defaults to `true`. A kept root volume keeps billing and the Nayatel portal has no UI to delete it, so only set this to `false` if you intend to manage the volume yourself via the API.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data InstanceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating instance", map[string]any{"name": data.Name.ValueString()})

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

	_, err := r.client.Instances.SafeCreate(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create instance: %s", err))
		return
	}

	// The create response has no instance ID; give the list endpoint a moment
	// to show the new instance, then find it by name.
	tflog.Debug(ctx, "Waiting for instance to be created")
	time.Sleep(5 * time.Second)

	instance, err := r.client.Instances.FindByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find created instance: %s", err))
		return
	}
	if instance == nil {
		resp.Diagnostics.AddError("Client Error", "Instance not found after creation")
		return
	}

	// Persist the instance as soon as it's confirmed created so a later
	// WaitForStatus failure doesn't leave an already-billed instance
	// untracked by Terraform.
	data.ID = types.StringValue(instance.GetID())
	data.Status = types.StringValue(string(instance.GetStatus()))
	data.Description = types.StringValue(createReq.Description)

	// monthly_cost comes out of the plan as unknown (ModifyPlan only warns
	// with an estimate, it never commits a plan value - see
	// applyCostPreview), so it must be resolved to a concrete number here:
	// State, unlike Plan, can never contain unknown values.
	data.MonthlyCost = types.Float64Null()
	if previewResp, err := r.client.Instances.Preview(ctx, createReq); err == nil {
		if cost := client.ExtractCostFromPreview(previewResp); cost > 0 {
			data.MonthlyCost = types.Float64Value(cost)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Waiting for instance to become active")
	instance, err = r.client.Instances.WaitForStatus(ctx, instance.GetID(), client.InstanceStatusActive, 10*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Instance did not become active: %s", err))
		return
	}

	data.Status = types.StringValue(string(instance.GetStatus()))

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

	data.Name = types.StringValue(instance.GetName())
	data.Status = types.StringValue(string(instance.GetStatus()))

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

	// monthly_cost doesn't change after creation; keep the state value.
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

	// All mutable attributes use RequiresReplace, so there is nothing to
	// update in place.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data InstanceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Null/unknown means state written by a provider version that predates
	// the attribute; those destroys always deleted the root volume.
	deleteRootVolume := true
	if !data.DeleteRootVolumeOnDestroy.IsNull() && !data.DeleteRootVolumeOnDestroy.IsUnknown() {
		deleteRootVolume = data.DeleteRootVolumeOnDestroy.ValueBool()
	}

	// The root volume must be identified before the instance is deleted:
	// afterwards it detaches and is indistinguishable from any other loose
	// volume. Managed data volumes are already detached at this point
	// (their nayatel_volume_attachment depends on the instance, so
	// Terraform destroys it first), leaving the root volume as the only
	// bootable volume still attached.
	var rootVolumeID string
	if deleteRootVolume {
		rootVolumeID = r.findAttachedRootVolumeID(ctx, data.Name.ValueString())
	}

	tflog.Debug(ctx, "Stopping instance before deletion", map[string]any{"id": data.ID.ValueString()})

	// Stop instance first. Delete itself blocks server-side until the
	// instance actually stops, so no client-side wait is needed here
	// (confirmed live: Delete succeeds immediately after Stop returns).
	_, err := r.client.Instances.Stop(ctx, data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		tflog.Warn(ctx, "Failed to stop instance", map[string]any{"error": err.Error()})
	}

	tflog.Debug(ctx, "Deleting instance", map[string]any{"id": data.ID.ValueString()})

	apiResp, err := r.client.Instances.DeleteWithOptions(ctx, data.ID.ValueString(), deleteRootVolume)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete instance: %s", err))
		return
	}
	if apiResp != nil && !apiResp.Status && apiResp.Message != "" {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete instance: %s", apiResp.Message))
		return
	}

	if deleteRootVolume && rootVolumeID != "" {
		r.ensureRootVolumeDeleted(ctx, rootVolumeID, &resp.Diagnostics)
	}

	tflog.Trace(ctx, "Deleted instance", map[string]any{"id": data.ID.ValueString()})
}

// findAttachedRootVolumeID returns the ID of the bootable volume attached to
// the named instance, or "" if it can't be identified unambiguously. Lookup
// failures are logged, not fatal: the volume ID is only needed to verify the
// API's own root-volume deletion afterwards.
func (r *InstanceResource) findAttachedRootVolumeID(ctx context.Context, instanceName string) string {
	volumes, err := r.client.Volumes.List(ctx)
	if err != nil {
		tflog.Warn(ctx, "Unable to list volumes to identify root volume; skipping post-delete verification", map[string]any{"error": err.Error()})
		return ""
	}

	var matches []string
	for _, v := range volumes {
		if v.Bootable == "true" && v.GetAttachedInstanceID() == instanceName {
			matches = append(matches, v.ID)
		}
	}

	if len(matches) != 1 {
		if len(matches) > 1 {
			tflog.Warn(ctx, "Multiple bootable volumes attached to instance; skipping post-delete root volume verification", map[string]any{"volume_ids": matches})
		}
		return ""
	}

	return matches[0]
}

// ensureRootVolumeDeleted verifies the root volume is gone after an instance
// delete with delete_root_volume=true. The API has been observed to accept
// the flag yet leave the volume behind (confirmed live 2026-07-19: detached,
// still billed, and the portal has no UI to delete a volume), so if the
// volume settles as a detached leftover it is deleted directly here.
func (r *InstanceResource) ensureRootVolumeDeleted(ctx context.Context, volumeID string, diags *diag.Diagnostics) {
	const (
		pollInterval = 5 * time.Second
		pollTimeout  = 2 * time.Minute
	)

	leakWarning := func(detail string) {
		diags.AddWarning(
			"Root Volume May Be Leaked",
			fmt.Sprintf("The instance was deleted, but its root volume %s could not be confirmed deleted (%s). "+
				"The Nayatel portal has no UI to delete a volume, so if it still exists it keeps billing until removed via the API.", volumeID, detail),
		)
	}

	deadline := time.Now().Add(pollTimeout)
	fallbackTried := false
	for {
		volume, err := r.client.Volumes.Get(ctx, volumeID)
		if err != nil {
			leakWarning(fmt.Sprintf("checking it failed: %s", err))
			return
		}
		if volume == nil {
			tflog.Trace(ctx, "Root volume deleted", map[string]any{"volume_id": volumeID})
			return
		}

		// "available" means the instance is gone and the volume is just
		// sitting there detached - the API is not going to delete it
		// anymore, so do it directly. Errors here are non-fatal; the API
		// may still be tearing the volume down, and the poll below settles
		// the outcome either way.
		if !fallbackTried && volume.Status == "available" {
			fallbackTried = true
			tflog.Warn(ctx, "Root volume survived instance deletion; deleting it directly", map[string]any{"volume_id": volumeID})
			if _, err := r.client.Volumes.Delete(ctx, volumeID); err != nil {
				tflog.Warn(ctx, "Direct root volume delete failed", map[string]any{"volume_id": volumeID, "error": err.Error()})
			} else {
				// Volume deletes complete synchronously (confirmed live
				// 2026-07-19), so re-check without waiting a poll interval.
				continue
			}
		}

		if time.Now().After(deadline) {
			leakWarning(fmt.Sprintf("still present with status %q after %s", volume.Status, pollTimeout))
			return
		}

		select {
		case <-ctx.Done():
			leakWarning(fmt.Sprintf("verification interrupted: %s", ctx.Err()))
			return
		case <-time.After(pollInterval):
		}
	}
}

func (r *InstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *InstanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	applyCostPreview(ctx, r.client, req, resp,
		func(m *InstanceResourceModel) types.String { return m.ID },
		func(ctx context.Context, plan *InstanceResourceModel) (map[string]interface{}, error) {
			previewReq := &client.InstanceCreateRequest{
				CPU:  int(plan.CPU.ValueInt64()),
				RAM:  int(plan.RAM.ValueInt64()),
				Disk: int(plan.Disk.ValueInt64()),
			}
			return r.client.Instances.Preview(ctx, previewReq)
		},
		"instance",
	)
}
