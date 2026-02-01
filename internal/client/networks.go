package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// NetworkService handles network operations.
type NetworkService struct {
	client *Client
}

// List returns all networks in the project.
func (s *NetworkService) List(ctx context.Context) ([]Network, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/project/%s/networks", projectID))
	if err != nil {
		return nil, err
	}

	// Response could be array or object with networks field
	var networks []Network
	if err := json.Unmarshal(resp, &networks); err != nil {
		// Try as object with networks field
		var result struct {
			Networks []Network `json:"networks"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		networks = result.Networks
	}

	return networks, nil
}

// Create creates a new network.
func (s *NetworkService) Create(ctx context.Context, req *NetworkCreateRequest) (*NetworkCreateResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/project/%s/networks", projectID), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var result NetworkCreateResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Preview previews network cost.
func (s *NetworkService) Preview(ctx context.Context, req *NetworkCreateRequest) (map[string]interface{}, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/networks/preview", s.client.Username, projectID), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var preview map[string]interface{}
	if err := json.Unmarshal(resp, &preview); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return preview, nil
}

// SafeCreate creates a network with safety checks:
// 1. Calls Preview API to get prorated cost (aborts if fails)
// 2. Checks account balance with retries (handles Nayatel API 0 balance glitch)
// 3. Only then proceeds with creation (with retries for transient errors).
func (s *NetworkService) SafeCreate(ctx context.Context, req *NetworkCreateRequest) (*NetworkCreateResponse, error) {
	// Step 1: Get required cost from preview API
	preview, err := s.Preview(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("preview failed (API may be having issues, aborting to protect against unwanted charges): %w", err)
	}

	// Extract prorated cost from preview response
	requiredBalance := ExtractCostFromPreview(preview)
	if requiredBalance <= 0 {
		return nil, fmt.Errorf("unable to determine cost from preview response (API may be having issues, aborting to protect against unwanted charges)")
	}

	// Step 2: Check balance with retries (uses common helper)
	if err := s.client.VerifyBalance(ctx, requiredBalance, "network"); err != nil {
		return nil, err
	}

	// Step 3: Create with retries for transient errors
	var createErr error
	var result *NetworkCreateResponse
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, createErr = s.Create(ctx, req)
		if createErr == nil {
			return result, nil
		}

		// Retry on transient balance errors
		if IsInsufficientBalance(createErr) {
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt*3) * time.Second)
				continue
			}
		}
		break
	}

	return nil, createErr
}

// Delete deletes a network.
func (s *NetworkService) Delete(ctx context.Context, networkID string) (*APIResponse, error) {
	payload := map[string]string{"network_id": networkID}

	resp, err := s.client.Delete(ctx, "/iaas/networks/project", payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// FindByID finds a network by ID.
func (s *NetworkService) FindByID(ctx context.Context, networkID string) (*Network, error) {
	networks, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, network := range networks {
		if network.ID == networkID {
			return &network, nil
		}
	}

	return nil, nil
}
