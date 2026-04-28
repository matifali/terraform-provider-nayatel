// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVolumeResource_basic(t *testing.T) {
	name := testAccName("tf-acc-vol")
	size := testAccVolumeSize(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVolumeResourceConfig_basic(name, size),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_volume.test", "id"),
					resource.TestCheckResourceAttr("nayatel_volume.test", "name", name),
					resource.TestCheckResourceAttr("nayatel_volume.test", "size", fmt.Sprintf("%d", size)),
					resource.TestCheckResourceAttrSet("nayatel_volume.test", "status"),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_volumes.all", "volumes", "id", "nayatel_volume.test", "id"),
				),
			},
			{
				ResourceName:      "nayatel_volume.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccVolumeResourceConfig_basic(name string, size int) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_volume" "test" {
  name = %q
  size = %d
}

data "nayatel_volumes" "all" {
  depends_on = [nayatel_volume.test]
}
`, name, size)
}
