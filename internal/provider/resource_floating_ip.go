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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

// =============================================================================
// nayatel_floating_ip - Allocates a floating IP (like aws_eip)
// =============================================================================

var _ resource.Resource = &FloatingIPResource{}
var _ resource.ResourceWithImportState = &FloatingIPResource{}
var _ resource.ResourceWithModifyPlan = &FloatingIPResource{}

func NewFloatingIPResource() resource.Resource {
	return &FloatingIPResource{}
}

type FloatingIPResource struct {
	resourceWithClient
}

type FloatingIPResourceModel struct {
	ID          types.String  `tfsdk:"id"`
	IPAddress   types.String  `tfsdk:"ip_address"`
	InstanceID  types.String  `tfsdk:"instance_id"`
	Status      types.String  `tfsdk:"status"`
	MonthlyCost types.Float64 `tfsdk:"monthly_cost"`
}

func (r *FloatingIPResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_floating_ip"
}

func (r *FloatingIPResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Allocates a Nayatel Cloud floating IP.

This resource allocates an actual floating IP address that can be attached to instances.
Similar to AWS's ` + "`aws_eip`" + ` resource.

~> Due to Nayatel API limitations, an ` + "`instance_id`" + ` is required to allocate
the IP. The IP will be attached to this instance initially. Use ` + "`nayatel_floating_ip_association`" + `
to move the IP to a different instance later.

## Example Usage

` + "```hcl" + `
# Allocate a floating IP (attached to instance)
resource "nayatel_floating_ip" "web" {
  instance_id = nayatel_instance.web.id
}

# Later, move it to a different instance
resource "nayatel_floating_ip_association" "web" {
  floating_ip = nayatel_floating_ip.web.ip_address
  instance_id = nayatel_instance.new_web.id
}
` + "```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Floating IP identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ip_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The allocated floating IP address",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"instance_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Instance ID to attach the floating IP to. Required to discover the allocated IP address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Floating IP status (ACTIVE when attached, DOWN when detached)",
			},
			"monthly_cost": schema.Float64Attribute{
				Computed:            true,
				MarkdownDescription: "Estimated monthly cost in Rs. for the current billing cycle",
			},
		},
	}
}

// floatingIPMonthlyCost previews the current cost of one floating IP,
// returning a null value if the preview call fails.
func floatingIPMonthlyCost(ctx context.Context, c *client.Client) types.Float64 {
	previewResp, err := c.FloatingIPs.Preview(ctx, 1)
	if err != nil {
		return types.Float64Null()
	}
	if cost := client.ExtractCostFromPreview(previewResp); cost > 0 {
		return types.Float64Value(cost)
	}
	return types.Float64Null()
}

func (r *FloatingIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FloatingIPResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	instanceID := data.InstanceID.ValueString()

	// Check if instance already has a floating IP
	instance, err := r.client.Instances.FindByID(ctx, instanceID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance: %s", err))
		return
	}
	if instance == nil {
		resp.Diagnostics.AddError("Client Error", "Instance not found")
		return
	}

	// If instance already has a public IP, use it (from previous run or manual allocation)
	existingIP := instance.GetPublicIP()
	if existingIP != "" {
		tflog.Info(ctx, "Instance already has a floating IP, using existing", map[string]any{
			"instance_id": instanceID,
			"ip_address":  existingIP,
		})
		data.ID = types.StringValue(existingIP)
		data.IPAddress = types.StringValue(existingIP)
		data.Status = types.StringValue("ACTIVE")
		data.MonthlyCost = floatingIPMonthlyCost(ctx, r.client)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	tflog.Debug(ctx, "Attaching floating IP to instance", map[string]any{"instance_id": instanceID})

	// Step 1: Try to attach using existing quota first
	_, err = r.client.FloatingIPs.Attach(ctx, instanceID)
	if err != nil {
		// If attach failed (likely no quota), allocate new quota and retry
		tflog.Debug(ctx, "Attach failed, allocating new floating IP quota", map[string]any{"error": err.Error()})

		// SafeAllocate does preview check, balance verification (with retry for 0 balance glitch),
		// and allocation with retries - all with safety checks to avoid unwanted charges
		_, allocErr := r.client.FloatingIPs.SafeAllocate(ctx, 1)
		if allocErr != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to allocate floating IP: %s", allocErr))
			return
		}

		// Retry attach after allocation
		_, err = r.client.FloatingIPs.Attach(ctx, instanceID)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to attach floating IP to instance: %s", err))
			return
		}
	}

	// Get the allocated IP address from the instance
	instance, err = r.client.Instances.FindByID(ctx, instanceID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read instance: %s", err))
		return
	}
	if instance == nil {
		resp.Diagnostics.AddError("Client Error", "Instance not found after allocating floating IP")
		return
	}

	ip := instance.GetPublicIP()
	if ip == "" {
		resp.Diagnostics.AddError("Client Error", "Floating IP allocated but not found on instance")
		return
	}

	// Persist a minimal identifying state as soon as the IP is known so a
	// later FindByIP failure doesn't leave an already-billed/attached
	// floating IP untracked by Terraform.
	//
	// monthly_cost comes out of the plan as unknown (ModifyPlan only warns
	// with an estimate, it never commits a plan value - see
	// applyCostPreview), so it must be resolved to a concrete number here,
	// before this first State.Set: State, unlike Plan, can never contain
	// unknown values.
	data.ID = types.StringValue(ip)
	data.IPAddress = types.StringValue(ip)
	data.Status = types.StringValue("ACTIVE")
	data.MonthlyCost = floatingIPMonthlyCost(ctx, r.client)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Find the floating IP details
	fip, err := r.client.FloatingIPs.FindByIP(ctx, ip)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find floating IP details: %s", err))
		return
	}
	// If the IP isn't immediately visible, keep the early-saved ID/status
	// (IP used as ID); otherwise refine them with the found details.
	if fip != nil {
		data.ID = types.StringValue(fip.ID)
		data.Status = types.StringValue(fip.Status)
	}

	tflog.Info(ctx, "Floating IP allocated", map[string]any{
		"id":         data.ID.ValueString(),
		"ip_address": ip,
		"status":     data.Status.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FloatingIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FloatingIPResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fip, err := r.client.FloatingIPs.FindByIP(ctx, data.IPAddress.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read floating IP: %s", err))
		return
	}
	if fip == nil {
		// IP not found in list - could be in DOWN state (detached but not released)
		// or actually released. Keep in state so Delete can attempt release.
		// If truly released, the Release API will just return not found which is fine.
		tflog.Warn(ctx, "Floating IP not visible in API (may be detached/DOWN state), keeping in state for cleanup", map[string]any{
			"ip_address": data.IPAddress.ValueString(),
		})
		data.Status = types.StringValue("DOWN")
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	data.Status = types.StringValue(fip.Status)
	// Preserve monthly_cost from state
	if data.MonthlyCost.IsNull() || data.MonthlyCost.IsUnknown() {
		data.MonthlyCost = types.Float64Null()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FloatingIPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FloatingIPResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FloatingIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FloatingIPResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ipAddress := data.IPAddress.ValueString()
	instanceID := data.InstanceID.ValueString()

	tflog.Info(ctx, "Releasing floating IP", map[string]any{"ip_address": ipAddress, "instance_id": instanceID})

	// Step 1: Detach the floating IP from the instance
	// The IP must be detached (DOWN state) before it can be released
	if instanceID != "" {
		tflog.Debug(ctx, "Detaching floating IP from instance", map[string]any{
			"ip_address":  ipAddress,
			"instance_id": instanceID,
		})
		_, detachErr := r.client.FloatingIPs.Detach(ctx, instanceID)
		if detachErr != nil {
			tflog.Warn(ctx, "Detach returned error (may already be detached)", map[string]any{
				"error": detachErr.Error(),
			})
		}
		// Wait for detach to complete
		time.Sleep(3 * time.Second)
	}

	// Step 2: Release the floating IP with retries
	// The IP should now be in DOWN state and releasable
	var releaseErr error
	for attempt := 1; attempt <= 5; attempt++ {
		tflog.Debug(ctx, "Attempting to release floating IP", map[string]any{
			"attempt":    attempt,
			"ip_address": ipAddress,
		})

		_, releaseErr = r.client.FloatingIPs.Release(ctx, ipAddress)
		if releaseErr == nil {
			tflog.Info(ctx, "Floating IP released successfully", map[string]any{"ip_address": ipAddress})
			return
		}

		if client.IsNotFound(releaseErr) {
			tflog.Info(ctx, "Floating IP already released or not found", map[string]any{"ip_address": ipAddress})
			return
		}

		tflog.Warn(ctx, "Release attempt failed, retrying", map[string]any{
			"attempt": attempt,
			"error":   releaseErr.Error(),
		})
		time.Sleep(time.Duration(attempt*2) * time.Second)
	}

	if releaseErr != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to release floating IP %s: %s. "+
				"You may need to manually release it via the Nayatel Cloud portal.", ipAddress, releaseErr))
		return
	}
}

func (r *FloatingIPResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by IP address
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_address"), req.ID)...)
}

func (r *FloatingIPResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	applyCostPreview(ctx, r.client, req, resp,
		func(m *FloatingIPResourceModel) types.String { return m.ID },
		func(ctx context.Context, plan *FloatingIPResourceModel) (map[string]interface{}, error) {
			return r.client.FloatingIPs.Preview(ctx, 1)
		},
		"floating IP",
	)
}

// =============================================================================
// nayatel_floating_ip_association - Attaches a floating IP to an instance
// (like aws_eip_association)
// =============================================================================

var _ resource.Resource = &FloatingIPAssociationResource{}
var _ resource.ResourceWithImportState = &FloatingIPAssociationResource{}

func NewFloatingIPAssociationResource() resource.Resource {
	return &FloatingIPAssociationResource{}
}

type FloatingIPAssociationResource struct {
	resourceWithClient
}

type FloatingIPAssociationResourceModel struct {
	ID               types.String `tfsdk:"id"`
	FloatingIP       types.String `tfsdk:"floating_ip"`
	InstanceID       types.String `tfsdk:"instance_id"`
	ReleaseOnDestroy types.Bool   `tfsdk:"release_on_destroy"`
}

func (r *FloatingIPAssociationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_floating_ip_association"
}

func (r *FloatingIPAssociationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Associates a floating IP with an instance.

Similar to AWS's ` + "`aws_eip_association`" + ` resource. Use this to attach an existing
floating IP to an instance.

!> By default, ` + "`release_on_destroy = false`" + `, which only detaches
the IP (it remains allocated and you keep paying). Set ` + "`release_on_destroy = true`" + `
to release the IP on destroy if you don't need it anymore.

## Example Usage (with nayatel_floating_ip)

` + "```hcl" + `
resource "nayatel_floating_ip" "web" {
  instance_id = nayatel_instance.bootstrap.id
}

resource "nayatel_floating_ip_association" "web" {
  floating_ip = nayatel_floating_ip.web.ip_address
  instance_id = nayatel_instance.web.id
}
` + "```" + `

## Example Usage (reattach existing IP)

` + "```hcl" + `
# Attach an existing floating IP (e.g., from portal or previous deployment)
resource "nayatel_floating_ip_association" "web" {
  floating_ip        = "101.50.85.100"
  instance_id        = nayatel_instance.web.id
  release_on_destroy = true  # Release IP when done
}
` + "```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Association identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"floating_ip": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The floating IP address to attach",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the instance to attach the floating IP to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"release_on_destroy": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether to release the floating IP when this association is destroyed. Defaults to `false` (only detaches). Set to `true` to release the IP and get a refund.",
			},
		},
	}
}

func (r *FloatingIPAssociationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FloatingIPAssociationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	floatingIP := data.FloatingIP.ValueString()
	instanceID := data.InstanceID.ValueString()

	tflog.Debug(ctx, "Attaching floating IP to instance", map[string]any{
		"floating_ip": floatingIP,
		"instance_id": instanceID,
	})

	// Check if IP is currently attached to another instance
	fip, err := r.client.FloatingIPs.FindByIP(ctx, floatingIP)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find floating IP %s: %s", floatingIP, err))
		return
	}
	if fip == nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Floating IP %s not found. Ensure it exists and is allocated.", floatingIP))
		return
	}

	// If attached to another instance, detach first
	if fip.Status == "ACTIVE" && fip.PortDetails.DeviceID != "" && fip.PortDetails.DeviceID != instanceID {
		tflog.Debug(ctx, "Detaching floating IP from current instance", map[string]any{
			"floating_ip":         floatingIP,
			"current_instance_id": fip.PortDetails.DeviceID,
		})
		_, err = r.client.FloatingIPs.Detach(ctx, fip.PortDetails.DeviceID)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to detach floating IP from current instance: %s", err))
			return
		}
	}

	// Attach to the new instance
	_, err = r.client.FloatingIPs.AttachIP(ctx, instanceID, floatingIP)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to attach floating IP: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%s:%s", instanceID, floatingIP))

	tflog.Info(ctx, "Floating IP attached", map[string]any{
		"floating_ip": floatingIP,
		"instance_id": instanceID,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FloatingIPAssociationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FloatingIPAssociationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	floatingIP := data.FloatingIP.ValueString()
	instanceID := data.InstanceID.ValueString()

	// Check if the IP is still attached to the instance
	fip, err := r.client.FloatingIPs.FindByIP(ctx, floatingIP)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read floating IP: %s", err))
		return
	}
	if fip == nil {
		// IP was released
		resp.State.RemoveResource(ctx)
		return
	}

	// Check if still attached to our instance
	if fip.PortDetails.DeviceID != instanceID {
		// IP was moved to another instance or detached
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FloatingIPAssociationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FloatingIPAssociationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FloatingIPAssociationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FloatingIPAssociationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	floatingIP := data.FloatingIP.ValueString()
	instanceID := data.InstanceID.ValueString()
	releaseOnDestroy := data.ReleaseOnDestroy.ValueBool()

	tflog.Debug(ctx, "Detaching floating IP", map[string]any{
		"floating_ip":        floatingIP,
		"instance_id":        instanceID,
		"release_on_destroy": releaseOnDestroy,
	})

	// Detach the floating IP
	_, err := r.client.FloatingIPs.Detach(ctx, instanceID)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to detach floating IP: %s", err))
		return
	}

	// Release if requested
	if releaseOnDestroy {
		tflog.Debug(ctx, "Releasing floating IP", map[string]any{"floating_ip": floatingIP})
		_, err = r.client.FloatingIPs.Release(ctx, floatingIP)
		if err != nil && !client.IsNotFound(err) {
			tflog.Warn(ctx, "Failed to release floating IP", map[string]any{
				"floating_ip": floatingIP,
				"error":       err.Error(),
			})
		} else {
			tflog.Info(ctx, "Floating IP released (refunded)", map[string]any{"floating_ip": floatingIP})
		}
	} else {
		tflog.Info(ctx, "Floating IP detached (still allocated, use release_on_destroy=true to release)", map[string]any{
			"floating_ip": floatingIP,
		})
	}
}

func (r *FloatingIPAssociationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: instance_id:floating_ip
	instanceID, floatingIP, found := strings.Cut(req.ID, ":")
	if !found || instanceID == "" || floatingIP == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected an import ID in the form \"instance_id:floating_ip\", got %q.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance_id"), instanceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("floating_ip"), floatingIP)...)
}
