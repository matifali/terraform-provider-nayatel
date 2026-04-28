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

	tokenAttr, ok := resp.Schema.Attributes["token"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("token attribute type = %T, want schema.StringAttribute", resp.Schema.Attributes["token"])
	}
	if !tokenAttr.Sensitive {
		t.Fatalf("token attribute must be sensitive")
	}
	if strings.Contains(tokenAttr.MarkdownDescription, "username/password are not required") {
		t.Fatalf("token description still claims username/password are not required: %s", tokenAttr.MarkdownDescription)
	}
	if !strings.Contains(tokenAttr.MarkdownDescription, "username") {
		t.Fatalf("token description should mention username is still required: %s", tokenAttr.MarkdownDescription)
	}
}

func TestValidateAuthenticationConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		username       string
		password       string
		token          string
		wantValid      bool
		wantAttribute  string
		wantSummary    string
		wantDetailText string
	}{
		{
			name:           "missing all credentials",
			wantAttribute:  "username",
			wantSummary:    "Missing Nayatel API Credentials",
			wantDetailText: "username` plus either `token` or `password",
		},
		{
			name:           "token without username",
			token:          "jwt-token",
			wantAttribute:  "username",
			wantSummary:    "Missing Username",
			wantDetailText: "Username is required even when using a token",
		},
		{
			name:           "username without password or token",
			username:       "user",
			wantAttribute:  "password",
			wantSummary:    "Missing Nayatel API Credentials",
			wantDetailText: "username` plus either `token` or `password",
		},
		{
			name:      "username and password",
			username:  "user",
			password:  "password",
			wantValid: true,
		},
		{
			name:      "username and token",
			username:  "user",
			token:     "jwt-token",
			wantValid: true,
		},
		{
			name:      "username token and password",
			username:  "user",
			password:  "password",
			token:     "jwt-token",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := validateAuthenticationConfig(tt.username, tt.password, tt.token)
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
	if v := os.Getenv("NAYATEL_TOKEN"); v == "" {
		if v := os.Getenv("NAYATEL_PASSWORD"); v == "" {
			t.Fatal("NAYATEL_TOKEN or NAYATEL_PASSWORD must be set with NAYATEL_USERNAME for acceptance tests")
		}
	}
}
