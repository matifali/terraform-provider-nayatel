package client

import (
	"context"
	"encoding/json"
	"fmt"
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
