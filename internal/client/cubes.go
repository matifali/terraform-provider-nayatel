// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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
	return parseLeadingInt(c.CPU)
}

// GetMemoryGB returns the memory size in GB (0 if unparseable).
func (c *Cube) GetMemoryGB() int {
	return parseLeadingInt(c.Memory)
}

// GetDiskGB returns the disk size in GB (0 if unparseable).
func (c *Cube) GetDiskGB() int {
	return parseLeadingInt(c.Disk)
}

// parseLeadingInt parses the leading integer from s (0 if unparseable).
func parseLeadingInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
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

	return decodeList[CubeImage](resp, "images")
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
	_, err := safeCreate(ctx, s.client, safeCreateConfig[struct{}]{
		resourceType: "cube",
		preview:      func(ctx context.Context) (map[string]interface{}, error) { return s.Preview(ctx, req) },
		create:       func(ctx context.Context) (struct{}, error) { return struct{}{}, s.Create(ctx, req) },
	})
	return err
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

// Delete destroys a cube permanently. The API returns an empty body.
func (s *CubeService) Delete(ctx context.Context, projectID, instanceName string) error {
	_, err := s.client.Delete(ctx, fmt.Sprintf("/cubes/user/%s/project/%s/instance/%s/delete", s.client.Username, projectID, instanceName), nil)
	return err
}

// WaitForStatus waits for a cube to reach the given status. Errors from the
// list endpoint are tolerated while waiting because the API returns 403
// during provisioning.
func (s *CubeService) WaitForStatus(ctx context.Context, projectID, instanceName, targetStatus string, timeout time.Duration) (*Cube, error) {
	return pollUntil(ctx, pollConfig[Cube]{
		interval:         10 * time.Second,
		timeout:          timeout,
		fetch:            func(ctx context.Context) (*Cube, error) { return s.FindByName(ctx, projectID, instanceName) },
		tolerateFetchErr: true, // 403 while provisioning — keep waiting
		done:             func(c *Cube) bool { return strings.EqualFold(c.Status, targetStatus) },
		failed:           func(c *Cube) bool { return strings.EqualFold(c.Status, "Error") },
		timeoutMsg:       fmt.Sprintf("timeout waiting for cube %s to reach status %s", instanceName, targetStatus),
		failedMsg:        fmt.Sprintf("cube %s entered Error state", instanceName),
	})
}
