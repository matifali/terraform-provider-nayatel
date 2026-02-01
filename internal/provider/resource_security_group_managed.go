// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

var _ resource.Resource = &SecurityGroupResource{}
var _ resource.ResourceWithImportState = &SecurityGroupResource{}

func NewSecurityGroupResource() resource.Resource {
	return &SecurityGroupResource{}
}

type SecurityGroupResource struct {
	client *client.Client
}

type SecurityGroupRuleModel struct {
	Direction  types.String `tfsdk:"direction"`
	Ethertype  types.String `tfsdk:"ethertype"`
	Protocol   types.String `tfsdk:"protocol"`
	PortNumber types.String `tfsdk:"port_number"`
	CIDR       types.String `tfsdk:"cidr"`
}

type SecurityGroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Rules       types.List   `tfsdk:"rule"`
}

func (r *SecurityGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group"
}

func (r *SecurityGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a Nayatel Cloud security group with rules.

## Example Usage

` + "```hcl" + `
resource "nayatel_security_group" "web" {
  name        = "web-servers"
  description = "Security group for web servers"

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "80"
    cidr        = "0.0.0.0/0"
  }

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "443"
    cidr        = "0.0.0.0/0"
  }

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "22"
    cidr        = "10.0.0.0/8"
  }
}
` + "```" + `

**Note:** Changing any rule will recreate the entire security group since the API does not support deleting individual rules.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Security group identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the security group (API may add a suffix)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Managed by Terraform"),
				MarkdownDescription: "Description of the security group",
			},
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				MarkdownDescription: "Security group rules",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"direction": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Direction: 'ingress' or 'egress'",
						},
						"ethertype": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("IPv4"),
							MarkdownDescription: "Ethertype: 'IPv4' or 'IPv6'. Defaults to 'IPv4'",
						},
						"protocol": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("tcp"),
							MarkdownDescription: "Protocol: 'tcp', 'udp', 'icmp'. Defaults to 'tcp'",
						},
						"port_number": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Port number (e.g., '80', '443', '22')",
						},
						"cidr": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("0.0.0.0/0"),
							MarkdownDescription: "CIDR block. Defaults to '0.0.0.0/0'",
						},
					},
				},
			},
		},
	}
}

func (r *SecurityGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SecurityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SecurityGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating security group", map[string]any{"name": data.Name.ValueString()})

	// Create security group
	createReq := &client.SecurityGroupCreateRequest{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
	}

	_, err := r.client.SecurityGroups.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create security group: %s", err))
		return
	}

	// Find the created security group by name
	sg, err := r.client.SecurityGroups.FindByName(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find created security group: %s", err))
		return
	}
	if sg == nil {
		resp.Diagnostics.AddError("Client Error", "Security group not found after creation")
		return
	}

	data.ID = types.StringValue(sg.ID)
	// Note: API may add suffix to name (e.g., "terraform-ssh-925"), but we keep
	// the configured name in state. The attachment resource will look up the
	// actual name by ID when needed.

	// Create rules
	var rules []SecurityGroupRuleModel
	if !data.Rules.IsNull() && !data.Rules.IsUnknown() {
		resp.Diagnostics.Append(data.Rules.ElementsAs(ctx, &rules, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		for _, rule := range rules {
			direction := rule.Direction.ValueString()
			switch direction {
			case "ingress":
				direction = "Ingress"
			case "egress":
				direction = "Egress"
			}

			ruleReq := &client.SecurityGroupRuleCreateRequest{
				Direction:  direction,
				Ethertype:  rule.Ethertype.ValueString(),
				Protocol:   rule.Protocol.ValueString(),
				PortNumber: rule.PortNumber.ValueString(),
				CIDR:       rule.CIDR.ValueString(),
			}

			_, err := r.client.SecurityGroups.CreateRule(ctx, sg.ID, ruleReq)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create security group rule: %s", err))
				return
			}
		}
	}

	tflog.Trace(ctx, "Created security group", map[string]any{"id": sg.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecurityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SecurityGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sg, err := r.client.SecurityGroups.FindByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read security group: %s", err))
		return
	}
	if sg == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Name = types.StringValue(sg.Name)
	data.Description = types.StringValue(sg.Description)

	// Read rules from API
	apiRules, err := r.client.SecurityGroups.ListRules(ctx, data.ID.ValueString())
	if err != nil {
		tflog.Warn(ctx, "Unable to read security group rules", map[string]any{"error": err.Error()})
	}

	// Convert API rules to model - only include non-default egress rules
	if len(apiRules) > 0 {
		var ruleModels []SecurityGroupRuleModel
		for _, apiRule := range apiRules {
			// Skip default egress rules (allow all outbound)
			if apiRule.Direction == "egress" && apiRule.Protocol == "Any" {
				continue
			}

			ruleModel := SecurityGroupRuleModel{
				Direction: types.StringValue(apiRule.Direction),
				Ethertype: types.StringValue(apiRule.Ethertype),
			}

			if apiRule.Protocol != "" && apiRule.Protocol != "Any" {
				ruleModel.Protocol = types.StringValue(apiRule.Protocol)
			} else {
				ruleModel.Protocol = types.StringValue("tcp")
			}

			if apiRule.RemoteIPPrefix != "" {
				ruleModel.CIDR = types.StringValue(apiRule.RemoteIPPrefix)
			} else {
				ruleModel.CIDR = types.StringValue("0.0.0.0/0")
			}

			// Extract port from port_range (format: "80 - 80" or "Any")
			if apiRule.PortRange != "" && apiRule.PortRange != "Any" {
				// Take the first port from range
				port := apiRule.PortRange
				if idx := len(port); idx > 0 {
					// Split "80 - 80" and take first part
					for i, c := range port {
						if c == ' ' {
							port = port[:i]
							break
						}
					}
				}
				ruleModel.PortNumber = types.StringValue(port)
			} else {
				ruleModel.PortNumber = types.StringNull()
			}

			ruleModels = append(ruleModels, ruleModel)
		}

		if len(ruleModels) > 0 {
			ruleType := types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"direction":   types.StringType,
					"ethertype":   types.StringType,
					"protocol":    types.StringType,
					"port_number": types.StringType,
					"cidr":        types.StringType,
				},
			}
			rulesList, diags := types.ListValueFrom(ctx, ruleType, ruleModels)
			resp.Diagnostics.Append(diags...)
			if !resp.Diagnostics.HasError() {
				data.Rules = rulesList
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecurityGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SecurityGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Keep the same ID
	plan.ID = state.ID

	// Check if rules changed
	if !plan.Rules.Equal(state.Rules) {
		var planRules, stateRules []SecurityGroupRuleModel

		if !plan.Rules.IsNull() && !plan.Rules.IsUnknown() {
			resp.Diagnostics.Append(plan.Rules.ElementsAs(ctx, &planRules, false)...)
		}
		if !state.Rules.IsNull() && !state.Rules.IsUnknown() {
			resp.Diagnostics.Append(state.Rules.ElementsAs(ctx, &stateRules, false)...)
		}
		if resp.Diagnostics.HasError() {
			return
		}

		// Find new rules to add (in plan but not in state)
		for _, planRule := range planRules {
			found := false
			for _, stateRule := range stateRules {
				if ruleMatches(planRule, stateRule) {
					found = true
					break
				}
			}
			if !found {
				// Add new rule
				direction := planRule.Direction.ValueString()
				switch direction {
				case "ingress":
					direction = "Ingress"
				case "egress":
					direction = "Egress"
				}

				ruleReq := &client.SecurityGroupRuleCreateRequest{
					Direction:  direction,
					Ethertype:  planRule.Ethertype.ValueString(),
					Protocol:   planRule.Protocol.ValueString(),
					PortNumber: planRule.PortNumber.ValueString(),
					CIDR:       planRule.CIDR.ValueString(),
				}

				tflog.Debug(ctx, "Adding new security group rule", map[string]any{
					"direction": direction,
					"port":      planRule.PortNumber.ValueString(),
				})

				_, err := r.client.SecurityGroups.CreateRule(ctx, state.ID.ValueString(), ruleReq)
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create security group rule: %s", err))
					return
				}
			}
		}

		// Check for removed rules (in state but not in plan)
		for _, stateRule := range stateRules {
			found := false
			for _, planRule := range planRules {
				if ruleMatches(planRule, stateRule) {
					found = true
					break
				}
			}
			if !found {
				// Rule was removed - warn user since API doesn't support deletion
				tflog.Warn(ctx, "Rule removal not supported by API - rule will remain in cloud but removed from Terraform state", map[string]any{
					"direction": stateRule.Direction.ValueString(),
					"port":      stateRule.PortNumber.ValueString(),
					"cidr":      stateRule.CIDR.ValueString(),
				})
				resp.Diagnostics.AddWarning(
					"Rule Deletion Not Supported",
					fmt.Sprintf("The Nayatel API does not support deleting security group rules. The rule (direction=%s, port=%s, cidr=%s) will remain in the cloud but is removed from Terraform state. Delete the entire security group to remove all rules.",
						stateRule.Direction.ValueString(),
						stateRule.PortNumber.ValueString(),
						stateRule.CIDR.ValueString(),
					),
				)
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// ruleMatches checks if two rules are equivalent.
func ruleMatches(a, b SecurityGroupRuleModel) bool {
	return a.Direction.ValueString() == b.Direction.ValueString() &&
		a.Ethertype.ValueString() == b.Ethertype.ValueString() &&
		a.Protocol.ValueString() == b.Protocol.ValueString() &&
		a.PortNumber.ValueString() == b.PortNumber.ValueString() &&
		a.CIDR.ValueString() == b.CIDR.ValueString()
}

func (r *SecurityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SecurityGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting security group", map[string]any{"id": data.ID.ValueString()})

	_, err := r.client.SecurityGroups.Delete(ctx, data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete security group: %s", err))
		return
	}

	tflog.Trace(ctx, "Deleted security group", map[string]any{"id": data.ID.ValueString()})
}

func (r *SecurityGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
