package client

import (
	"context"
	"encoding/json"
	"fmt"
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

	return decodeList[Network](resp, "networks")
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

// SafeCreate creates a network via the safeCreate preview/verify-balance wrapper.
func (s *NetworkService) SafeCreate(ctx context.Context, req *NetworkCreateRequest) (*NetworkCreateResponse, error) {
	return safeCreate(ctx, s.client, safeCreateConfig[*NetworkCreateResponse]{
		resourceType: "network",
		preview:      func(ctx context.Context) (map[string]interface{}, error) { return s.Preview(ctx, req) },
		create:       func(ctx context.Context) (*NetworkCreateResponse, error) { return s.Create(ctx, req) },
	})
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

// FindBySubnetID finds a network by its last subnet ID.
func (s *NetworkService) FindBySubnetID(ctx context.Context, subnetID string) (*Network, error) {
	networks, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, network := range networks {
		if network.SubnetID == subnetID {
			return &network, nil
		}
	}

	return nil, nil
}
