// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

var _ resource.Resource = &SecurityGroupAttachmentResource{}
var _ resource.ResourceWithImportState = &SecurityGroupAttachmentResource{}

func NewSecurityGroupAttachmentResource() resource.Resource {
	return &SecurityGroupAttachmentResource{}
}

type SecurityGroupAttachmentResource struct {
	client *client.Client
}

type SecurityGroupAttachmentResourceModel struct {
	ID                types.String `tfsdk:"id"`
	InstanceID        types.String `tfsdk:"instance_id"`
	SecurityGroupName types.String `tfsdk:"security_group_name"`
}

func (r *SecurityGroupAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group_attachment"
}

func (r *SecurityGroupAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a security group to an instance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Attachment identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"instance_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the instance",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"security_group_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the security group to attach",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *SecurityGroupAttachmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *SecurityGroupAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SecurityGroupAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Look up the actual security group name (API may have added suffix)
	configuredName := data.SecurityGroupName.ValueString()
	sg, err := r.client.SecurityGroups.FindByName(ctx, configuredName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find security group: %s", err))
		return
	}
	if sg == nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Security group '%s' not found", configuredName))
		return
	}

	// Use the actual API name (may have suffix like "terraform-ssh-925")
	actualName := sg.Name

	tflog.Debug(ctx, "Attaching security group", map[string]any{
		"instance_id":     data.InstanceID.ValueString(),
		"configured_name": configuredName,
		"actual_name":     actualName,
	})

	_, err = r.client.SecurityGroups.AddToInstance(ctx, data.InstanceID.ValueString(), actualName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to attach security group: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", data.InstanceID.ValueString(), configuredName))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecurityGroupAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SecurityGroupAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sgs, err := r.client.SecurityGroups.ListForInstance(ctx, data.InstanceID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read security groups: %s", err))
		return
	}

	// Check if the security group is still attached (using prefix match for API suffix)
	configuredName := data.SecurityGroupName.ValueString()
	found := false
	for _, sg := range sgs {
		if sg.Name == configuredName || strings.HasPrefix(sg.Name, configuredName+"-") {
			found = true
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecurityGroupAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SecurityGroupAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecurityGroupAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SecurityGroupAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Look up the actual security group name (API may have added suffix)
	configuredName := data.SecurityGroupName.ValueString()
	sg, err := r.client.SecurityGroups.FindByName(ctx, configuredName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find security group: %s", err))
		return
	}
	if sg == nil {
		// Security group doesn't exist, nothing to remove
		return
	}

	actualName := sg.Name

	tflog.Debug(ctx, "Removing security group", map[string]any{
		"instance_id":     data.InstanceID.ValueString(),
		"configured_name": configuredName,
		"actual_name":     actualName,
	})

	_, err = r.client.SecurityGroups.RemoveFromInstance(ctx, data.InstanceID.ValueString(), actualName)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove security group: %s", err))
		return
	}
}

func (r *SecurityGroupAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: instance_id:security_group_name
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in format: instance_id:security_group_name")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("security_group_name"), parts[1])...)
}
