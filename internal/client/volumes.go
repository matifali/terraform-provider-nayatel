package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// VolumeService handles volume operations.
type VolumeService struct {
	client *Client
}

// List returns all volumes in the project.
func (s *VolumeService) List(ctx context.Context) ([]Volume, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volumes", s.client.Username, projectID))
	if err != nil {
		return nil, err
	}

	var volumes []Volume
	if err := json.Unmarshal(resp, &volumes); err != nil {
		// Try object with volumes field
		var result struct {
			Status  bool     `json:"status"`
			Volumes []Volume `json:"volumes"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		volumes = result.Volumes
	}

	return volumes, nil
}

// Get returns a volume by ID.
func (s *VolumeService) Get(ctx context.Context, volumeID string) (*Volume, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	// Try to get volume details
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volume/%s", s.client.Username, projectID, volumeID))
	if err != nil {
		// Fallback: search in volume list
		volumes, listErr := s.List(ctx)
		if listErr != nil {
			return nil, err // Return original error
		}

		for _, v := range volumes {
			if v.ID == volumeID {
				return &v, nil
			}
		}
		return nil, err
	}

	var volume Volume
	if err := json.Unmarshal(resp, &volume); err != nil {
		// Try object with volume field
		var result struct {
			Volume Volume `json:"volume"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		volume = result.Volume
	}

	return &volume, nil
}

// Create creates a new volume.
func (s *VolumeService) Create(ctx context.Context, req *VolumeCreateRequest) (*Volume, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	// Build the payload
	payload := map[string]interface{}{
		"size": req.Size,
	}
	if req.Name != "" {
		payload["name"] = req.Name
	}
	if req.Description != "" {
		payload["description"] = req.Description
	}
	if req.VolumeType != "" {
		payload["volume_type"] = req.VolumeType
	}
	if req.AvailabilityZone != "" {
		payload["availability_zone"] = req.AvailabilityZone
	}
	if req.SnapshotID != "" {
		payload["snapshot_id"] = req.SnapshotID
	}
	if req.SourceVolumeID != "" {
		payload["source_volid"] = req.SourceVolumeID
	}
	if req.ImageID != "" {
		payload["image_id"] = req.ImageID
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volumes/add", s.client.Username, projectID), payload)
	if err != nil {
		return nil, err
	}

	// Try to parse volume from response
	var volume Volume
	if err := json.Unmarshal(resp, &volume); err != nil {
		// Try with wrapper
		var result struct {
			Status  bool   `json:"status"`
			Message string `json:"message"`
			Volume  Volume `json:"volume"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		if !result.Status {
			return nil, fmt.Errorf("volume creation failed: %s", result.Message)
		}
		volume = result.Volume
	}

	return &volume, nil
}

// Delete deletes a volume.
func (s *VolumeService) Delete(ctx context.Context, volumeID string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Delete(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volume/%s/delete", s.client.Username, projectID, volumeID), nil)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Attach attaches a volume to an instance.
func (s *VolumeService) Attach(ctx context.Context, volumeID, instanceID string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{
		"volume_id":   volumeID,
		"instance_id": instanceID,
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volume/%s/attach", s.client.Username, projectID, volumeID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Detach detaches a volume from an instance.
func (s *VolumeService) Detach(ctx context.Context, volumeID, instanceID string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{
		"volume_id":   volumeID,
		"instance_id": instanceID,
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volume/%s/detach", s.client.Username, projectID, volumeID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Extend increases the size of a volume.
func (s *VolumeService) Extend(ctx context.Context, volumeID string, newSize int) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]int{
		"new_size": newSize,
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volume/%s/extend", s.client.Username, projectID, volumeID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// WaitForStatus waits for a volume to reach a specific status.
func (s *VolumeService) WaitForStatus(ctx context.Context, volumeID string, targetStatus string, timeout time.Duration) (*Volume, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for volume %s to reach status %s", volumeID, targetStatus)
		case <-ticker.C:
			volume, err := s.Get(ctx, volumeID)
			if err != nil {
				return nil, err
			}

			if volume.Status == targetStatus {
				return volume, nil
			}

			if volume.Status == "error" {
				return nil, fmt.Errorf("volume %s entered error state", volumeID)
			}
		}
	}
}

// FindByName finds a volume by name.
func (s *VolumeService) FindByName(ctx context.Context, name string) (*Volume, error) {
	volumes, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, volume := range volumes {
		if volume.Name == name {
			return &volume, nil
		}
	}

	return nil, nil
}
