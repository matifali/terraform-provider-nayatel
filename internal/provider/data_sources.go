// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

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
		MarkdownDescription: "List available OS images. Use this to discover the image catalog; to look up a single image by name, use the `nayatel_image` data source instead.",
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
// Image Data Source (singular)
// ============================================================================

var _ datasource.DataSource = &ImageDataSource{}

func NewImageDataSource() datasource.DataSource {
	return &ImageDataSource{}
}

type ImageDataSource struct {
	client *client.Client
}

type ImageDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *ImageDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image"
}

func (d *ImageDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a single OS image by name. The name is matched case-insensitively: an exact match wins, otherwise a substring match is used (e.g. `Ubuntu 24.04` matches `Ubuntu 24.04 LTS (Noble Numbat)`). The lookup fails if no image or more than one image matches.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Image name to match (case-insensitive exact match, falling back to substring match)",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the matched image",
			},
		},
	}
}

func (d *ImageDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ImageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ImageDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	images, err := d.client.Images.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list images: %s", err))
		return
	}

	match, errSummary, errDetail := matchImageByName(images, data.Name.ValueString())
	if match == nil {
		resp.Diagnostics.AddError(errSummary, errDetail)
		return
	}

	data.ID = types.StringValue(match.ID)
	data.Name = types.StringValue(match.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// matchImageByName selects a single image by name. A case-insensitive exact
// match wins; otherwise a case-insensitive substring match is used. Zero or
// multiple matches produce an error summary and detail.
func matchImageByName(images []client.Image, name string) (match *client.Image, errSummary, errDetail string) {
	nameLower := strings.ToLower(name)

	var substringMatches []client.Image
	for i := range images {
		imgNameLower := strings.ToLower(images[i].Name)
		if imgNameLower == nameLower {
			return &images[i], "", ""
		}
		if strings.Contains(imgNameLower, nameLower) {
			substringMatches = append(substringMatches, images[i])
		}
	}

	switch len(substringMatches) {
	case 1:
		return &substringMatches[0], "", ""
	case 0:
		return nil, "Image Not Found",
			fmt.Sprintf("No image matches name %q. Available images: %s.", name, joinImageNames(images))
	default:
		return nil, "Multiple Images Match",
			fmt.Sprintf("Name %q matches multiple images: %s. Use a more specific name.", name, joinImageNames(substringMatches))
	}
}

func joinImageNames(images []client.Image) string {
	names := make([]string, 0, len(images))
	for _, img := range images {
		names = append(names, img.Name)
	}
	return strings.Join(names, ", ")
}

// ============================================================================
// SSH Key Data Source (singular)
// ============================================================================

var _ datasource.DataSource = &SSHKeyDataSource{}

func NewSSHKeyDataSource() datasource.DataSource {
	return &SSHKeyDataSource{}
}

type SSHKeyDataSource struct {
	client *client.Client
}

type SSHKeyDataSourceModel struct {
	Name        types.String `tfsdk:"name"`
	Fingerprint types.String `tfsdk:"fingerprint"`
}

func (d *SSHKeyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (d *SSHKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing SSH key by name. Use this to reference a key registered outside Terraform; keys managed in the same configuration should use the `nayatel_ssh_key` resource instead.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the SSH key",
			},
			"fingerprint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Fingerprint of the SSH key, as expected by `nayatel_instance.ssh_fingerprint`",
			},
		},
	}
}

func (d *SSHKeyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SSHKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SSHKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := d.client.SSHKeys.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list SSH keys: %s", err))
		return
	}

	name := data.Name.ValueString()
	for _, k := range keys {
		if k.Name == name {
			data.Fingerprint = types.StringValue(k.GetFingerprint())
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, k.Name)
	}
	resp.Diagnostics.AddError(
		"SSH Key Not Found",
		fmt.Sprintf("No SSH key named %q. Available keys: %s.", name, strings.Join(names, ", ")),
	)
}
