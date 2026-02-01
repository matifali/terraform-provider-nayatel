package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// RouterService handles router operations.
type RouterService struct {
	client *Client
}

// List returns all routers in the project.
func (s *RouterService) List(ctx context.Context) ([]Router, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/project/%s/routers", projectID))
	if err != nil {
		return nil, err
	}

	var routers []Router
	if err := json.Unmarshal(resp, &routers); err != nil {
		var result struct {
			Routers []Router `json:"routers"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		routers = result.Routers
	}

	return routers, nil
}

// Create creates a new router.
func (s *RouterService) Create(ctx context.Context, req *RouterCreateRequest) (*RouterCreateResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/project/%s/routers", projectID), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var result RouterCreateResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AddInterface adds an interface to a router.
func (s *RouterService) AddInterface(ctx context.Context, routerID, subnetID string) (*APIResponse, error) {
	payload := map[string]string{"subnet_id": subnetID}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/router/%s/interface", routerID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Delete deletes a router.
func (s *RouterService) Delete(ctx context.Context, routerID string) (*APIResponse, error) {
	payload := map[string]string{"router_id": routerID}

	resp, err := s.client.Delete(ctx, "/iaas/router", payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// FindByID finds a router by ID.
func (s *RouterService) FindByID(ctx context.Context, routerID string) (*Router, error) {
	routers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, router := range routers {
		if router.ID == routerID {
			return &router, nil
		}
	}

	return nil, nil
}

// GetProviderNetworkID returns the Provider Network ID needed for router creation.
func (s *RouterService) GetProviderNetworkID(ctx context.Context) (string, error) {
	resp, err := s.client.Get(ctx, "/iaas/networks")
	if err != nil {
		return "", err
	}

	var result struct {
		Networks []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"networks"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to decode networks: %w", err)
	}

	// Find Provider Network
	for _, net := range result.Networks {
		if net.Name == "Provider Network" {
			return net.ID, nil
		}
	}

	return "", fmt.Errorf("provider network not found")
}
