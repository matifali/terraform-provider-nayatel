// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"
)

func TestExtractCostFromPreview(t *testing.T) {
	tests := []struct {
		name     string
		preview  map[string]interface{}
		expected float64
	}{
		{
			name: "nested data.charges.total_amount",
			preview: map[string]interface{}{
				"status": true,
				"data": map[string]interface{}{
					"charges": map[string]interface{}{
						"total_amount": 546.0,
					},
				},
			},
			expected: 546.0,
		},
		{
			name: "nested data.charge",
			preview: map[string]interface{}{
				"status": true,
				"data": map[string]interface{}{
					"charge": 4438.0,
				},
			},
			expected: 4438.0,
		},
		{
			name: "top-level charge",
			preview: map[string]interface{}{
				"status": true,
				"charge": 1372.0,
			},
			expected: 1372.0,
		},
		{
			name: "top-level monthly_cost",
			preview: map[string]interface{}{
				"status":       true,
				"monthly_cost": 2500.0,
			},
			expected: 2500.0,
		},
		{
			// total_amount wins as soon as the key is present, even at 0;
			// there is no fall-through to other cost fields.
			name: "zero total_amount is returned, not skipped",
			preview: map[string]interface{}{
				"status": true,
				"data": map[string]interface{}{
					"charges": map[string]interface{}{
						"total_amount": 0.0,
					},
					"charge": 4438.0,
				},
			},
			expected: 0.0,
		},
		{
			name: "no cost field",
			preview: map[string]interface{}{
				"status": true,
			},
			expected: 0.0,
		},
		{
			name:     "nil preview",
			preview:  nil,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := ExtractCostFromPreview(tt.preview)
			if cost != tt.expected {
				t.Errorf("ExtractCostFromPreview() = %v, want %v", cost, tt.expected)
			}
		})
	}
}
