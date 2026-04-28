// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccFloatingIPResource_basic tests basic floating IP allocation.
func TestAccFloatingIPResource_basic(t *testing.T) {
	sshKeyName := testAccName("tf-acc-fip-key")
	publicKey := testAccPublicKey(t)
	routerName := testAccName("tf-acc-fip-router")
	instanceName := testAccName("tf-acc-fip")
	imageIDExpression := testAccImageIDExpression()
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckNetworkBandwidth(t, bandwidth)
			testAccPreCheckImageSelection(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccFloatingIPResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, imageIDExpression, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_floating_ip.test", "id"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip.test", "ip_address"),
					resource.TestCheckResourceAttr("nayatel_floating_ip.test", "status", "ACTIVE"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip.test", "monthly_cost"),
					resource.TestMatchResourceAttr("nayatel_floating_ip.test", "monthly_cost", regexPositiveNumber()),
					resource.TestCheckResourceAttrSet("data.nayatel_floating_ips.all", "floating_ips.#"),
				),
			},
		},
	})
}

// TestAccFloatingIPAssociationResource_basic tests floating IP association.
func TestAccFloatingIPAssociationResource_basic(t *testing.T) {
	sshKeyName := testAccName("tf-acc-fip-assoc-key")
	publicKey := testAccPublicKey(t)
	routerName := testAccName("tf-acc-fip-assoc-router")
	bootstrapName := testAccName("tf-acc-fip-bootstrap")
	targetName := testAccName("tf-acc-fip-target")
	imageIDExpression := testAccImageIDExpression()
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckNetworkBandwidth(t, bandwidth)
			testAccPreCheckImageSelection(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create floating IP and then associate with different instance
			{
				Config: testAccFloatingIPAssociationResourceConfig_basic(sshKeyName, publicKey, routerName, bootstrapName, targetName, imageIDExpression, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_floating_ip_association.test", "id"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip_association.test", "floating_ip"),
					resource.TestCheckResourceAttrSet("nayatel_floating_ip_association.test", "instance_id"),
				),
			},
		},
	})
}

// TestAccFloatingIPAssociationResource_releaseOnDestroy tests the release_on_destroy behavior.
func TestAccFloatingIPAssociationResource_releaseOnDestroy(t *testing.T) {
	sshKeyName := testAccName("tf-acc-fip-release-key")
	publicKey := testAccPublicKey(t)
	routerName := testAccName("tf-acc-fip-release-router")
	instanceName := testAccName("tf-acc-fip-release")
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
				Config: testAccFloatingIPAssociationResourceConfig_releaseOnDestroy(sshKeyName, publicKey, routerName, instanceName, imageIDExpression, bandwidth, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nayatel_floating_ip_association.test", "release_on_destroy", "true"),
				),
			},
			{
				Config: testAccFloatingIPAssociationResourceConfig_releaseOnDestroy(sshKeyName, publicKey, routerName, instanceName, imageIDExpression, bandwidth, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nayatel_floating_ip_association.test", "release_on_destroy", "false"),
				),
			},
		},
	})
}

// Helper function to match positive numbers.
func regexPositiveNumber() *regexp.Regexp {
	return regexp.MustCompile(`^([1-9][0-9]*(\.[0-9]+)?|0\.[0-9]*[1-9][0-9]*)$`)
}

// Test configurations

func testAccFloatingIPResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, imageIDExpression string, bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}
%s
# Create SSH key for testing
resource "nayatel_ssh_key" "test" {
  name       = %q
  public_key = %q
}

# Create prerequisite resources
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

# Test floating IP resource
resource "nayatel_floating_ip" "test" {
  instance_id = nayatel_instance.test.id
}

data "nayatel_floating_ips" "all" {
  depends_on = [nayatel_floating_ip.test]
}
`, testAccImageDataSourceConfig(), sshKeyName, publicKey, bandwidth, routerName, instanceName, imageIDExpression)
}

func testAccFloatingIPAssociationResourceConfig_basic(sshKeyName, publicKey, routerName, bootstrapName, targetName, imageIDExpression string, bandwidth int) string {
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

# First instance to allocate the IP
resource "nayatel_instance" "bootstrap" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  ssh_fingerprint = nayatel_ssh_key.test.fingerprint

  depends_on = [nayatel_router.test]
}

# Allocate floating IP via bootstrap instance
resource "nayatel_floating_ip" "test" {
  instance_id = nayatel_instance.bootstrap.id
}

# Second instance to associate the IP with
resource "nayatel_instance" "target" {
  name            = %q
  image_id        = %s
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  ssh_fingerprint = nayatel_ssh_key.test.fingerprint

  depends_on = [nayatel_router.test]
}

# Associate floating IP with target instance
resource "nayatel_floating_ip_association" "test" {
  floating_ip = nayatel_floating_ip.test.ip_address
  instance_id = nayatel_instance.target.id
}
`, testAccImageDataSourceConfig(), sshKeyName, publicKey, bandwidth, routerName, bootstrapName, imageIDExpression, targetName, imageIDExpression)
}

func testAccFloatingIPAssociationResourceConfig_releaseOnDestroy(sshKeyName, publicKey, routerName, instanceName, imageIDExpression string, bandwidth int, release bool) string {
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

resource "nayatel_floating_ip" "test" {
  instance_id = nayatel_instance.test.id
}

resource "nayatel_floating_ip_association" "test" {
  floating_ip        = nayatel_floating_ip.test.ip_address
  instance_id        = nayatel_instance.test.id
  release_on_destroy = %t
}
`, testAccImageDataSourceConfig(), sshKeyName, publicKey, bandwidth, routerName, instanceName, imageIDExpression, release)
}
