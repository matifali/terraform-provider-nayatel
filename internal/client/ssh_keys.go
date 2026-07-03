package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// SSHKeyService handles SSH key operations.
type SSHKeyService struct {
	client *Client
}

// List returns all SSH keys for the user.
func (s *SSHKeyService) List(ctx context.Context) ([]SSHKey, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/user/%s/ssh", s.client.Username))
	if err != nil {
		return nil, err
	}

	return decodeList[SSHKey](resp, "ssh_keys", "keys")
}

// Get returns an SSH key by name.
func (s *SSHKeyService) Get(ctx context.Context, name string) (*SSHKey, error) {
	keys, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if key.Name == name {
			return &key, nil
		}
	}

	return nil, nil
}

// SSHKeyCreateRequest represents the request body for creating an SSH key.
type SSHKeyCreateRequest struct {
	Name       string `json:"name"`
	KeyContent string `json:"key_content"`
}

// SSHKeyDeleteRequest represents the request body for deleting an SSH key.
type SSHKeyDeleteRequest struct {
	Name string `json:"name"`
}

// Create creates a new SSH key.
func (s *SSHKeyService) Create(ctx context.Context, req *SSHKeyCreateRequest) (*SSHKey, error) {
	resp, err := s.client.Post(ctx, fmt.Sprintf("/user/%s/ssh", s.client.Username), req)
	if err != nil {
		return nil, err
	}

	// Try to decode the response
	var result struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp, &result); err == nil {
		if !result.Status {
			return nil, fmt.Errorf("failed to create SSH key: %s", result.Message)
		}
	}

	// Return the key by looking it up (API may not return it directly)
	return s.Get(ctx, req.Name)
}

// Delete deletes an SSH key by name.
func (s *SSHKeyService) Delete(ctx context.Context, name string) error {
	_, err := s.client.Delete(ctx, fmt.Sprintf("/user/%s/ssh", s.client.Username), &SSHKeyDeleteRequest{Name: name})
	if err != nil {
		return err
	}
	return nil
}
