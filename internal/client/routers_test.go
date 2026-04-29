package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterRemoveInterfaceUsesDeleteEndpointAndPostRemoveEndpoint(t *testing.T) {
	ctx := context.Background()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":true}`))
		case "/api/iaas/router/router-123/interface/remove":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if _, err := c.Routers.RemoveInterface(ctx, "router-123", "subnet-abc"); err != nil {
		t.Fatalf("RemoveInterface returned error: %v", err)
	}

	wantCalls := []string{
		"DELETE /api/iaas/router/router-123/interface",
		"POST /api/iaas/router/router-123/interface/remove",
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v, want %v", calls, wantCalls)
		}
	}
}

func TestRouterRemoveInterfaceFallsBackToPostRemove(t *testing.T) {
	ctx := context.Background()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			assertRouterInterfacePayload(t, r)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case "/api/iaas/router/router-123/interface/remove":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if _, err := c.Routers.RemoveInterface(ctx, "router-123", "subnet-abc"); err != nil {
		t.Fatalf("RemoveInterface returned error: %v", err)
	}

	wantCalls := []string{
		"DELETE /api/iaas/router/router-123/interface",
		"POST /api/iaas/router/router-123/interface/remove",
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v, want %v", calls, wantCalls)
		}
	}
}

func TestRouterRemoveInterfaceFallsBackWhenDeleteResponseStatusFalse(t *testing.T) {
	ctx := context.Background()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":false,"message":"delete endpoint did not remove interface"}`))
		case "/api/iaas/router/router-123/interface/remove":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if _, err := c.Routers.RemoveInterface(ctx, "router-123", "subnet-abc"); err != nil {
		t.Fatalf("RemoveInterface returned error: %v", err)
	}

	wantCalls := []string{
		"DELETE /api/iaas/router/router-123/interface",
		"POST /api/iaas/router/router-123/interface/remove",
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v, want %v", calls, wantCalls)
		}
	}
}

func TestRouterRemoveInterfaceFallsBackToPostRemoveOnBadRequest(t *testing.T) {
	ctx := context.Background()

	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			assertRouterInterfacePayload(t, r)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"delete endpoint rejected interface removal"}`))
		case "/api/iaas/router/router-123/interface/remove":
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":true}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if _, err := c.Routers.RemoveInterface(ctx, "router-123", "subnet-abc"); err != nil {
		t.Fatalf("RemoveInterface returned error: %v", err)
	}

	wantCalls := []string{
		"DELETE /api/iaas/router/router-123/interface",
		"POST /api/iaas/router/router-123/interface/remove",
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v, want %v", calls, wantCalls)
		}
	}
}

func TestRouterRemoveInterfaceFallbackStatusFalseReturnsUsefulError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router/router-123/interface":
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			assertRouterInterfacePayload(t, r)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"delete endpoint rejected interface removal"}`))
		case "/api/iaas/router/router-123/interface/remove":
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
			}
			assertRouterInterfacePayload(t, r)
			_, _ = w.Write([]byte(`{"status":false,"message":"router still has active interface"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	_, err := c.Routers.RemoveInterface(ctx, "router-123", "subnet-abc")
	if err == nil {
		t.Fatalf("RemoveInterface returned nil error")
	}

	errText := err.Error()
	for _, want := range []string{"status=false", "router still has active interface", "delete failed", "post failed"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("error %q does not contain %q", errText, want)
		}
	}
}

func assertRouterInterfacePayload(t *testing.T, r *http.Request) {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("failed to read request body: %v", err)
		return
	}

	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Errorf("failed to decode request body %q: %v", string(body), err)
		return
	}
	if got, want := payload["subnet_id"], "subnet-abc"; got != want {
		t.Errorf("subnet_id = %q, want %q", got, want)
	}
	if len(payload) != 1 {
		t.Errorf("payload = %v, want only subnet_id", payload)
	}
}

func TestRouterDeleteStillUsesRouterPayload(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/router":
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
				return
			}
			var payload map[string]string
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("failed to decode request body %q: %v", string(body), err)
				return
			}
			if got, want := payload["router_id"], "router-123"; got != want {
				t.Errorf("router_id = %q, want %q", got, want)
			}
			_, _ = fmt.Fprint(w, `{"status":true}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if _, err := c.Routers.Delete(ctx, "router-123"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
