// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVolumeAttachmentResource_basic(t *testing.T) {
	sshKeyName := testAccName("tf-acc-vol-att-key")
	publicKey := testAccPublicKey(t)
	routerName := testAccName("tf-acc-vol-att-router")
	instanceName := testAccName("tf-acc-vol-att-inst")
	volumeName := testAccName("tf-acc-vol-att")
	imageID := testAccImageID()
	volumeSize := testAccVolumeSize(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVolumeAttachmentResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, volumeName, imageID, volumeSize),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_volume_attachment.test", "id"),
					resource.TestCheckResourceAttrPair("nayatel_volume_attachment.test", "volume_id", "nayatel_volume.test", "id"),
					resource.TestCheckResourceAttrPair("nayatel_volume_attachment.test", "instance_id", "nayatel_instance.test", "id"),
				),
			},
			{
				ResourceName:            "nayatel_volume_attachment.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccCompositeImportID("nayatel_volume_attachment.test", "volume_id", "instance_id"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"device"},
			},
		},
	})
}

func testAccVolumeAttachmentResourceConfig_basic(sshKeyName, publicKey, routerName, instanceName, volumeName, imageID string, volumeSize int) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_ssh_key" "test" {
  name       = %q
  public_key = %q
}

resource "nayatel_network" "test" {
  bandwidth_limit = 1
}

resource "nayatel_router" "test" {
  name      = %q
  subnet_id = nayatel_network.test.subnet_id
}

resource "nayatel_instance" "test" {
  name            = %q
  image_id        = %q
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.test.id
  ssh_fingerprint = nayatel_ssh_key.test.fingerprint

  depends_on = [nayatel_router.test]
}

resource "nayatel_volume" "test" {
  name = %q
  size = %d
}

resource "nayatel_volume_attachment" "test" {
  volume_id   = nayatel_volume.test.id
  instance_id = nayatel_instance.test.id
}
`, sshKeyName, publicKey, routerName, instanceName, imageID, volumeName, volumeSize)
}
