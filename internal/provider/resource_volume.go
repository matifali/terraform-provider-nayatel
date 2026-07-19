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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

var _ resource.Resource = &VolumeResource{}
var _ resource.ResourceWithImportState = &VolumeResource{}

func NewVolumeResource() resource.Resource {
	return &VolumeResource{}
}

type VolumeResource struct {
	resourceWithClient
}

type VolumeResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Size        types.Int64  `tfsdk:"size"`
	Status      types.String `tfsdk:"status"`
	VolumeType  types.String `tfsdk:"volume_type"`
	Bootable    types.Bool   `tfsdk:"bootable"`
	ImageID     types.String `tfsdk:"image_id"`
	SnapshotID  types.String `tfsdk:"snapshot_id"`
	InstanceID  types.String `tfsdk:"instance_id"`
}

func (r *VolumeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_volume"
}

func (r *VolumeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nayatel Cloud volume.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Volume identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Name of the volume. Changing this forces a new volume.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the volume",
			},
			"size": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Size of the volume in GB",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Volume status (available, in-use, creating, deleting, error)",
			},
			"volume_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Type of the volume (e.g., ssd, hdd)",
			},
			"bootable": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the volume is bootable",
			},
			"image_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Image ID to create a bootable volume from",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"snapshot_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Snapshot ID to create the volume from",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the instance the volume is attached to (if any)",
			},
		},
	}
}

func (r *VolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VolumeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating volume", map[string]any{"size": data.Size.ValueInt64()})

	createReq := &client.VolumeCreateRequest{
		Size: int(data.Size.ValueInt64()),
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		createReq.Name = data.Name.ValueString()
	}
	if !data.Description.IsNull() {
		createReq.Description = data.Description.ValueString()
	}
	if !data.VolumeType.IsNull() {
		createReq.VolumeType = data.VolumeType.ValueString()
	}
	if !data.ImageID.IsNull() {
		createReq.ImageID = data.ImageID.ValueString()
	}
	if !data.SnapshotID.IsNull() {
		createReq.SnapshotID = data.SnapshotID.ValueString()
	}

	// Snapshot existing volumes first: the create API returns only a status
	// message, so the new volume is identified by diffing the list.
	// Matching by name alone could adopt a pre-existing volume with the
	// same name (or the same server-assigned default name).
	before, err := r.client.Volumes.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list volumes before creation: %s", err))
		return
	}
	existing := snapshotIDs(before, func(v client.Volume) string { return v.ID })

	volume, err := r.client.Volumes.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create volume: %s", err))
		return
	}

	if volume.ID == "" {
		found, err := identifyCreated(ctx, existing, createReq.Name, r.client.Volumes.List,
			func(v client.Volume) string { return v.ID },
			func(v client.Volume) string { return v.Name },
		)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list volumes after creation: %s", err))
			return
		}
		if found == nil {
			resp.Diagnostics.AddError("Client Error", "Unable to identify the created volume: no new volume appeared in the volume list")
			return
		}
		volume = found
	}

	data.ID = types.StringValue(volume.ID)
	data.Name = types.StringValue(volume.Name)
	data.Status = types.StringValue(volume.Status)
	data.Bootable = types.BoolValue(volume.IsBootable())

	// Prefer the API's value, but if it's momentarily empty (the volume can
	// still be provisioning right after create) don't clobber a
	// user-configured value in the plan -- only fill the gap when the
	// attribute would otherwise be left Unknown (nothing configured).
	if volume.VolumeType != "" {
		data.VolumeType = types.StringValue(volume.VolumeType)
	} else if data.VolumeType.IsUnknown() {
		data.VolumeType = types.StringValue("")
	}

	// Wait for volume to become available
	if volume.Status != "available" && volume.Status != "in-use" {
		tflog.Debug(ctx, "Waiting for volume to become available")
		waited, err := r.client.Volumes.WaitForStatus(ctx, volume.ID, "available", 5*time.Minute)
		if err != nil {
			// The volume was already created (and is billing); keep the
			// pre-wait volume rather than the nil WaitForStatus result so
			// the rest of Create can still populate state from it.
			resp.Diagnostics.AddWarning("Volume Status", fmt.Sprintf("Volume created but not yet available: %s", err))
		} else {
			volume = waited
			data.Status = types.StringValue(volume.Status)
		}
	}

	data.InstanceID = resolveVolumeInstanceID(ctx, r.client, volume)

	tflog.Trace(ctx, "Created volume", map[string]any{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VolumeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	volume, err := r.client.Volumes.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read volume: %s", err))
		return
	}

	if volume == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Name = types.StringValue(volume.Name)
	data.Size = types.Int64Value(int64(volume.Size))
	data.Status = types.StringValue(volume.Status)
	data.Bootable = types.BoolValue(volume.IsBootable())

	if volume.Description != "" {
		data.Description = types.StringValue(volume.Description)
	}
	if volume.VolumeType != "" {
		data.VolumeType = types.StringValue(volume.VolumeType)
	}

	data.InstanceID = resolveVolumeInstanceID(ctx, r.client, volume)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VolumeResourceModel
	var state VolumeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if size increased (volume extension)
	if data.Size.ValueInt64() > state.Size.ValueInt64() {
		tflog.Debug(ctx, "Extending volume", map[string]any{
			"id":       data.ID.ValueString(),
			"old_size": state.Size.ValueInt64(),
			"new_size": data.Size.ValueInt64(),
		})

		addSize := int(data.Size.ValueInt64() - state.Size.ValueInt64())
		_, err := r.client.Volumes.Extend(ctx, data.ID.ValueString(), addSize)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to extend volume: %s", err))
			return
		}

		// Wait for volume to become available again
		volume, err := r.client.Volumes.WaitForStatus(ctx, data.ID.ValueString(), "available", 5*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning("Volume Status", fmt.Sprintf("Volume extended but status unknown: %s", err))
		} else {
			data.Status = types.StringValue(volume.Status)
		}
	} else if data.Size.ValueInt64() < state.Size.ValueInt64() {
		resp.Diagnostics.AddError("Invalid Operation", "Volume size cannot be decreased")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VolumeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting volume", map[string]any{"id": data.ID.ValueString()})

	// Check if volume is attached and detach it first
	volume, err := r.client.Volumes.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read volume: %s", err))
		return
	}

	if volume != nil && volume.IsAttached() {
		instanceID, err := r.client.Volumes.ResolveAttachedInstanceID(ctx, volume)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve instance attached to volume: %s", err))
			return
		}

		tflog.Debug(ctx, "Detaching volume before deletion", map[string]any{
			"volume_id":   data.ID.ValueString(),
			"instance_id": instanceID,
		})
		_, err = r.client.Volumes.Detach(ctx, data.ID.ValueString(), instanceID)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to detach volume: %s", err))
			return
		}

		// Wait for volume to become available
		_, err = r.client.Volumes.WaitForStatus(ctx, data.ID.ValueString(), "available", 2*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning("Volume Status", fmt.Sprintf("Volume detached but status unknown: %s", err))
		}
	}

	_, err = r.client.Volumes.Delete(ctx, data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete volume: %s", err))
		return
	}

	tflog.Trace(ctx, "Deleted volume", map[string]any{"id": data.ID.ValueString()})
}

func (r *VolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// resolveVolumeInstanceID returns the ID of the instance a volume is
// attached to, for the computed instance_id attribute. This is
// informational, so a resolution failure falls back to the volume's raw
// (name-valued) attached-instance field rather than failing the operation.
func resolveVolumeInstanceID(ctx context.Context, c *client.Client, volume *client.Volume) types.String {
	if !volume.IsAttached() {
		return types.StringNull()
	}

	instanceID, err := c.Volumes.ResolveAttachedInstanceID(ctx, volume)
	if err != nil || instanceID == "" {
		return types.StringValue(volume.GetAttachedInstanceID())
	}

	return types.StringValue(instanceID)
}
