// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

// TestExtractCostFromPreview tests cost extraction from API preview responses.
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
			cost := extractCostFromPreview(tt.preview)
			if cost != tt.expected {
				t.Errorf("extractCostFromPreview() = %v, want %v", cost, tt.expected)
			}
		})
	}
}

// extractCostFromPreview extracts the cost from various API response formats.
func extractCostFromPreview(preview map[string]interface{}) float64 {
	if preview == nil {
		return 0.0
	}

	var cost float64

	// Check nested data.charges.total_amount (Nayatel floating_ip/network API format)
	if data, ok := preview["data"].(map[string]interface{}); ok {
		if charges, ok := data["charges"].(map[string]interface{}); ok {
			if c, ok := charges["total_amount"].(float64); ok {
				cost = c
			}
		}
		// Check data.charge (Nayatel instance API format)
		if cost == 0 {
			if c, ok := data["charge"].(float64); ok {
				cost = c
			}
		}
	}

	// Fallback to top-level fields
	if cost == 0 {
		if c, ok := preview["charge"].(float64); ok {
			cost = c
		} else if c, ok := preview["monthly_cost"].(float64); ok {
			cost = c
		}
	}

	return cost
}
