// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccInstanceResource_basic(t *testing.T) {
	sshKeyName := testAccName("tf-acc-inst-key")
	publicKey := testAccPublicKey(t)
	routerName := testAccName("tf-acc-inst-router")
	instanceName := testAccName("tf-acc-inst")
	imageIDExpression := testAccImageIDExpression()
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckNetworkBandwidth(t, bandwidth)
			testAccPreCheckImageSelection(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccInstanceResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, imageIDExpression, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_instance.test", "id"),
					resource.TestCheckResourceAttr("nayatel_instance.test", "name", instanceName),
					resource.TestCheckResourceAttr("nayatel_instance.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("nayatel_instance.test", "private_ip"),
					resource.TestCheckResourceAttr("nayatel_instance.test", "cpu", "2"),
					resource.TestCheckResourceAttr("nayatel_instance.test", "ram", "2"),
					resource.TestCheckResourceAttr("nayatel_instance.test", "disk", "20"),
					resource.TestCheckResourceAttrPair("nayatel_instance.test", "network_id", "nayatel_network.test", "id"),
					resource.TestCheckResourceAttrPair("nayatel_instance.test", "ssh_fingerprint", "nayatel_ssh_key.test", "fingerprint"),
				),
			},
			{
				ResourceName:      "nayatel_instance.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"description",
					"image_id",
					"disk",
					"network_id",
					"ssh_fingerprint",
					"monthly_cost",
				},
			},
		},
	})
}

func testAccInstanceResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, imageIDExpression string, bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}
%s
resource "nayatel_ssh_key" "test" {
  name       = %q
  public_key = %q
}

resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

resource "nayatel_router" "test" {
  name      = %q
  subnet_id = nayatel_network.test.subnet_id
}

resource "nayatel_instance" "test" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  ssh_fingerprint = nayatel_ssh_key.test.fingerprint

  depends_on = [nayatel_router.test]
}
`, testAccImageDataSourceConfig(), sshKeyName, publicKey, bandwidth, routerName, instanceName, imageIDExpression)
}
