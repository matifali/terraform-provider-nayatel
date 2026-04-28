// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNetworkResource_basic(t *testing.T) {
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckNetworkBandwidth(t, bandwidth) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNetworkResourceConfig_basic(bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_network.test", "id"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "name"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "subnet_id"),
					resource.TestCheckResourceAttrSet("nayatel_network.test", "subnet_cidr"),
					resource.TestCheckResourceAttr("nayatel_network.test", "bandwidth_limit", fmt.Sprintf("%d", bandwidth)),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_networks.all", "networks", "id", "nayatel_network.test", "id"),
				),
			},
			{
				ResourceName:            "nayatel_network.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bandwidth_limit", "monthly_cost", "status"},
			},
		},
	})
}

func testAccNetworkResourceConfig_basic(bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

data "nayatel_networks" "all" {
  depends_on = [nayatel_network.test]
}
`, bandwidth)
}
