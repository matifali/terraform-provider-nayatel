package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// FloatingIPService handles floating IP operations.
type FloatingIPService struct {
	client *Client
}

// List returns all floating IPs in the project.
func (s *FloatingIPService) List(ctx context.Context) ([]FloatingIP, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/project/%s/floating-ip", projectID))
	if err != nil {
		return nil, err
	}

	var floatingIPs []FloatingIP
	if err := json.Unmarshal(resp, &floatingIPs); err != nil {
		var result struct {
			FloatingIPs []FloatingIP `json:"floating_ips"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		floatingIPs = result.FloatingIPs
	}

	return floatingIPs, nil
}

// Allocate allocates new floating IP(s) - raw API call without safety checks.
// Prefer SafeAllocate for production use.
func (s *FloatingIPService) Allocate(ctx context.Context, count int) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	if count == 0 {
		count = 1
	}

	payload := map[string]int{"count": count}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/floating-ips/add", s.client.Username, projectID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// SafeAllocate allocates floating IP(s) with safety checks:
// 1. Calls Preview API to get prorated cost (aborts if fails)
// 2. Checks account balance with retries (handles Nayatel API 0 balance glitch)
// 3. Only then proceeds with allocation (with retries for transient errors).
func (s *FloatingIPService) SafeAllocate(ctx context.Context, count int) (*APIResponse, error) {
	if count == 0 {
		count = 1
	}

	// Step 1: Get required cost from preview API
	preview, err := s.Preview(ctx, count)
	if err != nil {
		return nil, fmt.Errorf("preview failed (API may be having issues, aborting to protect against unwanted charges): %w", err)
	}

	// Extract prorated cost from preview response
	requiredBalance := ExtractCostFromPreview(preview)
	if requiredBalance <= 0 {
		return nil, fmt.Errorf("unable to determine cost from preview response (API may be having issues, aborting to protect against unwanted charges)")
	}

	// Step 2: Check balance with retries (uses common helper)
	if err := s.client.VerifyBalance(ctx, requiredBalance, "floating IP"); err != nil {
		return nil, err
	}

	// Step 3: Allocate with retries for transient errors
	var allocErr error
	var apiResp *APIResponse
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		apiResp, allocErr = s.Allocate(ctx, count)
		if allocErr == nil {
			return apiResp, nil
		}

		// Retry on transient balance errors
		if IsInsufficientBalance(allocErr) {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt*3) * time.Second)
				continue
			}
		}
		break
	}

	return nil, allocErr
}

// ExtractCostFromPreview extracts the cost from various API preview response formats.
// Used by all billable resources that have preview endpoints.
func ExtractCostFromPreview(preview map[string]interface{}) float64 {
	if preview == nil {
		return 0
	}

	// Check nested data.charges.total_amount (Nayatel floating_ip/network API format)
	if data, ok := preview["data"].(map[string]interface{}); ok {
		if charges, ok := data["charges"].(map[string]interface{}); ok {
			if c, ok := charges["total_amount"].(float64); ok {
				return c
			}
		}
		// Check data.charge (Nayatel instance API format)
		if c, ok := data["charge"].(float64); ok {
			return c
		}
	}

	// Fallback to top-level fields
	if c, ok := preview["charge"].(float64); ok {
		return c
	}
	if c, ok := preview["monthly_cost"].(float64); ok {
		return c
	}

	return 0
}

// Preview previews floating IP cost.
func (s *FloatingIPService) Preview(ctx context.Context, count int) (map[string]interface{}, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	if count == 0 {
		count = 1
	}

	payload := map[string]int{"count": count}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/floating-ips/preview", s.client.Username, projectID), payload)
	if err != nil {
		return nil, err
	}

	var preview map[string]interface{}
	if err := json.Unmarshal(resp, &preview); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return preview, nil
}

// Attach attaches a floating IP to an instance (allocates a new IP).
func (s *FloatingIPService) Attach(ctx context.Context, instanceID string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/project/%s/instance/%s/floating-ip", projectID, instanceID), nil)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// AttachIP attaches a specific floating IP to an instance.
func (s *FloatingIPService) AttachIP(ctx context.Context, instanceID string, floatingIP string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{"floating_ip": floatingIP}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/project/%s/instance/%s/floating-ip", projectID, instanceID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Detach detaches a floating IP from an instance.
func (s *FloatingIPService) Detach(ctx context.Context, instanceID string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Delete(ctx, fmt.Sprintf("/iaas/project/%s/instance/%s/floating-ip", projectID, instanceID), nil)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Release releases a floating IP.
func (s *FloatingIPService) Release(ctx context.Context, floatingIP string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{"floating_ip": floatingIP}

	resp, err := s.client.Delete(ctx, fmt.Sprintf("/iaas/project/%s/floating-ip", projectID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// FindByIP finds a floating IP by its address.
func (s *FloatingIPService) FindByIP(ctx context.Context, ipAddress string) (*FloatingIP, error) {
	floatingIPs, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, fip := range floatingIPs {
		if fip.GetIPAddress() == ipAddress {
			return &fip, nil
		}
	}

	return nil, nil
}
