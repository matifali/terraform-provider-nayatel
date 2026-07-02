// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// CubeService handles Cubes operations. Cubes are Nayatel's managed
// container service (LXD-backed), separate from the IaaS VPS offering.
type CubeService struct {
	client *Client
}

// CubeCombination is an allowed CPU/RAM pair for a cube.
type CubeCombination struct {
	CPU int `json:"cpu"`
	RAM int `json:"ram"` // in GB
}

// CubeImage is an OS image available for cubes.
type CubeImage struct {
	Alias   string `json:"alias"`
	Version string `json:"version"`
}

// Cube represents a cube as returned by the project instance list.
// CPU/Memory/Disk are strings in the API ("2", "2GiB", "20GiB").
type Cube struct {
	Name        string   `json:"-"` // map key in the API response
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Type        string   `json:"type"`
	Image       string   `json:"image"`
	CPU         string   `json:"cpu"`
	Memory      string   `json:"memory"`
	Disk        string   `json:"disk"`
	CreatedAt   string   `json:"created_at"`
	Network     []string `json:"network"`
	Location    string   `json:"location"`
}

// GetPublicIP returns the cube's public IPv4 address.
func (c *Cube) GetPublicIP() string {
	for _, entry := range c.Network {
		ip := net.ParseIP(entry)
		if ip == nil || ip.To4() == nil {
			continue
		}
		if !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return entry
		}
	}
	return ""
}

// GetCPU returns the CPU count as an int (0 if unparseable).
func (c *Cube) GetCPU() int {
	var cpu int
	_, _ = fmt.Sscanf(c.CPU, "%d", &cpu)
	return cpu
}

// GetMemoryGB returns the memory size in GB (0 if unparseable).
func (c *Cube) GetMemoryGB() int {
	var mem int
	_, _ = fmt.Sscanf(c.Memory, "%d", &mem)
	return mem
}

// GetDiskGB returns the disk size in GB (0 if unparseable).
func (c *Cube) GetDiskGB() int {
	var disk int
	_, _ = fmt.Sscanf(c.Disk, "%d", &disk)
	return disk
}

// CubeProject is the cube project (quota container) for a user.
type CubeProject struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
}

// CubeCreateRequest describes a cube to create.
type CubeCreateRequest struct {
	Name          string
	Description   string
	ImageName     string // e.g. "ubuntu"
	ImageVersion  string // e.g. "22.04"
	CPU           int
	RAM           int // in GB
	Storage       int // in GB
	FloatingIPs   int
	InstanceCount int
	SSHPublicKey  string // full public key, sent as the auth fingerprint
}

// ToAPIPayload converts the request to the API payload format.
// The INITIALIZATION field must be a JSON-encoded string, not a nested
// object (unlike the IaaS create payload).
func (r *CubeCreateRequest) ToAPIPayload(username string) (map[string]interface{}, error) {
	initialization := map[string]interface{}{
		"name":        r.Name,
		"description": r.Description,
		"image": map[string]string{
			"name":    r.ImageName,
			"version": r.ImageVersion,
		},
		"auth": map[string]string{
			"method":      "ssh",
			"fingerprint": r.SSHPublicKey,
			"user":        username,
		},
	}

	initJSON, err := json.Marshal(initialization)
	if err != nil {
		return nil, fmt.Errorf("failed to encode initialization: %w", err)
	}

	floatingIPs := r.FloatingIPs
	if floatingIPs == 0 {
		floatingIPs = 1
	}
	instanceCount := r.InstanceCount
	if instanceCount == 0 {
		instanceCount = 1
	}

	return map[string]interface{}{
		"conf": map[string]interface{}{
			"STORAGE":        r.Storage,
			"FLOATING_IPS":   floatingIPs,
			"INSTANCE_COUNT": instanceCount,
			"CPU":            r.CPU,
			"RAM":            r.RAM,
			"INITIALIZATION": string(initJSON),
		},
	}, nil
}

// InstanceName returns the cube's server-side instance name, which the API
// derives as "{name}-{username}".
func (r *CubeCreateRequest) InstanceName(username string) string {
	return fmt.Sprintf("%s-%s", r.Name, username)
}

// Combinations returns the allowed CPU/RAM combinations per plan tier.
func (s *CubeService) Combinations(ctx context.Context) (map[string][]CubeCombination, error) {
	resp, err := s.client.Get(ctx, "/service/combinations/cubes")
	if err != nil {
		return nil, err
	}

	var combinations map[string][]CubeCombination
	if err := json.Unmarshal(resp, &combinations); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return combinations, nil
}

// ValidateCombination checks that the CPU/RAM pair is offered.
func (s *CubeService) ValidateCombination(ctx context.Context, cpu, ram int) error {
	combinations, err := s.Combinations(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch allowed combinations: %w", err)
	}

	var allowed []string
	for _, combos := range combinations {
		for _, c := range combos {
			if c.CPU == cpu && c.RAM == ram {
				return nil
			}
			allowed = append(allowed, fmt.Sprintf("%dc/%dGB", c.CPU, c.RAM))
		}
	}

	return fmt.Errorf("cpu=%d ram=%dGB is not an offered combination; allowed: %s", cpu, ram, strings.Join(allowed, ", "))
}

// Images returns the OS images available for cubes.
func (s *CubeService) Images(ctx context.Context) ([]CubeImage, error) {
	resp, err := s.client.Get(ctx, "/cubes/images")
	if err != nil {
		return nil, err
	}

	var result struct {
		Status bool        `json:"status"`
		Images []CubeImage `json:"images"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Images, nil
}

// Preview previews cube cost before creation.
func (s *CubeService) Preview(ctx context.Context, req *CubeCreateRequest) (map[string]interface{}, error) {
	payload, err := req.ToAPIPayload(s.client.Username)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/cubes/user/%s/preview", s.client.Username), payload)
	if err != nil {
		return nil, err
	}

	var preview map[string]interface{}
	if err := json.Unmarshal(resp, &preview); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return preview, nil
}

// Create creates a new cube (and its project if this is the first cube).
func (s *CubeService) Create(ctx context.Context, req *CubeCreateRequest) error {
	payload, err := req.ToAPIPayload(s.client.Username)
	if err != nil {
		return err
	}

	_, err = s.client.Post(ctx, fmt.Sprintf("/cubes/user/%s/project", s.client.Username), payload)
	return err
}

// SafeCreate creates a cube with the same safety checks as instances:
// preview cost, verify balance, then create with retries.
func (s *CubeService) SafeCreate(ctx context.Context, req *CubeCreateRequest) error {
	preview, err := s.Preview(ctx, req)
	if err != nil {
		return fmt.Errorf("preview failed (API may be having issues, aborting to protect against unwanted charges): %w", err)
	}

	requiredBalance := ExtractCostFromPreview(preview)
	if requiredBalance <= 0 {
		return fmt.Errorf("unable to determine cost from preview response (API may be having issues, aborting to protect against unwanted charges)")
	}

	if err := s.client.VerifyBalance(ctx, requiredBalance, "cube"); err != nil {
		return err
	}

	var createErr error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		createErr = s.Create(ctx, req)
		if createErr == nil {
			return nil
		}
		if IsInsufficientBalance(createErr) && attempt < maxRetries {
			time.Sleep(time.Duration(attempt*3) * time.Second)
			continue
		}
		break
	}

	return createErr
}

// GetProject returns the user's cube project (quota container).
func (s *CubeService) GetProject(ctx context.Context) (*CubeProject, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/cubes/user/%s/project", s.client.Username))
	if err != nil {
		return nil, err
	}

	var result struct {
		Metadata CubeProject `json:"metadata"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if result.Metadata.Name == "" {
		return nil, nil
	}

	return &result.Metadata, nil
}

// List returns all cubes in a project, keyed by cube instance name.
func (s *CubeService) List(ctx context.Context, projectID string) (map[string]Cube, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/cubes/project/%s/instance", projectID))
	if err != nil {
		return nil, err
	}

	var result struct {
		Metadata map[string]Cube `json:"metadata"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	cubes := make(map[string]Cube, len(result.Metadata))
	for name, cube := range result.Metadata {
		cube.Name = name
		cubes[name] = cube
	}

	return cubes, nil
}

// FindByName finds a cube by its instance name ("{name}-{username}").
func (s *CubeService) FindByName(ctx context.Context, projectID, instanceName string) (*Cube, error) {
	cubes, err := s.List(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if cube, ok := cubes[instanceName]; ok {
		return &cube, nil
	}
	return nil, nil
}

// CubeState is a target power state for SetState.
type CubeState string

const (
	CubeStateStart CubeState = "start"
	CubeStateStop  CubeState = "stop"
)

// SetState starts or stops a cube. The API returns an empty body; success
// is indicated by the HTTP status alone.
func (s *CubeService) SetState(ctx context.Context, projectID, instanceName string, state CubeState) error {
	payload := map[string]interface{}{
		"state":    string(state),
		"timeout":  25,
		"force":    false,
		"stateful": false,
	}

	_, err := s.client.Request(ctx, http.MethodPut, fmt.Sprintf("/cubes/project/%s/instance/instances/%s/state", projectID, instanceName), payload)
	return err
}

// Start starts a cube.
func (s *CubeService) Start(ctx context.Context, projectID, instanceName string) error {
	return s.SetState(ctx, projectID, instanceName, CubeStateStart)
}

// Stop stops a cube.
func (s *CubeService) Stop(ctx context.Context, projectID, instanceName string) error {
	return s.SetState(ctx, projectID, instanceName, CubeStateStop)
}

// Delete destroys a cube permanently. The API returns an empty body.
func (s *CubeService) Delete(ctx context.Context, projectID, instanceName string) error {
	_, err := s.client.Delete(ctx, fmt.Sprintf("/cubes/user/%s/project/%s/instance/%s/delete", s.client.Username, projectID, instanceName), nil)
	return err
}

// WaitForStatus waits for a cube to reach the given status. Errors from the
// list endpoint are tolerated while waiting because the API returns 403
// during provisioning.
func (s *CubeService) WaitForStatus(ctx context.Context, projectID, instanceName, targetStatus string, timeout time.Duration) (*Cube, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for cube %s to reach status %s", instanceName, targetStatus)
		case <-ticker.C:
			cube, err := s.FindByName(ctx, projectID, instanceName)
			if err != nil {
				continue // 403 while provisioning — keep waiting
			}
			if cube == nil {
				continue
			}
			if strings.EqualFold(cube.Status, targetStatus) {
				return cube, nil
			}
			if strings.EqualFold(cube.Status, "Error") {
				return nil, fmt.Errorf("cube %s entered Error state", instanceName)
			}
		}
	}
}
