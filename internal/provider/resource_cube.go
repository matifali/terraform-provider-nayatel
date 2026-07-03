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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CubeResource{}
var _ resource.ResourceWithImportState = &CubeResource{}

func NewCubeResource() resource.Resource {
	return &CubeResource{}
}

// CubeResource defines the resource implementation.
type CubeResource struct {
	client *client.Client
}

// CubeResourceModel describes the resource data model.
type CubeResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	ImageName    types.String `tfsdk:"image_name"`
	ImageVersion types.String `tfsdk:"image_version"`
	CPU          types.Int64  `tfsdk:"cpu"`
	RAM          types.Int64  `tfsdk:"ram"`
	Storage      types.Int64  `tfsdk:"storage"`
	FloatingIPs  types.Int64  `tfsdk:"floating_ips"`
	SSHPublicKey types.String `tfsdk:"ssh_public_key"`
	ProjectID    types.String `tfsdk:"project_id"`
	Status       types.String `tfsdk:"status"`
	PublicIP     types.String `tfsdk:"public_ip"`
}

func (r *CubeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cube"
}

func (r *CubeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a Nayatel Cube — a managed container (LXD-backed), distinct from IaaS virtual machines.

~> Cubes support is **experimental and a work in progress**. The underlying Nayatel API is still evolving, so this resource may change in backwards-incompatible ways or be temporarily unavailable between provider releases. Pin the provider version and use with caution.

-> The CPU/RAM pair must be one of the combinations offered by Nayatel (e.g. 2/2, 2/4, 4/8, 4/16). The provider validates the pair against the live catalog before creating and lists the allowed combinations on mismatch. Root disk size is independent of the pair.

~> All attributes force replacement. Changing any of them destroys the cube — including its data — and provisions a new one.

!> Creating a cube incurs charges on your Nayatel Cloud account. The provider previews the cost and verifies your balance before provisioning, and aborts if either check fails.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cube instance identifier (`{name}-{username}`)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the cube. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the cube. Changing this forces a new resource.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_name": schema.StringAttribute{
				MarkdownDescription: "OS image name. Defaults to `ubuntu`. Changing this forces a new resource.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ubuntu"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_version": schema.StringAttribute{
				MarkdownDescription: "OS image version (e.g. `22.04`, `24.04`). Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cpu": schema.Int64Attribute{
				MarkdownDescription: "Number of CPU cores. Must form an offered combination with `ram`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"ram": schema.Int64Attribute{
				MarkdownDescription: "RAM in GB. Must form an offered combination with `cpu`. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"storage": schema.Int64Attribute{
				MarkdownDescription: "Root disk size in GB. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"floating_ips": schema.Int64Attribute{
				MarkdownDescription: "Number of floating IPs. Defaults to 1. Changing this forces a new resource.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"ssh_public_key": schema.StringAttribute{
				MarkdownDescription: "SSH public key for authentication. Changing this forces a new resource.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Cube project identifier (e.g. `cubes-prj-<user>-<n>`)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the cube (Running, Stopped, Error)",
				Computed:            true,
			},
			"public_ip": schema.StringAttribute{
				MarkdownDescription: "Public IP address of the cube",
				Computed:            true,
			},
		},
	}
}

func (r *CubeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CubeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CubeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate CPU/RAM against the offered combinations before spending money
	if err := r.client.Cubes.ValidateCombination(ctx, int(data.CPU.ValueInt64()), int(data.RAM.ValueInt64())); err != nil {
		resp.Diagnostics.AddError("Invalid CPU/RAM Combination", err.Error())
		return
	}

	createReq := &client.CubeCreateRequest{
		Name:         data.Name.ValueString(),
		Description:  data.Description.ValueString(),
		ImageName:    data.ImageName.ValueString(),
		ImageVersion: data.ImageVersion.ValueString(),
		CPU:          int(data.CPU.ValueInt64()),
		RAM:          int(data.RAM.ValueInt64()),
		Storage:      int(data.Storage.ValueInt64()),
		FloatingIPs:  int(data.FloatingIPs.ValueInt64()),
		SSHPublicKey: data.SSHPublicKey.ValueString(),
	}

	tflog.Debug(ctx, "Creating cube", map[string]any{"name": createReq.Name})

	// SafeCreate does preview check, balance verification, and creation with
	// retries - all with safety checks to avoid unwanted charges
	if err := r.client.Cubes.SafeCreate(ctx, createReq); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create cube: %s", err))
		return
	}

	// Find the cube project (created alongside the first cube)
	tflog.Debug(ctx, "Waiting for cube project")
	project, err := r.waitForProject(ctx, 2*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find cube project after creation: %s", err))
		return
	}

	instanceName := createReq.InstanceName(r.client.Username)

	// Wait for the cube to be Running. The instance endpoint returns 403
	// while provisioning, which WaitForStatus tolerates.
	tflog.Debug(ctx, "Waiting for cube to become running", map[string]any{"instance": instanceName})
	cube, err := r.client.Cubes.WaitForStatus(ctx, project.Name, instanceName, "Running", 10*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Cube did not become running: %s", err))
		return
	}

	data.ID = types.StringValue(instanceName)
	data.ProjectID = types.StringValue(project.Name)
	data.Status = types.StringValue(cube.Status)
	if publicIP := cube.GetPublicIP(); publicIP != "" {
		data.PublicIP = types.StringValue(publicIP)
	} else {
		data.PublicIP = types.StringNull()
	}

	tflog.Trace(ctx, "Created cube", map[string]any{"id": instanceName})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CubeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CubeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		// After import only the ID is known; resolve the project first.
		project, err := r.client.Cubes.GetProject(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cube project: %s", err))
			return
		}
		if project == nil {
			resp.State.RemoveResource(ctx)
			return
		}
		projectID = project.Name
		data.ProjectID = types.StringValue(projectID)
	}

	cube, err := r.client.Cubes.FindByName(ctx, projectID, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cube: %s", err))
		return
	}
	if cube == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Status = types.StringValue(cube.Status)
	if cpu := cube.GetCPU(); cpu > 0 {
		data.CPU = types.Int64Value(int64(cpu))
	}
	if ram := cube.GetMemoryGB(); ram > 0 {
		data.RAM = types.Int64Value(int64(ram))
	}
	if disk := cube.GetDiskGB(); disk > 0 {
		data.Storage = types.Int64Value(int64(disk))
	}
	if publicIP := cube.GetPublicIP(); publicIP != "" {
		data.PublicIP = types.StringValue(publicIP)
	} else {
		data.PublicIP = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CubeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CubeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// All cube attributes require replacement; nothing updates in place.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CubeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CubeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting cube", map[string]any{"id": data.ID.ValueString()})

	err := r.client.Cubes.Delete(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete cube: %s", err))
		return
	}

	tflog.Trace(ctx, "Deleted cube", map[string]any{"id": data.ID.ValueString()})
}

func (r *CubeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitForProject polls for the user's cube project, which is created
// asynchronously alongside the first cube.
func (r *CubeResource) waitForProject(ctx context.Context, timeout time.Duration) (*client.CubeProject, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for cube project")
		case <-ticker.C:
			project, err := r.client.Cubes.GetProject(ctx)
			if err != nil {
				continue
			}
			if project != nil {
				return project, nil
			}
		}
	}
}
