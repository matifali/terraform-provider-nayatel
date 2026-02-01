package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// FlavorService handles flavor operations.
type FlavorService struct {
	client *Client
}

// List returns all available flavors.
func (s *FlavorService) List(ctx context.Context) ([]Flavor, error) {
	resp, err := s.client.Get(ctx, "/service/combinations/iaas")
	if err != nil {
		return nil, err
	}

	var flavors []Flavor
	if err := json.Unmarshal(resp, &flavors); err != nil {
		var result struct {
			Combinations []Flavor `json:"combinations"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		flavors = result.Combinations
	}

	return flavors, nil
}

// Get returns a flavor by ID.
func (s *FlavorService) Get(ctx context.Context, flavorID string) (*Flavor, error) {
	flavors, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, flavor := range flavors {
		if flavor.ID == flavorID {
			return &flavor, nil
		}
	}

	return nil, nil
}

// FindBySpecs finds a flavor by CPU, RAM, and disk specs.
func (s *FlavorService) FindBySpecs(ctx context.Context, cpu, ram, disk int) (*Flavor, error) {
	flavors, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, flavor := range flavors {
		if flavor.GetVCPUs() == cpu && flavor.GetRAM() == ram && flavor.GetDisk() == disk {
			return &flavor, nil
		}
	}

	return nil, nil
}
