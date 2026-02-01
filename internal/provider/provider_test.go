// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"nayatel": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates the necessary environment variables exist for acceptance tests.
func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("NAYATEL_USERNAME"); v == "" {
		t.Fatal("NAYATEL_USERNAME must be set for acceptance tests")
	}
	if v := os.Getenv("NAYATEL_TOKEN"); v == "" {
		if v := os.Getenv("NAYATEL_PASSWORD"); v == "" {
			t.Fatal("NAYATEL_TOKEN or NAYATEL_PASSWORD must be set for acceptance tests")
		}
	}
}
