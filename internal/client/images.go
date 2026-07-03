package client

import (
	"context"
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

	return decodeList[Image](resp, "images")
}
