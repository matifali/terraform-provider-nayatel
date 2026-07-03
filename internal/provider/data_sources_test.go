// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

func TestAccDataSourceImages_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckImages(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceImagesConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNestedListNotEmpty("data.nayatel_images.test", "images"),
					testAccCheckAnyNestedAttrsSet("data.nayatel_images.test", "images", "id", "name"),
				),
			},
		},
	})
}

func TestAccDataSourceImage_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckImages(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceImageConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.nayatel_image.test", "id"),
					resource.TestCheckResourceAttrSet("data.nayatel_image.test", "name"),
				),
			},
		},
	})
}

func testAccDataSourceImagesConfig() string {
	return `
provider "nayatel" {}

data "nayatel_images" "test" {}
`
}

func testAccDataSourceImageConfig() string {
	return `
provider "nayatel" {}

data "nayatel_images" "all" {}

data "nayatel_image" "test" {
  name = data.nayatel_images.all.images[0].name
}
`
}

func TestMatchImageByName(t *testing.T) {
	t.Parallel()

	images := []client.Image{
		{ID: "jammy", Name: "Ubuntu 22.04 LTS (Jammy Jellyfish)"},
		{ID: "noble", Name: "Ubuntu 24.04 LTS (Noble Numbat)"},
		{ID: "bookworm", Name: "Debian 12 (Bookworm)"},
	}

	tests := []struct {
		name       string
		lookup     string
		wantID     string
		wantErrSum string
	}{
		{name: "exact match", lookup: "Ubuntu 24.04 LTS (Noble Numbat)", wantID: "noble"},
		{name: "exact match case-insensitive", lookup: "ubuntu 24.04 lts (noble numbat)", wantID: "noble"},
		{name: "substring match", lookup: "Ubuntu 24.04", wantID: "noble"},
		{name: "substring match case-insensitive", lookup: "debian 12", wantID: "bookworm"},
		{name: "no match", lookup: "Fedora", wantErrSum: "Image Not Found"},
		{name: "ambiguous match", lookup: "Ubuntu", wantErrSum: "Multiple Images Match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			match, errSummary, _ := matchImageByName(images, tt.lookup)
			if tt.wantErrSum != "" {
				if match != nil {
					t.Fatalf("expected no match, got %q", match.ID)
				}
				if errSummary != tt.wantErrSum {
					t.Fatalf("expected error summary %q, got %q", tt.wantErrSum, errSummary)
				}
				return
			}
			if match == nil {
				t.Fatalf("expected match %q, got error %q", tt.wantID, errSummary)
			}
			if match.ID != tt.wantID {
				t.Fatalf("expected match %q, got %q", tt.wantID, match.ID)
			}
		})
	}
}
