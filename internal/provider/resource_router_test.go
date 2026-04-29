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
	var postDetachCalls int
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
		case "/api/iaas/router/router-123/interface/remove":
			postDetachCalls++
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
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
	if err := r.deleteRouterWithInterfaceRetryBackoffs(ctx, "router-123", "subnet-abc", false, []time.Duration{0, 0}); err != nil {
		t.Fatalf("deleteRouterWithInterfaceRetryBackoffs returned error: %v", err)
	}

	if detachCalls != 2 {
		t.Fatalf("detachCalls = %d, want 2", detachCalls)
	}
	if postDetachCalls != 2 {
		t.Fatalf("postDetachCalls = %d, want 2", postDetachCalls)
	}
	if deleteCalls != 2 {
		t.Fatalf("deleteCalls = %d, want 2", deleteCalls)
	}
}

func TestRouterDeleteDoesNotDeleteSubnetNetworkWithoutOptIn(t *testing.T) {
	ctx := context.Background()

	var networkLookupCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface", "/api/iaas/router/router-123/interface/remove":
			_, _ = w.Write([]byte(`{"status":true}`))
		case "/api/iaas/router":
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"Cannot delete router. It has 1 active interface(s). Please detach all interfaces first."}`))
		case "/api/iaas/project/project-123/networks":
			networkLookupCalls++
			_, _ = w.Write([]byte(`[{"id":"network-123","name":"example-network","last_subnet_id":"subnet-abc"}]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := client.NewClient("test-user", "api-token", client.WithBaseURL(server.URL+"/api"), client.WithHTTPClient(server.Client()), client.WithProjectID("project-123"))
	r := &RouterResource{client: c}
	if err := r.deleteRouterWithInterfaceRetryBackoffsAndNetworkFallbacks(ctx, "router-123", "subnet-abc", false, []time.Duration{0}, []time.Duration{0}); err == nil {
		t.Fatalf("deleteRouterWithInterfaceRetryBackoffsAndNetworkFallbacks returned nil error")
	}

	if networkLookupCalls != 0 {
		t.Fatalf("networkLookupCalls = %d, want 0", networkLookupCalls)
	}
}

func TestRouterDeleteFallsBackToDeletingSubnetNetwork(t *testing.T) {
	ctx := context.Background()

	var detachCalls int
	var postDetachCalls int
	var deleteCalls int
	var networkDeleteCalls int
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
		case "/api/iaas/router/router-123/interface/remove":
			postDetachCalls++
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
			}
			_, _ = w.Write([]byte(`{"status":true}`))
		case "/api/iaas/router":
			deleteCalls++
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			if deleteCalls <= 2 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"Cannot delete router. It has 1 active interface(s). Please detach all interfaces first."}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":true}`))
		case "/api/iaas/project/project-123/networks":
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
			}
			_, _ = w.Write([]byte(`[{"id":"network-123","name":"example-network","last_subnet_id":"subnet-abc"}]`))
		case "/api/iaas/networks/project":
			networkDeleteCalls++
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			_, _ = w.Write([]byte(`{"status":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := client.NewClient("test-user", "api-token", client.WithBaseURL(server.URL+"/api"), client.WithHTTPClient(server.Client()), client.WithProjectID("project-123"))
	r := &RouterResource{client: c}
	if err := r.deleteRouterWithInterfaceRetryBackoffsAndNetworkFallbacks(ctx, "router-123", "subnet-abc", true, []time.Duration{0, 0}, []time.Duration{0}); err != nil {
		t.Fatalf("deleteRouterWithInterfaceRetryBackoffsAndNetworkFallbacks returned error: %v", err)
	}

	if detachCalls != 2 {
		t.Fatalf("detachCalls = %d, want 2", detachCalls)
	}
	if postDetachCalls != 2 {
		t.Fatalf("postDetachCalls = %d, want 2", postDetachCalls)
	}
	if networkDeleteCalls != 1 {
		t.Fatalf("networkDeleteCalls = %d, want 1", networkDeleteCalls)
	}
	if deleteCalls != 3 {
		t.Fatalf("deleteCalls = %d, want 3", deleteCalls)
	}
}

func TestAccRouterResource_basic(t *testing.T) {
	name := testAccName("tf-acc-router")
	bandwidth := testAccNetworkBandwidthLimit(t)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckRouterTests(t)
			testAccPreCheckNetworkBandwidth(t, bandwidth)
		},
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
				ImportStateVerifyIgnore: []string{"subnet_id", "force_delete_network_on_destroy"},
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
  name                            = %q
  subnet_id                       = nayatel_network.test.subnet_id
  force_delete_network_on_destroy = true
}

data "nayatel_routers" "all" {
  depends_on = [nayatel_router.test]
}
`, bandwidth, name)
}
