// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	nayatelclient "github.com/matifali/terraform-provider-nayatel/internal/client"
)

func TestSSHKeyResourceDeleteStaleNotFoundReturnsDiagnostic(t *testing.T) {
	resp := testSSHKeyResourceDeleteWithListResponse(t, http.StatusNotFound, `{"ssh_keys":[{"name":"tf-acc-key","fingerprint":"fp"}]}`)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned no diagnostics for stale not-found delete while key still exists")
	}
}

func TestSSHKeyResourceDeleteGenuineNotFoundSucceeds(t *testing.T) {
	resp := testSSHKeyResourceDeleteWithListResponse(t, http.StatusNotFound, `{"ssh_keys":[]}`)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned diagnostics for genuine not-found delete: %v", resp.Diagnostics)
	}
}

func testSSHKeyResourceDeleteWithListResponse(t *testing.T, deleteStatus int, listResponse string) frameworkresource.DeleteResponse {
	t.Helper()

	ctx := context.Background()
	const keyName = "tf-acc-key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/user/test-user/ssh":
			switch r.Method {
			case http.MethodDelete:
				w.WriteHeader(deleteStatus)
				_, _ = fmt.Fprintf(w, `{"message":"delete failed"}`)
			case http.MethodGet:
				_, _ = w.Write([]byte(listResponse))
			default:
				t.Errorf("unexpected method %s for %s", r.Method, r.URL.Path)
				http.NotFound(w, r)
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resource := &SSHKeyResource{
		client: nayatelclient.NewClient("test-user", "api-token", nayatelclient.WithBaseURL(server.URL+"/api"), nayatelclient.WithHTTPClient(server.Client())),
	}

	var schemaResp frameworkresource.SchemaResponse
	resource.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, &SSHKeyResourceModel{
		ID:          types.StringValue(keyName),
		Name:        types.StringValue(keyName),
		PublicKey:   types.StringValue("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest"),
		Fingerprint: types.StringValue("fp"),
	})
	if diags.HasError() {
		t.Fatalf("failed to set test state: %v", diags)
	}

	var resp frameworkresource.DeleteResponse
	resource.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &resp)
	return resp
}
