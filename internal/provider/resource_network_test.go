// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNetworkResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkResourceConfig_basic(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_network.test", "id"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "name"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "status"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "subnet_id"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "subnet_cidr"),
					resource.TestCheckResourceAttr("nayatel_network.test", "bandwidth_limit", "1"),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_networks.all", "networks", "id", "nayatel_network.test", "id"),
				),
			},
			{
				ResourceName:            "nayatel_network.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bandwidth_limit", "monthly_cost"},
			},
		},
	})
}

func testAccNetworkResourceConfig_basic() string {
	return `
provider "nayatel" {}

resource "nayatel_network" "test" {
  bandwidth_limit = 1
}

data "nayatel_networks" "all" {
  depends_on = [nayatel_network.test]
}
`
}
