package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSSHKeyDeleteUsesCollectionEndpointAndBody(t *testing.T) {
	ctx := context.Background()
	const keyName = "tf-acc-key"

	deleteCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/user/test-user/ssh":
			deleteCalled = true
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want %s", r.Method, http.MethodDelete)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
				return
			}

			var payload SSHKeyDeleteRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("failed to decode request body %q: %v", string(body), err)
			}
			if payload.Name != keyName {
				t.Errorf("payload name = %q, want %q", payload.Name, keyName)
			}

			_, _ = w.Write([]byte(`{"status":true}`))
		case "/api/user/test-user/ssh/" + keyName:
			t.Errorf("unexpected legacy SSH key delete path %s", r.URL.Path)
			http.NotFound(w, r)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	c := NewClient("test-user", "api-token", WithBaseURL(server.URL+"/api"), WithHTTPClient(server.Client()))
	if err := c.SSHKeys.Delete(ctx, keyName); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !deleteCalled {
		t.Fatalf("DELETE /api/user/test-user/ssh was not called")
	}
}
