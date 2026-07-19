package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// InstanceService handles instance operations.
type InstanceService struct {
	client *Client
}

// List returns all instances in the project.
func (s *InstanceService) List(ctx context.Context) ([]Instance, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/project/%s", projectID))
	if err != nil {
		return nil, err
	}

	return decodeList[Instance](resp, "instances")
}

// Create creates a new instance.
func (s *InstanceService) Create(ctx context.Context, req *InstanceCreateRequest) (*APIResponse, error) {
	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project", s.client.Username), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Preview previews instance cost before creation.
func (s *InstanceService) Preview(ctx context.Context, req *InstanceCreateRequest) (map[string]interface{}, error) {
	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/preview", s.client.Username), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var preview map[string]interface{}
	if err := json.Unmarshal(resp, &preview); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return preview, nil
}

// SafeCreate creates an instance via the safeCreate preview/verify-balance wrapper.
func (s *InstanceService) SafeCreate(ctx context.Context, req *InstanceCreateRequest) (*APIResponse, error) {
	return safeCreate(ctx, s.client, safeCreateConfig[*APIResponse]{
		resourceType: "instance",
		preview:      func(ctx context.Context) (map[string]interface{}, error) { return s.Preview(ctx, req) },
		create:       func(ctx context.Context) (*APIResponse, error) { return s.Create(ctx, req) },
	})
}

// Delete deletes an instance and optionally its root volume.
func (s *InstanceService) Delete(ctx context.Context, instanceID string) (*APIResponse, error) {
	return s.DeleteWithOptions(ctx, instanceID, true)
}

// DeleteWithOptions deletes an instance with optional root volume deletion.
func (s *InstanceService) DeleteWithOptions(ctx context.Context, instanceID string, deleteRootVolume bool) (*APIResponse, error) {
	endpoint := fmt.Sprintf("/iaas/user/%s/instance/%s/delete", s.client.Username, instanceID)
	if deleteRootVolume {
		endpoint += "?delete_root_volume=true"
	}

	resp, err := s.client.Delete(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Action performs an action on an instance (start, stop, reboot).
func (s *InstanceService) Action(ctx context.Context, instanceID string, action InstanceAction) (*APIResponse, error) {
	payload := map[string]string{"action_type": string(action)}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/instance/%s/state", instanceID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Stop stops an instance.
func (s *InstanceService) Stop(ctx context.Context, instanceID string) (*APIResponse, error) {
	return s.Action(ctx, instanceID, InstanceActionStop)
}

// WaitForStatus waits for an instance to reach a specific status.
func (s *InstanceService) WaitForStatus(ctx context.Context, instanceID string, targetStatus InstanceStatus, timeout time.Duration) (*Instance, error) {
	return pollUntil(ctx, pollConfig[Instance]{
		interval: 10 * time.Second,
		timeout:  timeout,
		// Use FindByID which uses List - more reliable than Get during BUILD.
		fetch: func(ctx context.Context) (*Instance, error) { return s.FindByID(ctx, instanceID) },
		done:  func(i *Instance) bool { return i.GetStatus() == targetStatus },
		failed: func(i *Instance) bool {
			return i.GetStatus() == InstanceStatusError
		},
		timeoutMsg: fmt.Sprintf("timeout waiting for instance %s to reach status %s", instanceID, targetStatus),
		failedMsg:  fmt.Sprintf("instance %s entered ERROR state", instanceID),
	})
}

// FindByID finds an instance by ID using the List function.
func (s *InstanceService) FindByID(ctx context.Context, instanceID string) (*Instance, error) {
	instances, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if instance.GetID() == instanceID {
			return &instance, nil
		}
	}

	return nil, nil
}

// FindByName finds an instance by name.
func (s *InstanceService) FindByName(ctx context.Context, name string) (*Instance, error) {
	instances, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if instance.Name == name {
			return &instance, nil
		}
	}

	return nil, nil
}
