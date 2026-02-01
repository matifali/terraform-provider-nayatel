package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ImageService handles image operations.
type ImageService struct {
	client *Client
}

// List returns all available images.
func (s *ImageService) List(ctx context.Context) ([]Image, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/user/%s/images", s.client.Username))
	if err != nil {
		return nil, err
	}

	var images []Image
	if err := json.Unmarshal(resp, &images); err != nil {
		var result struct {
			Images []Image `json:"images"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		images = result.Images
	}

	return images, nil
}

// Get returns an image by ID.
func (s *ImageService) Get(ctx context.Context, imageID string) (*Image, error) {
	images, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, image := range images {
		if image.ID == imageID {
			return &image, nil
		}
	}

	return nil, nil
}

// FindByName finds an image by name (partial match, case-insensitive).
func (s *ImageService) FindByName(ctx context.Context, name string) (*Image, error) {
	images, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	nameLower := strings.ToLower(name)
	for _, image := range images {
		if strings.Contains(strings.ToLower(image.Name), nameLower) {
			return &image, nil
		}
	}

	return nil, nil
}
