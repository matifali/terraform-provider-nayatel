// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	nayatelclient "github.com/matifali/terraform-provider-nayatel/internal/client"
)

// The live API has been observed to accept delete_root_volume=true yet leave
// the root volume behind, detached and still billed. Delete must notice the
// leftover and remove it directly.
func TestInstanceResourceDeleteRemovesLeakedRootVolume(t *testing.T) {
	var volumeDeleted, sawDeleteRootVolumeFlag atomic.Bool

	resp := testInstanceResourceDelete(t, true, &volumeDeleted, &sawDeleteRootVolumeFlag)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned error diagnostics: %v", resp.Diagnostics)
	}
	if len(resp.Diagnostics.Warnings()) != 0 {
		t.Fatalf("Delete returned warnings for a recovered leak: %v", resp.Diagnostics)
	}
	if !sawDeleteRootVolumeFlag.Load() {
		t.Fatal("instance delete request did not include delete_root_volume=true")
	}
	if !volumeDeleted.Load() {
		t.Fatal("leaked root volume was not deleted")
	}
}

func TestInstanceResourceDeleteKeepsRootVolumeWhenConfigured(t *testing.T) {
	var volumeDeleted, sawDeleteRootVolumeFlag atomic.Bool

	resp := testInstanceResourceDelete(t, false, &volumeDeleted, &sawDeleteRootVolumeFlag)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete returned error diagnostics: %v", resp.Diagnostics)
	}
	if sawDeleteRootVolumeFlag.Load() {
		t.Fatal("instance delete request included delete_root_volume=true despite delete_root_volume_on_destroy = false")
	}
	if volumeDeleted.Load() {
		t.Fatal("root volume was deleted despite delete_root_volume_on_destroy = false")
	}
}

// testInstanceResourceDelete drives InstanceResource.Delete against a mock
// API whose instance delete always leaks the root volume: the volume stays
// in the project list (status "available") until deleted directly.
func testInstanceResourceDelete(t *testing.T, deleteRootVolume bool, volumeDeleted, sawDeleteRootVolumeFlag *atomic.Bool) frameworkresource.DeleteResponse {
	t.Helper()

	ctx := context.Background()
	const (
		instanceID   = "inst-1"
		instanceName = "web-server"
		volumeID     = "vol-root-1"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/csrf-token":
			w.Header().Set("X-CSRF-Token", "csrf-test")
			_, _ = w.Write([]byte(`{"token":"csrf-test"}`))
		case "/api/iaas/user/test-user/project":
			_, _ = w.Write([]byte(`{"projects":[{"id":"proj-1"}]}`))
		case "/api/iaas/user/test-user/project/proj-1/volumes":
			switch r.Method {
			case http.MethodGet:
				if volumeDeleted.Load() {
					_, _ = w.Write([]byte(`{"volumes":[]}`))
					return
				}
				_, _ = w.Write([]byte(`{"volumes":[{"id":"` + volumeID + `","name":"default","size":20,"status":"available","bootable":"true","attached_to":"` + instanceName + `"}]}`))
			case http.MethodDelete:
				volumeDeleted.Store(true)
				_, _ = w.Write([]byte(`{"status":true,"message":"deleted"}`))
			default:
				t.Errorf("unexpected method %s for %s", r.Method, r.URL.Path)
				http.NotFound(w, r)
			}
		case "/api/iaas/instance/" + instanceID + "/state":
			_, _ = w.Write([]byte(`{"status":true,"message":"stopped"}`))
		case "/api/iaas/user/test-user/instance/" + instanceID + "/delete":
			if r.URL.Query().Get("delete_root_volume") == "true" {
				sawDeleteRootVolumeFlag.Store(true)
			}
			_, _ = w.Write([]byte(`{"status":true,"message":"deleted"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resource := &InstanceResource{}
	resource.client = nayatelclient.NewClient("test-user", "api-token", nayatelclient.WithBaseURL(server.URL+"/api"), nayatelclient.WithHTTPClient(server.Client()))

	var schemaResp frameworkresource.SchemaResponse
	resource.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, &InstanceResourceModel{
		ID:                        types.StringValue(instanceID),
		Name:                      types.StringValue(instanceName),
		ImageID:                   types.StringValue("img-1"),
		CPU:                       types.Int64Value(2),
		RAM:                       types.Int64Value(2),
		Disk:                      types.Int64Value(20),
		NetworkID:                 types.StringValue("net-1"),
		SSHFingerprint:            types.StringValue("fp"),
		DeleteRootVolumeOnDestroy: types.BoolValue(deleteRootVolume),
	})
	if diags.HasError() {
		t.Fatalf("failed to set test state: %v", diags)
	}

	var resp frameworkresource.DeleteResponse
	resource.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &resp)
	return resp
}
