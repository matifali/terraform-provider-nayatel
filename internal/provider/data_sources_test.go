// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSourceImages_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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

func TestAccDataSourceFlavors_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceFlavorsConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckNestedListNotEmpty("data.nayatel_flavors.test", "flavors"),
					testAccCheckAnyNestedAttrsSetAndIntAttrsPositive(
						"data.nayatel_flavors.test",
						"flavors",
						[]string{},
						[]string{"vcpus", "ram", "disk"},
					),
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

func testAccDataSourceFlavorsConfig() string {
	return `
provider "nayatel" {}

data "nayatel_flavors" "test" {}
`
}
