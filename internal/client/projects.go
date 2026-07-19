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

	if projects, err := decodeList[Project](resp, "projects"); err == nil {
		return projects, nil
	}

	// Fall back to a single project object, but only if it actually looks
	// like a project (an error payload would otherwise decode to an empty
	// project and mask the failure).
	var project Project
	if err := json.Unmarshal(resp, &project); err == nil && project.GetID() != "" {
		return []Project{project}, nil
	}

	return nil, fmt.Errorf("unexpected projects response: %s", truncateForError(resp))
}
