// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRouterResource_basic(t *testing.T) {
	name := testAccName("tf-acc-router")
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckNetworkBandwidth(t, bandwidth) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRouterResourceConfig_basic(name, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_router.test", "id"),
					resource.TestCheckResourceAttr("nayatel_router.test", "name", name),
					resource.TestCheckResourceAttrSet("nayatel_router.test", "status"),
					resource.TestCheckResourceAttrPair("nayatel_router.test", "subnet_id", "nayatel_network.test", "subnet_id"),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_routers.all", "routers", "id", "nayatel_router.test", "id"),
				),
			},
			{
				ResourceName:            "nayatel_router.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"subnet_id"},
			},
		},
	})
}

func testAccRouterResourceConfig_basic(name string, bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

resource "nayatel_router" "test" {
  name      = %q
  subnet_id = nayatel_network.test.subnet_id
}

data "nayatel_routers" "all" {
  depends_on = [nayatel_router.test]
}
`, bandwidth, name)
}
