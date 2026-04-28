package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

// RemoveInterface removes an interface from a router.
func (s *RouterService) RemoveInterface(ctx context.Context, routerID, subnetID string) (*APIResponse, error) {
	payload := map[string]string{"subnet_id": subnetID}
	deleteEndpoint := fmt.Sprintf("/iaas/router/%s/interface", routerID)
	postRemoveEndpoint := fmt.Sprintf("/iaas/router/%s/interface/remove", routerID)

	resp, err := s.client.Delete(ctx, deleteEndpoint, payload)
	if err == nil {
		apiResp, appErr := decodeRouterInterfaceResponse(resp, "DELETE", deleteEndpoint)
		if appErr == nil {
			return apiResp, nil
		}
		err = appErr
	} else if !shouldFallbackRouterInterfaceRemove(err) {
		return nil, err
	}

	primaryErr := err
	resp, err = s.client.Post(ctx, postRemoveEndpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("unable to remove router interface with DELETE %s or POST %s fallback: delete failed: %v; post failed: %w", deleteEndpoint, postRemoveEndpoint, primaryErr, err)
	}

	apiResp, err := decodeRouterInterfaceResponse(resp, "POST", postRemoveEndpoint)
	if err != nil {
		return nil, fmt.Errorf("unable to remove router interface with DELETE %s or POST %s fallback: delete failed: %v; post failed: %w", deleteEndpoint, postRemoveEndpoint, primaryErr, err)
	}

	return apiResp, nil
}

func shouldFallbackRouterInterfaceRemove(err error) bool {
	return isAPIStatus(err, http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed) || isRouterAPIResponseError(err)
}

func decodeRouterInterfaceResponse(resp []byte, method, endpoint string) (*APIResponse, error) {
	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if routerAPIResponseFailed(resp, apiResp) {
		return nil, &routerAPIResponseError{
			Method:   method,
			Endpoint: endpoint,
			Response: apiResp,
		}
	}

	return &apiResp, nil
}

func routerAPIResponseFailed(resp []byte, apiResp APIResponse) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(resp, &fields); err != nil {
		return false
	}

	_, statusPresent := fields["status"]
	return statusPresent && !apiResp.Status
}

type routerAPIResponseError struct {
	Method   string
	Endpoint string
	Response APIResponse
}

func (e *routerAPIResponseError) Error() string {
	return fmt.Sprintf("%s %s returned unsuccessful response: status=%t message=%q", e.Method, e.Endpoint, e.Response.Status, e.Response.Message)
}

func isRouterAPIResponseError(err error) bool {
	var apiRespErr *routerAPIResponseError
	return errors.As(err, &apiRespErr)
}

func isAPIStatus(err error, statusCodes ...int) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	for _, statusCode := range statusCodes {
		if apiErr.StatusCode == statusCode {
			return true
		}
	}
	return false
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
