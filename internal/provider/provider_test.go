// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"
	"strings"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"nayatel": providerserver.NewProtocol6WithError(New("test")()),
}

func TestProviderSchemaAuthenticationAttributes(t *testing.T) {
	t.Parallel()

	var resp frameworkprovider.SchemaResponse
	(&NayatelProvider{}).Schema(context.Background(), frameworkprovider.SchemaRequest{}, &resp)

	passwordAttr, ok := resp.Schema.Attributes["password"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("password attribute type = %T, want schema.StringAttribute", resp.Schema.Attributes["password"])
	}
	if !passwordAttr.Sensitive {
		t.Fatalf("password attribute must be sensitive")
	}

	if _, exists := resp.Schema.Attributes["token"]; exists {
		t.Fatalf("token attribute should no longer exist; authentication uses username/password only")
	}
}

func TestValidateAuthenticationConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		username       string
		password       string
		wantValid      bool
		wantAttribute  string
		wantSummary    string
		wantDetailText string
	}{
		{
			name:           "missing all credentials",
			wantAttribute:  "username",
			wantSummary:    "Missing Nayatel API Credentials",
			wantDetailText: "username` and `password",
		},
		{
			name:           "username without password",
			username:       "user",
			wantAttribute:  "password",
			wantSummary:    "Missing Nayatel API Credentials",
			wantDetailText: "username` and `password",
		},
		{
			name:           "password without username",
			password:       "password",
			wantAttribute:  "username",
			wantSummary:    "Missing Nayatel API Credentials",
			wantDetailText: "username` and `password",
		},
		{
			name:      "username and password",
			username:  "user",
			password:  "password",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := validateAuthenticationConfig(tt.username, tt.password)
			if tt.wantValid {
				if diag != nil {
					t.Fatalf("validateAuthenticationConfig returned diagnostic: %#v", diag)
				}
				return
			}

			if diag == nil {
				t.Fatalf("validateAuthenticationConfig returned nil diagnostic")
			}
			if got := diag.attribute.String(); got != tt.wantAttribute {
				t.Fatalf("attribute = %q, want %q", got, tt.wantAttribute)
			}
			if diag.summary != tt.wantSummary {
				t.Fatalf("summary = %q, want %q", diag.summary, tt.wantSummary)
			}
			if !strings.Contains(diag.detail, tt.wantDetailText) {
				t.Fatalf("detail = %q, want it to contain %q", diag.detail, tt.wantDetailText)
			}
		})
	}
}

// testAccPreCheck validates the necessary environment variables exist for acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	if v := os.Getenv("NAYATEL_USERNAME"); v == "" {
		t.Fatal("NAYATEL_USERNAME must be set for acceptance tests")
	}
	if v := os.Getenv("NAYATEL_PASSWORD"); v == "" {
		t.Fatal("NAYATEL_PASSWORD must be set with NAYATEL_USERNAME for acceptance tests")
	}
}
