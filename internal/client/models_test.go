// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"testing"
)

func TestVolumeUnmarshalJSON_ListShape(t *testing.T) {
	// Captured live from GET /iaas/user/{u}/project/{p}/volumes: identifies
	// the volume via "volume_id", encodes "bootable" as a JSON bool, and
	// reports attachment via a flat "attached_to" string.
	raw := `{"serial_no":1,"volume_id":"ea5e0064-130d-4761-a457-dff32160ec2a","name":"default","description":"Root volume","size":20,"status":"in-use","volume_type":"SSD","bootable":true,"attached_to":"tf-acc-fip-bootstrap-2950884030323807859"}`

	var v Volume
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal failed: %s", err)
	}

	if v.ID != "ea5e0064-130d-4761-a457-dff32160ec2a" {
		t.Errorf("ID = %q, want volume_id fallback", v.ID)
	}
	if !v.IsBootable() {
		t.Errorf("IsBootable() = false, want true")
	}
	if !v.IsAttached() {
		t.Errorf("IsAttached() = false, want true")
	}
	if got := v.GetAttachedInstanceID(); got != "tf-acc-fip-bootstrap-2950884030323807859" {
		t.Errorf("GetAttachedInstanceID() = %q, want attached_to fallback", got)
	}
	if v.Size != 20 {
		t.Errorf("Size = %d, want 20", v.Size)
	}
}

func TestVolumeUnmarshalJSON_DocumentedShape(t *testing.T) {
	raw := `{"id":"vol-1","name":"default","size":10,"status":"available","bootable":"false","attachments":[{"instance_id":"inst-1"}]}`

	var v Volume
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal failed: %s", err)
	}

	if v.ID != "vol-1" {
		t.Errorf("ID = %q, want vol-1", v.ID)
	}
	if v.IsBootable() {
		t.Errorf("IsBootable() = true, want false")
	}
	if got := v.GetAttachedInstanceID(); got != "inst-1" {
		t.Errorf("GetAttachedInstanceID() = %q, want inst-1", got)
	}
}

func TestVolumeUnmarshalJSON_ListDecode(t *testing.T) {
	raw := `[{"volume_id":"a","name":"one","bootable":false},{"volume_id":"b","name":"two","bootable":true}]`

	var volumes []Volume
	if err := json.Unmarshal([]byte(raw), &volumes); err != nil {
		t.Fatalf("unmarshal failed: %s", err)
	}
	if len(volumes) != 2 {
		t.Fatalf("len(volumes) = %d, want 2", len(volumes))
	}
	if volumes[0].ID != "a" || volumes[1].ID != "b" {
		t.Errorf("unexpected IDs: %q, %q", volumes[0].ID, volumes[1].ID)
	}
}
