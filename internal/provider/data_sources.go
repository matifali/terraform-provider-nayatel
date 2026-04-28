// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

// ============================================================================
// Images Data Source
// ============================================================================

var _ datasource.DataSource = &ImagesDataSource{}

func NewImagesDataSource() datasource.DataSource {
	return &ImagesDataSource{}
}

type ImagesDataSource struct {
	client *client.Client
}

type ImagesDataSourceModel struct {
	Images []ImageModel `tfsdk:"images"`
}

type ImageModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *ImagesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_images"
}

func (d *ImagesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available OS images.",
		Attributes: map[string]schema.Attribute{
			"images": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of available images",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ImagesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *ImagesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ImagesDataSourceModel

	images, err := d.client.Images.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list images: %s", err))
		return
	}

	for _, img := range images {
		data.Images = append(data.Images, ImageModel{
			ID:   types.StringValue(img.ID),
			Name: types.StringValue(img.Name),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// Flavors Data Source
// ============================================================================

var _ datasource.DataSource = &FlavorsDataSource{}

func NewFlavorsDataSource() datasource.DataSource {
	return &FlavorsDataSource{}
}

type FlavorsDataSource struct {
	client *client.Client
}

type FlavorsDataSourceModel struct {
	Flavors []FlavorModel `tfsdk:"flavors"`
}

type FlavorModel struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	VCPUs types.Int64  `tfsdk:"vcpus"`
	RAM   types.Int64  `tfsdk:"ram"`
	Disk  types.Int64  `tfsdk:"disk"`
}

func (d *FlavorsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flavors"
}

func (d *FlavorsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available instance flavors (CPU/RAM/Disk combinations).",
		Attributes: map[string]schema.Attribute{
			"flavors": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of available flavors",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":    schema.StringAttribute{Computed: true},
						"name":  schema.StringAttribute{Computed: true},
						"vcpus": schema.Int64Attribute{Computed: true},
						"ram":   schema.Int64Attribute{Computed: true},
						"disk":  schema.Int64Attribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *FlavorsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *FlavorsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	data := FlavorsDataSourceModel{Flavors: []FlavorModel{}}

	flavors, err := d.client.Flavors.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list flavors: %s", err))
		return
	}

	for _, f := range flavors {
		data.Flavors = append(data.Flavors, FlavorModel{
			ID:    types.StringValue(f.ID),
			Name:  types.StringValue(f.Name),
			VCPUs: types.Int64Value(int64(f.GetVCPUs())),
			RAM:   types.Int64Value(int64(f.GetRAM())),
			Disk:  types.Int64Value(int64(f.GetDisk())),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// SSH Keys Data Source
// ============================================================================

var _ datasource.DataSource = &SSHKeysDataSource{}

func NewSSHKeysDataSource() datasource.DataSource {
	return &SSHKeysDataSource{}
}

type SSHKeysDataSource struct {
	client *client.Client
}

type SSHKeysDataSourceModel struct {
	Keys []SSHKeyModel `tfsdk:"keys"`
}

type SSHKeyModel struct {
	Name        types.String `tfsdk:"name"`
	Fingerprint types.String `tfsdk:"fingerprint"`
}

func (d *SSHKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_keys"
}

func (d *SSHKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available SSH keys.",
		Attributes: map[string]schema.Attribute{
			"keys": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of SSH keys",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Computed: true},
						"fingerprint": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *SSHKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *SSHKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SSHKeysDataSourceModel

	keys, err := d.client.SSHKeys.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list SSH keys: %s", err))
		return
	}

	for _, k := range keys {
		data.Keys = append(data.Keys, SSHKeyModel{
			Name:        types.StringValue(k.Name),
			Fingerprint: types.StringValue(k.GetFingerprint()),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// Networks Data Source
// ============================================================================

var _ datasource.DataSource = &NetworksDataSource{}

func NewNetworksDataSource() datasource.DataSource {
	return &NetworksDataSource{}
}

type NetworksDataSource struct {
	client *client.Client
}

type NetworksDataSourceModel struct {
	Networks []NetworkModel `tfsdk:"networks"`
}

type NetworkModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Status types.String `tfsdk:"status"`
}

func (d *NetworksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_networks"
}

func (d *NetworksDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available networks.",
		Attributes: map[string]schema.Attribute{
			"networks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of networks",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":     schema.StringAttribute{Computed: true},
						"name":   schema.StringAttribute{Computed: true},
						"status": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *NetworksDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *NetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NetworksDataSourceModel

	networks, err := d.client.Networks.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list networks: %s", err))
		return
	}

	for _, n := range networks {
		data.Networks = append(data.Networks, NetworkModel{
			ID:     types.StringValue(n.ID),
			Name:   types.StringValue(n.Name),
			Status: types.StringValue(n.Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// Security Groups Data Source
// ============================================================================

var _ datasource.DataSource = &SecurityGroupsDataSource{}

func NewSecurityGroupsDataSource() datasource.DataSource {
	return &SecurityGroupsDataSource{}
}

type SecurityGroupsDataSource struct {
	client *client.Client
}

type SecurityGroupsDataSourceModel struct {
	SecurityGroups []SecurityGroupModel `tfsdk:"security_groups"`
}

type SecurityGroupModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (d *SecurityGroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_groups"
}

func (d *SecurityGroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available security groups.",
		Attributes: map[string]schema.Attribute{
			"security_groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of security groups",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *SecurityGroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *SecurityGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SecurityGroupsDataSourceModel

	sgs, err := d.client.SecurityGroups.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list security groups: %s", err))
		return
	}

	for _, sg := range sgs {
		data.SecurityGroups = append(data.SecurityGroups, SecurityGroupModel{
			ID:          types.StringValue(sg.ID),
			Name:        types.StringValue(sg.Name),
			Description: types.StringValue(sg.Description),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// Routers Data Source
// ============================================================================

var _ datasource.DataSource = &RoutersDataSource{}

func NewRoutersDataSource() datasource.DataSource {
	return &RoutersDataSource{}
}

type RoutersDataSource struct {
	client *client.Client
}

type RoutersDataSourceModel struct {
	Routers []RouterDataModel `tfsdk:"routers"`
}

type RouterDataModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Status          types.String `tfsdk:"status"`
	ExternalNetwork types.String `tfsdk:"external_network"`
	ExternalIP      types.String `tfsdk:"external_ip"`
	AttachedSubnet  types.String `tfsdk:"attached_subnet"`
}

func (d *RoutersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_routers"
}

func (d *RoutersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available routers. Use this to reference existing routers instead of creating new ones.",
		Attributes: map[string]schema.Attribute{
			"routers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of routers",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":               schema.StringAttribute{Computed: true, MarkdownDescription: "Router ID"},
						"name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Router name"},
						"status":           schema.StringAttribute{Computed: true, MarkdownDescription: "Router status"},
						"external_network": schema.StringAttribute{Computed: true, MarkdownDescription: "External network name"},
						"external_ip":      schema.StringAttribute{Computed: true, MarkdownDescription: "External IP address"},
						"attached_subnet":  schema.StringAttribute{Computed: true, MarkdownDescription: "Attached subnet name"},
					},
				},
			},
		},
	}
}

func (d *RoutersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *RoutersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RoutersDataSourceModel

	routers, err := d.client.Routers.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list routers: %s", err))
		return
	}

	for _, r := range routers {
		data.Routers = append(data.Routers, RouterDataModel{
			ID:     types.StringValue(r.ID),
			Name:   types.StringValue(r.Name),
			Status: types.StringValue(r.Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// Floating IPs Data Source
// ============================================================================

var _ datasource.DataSource = &FloatingIPsDataSource{}

func NewFloatingIPsDataSource() datasource.DataSource {
	return &FloatingIPsDataSource{}
}

type FloatingIPsDataSource struct {
	client *client.Client
}

type FloatingIPsDataSourceModel struct {
	FloatingIPs []FloatingIPDataModel `tfsdk:"floating_ips"`
}

type FloatingIPDataModel struct {
	ID        types.String `tfsdk:"id"`
	IPAddress types.String `tfsdk:"ip_address"`
	Status    types.String `tfsdk:"status"`
}

func (d *FloatingIPsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_floating_ips"
}

func (d *FloatingIPsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available floating IPs.",
		Attributes: map[string]schema.Attribute{
			"floating_ips": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of floating IPs",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true},
						"ip_address": schema.StringAttribute{Computed: true},
						"status":     schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *FloatingIPsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *FloatingIPsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FloatingIPsDataSourceModel

	fips, err := d.client.FloatingIPs.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list floating IPs: %s", err))
		return
	}

	for _, fip := range fips {
		data.FloatingIPs = append(data.FloatingIPs, FloatingIPDataModel{
			ID:        types.StringValue(fip.ID),
			IPAddress: types.StringValue(fip.GetIPAddress()),
			Status:    types.StringValue(fip.Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ============================================================================
// Volumes Data Source
// ============================================================================

var _ datasource.DataSource = &VolumesDataSource{}

func NewVolumesDataSource() datasource.DataSource {
	return &VolumesDataSource{}
}

type VolumesDataSource struct {
	client *client.Client
}

type VolumesDataSourceModel struct {
	Volumes []VolumeDataModel `tfsdk:"volumes"`
}

type VolumeDataModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Size       types.Int64  `tfsdk:"size"`
	Status     types.String `tfsdk:"status"`
	Bootable   types.Bool   `tfsdk:"bootable"`
	InstanceID types.String `tfsdk:"instance_id"`
}

func (d *VolumesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_volumes"
}

func (d *VolumesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available volumes.",
		Attributes: map[string]schema.Attribute{
			"volumes": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of volumes",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"size":        schema.Int64Attribute{Computed: true},
						"status":      schema.StringAttribute{Computed: true},
						"bootable":    schema.BoolAttribute{Computed: true},
						"instance_id": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *VolumesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *VolumesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VolumesDataSourceModel

	volumes, err := d.client.Volumes.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list volumes: %s", err))
		return
	}

	for _, v := range volumes {
		instanceID := ""
		if v.IsAttached() {
			instanceID = v.GetAttachedInstanceID()
		}
		data.Volumes = append(data.Volumes, VolumeDataModel{
			ID:         types.StringValue(v.ID),
			Name:       types.StringValue(v.Name),
			Size:       types.Int64Value(int64(v.Size)),
			Status:     types.StringValue(v.Status),
			Bootable:   types.BoolValue(v.IsBootable()),
			InstanceID: types.StringValue(instanceID),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
