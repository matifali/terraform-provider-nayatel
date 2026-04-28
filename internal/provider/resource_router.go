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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

var _ resource.Resource = &RouterResource{}
var _ resource.ResourceWithImportState = &RouterResource{}

func NewRouterResource() resource.Resource {
	return &RouterResource{}
}

type RouterResource struct {
	client *client.Client
}

type RouterResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	SubnetID types.String `tfsdk:"subnet_id"`
	Status   types.String `tfsdk:"status"`
}

func (r *RouterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_router"
}

func (r *RouterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a Nayatel Cloud router.

The router is automatically connected to the Provider Network (external network). Use subnet_id to attach your private network subnet to the router for internet connectivity.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Router identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the router",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("default"),
			},
			"subnet_id": schema.StringAttribute{
				MarkdownDescription: "ID of the subnet to attach as interface (connects your private network to the router)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Router status",
			},
		},
	}
}

func (r *RouterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RouterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RouterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating router")

	// Get Provider Network ID automatically
	providerNetworkID, err := r.client.Routers.GetProviderNetworkID(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get Provider Network ID: %s", err))
		return
	}

	tflog.Debug(ctx, "Using Provider Network", map[string]any{"network_id": providerNetworkID})

	createReq := &client.RouterCreateRequest{
		NetworkID:  providerNetworkID,
		RouterName: data.Name.ValueString(),
	}

	_, err = r.client.Routers.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create router: %s", err))
		return
	}

	// The API only returns status/message, so fetch router list to find the created router
	routers, err := r.client.Routers.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list routers after creation: %s", err))
		return
	}

	if len(routers) == 0 {
		resp.Diagnostics.AddError("Client Error", "No routers found after creation")
		return
	}

	// Find the router by name or take the last one
	var router *client.Router
	for i := range routers {
		if routers[i].Name == data.Name.ValueString() {
			router = &routers[i]
			break
		}
	}
	if router == nil {
		// Take the last router in the list (most recent)
		router = &routers[len(routers)-1]
	}

	data.ID = types.StringValue(router.ID)
	data.Name = types.StringValue(router.Name)
	data.Status = types.StringValue(router.Status)

	// Add interface to connect private subnet
	tflog.Debug(ctx, "Attaching subnet to router", map[string]any{"subnet_id": data.SubnetID.ValueString()})
	_, err = r.client.Routers.AddInterface(ctx, data.ID.ValueString(), data.SubnetID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add router interface: %s", err))
		return
	}

	tflog.Trace(ctx, "Created router", map[string]any{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RouterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	router, err := r.client.Routers.FindByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read router: %s", err))
		return
	}
	if router == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Name = types.StringValue(router.Name)
	data.Status = types.StringValue(router.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RouterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RouterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	routerID := data.ID.ValueString()
	tflog.Debug(ctx, "Deleting router", map[string]any{"id": routerID})

	if !data.SubnetID.IsNull() && !data.SubnetID.IsUnknown() && data.SubnetID.ValueString() != "" {
		subnetID := data.SubnetID.ValueString()
		tflog.Debug(ctx, "Detaching subnet from router before deletion", map[string]any{"id": routerID, "subnet_id": subnetID})
		_, err := r.client.Routers.RemoveInterface(ctx, routerID, subnetID)
		if err != nil && !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove router interface before deleting router: %s", err))
			return
		}
	}

	if err := r.deleteRouterWithInterfaceRetry(ctx, routerID); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete router: %s", err))
		return
	}
}

func (r *RouterResource) deleteRouterWithInterfaceRetry(ctx context.Context, routerID string) error {
	var err error
	backoffs := []time.Duration{0, 2 * time.Second, 4 * time.Second}
	for attempt, backoff := range backoffs {
		if backoff > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		_, err = r.client.Routers.Delete(ctx, routerID)
		if err == nil || client.IsNotFound(err) || !routerDeleteErrorMentionsActiveInterface(err) {
			return err
		}

		tflog.Debug(ctx, "Retrying router deletion after active-interface error", map[string]any{"id": routerID, "attempt": attempt + 1, "error": err.Error()})
	}

	return err
}

func routerDeleteErrorMentionsActiveInterface(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	mentionsAttachment := strings.Contains(message, "active") ||
		strings.Contains(message, "attached") ||
		strings.Contains(message, "in use")
	return (strings.Contains(message, "interface") && (mentionsAttachment || strings.Contains(message, "port"))) ||
		(strings.Contains(message, "port") && mentionsAttachment)
}

func (r *RouterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
