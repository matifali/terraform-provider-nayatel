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
	client *client.Client
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
				MarkdownDescription: "Name of the volume",
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

func (r *VolumeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	volume, err := r.client.Volumes.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create volume: %s", err))
		return
	}

	data.ID = types.StringValue(volume.ID)
	data.Name = types.StringValue(volume.Name)
	data.Status = types.StringValue(volume.Status)
	data.Bootable = types.BoolValue(volume.IsBootable())

	if volume.VolumeType != "" {
		data.VolumeType = types.StringValue(volume.VolumeType)
	}

	// Wait for volume to become available
	if volume.Status != "available" && volume.Status != "in-use" {
		tflog.Debug(ctx, "Waiting for volume to become available")
		volume, err = r.client.Volumes.WaitForStatus(ctx, volume.ID, "available", 5*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning("Volume Status", fmt.Sprintf("Volume created but not yet available: %s", err))
		} else {
			data.Status = types.StringValue(volume.Status)
		}
	}

	if volume.IsAttached() {
		data.InstanceID = types.StringValue(volume.GetAttachedInstanceID())
	} else {
		data.InstanceID = types.StringNull()
	}

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
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
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

	if volume.IsAttached() {
		data.InstanceID = types.StringValue(volume.GetAttachedInstanceID())
	} else {
		data.InstanceID = types.StringNull()
	}

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

		_, err := r.client.Volumes.Extend(ctx, data.ID.ValueString(), int(data.Size.ValueInt64()))
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
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read volume: %s", err))
		return
	}

	if volume != nil && volume.IsAttached() {
		tflog.Debug(ctx, "Detaching volume before deletion", map[string]any{
			"volume_id":   data.ID.ValueString(),
			"instance_id": volume.GetAttachedInstanceID(),
		})
		_, err := r.client.Volumes.Detach(ctx, data.ID.ValueString(), volume.GetAttachedInstanceID())
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
