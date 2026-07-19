// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These endpoints and body shapes were captured from live browser traffic
// against cloud.nayatel.com on 2026-07-18; see the "iaas-browser-capture"
// memory. volumes.go previously called different paths/methods entirely.

func newVolumeTestClient(server *httptest.Server) *Client {
	return NewClient("test-user", "api-token",
		WithBaseURL(server.URL+"/api"),
		WithHTTPClient(server.Client()),
		WithProjectID("project-1"),
	)
}

func csrfHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/api/csrf-token" {
		w.Header().Set("X-CSRF-Token", "csrf-test")
		_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		return true
	}
	return false
}

func TestVolumeCreateUsesVolumesEndpoint(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/project/project-1/volumes" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["name"] != "tf-test-volume" {
			t.Errorf("body[name] = %v, want tf-test-volume", body["name"])
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"Volume 'tf-test-volume' created successfully."}`))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	volume, err := c.Volumes.Create(ctx, &VolumeCreateRequest{Name: "tf-test-volume", Size: 10})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if volume.ID != "" {
		t.Errorf("volume.ID = %q, want empty (create response carries no volume object)", volume.ID)
	}
}

func TestVolumeAttachUsesInstanceScopedEndpoint(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/instance/instance-1/volume" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["volume_id"] != "volume-1" {
			t.Errorf("body[volume_id] = %q, want volume-1", body["volume_id"])
		}
		if _, ok := body["instance_id"]; ok {
			t.Errorf("body contains instance_id, want it omitted (instance is in the path)")
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"Volume attached to instance successfully."}`))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	if _, err := c.Volumes.Attach(ctx, "volume-1", "instance-1"); err != nil {
		t.Fatalf("Attach returned error: %v", err)
	}
}

func TestVolumeDetachUsesDeleteOnInstanceScopedEndpoint(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/instance/instance-1/volume" || r.Method != http.MethodDelete {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["volume_id"] != "volume-1" {
			t.Errorf("body[volume_id] = %q, want volume-1", body["volume_id"])
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"Volume detached from instance successfully."}`))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	if _, err := c.Volumes.Detach(ctx, "volume-1", "instance-1"); err != nil {
		t.Fatalf("Detach returned error: %v", err)
	}
}

func TestVolumeExtendSendsUpsizeDelta(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/project/project-1/volumes/upsize" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["volume_id"] != "volume-1" {
			t.Errorf("body[volume_id] = %q, want volume-1", body["volume_id"])
		}
		if body["add_size"] != "10" {
			t.Errorf("body[add_size] = %q, want \"10\" (string delta, not absolute new size)", body["add_size"])
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"Volume upgraded successfully."}`))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	// Growing a 10GB volume to 20GB should send add_size=10, not new_size=20.
	if _, err := c.Volumes.Extend(ctx, "volume-1", 10); err != nil {
		t.Fatalf("Extend returned error: %v", err)
	}
}

func TestVolumeDeleteUsesPluralEndpointWithBodyID(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/project/project-1/volumes" || r.Method != http.MethodDelete {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["volume_id"] != "volume-1" {
			t.Errorf("body[volume_id] = %q, want volume-1", body["volume_id"])
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"Volume 'tf-test-volume' deleted successfully. Refunded: Rs. 29"}`))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	if _, err := c.Volumes.Delete(ctx, "volume-1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestVolumeCreateReturnsErrorOnStatusFalse(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		_, _ = w.Write([]byte(`{"status":false,"message":"Insufficient balance."}`))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	_, err := c.Volumes.Create(ctx, &VolumeCreateRequest{Name: "tf-test-volume", Size: 10})
	if err == nil {
		t.Fatal("Create returned nil error, want an error for status:false response")
	}
}

func TestResolveAttachedInstanceIDMapsNameToID(t *testing.T) {
	ctx := context.Background()

	// The volumes list reports the attached instance by name ("attached_to"),
	// not by ID, so ResolveAttachedInstanceID must cross-reference the
	// instance list to recover the real instance ID.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path == "/api/iaas/project/project-1" {
			_, _ = w.Write([]byte(`[{"Instance ID":"e1597c53-1fef-4d3e-a3fa-f5d3cdaa5bc4","Name":"tf-test-instance"}]`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	v := &Volume{Attachments: []VolumeAttachment{{InstanceID: "tf-test-instance"}}}

	id, err := c.Volumes.ResolveAttachedInstanceID(ctx, v)
	if err != nil {
		t.Fatalf("ResolveAttachedInstanceID returned error: %v", err)
	}
	if id != "e1597c53-1fef-4d3e-a3fa-f5d3cdaa5bc4" {
		t.Errorf("id = %q, want the resolved instance UUID, not the raw name", id)
	}
}

func TestResolveAttachedInstanceIDErrorsWhenInstanceNameNotFound(t *testing.T) {
	ctx := context.Background()

	// If the attached instance's name no longer resolves (renamed, deleted,
	// or a stale attached_to value), ResolveAttachedInstanceID must error
	// rather than return the raw name as if it were a usable instance ID --
	// callers key URLs and state comparisons on this value.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path == "/api/iaas/project/project-1" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	v := &Volume{Attachments: []VolumeAttachment{{InstanceID: "renamed-or-deleted-instance"}}}

	id, err := c.Volumes.ResolveAttachedInstanceID(ctx, v)
	if err == nil {
		t.Fatalf("ResolveAttachedInstanceID returned nil error and id %q, want an error", id)
	}
}

func TestVolumeListDecodesBareArrayWithLiveFieldNames(t *testing.T) {
	ctx := context.Background()

	const listBody = `[
		{"serial_no":1,"volume_id":"volume-1","name":"tf-test-volume","description":"","size":10,"status":"in-use","volume_type":"ssd","bootable":false,"attached_to":"instance-1"}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/project/project-1/volumes" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(listBody))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)
	volumes, err := c.Volumes.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(volumes) != 1 {
		t.Fatalf("len(volumes) = %d, want 1", len(volumes))
	}
	v := volumes[0]
	if v.ID != "volume-1" {
		t.Errorf("v.ID = %q, want volume-1", v.ID)
	}
	if !v.IsAttached() || v.GetAttachedInstanceID() != "instance-1" {
		t.Errorf("v.IsAttached()/GetAttachedInstanceID() = %v/%q, want true/instance-1", v.IsAttached(), v.GetAttachedInstanceID())
	}
}

// Get doesn't hit a singular volume endpoint: confirmed live (direct request
// and network panel both showed a 404 with an HTML body, not JSON), so it
// scans the list instead. This test asserts only the list endpoint is ever
// called, and that a missing ID returns (nil, nil), matching the other
// services' FindByID not-found convention.
func TestVolumeGetScansListInsteadOfSingularEndpoint(t *testing.T) {
	ctx := context.Background()

	const listBody = `[{"volume_id":"volume-1","name":"tf-test-volume","size":10,"status":"available","bootable":false,"attached_to":"-"}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if csrfHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/iaas/user/test-user/project/project-1/volumes" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s (Get should only call the list endpoint)", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(listBody))
	}))
	defer server.Close()

	c := newVolumeTestClient(server)

	v, err := c.Volumes.Get(ctx, "volume-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if v.ID != "volume-1" {
		t.Errorf("v.ID = %q, want volume-1", v.ID)
	}

	missing, err := c.Volumes.Get(ctx, "does-not-exist")
	if err != nil || missing != nil {
		t.Errorf("Get(\"does-not-exist\") = (%v, %v), want (nil, nil), matching the other services' FindByID not-found convention", missing, err)
	}
}
