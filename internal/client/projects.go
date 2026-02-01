package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// ProjectService handles project operations.
type ProjectService struct {
	client *Client
}

// List returns all projects for the user.
func (s *ProjectService) List(ctx context.Context) ([]Project, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/user/%s/project", s.client.Username))
	if err != nil {
		return nil, err
	}

	var projects []Project
	if err := json.Unmarshal(resp, &projects); err != nil {
		// Try single project response
		var project Project
		if err := json.Unmarshal(resp, &project); err != nil {
			// Try object with projects field
			var result struct {
				Projects []Project `json:"projects"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return nil, fmt.Errorf("failed to decode response: %w", err)
			}
			projects = result.Projects
		} else {
			projects = []Project{project}
		}
	}

	return projects, nil
}

// Get returns a project by ID.
func (s *ProjectService) Get(ctx context.Context, projectID string) (*Project, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/project/%s", projectID))
	if err != nil {
		return nil, err
	}

	var project Project
	if err := json.Unmarshal(resp, &project); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &project, nil
}
