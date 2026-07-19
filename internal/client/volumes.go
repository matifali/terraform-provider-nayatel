package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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

	return decodeList[Volume](resp, "volumes")
}

// Get returns a volume by ID, or (nil, nil) if no volume with that ID
// exists -- matching the not-found convention of every other service's
// FindByID (InstanceService, RouterService, NetworkService,
// SecurityGroupService). There is no working singular get-by-ID endpoint
// for volumes on the live API -- confirmed by direct request, it 404s with
// an HTML body rather than a JSON error -- so this scans the project's
// volume list instead.
func (s *VolumeService) Get(ctx context.Context, volumeID string) (*Volume, error) {
	volumes, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range volumes {
		if v.ID == volumeID {
			return &v, nil
		}
	}

	return nil, nil
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

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volumes", s.client.Username, projectID), payload)
	if err != nil {
		return nil, err
	}

	// The API responds with either a status message ({"status","message"})
	// or, on some paths, that plus a nested volume object. Volume's custom
	// UnmarshalJSON never errors on a status-only body, so the status must
	// be checked explicitly here rather than relying on a decode failure.
	var result struct {
		Status  bool    `json:"status"`
		Message string  `json:"message"`
		Volume  *Volume `json:"volume"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if result.Volume == nil && !result.Status {
		return nil, fmt.Errorf("volume creation failed: %s", result.Message)
	}
	if result.Volume != nil {
		return result.Volume, nil
	}

	return &Volume{}, nil
}

// Delete deletes a volume.
func (s *VolumeService) Delete(ctx context.Context, volumeID string) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{"volume_id": volumeID}

	resp, err := s.client.Delete(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volumes", s.client.Username, projectID), payload)
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
	payload := map[string]string{"volume_id": volumeID}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/instance/%s/volume", s.client.Username, instanceID), payload)
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
	payload := map[string]string{"volume_id": volumeID}

	resp, err := s.client.Delete(ctx, fmt.Sprintf("/iaas/user/%s/instance/%s/volume", s.client.Username, instanceID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Extend increases the size of a volume by addSize GB. Nayatel's upsize API
// takes a delta to add to the current size, not the new absolute size.
func (s *VolumeService) Extend(ctx context.Context, volumeID string, addSize int) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{
		"volume_id": volumeID,
		"add_size":  strconv.Itoa(addSize),
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/project/%s/volumes/upsize", s.client.Username, projectID), payload)
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
	return pollUntil(ctx, pollConfig[Volume]{
		interval:        5 * time.Second,
		timeout:         timeout,
		fetch:           func(ctx context.Context) (*Volume, error) { return s.Get(ctx, volumeID) },
		notFoundIsFatal: true,
		done:            func(v *Volume) bool { return v.Status == targetStatus },
		failed:          func(v *Volume) bool { return v.Status == "error" },
		timeoutMsg:      fmt.Sprintf("timeout waiting for volume %s to reach status %s", volumeID, targetStatus),
		notFoundMsg:     fmt.Sprintf("volume %s not found while waiting for status %s", volumeID, targetStatus),
		failedMsg:       fmt.Sprintf("volume %s entered error state", volumeID),
	})
}

// ResolveAttachedInstanceID returns the ID of the instance a volume is
// attached to. Nayatel's volume list reports the attached instance by
// name in "attached_to" rather than by ID, so this cross-references the
// instance list to recover the actual ID that Attach/Detach and other
// resources key on. Errors if the volume is attached but no instance with
// that name currently exists (renamed or deleted) -- the raw name is not a
// usable substitute for the real ID, so it must never be returned as if it
// were one.
func (s *VolumeService) ResolveAttachedInstanceID(ctx context.Context, v *Volume) (string, error) {
	name := v.GetAttachedInstanceID()
	if name == "" {
		return "", nil
	}

	instance, err := s.client.Instances.FindByName(ctx, name)
	if err != nil {
		return "", err
	}
	if instance == nil {
		return "", fmt.Errorf("volume is attached to instance %q, but no instance with that name was found", name)
	}

	return instance.GetID(), nil
}
