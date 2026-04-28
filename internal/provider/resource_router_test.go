// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/matifali/terraform-provider-nayatel/internal/client"
)

func TestRouterDeleteErrorMentionsActiveInterface(t *testing.T) {
	liveMessage := `API error (status 400): {"message":"Router router-123 still has active ports"}`
	if !routerDeleteErrorMentionsActiveInterface(fmt.Errorf("%s", liveMessage)) {
		t.Fatalf("routerDeleteErrorMentionsActiveInterface(%q) = false, want true", liveMessage)
	}

	quotaMessage := `API error (status 400): {"message":"quota exceeded"}`
	if routerDeleteErrorMentionsActiveInterface(fmt.Errorf("%s", quotaMessage)) {
		t.Fatalf("routerDeleteErrorMentionsActiveInterface(%q) = true, want false", quotaMessage)
	}
}

func TestRouterDeleteRetriesInterfaceDetachAfterActiveInterfaceDelete(t *testing.T) {
	ctx := context.Background()

	var detachCalls int
	var deleteCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface":
			detachCalls++
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			_, _ = w.Write([]byte(`{"status":true}`))
		case "/api/iaas/router":
			deleteCalls++
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			if deleteCalls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"Router router-123 still has active ports"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := client.NewClient("test-user", "api-token", client.WithBaseURL(server.URL+"/api"), client.WithHTTPClient(server.Client()))
	r := &RouterResource{client: c}
	if err := r.deleteRouterWithInterfaceRetryBackoffs(ctx, "router-123", "subnet-abc", []time.Duration{0, 0}); err != nil {
		t.Fatalf("deleteRouterWithInterfaceRetryBackoffs returned error: %v", err)
	}

	if detachCalls != 2 {
		t.Fatalf("detachCalls = %d, want 2", detachCalls)
	}
	if deleteCalls != 2 {
		t.Fatalf("deleteCalls = %d, want 2", deleteCalls)
	}
}

func TestAccRouterResource_basic(t *testing.T) {
	name := testAccName("tf-acc-router")
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckNetworkBandwidth(t, bandwidth) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRouterResourceConfig_basic(name, bandwidth),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("nayatel_router.test", "id"),
					resource.TestCheckResourceAttr("nayatel_router.test", "name", name),
					resource.TestCheckResourceAttrSet("nayatel_router.test", "status"),
					resource.TestCheckResourceAttrPair("nayatel_router.test", "subnet_id", "nayatel_network.test", "subnet_id"),
					testAccCheckNestedListContainsResourceAttr("data.nayatel_routers.all", "routers", "id", "nayatel_router.test", "id"),
				),
			},
			{
				ResourceName:            "nayatel_router.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"subnet_id"},
			},
		},
	})
}

func testAccRouterResourceConfig_basic(name string, bandwidth int) string {
	return fmt.Sprintf(`
provider "nayatel" {}

resource "nayatel_network" "test" {
  bandwidth_limit = %d
}

resource "nayatel_router" "test" {
  name      = %q
  subnet_id = nayatel_network.test.subnet_id
}

data "nayatel_routers" "all" {
  depends_on = [nayatel_router.test]
}
`, bandwidth, name)
}
