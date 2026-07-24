package provider

import "testing"

func TestValidateInstancePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantOK   bool
	}{
		{"valid", "Tvm7nayatel2Kx", true},
		{"too short", "Ab1x", false},
		{"no digit", "TvmNayatelKx", false},
		{"no inner uppercase", "tvm7nayatel2kx", false},
		{"uppercase only first", "Tvm7nayatel2kx", false},
		{"ends in digit", "Tvm7nayatel2K9", false},
		{"ends in special char", "Tvm7nayatel2K!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateInstancePassword(tt.password)
			if (got == "") != tt.wantOK {
				t.Errorf("validateInstancePassword(%q) = %q, want ok=%v", tt.password, got, tt.wantOK)
			}
		})
	}
}
