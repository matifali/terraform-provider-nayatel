// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

var _ resource.Resource = &VolumeAttachmentResource{}
var _ resource.ResourceWithImportState = &VolumeAttachmentResource{}

func NewVolumeAttachmentResource() resource.Resource {
	return &VolumeAttachmentResource{}
}

type VolumeAttachmentResource struct {
	resourceWithClient
}

type VolumeAttachmentResourceModel struct {
	ID         types.String `tfsdk:"id"`
	VolumeID   types.String `tfsdk:"volume_id"`
	InstanceID types.String `tfsdk:"instance_id"`
	Device     types.String `tfsdk:"device"`
}

func (r *VolumeAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_volume_attachment"
}

func (r *VolumeAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a volume to an instance.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Attachment identifier (volume_id:instance_id)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"volume_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the volume to attach",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the instance to attach the volume to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"device": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The device path of the attached volume (e.g., /dev/vdb)",
			},
		},
	}
}

func (r *VolumeAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VolumeAttachmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	volumeID := data.VolumeID.ValueString()
	instanceID := data.InstanceID.ValueString()

	tflog.Debug(ctx, "Attaching volume to instance", map[string]any{
		"volume_id":   volumeID,
		"instance_id": instanceID,
	})

	_, err := r.client.Volumes.Attach(ctx, volumeID, instanceID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to attach volume: %s", err))
		return
	}

	// Wait for volume to become in-use
	volume, err := r.client.Volumes.WaitForStatus(ctx, volumeID, "in-use", 5*time.Minute)
	if err != nil {
		resp.Diagnostics.AddWarning("Volume Status", fmt.Sprintf("Volume attached but status unknown: %s", err))
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", volumeID, instanceID))

	// Get device path from attachment info. The volume API reports the
	// attached instance by name, not ID, so resolve it before comparing
	// against the instance_id we just attached to.
	if volume != nil {
		if resolvedID, err := r.client.Volumes.ResolveAttachedInstanceID(ctx, volume); err == nil && resolvedID == instanceID {
			for _, att := range volume.Attachments {
				if att.Device != "" {
					data.Device = types.StringValue(att.Device)
					break
				}
			}
		}
	}

	if data.Device.IsNull() || data.Device.IsUnknown() {
		data.Device = types.StringValue("")
	}

	tflog.Trace(ctx, "Attached volume to instance", map[string]any{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VolumeAttachmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	volumeID := data.VolumeID.ValueString()
	instanceID := data.InstanceID.ValueString()

	volume, err := r.client.Volumes.Get(ctx, volumeID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read volume: %s", err))
		return
	}

	if volume == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Volumes report the attached instance by name (attached_to), not by
	// ID, so resolve the real instance ID before comparing against state.
	attachedInstanceID, err := r.client.Volumes.ResolveAttachedInstanceID(ctx, volume)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve instance attached to volume: %s", err))
		return
	}

	if attachedInstanceID != instanceID {
		// Volume is no longer attached to this instance
		resp.State.RemoveResource(ctx)
		return
	}

	for _, att := range volume.Attachments {
		if att.Device != "" {
			data.Device = types.StringValue(att.Device)
			break
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Volume attachments cannot be updated in-place, they require replacement
	var data VolumeAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VolumeAttachmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	volumeID := data.VolumeID.ValueString()
	instanceID := data.InstanceID.ValueString()

	tflog.Debug(ctx, "Detaching volume from instance", map[string]any{
		"volume_id":   volumeID,
		"instance_id": instanceID,
	})

	_, err := r.client.Volumes.Detach(ctx, volumeID, instanceID)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to detach volume: %s", err))
		return
	}

	// Wait for volume to become available
	_, err = r.client.Volumes.WaitForStatus(ctx, volumeID, "available", 2*time.Minute)
	if err != nil {
		tflog.Warn(ctx, "Volume detached but status unknown", map[string]any{"error": err.Error()})
	}

	tflog.Trace(ctx, "Detached volume from instance", map[string]any{"id": data.ID.ValueString()})
}

func (r *VolumeAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: volume_id:instance_id
	volumeID, instanceID, found := strings.Cut(req.ID, ":")
	if !found || volumeID == "" || instanceID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format: volume_id:instance_id",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("volume_id"), volumeID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_id"), instanceID)...)
}
