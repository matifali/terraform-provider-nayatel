// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import "testing"

func TestDecodeList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		resp      string
		keys      []string
		wantLen   int
		wantError bool
	}{
		{name: "bare array", resp: `[{"id":"a"},{"id":"b"}]`, keys: []string{"instances"}, wantLen: 2},
		{name: "empty array", resp: `[]`, keys: []string{"instances"}, wantLen: 0},
		{name: "wrapped array", resp: `{"instances":[{"id":"a"}]}`, keys: []string{"instances"}, wantLen: 1},
		{name: "wrapped empty array", resp: `{"instances":[]}`, keys: []string{"instances"}, wantLen: 0},
		{name: "second key", resp: `{"keys":[{"id":"a"}]}`, keys: []string{"ssh_keys", "keys"}, wantLen: 1},
		{name: "error payload is not an empty list", resp: `{"error":"instance no longer available"}`, keys: []string{"instances"}, wantError: true},
		{name: "status-false payload", resp: `{"status":false,"message":"invalid token"}`, keys: []string{"instances"}, wantError: true},
		{name: "sole message field is an empty list", resp: `{"message":"No instances found."}`, keys: []string{"instances"}, wantLen: 0},
		{name: "invalid json", resp: `not json`, keys: []string{"instances"}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			items, err := decodeList[Instance]([]byte(tt.resp), tt.keys...)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got %d items", len(items))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if len(items) != tt.wantLen {
				t.Fatalf("expected %d items, got %d", tt.wantLen, len(items))
			}
		})
	}
}
