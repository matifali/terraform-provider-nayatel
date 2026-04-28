// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSecurityGroupResource_basic(t *testing.T) {
	name := testAccName("tf-acc-sg")
	description := "Terraform acceptance test security group"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupResourceConfig_basic(name, description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_security_group.test", "id"),
					resource.TestMatchResourceAttr("nayatel_security_group.test", "name", regexp.MustCompile("^"+regexp.QuoteMeta(name)+"(-.*)?$")),
					resource.TestCheckResourceAttr("nayatel_security_group.test", "description", description),
					resource.TestCheckResourceAttr("nayatel_security_group.test", "rule.#", "1"),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_security_groups.all", "security_groups", "id", "nayatel_security_group.test", "id"),
				),
			},
		},
	})
}

func TestAccSecurityGroupAttachmentResource_basic(t *testing.T) {
	sshKeyName := testAccName("tf-acc-sg-att-key")
	publicKey := testAccPublicKey(t)
	routerName := testAccName("tf-acc-sg-att-router")
	instanceName := testAccName("tf-acc-sg-att-inst")
	securityGroupName := testAccName("tf-acc-sg-att")
	imageIDExpression := testAccImageIDExpression()
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckRouterTests(t)
			testAccPreCheckNetworkBandwidth(t, bandwidth)
			testAccPreCheckImageSelection(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSecurityGroupAttachmentResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, securityGroupName, imageIDExpression, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_security_group_attachment.test", "id"),
					resource.TestCheckResourceAttrPair("nayatel_security_group_attachment.test", "instance_id", "nayatel_instance.test", "id"),
					resource.TestCheckResourceAttrPair("nayatel_security_group_attachment.test", "security_group_name", "nayatel_security_group.test", "name"),
				),
			},
			{
				ResourceName:      "nayatel_security_group_attachment.test",
				ImportState:       true,
				ImportStateIdFunc: testAccCompositeImportID("nayatel_security_group_attachment.test", "instance_id", "security_group_name"),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSecurityGroupResourceConfig_basic(name, description string) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_security_group" "test" {
  name        = %q
  description = %q

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "22"
    cidr        = "0.0.0.0/0"
  }
}

data "nayatel_security_groups" "all" {
  depends_on = [nayatel_security_group.test]
}
`, name, description)
}

func testAccSecurityGroupAttachmentResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, securityGroupName, imageIDExpression string, bandwidth int) string {
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

resource "nayatel_security_group" "test" {
  name        = %q
  description = "Terraform acceptance test attachment security group"

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "22"
    cidr        = "0.0.0.0/0"
  }
}

resource "nayatel_security_group_attachment" "test" {
  instance_id         = nayatel_instance.test.id
  security_group_name = nayatel_security_group.test.name
}
`, testAccImageDataSourceConfig(), sshKeyName, publicKey, bandwidth, routerName, instanceName, imageIDExpression, securityGroupName)
}
