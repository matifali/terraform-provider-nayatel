// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFloatingIPResource_basic(t *testing.T) {
	routerName := testAccName("tf-acc-fip-router")
	instanceName := testAccName("tf-acc-fip")
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
				Config: testAccFloatingIPResourceConfig_basic(routerName, instanceName, imageIDExpression, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_floating_ip.test", "id"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip.test", "ip_address"),
					resource.TestCheckResourceAttr("nayatel_floating_ip.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip.test", "monthly_cost"),
					resource.TestMatchResourceAttr("nayatel_floating_ip.test", "monthly_cost", regexPositiveNumber()),
				),
			},
		},
	})
}

func TestAccFloatingIPAssociationResource_basic(t *testing.T) {
	routerName := testAccName("tf-acc-fip-assoc-router")
	bootstrapName := testAccName("tf-acc-fip-bootstrap")
	targetName := testAccName("tf-acc-fip-target")
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
				Config: testAccFloatingIPAssociationResourceConfig_basic(routerName, bootstrapName, targetName, imageIDExpression, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_floating_ip_association.test", "id"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip_association.test", "floating_ip"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip_association.test", "instance_id"),
				),
			},
		},
	})
}

func TestAccFloatingIPAssociationResource_releaseOnDestroy(t *testing.T) {
	routerName := testAccName("tf-acc-fip-release-router")
	instanceName := testAccName("tf-acc-fip-release")
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
				Config: testAccFloatingIPAssociationResourceConfig_releaseOnDestroy(routerName, instanceName, imageIDExpression, bandwidth, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nayatel_floating_ip_association.test", "release_on_destroy", "true"),
				),
			},
			{
				Config: testAccFloatingIPAssociationResourceConfig_releaseOnDestroy(routerName, instanceName, imageIDExpression, bandwidth, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nayatel_floating_ip_association.test", "release_on_destroy", "false"),
				),
			},
		},
	})
}

func regexPositiveNumber() *regexp.Regexp {
	return regexp.MustCompile(`^([1-9][0-9]*(\.[0-9]+)?|0\.[0-9]*[1-9][0-9]*)$`)
}

func testAccFloatingIPResourceConfig_basic(routerName, instanceName, imageIDExpression string, bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}
%s
resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

resource "nayatel_router" "test" {
  name                            = %q
  subnet_id                       = nayatel_network.test.subnet_id
  force_delete_network_on_destroy = true
}

resource "nayatel_instance" "test" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  password        = %q

  depends_on = [nayatel_router.test]
}

resource "nayatel_floating_ip" "test" {
  instance_id = nayatel_instance.test.id
}
`, testAccImageDataSourceConfig(), bandwidth, routerName, instanceName, imageIDExpression, testAccInstancePassword)
}

func testAccFloatingIPAssociationResourceConfig_basic(routerName, bootstrapName, targetName, imageIDExpression string, bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}
%s
resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

resource "nayatel_router" "test" {
  name                            = %q
  subnet_id                       = nayatel_network.test.subnet_id
  force_delete_network_on_destroy = true
}

resource "nayatel_instance" "bootstrap" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  password        = %q

  depends_on = [nayatel_router.test]
}

resource "nayatel_floating_ip" "test" {
  instance_id = nayatel_instance.bootstrap.id
}

# Second instance to associate the IP with. Depends on bootstrap (not just
# the network) to force serialized creation: Nayatel's instance-create API
# is unreliable when two instances request an IP/port from the same
# just-created network concurrently (confirmed via direct client repro:
# concurrent creates reliably put one instance into ERROR, sequential
# creates did not). nayatel_instance's Create already blocks until ACTIVE,
# so this ordering is sufficient to avoid the race.
resource "nayatel_instance" "target" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  password        = %q

  depends_on = [nayatel_instance.bootstrap]
}

resource "nayatel_floating_ip_association" "test" {
  floating_ip = nayatel_floating_ip.test.ip_address
  instance_id = nayatel_instance.target.id
}
`, testAccImageDataSourceConfig(), bandwidth, routerName, bootstrapName, imageIDExpression, testAccInstancePassword, targetName, imageIDExpression, testAccInstancePassword)
}

func testAccFloatingIPAssociationResourceConfig_releaseOnDestroy(routerName, instanceName, imageIDExpression string, bandwidth int, release bool) string {
	return fmt.Sprintf(`
provider "nayatel" {}
%s
resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

resource "nayatel_router" "test" {
  name                            = %q
  subnet_id                       = nayatel_network.test.subnet_id
  force_delete_network_on_destroy = true
}

resource "nayatel_instance" "test" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  password        = %q

  depends_on = [nayatel_router.test]
}

resource "nayatel_floating_ip" "test" {
  instance_id = nayatel_instance.test.id
}

resource "nayatel_floating_ip_association" "test" {
  floating_ip        = nayatel_floating_ip.test.ip_address
  instance_id        = nayatel_instance.test.id
  release_on_destroy = %t
}
`, testAccImageDataSourceConfig(), bandwidth, routerName, instanceName, imageIDExpression, testAccInstancePassword, release)
}
