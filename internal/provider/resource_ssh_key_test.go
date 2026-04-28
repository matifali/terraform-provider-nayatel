// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSSHKeyResource_basic(t *testing.T) {
	name := testAccName("tf-acc-key")
	publicKey := testAccPublicKey(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSHKeyResourceConfig_basic(name, publicKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nayatel_ssh_key.test", "id", name),
					resource.TestCheckResourceAttr("nayatel_ssh_key.test", "name", name),
					resource.TestCheckResourceAttrSet("nayatel_ssh_key.test", "fingerprint"),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_ssh_keys.all", "keys", "name", "nayatel_ssh_key.test", "name"),
				),
			},
			{
				ResourceName:            "nayatel_ssh_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"public_key"},
			},
		},
	})
}

func testAccSSHKeyResourceConfig_basic(name, publicKey string) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_ssh_key" "test" {
  name       = %q
  public_key = %q
}

data "nayatel_ssh_keys" "all" {
  depends_on = [nayatel_ssh_key.test]
}
`, name, publicKey)
}
