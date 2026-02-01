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

	// Try parsing as array first (new API format)
	var instances []Instance
	if err := json.Unmarshal(resp, &instances); err != nil {
		// Fallback to Project object (old API format)
		var project Project
		if err := json.Unmarshal(resp, &project); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return project.Instances, nil
	}

	return instances, nil
}

// Get returns an instance by ID.
func (s *InstanceService) Get(ctx context.Context, instanceID string) (*Instance, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/instance/%s/details", instanceID))
	if err != nil {
		return nil, err
	}

	var instance Instance
	if err := json.Unmarshal(resp, &instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &instance, nil
}

// GetDiagnostics returns instance diagnostics.
func (s *InstanceService) GetDiagnostics(ctx context.Context, instanceID string) (map[string]interface{}, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/instance/%s/diagnostics", instanceID))
	if err != nil {
		return nil, err
	}

	var diagnostics map[string]interface{}
	if err := json.Unmarshal(resp, &diagnostics); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return diagnostics, nil
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

// SafeCreate creates an instance with safety checks:
// 1. Calls Preview API to get prorated cost (aborts if fails)
// 2. Checks account balance with retries (handles Nayatel API 0 balance glitch)
// 3. Only then proceeds with creation (with retries for transient errors).
func (s *InstanceService) SafeCreate(ctx context.Context, req *InstanceCreateRequest) (*APIResponse, error) {
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
	if err := s.client.VerifyBalance(ctx, requiredBalance, "instance"); err != nil {
		return nil, err
	}

	// Step 3: Create with retries for transient errors
	var createErr error
	var result *APIResponse
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

// Start starts an instance.
func (s *InstanceService) Start(ctx context.Context, instanceID string) (*APIResponse, error) {
	return s.Action(ctx, instanceID, InstanceActionStart)
}

// Stop stops an instance.
func (s *InstanceService) Stop(ctx context.Context, instanceID string) (*APIResponse, error) {
	return s.Action(ctx, instanceID, InstanceActionStop)
}

// Reboot reboots an instance.
func (s *InstanceService) Reboot(ctx context.Context, instanceID string) (*APIResponse, error) {
	return s.Action(ctx, instanceID, InstanceActionReboot)
}

// WaitForStatus waits for an instance to reach a specific status.
func (s *InstanceService) WaitForStatus(ctx context.Context, instanceID string, targetStatus InstanceStatus, timeout time.Duration) (*Instance, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for instance %s to reach status %s", instanceID, targetStatus)
		case <-ticker.C:
			// Use FindByID which uses List - more reliable than Get during BUILD
			instance, err := s.FindByID(ctx, instanceID)
			if err != nil {
				return nil, err
			}
			if instance == nil {
				continue // Instance not found yet, keep waiting
			}

			currentStatus := instance.GetStatus()
			// Debug: log current status
			fmt.Printf("[DEBUG] Instance %s current status: '%s' (target: '%s')\n", instanceID, currentStatus, targetStatus)

			if currentStatus == targetStatus {
				return instance, nil
			}

			if currentStatus == InstanceStatusError {
				return nil, fmt.Errorf("instance %s entered ERROR state", instanceID)
			}
		}
	}
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
