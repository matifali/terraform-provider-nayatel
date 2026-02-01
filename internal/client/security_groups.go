package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// SecurityGroupService handles security group operations.
type SecurityGroupService struct {
	client *Client
}

// List returns all security groups in the project.
func (s *SecurityGroupService) List(ctx context.Context) ([]SecurityGroup, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/project/%s/security-groups", projectID))
	if err != nil {
		return nil, err
	}

	var securityGroups []SecurityGroup
	if err := json.Unmarshal(resp, &securityGroups); err != nil {
		var result struct {
			SecurityGroups []SecurityGroup `json:"security_groups"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		securityGroups = result.SecurityGroups
	}

	return securityGroups, nil
}

// ListForInstance returns security groups attached to an instance.
func (s *SecurityGroupService) ListForInstance(ctx context.Context, instanceID string) ([]SecurityGroup, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/instance/%s/security-group", instanceID))
	if err != nil {
		return nil, err
	}

	var securityGroups []SecurityGroup
	if err := json.Unmarshal(resp, &securityGroups); err != nil {
		var result struct {
			SecurityGroups []SecurityGroup `json:"security_groups"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		securityGroups = result.SecurityGroups
	}

	return securityGroups, nil
}

// AddToInstance adds a security group to an instance.
func (s *SecurityGroupService) AddToInstance(ctx context.Context, instanceID, groupName string) (*APIResponse, error) {
	payload := map[string]string{"group_name": groupName}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/instance/%s/security-group/add", instanceID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// RemoveFromInstance removes a security group from an instance.
func (s *SecurityGroupService) RemoveFromInstance(ctx context.Context, instanceID, groupName string) (*APIResponse, error) {
	payload := map[string]string{"group_name": groupName}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/instance/%s/security-group/remove", instanceID), payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// FindByName finds a security group by name (also matches names with API-added suffixes).
func (s *SecurityGroupService) FindByName(ctx context.Context, name string) (*SecurityGroup, error) {
	securityGroups, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	// First try exact match
	for _, sg := range securityGroups {
		if sg.Name == name {
			return &sg, nil
		}
	}

	// Then try prefix match (API may add suffix like "-331")
	for _, sg := range securityGroups {
		if len(sg.Name) > len(name) && sg.Name[:len(name)] == name && sg.Name[len(name)] == '-' {
			return &sg, nil
		}
	}

	return nil, nil
}

// FindByID finds a security group by ID.
func (s *SecurityGroupService) FindByID(ctx context.Context, id string) (*SecurityGroup, error) {
	securityGroups, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, sg := range securityGroups {
		if sg.ID == id {
			return &sg, nil
		}
	}

	return nil, nil
}

// Create creates a new security group.
func (s *SecurityGroupService) Create(ctx context.Context, req *SecurityGroupCreateRequest) (*APIResponse, error) {
	projectID, err := s.client.GetProjectID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/project/%s/security-groups", projectID), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// Delete deletes a security group.
func (s *SecurityGroupService) Delete(ctx context.Context, securityGroupID string) (*APIResponse, error) {
	payload := map[string]string{"security_group_id": securityGroupID}

	resp, err := s.client.Delete(ctx, "/iaas/security-group", payload)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// ListRules returns all rules for a security group.
func (s *SecurityGroupService) ListRules(ctx context.Context, securityGroupID string) ([]SecurityGroupRule, error) {
	resp, err := s.client.Get(ctx, fmt.Sprintf("/iaas/security-group/rule/%s", securityGroupID))
	if err != nil {
		return nil, err
	}

	var rules []SecurityGroupRule
	if err := json.Unmarshal(resp, &rules); err != nil {
		var result struct {
			Rules []SecurityGroupRule `json:"rules"`
		}
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		rules = result.Rules
	}

	return rules, nil
}

// CreateRule creates a new rule in a security group.
func (s *SecurityGroupService) CreateRule(ctx context.Context, securityGroupID string, req *SecurityGroupRuleCreateRequest) (*APIResponse, error) {
	resp, err := s.client.Post(ctx, fmt.Sprintf("/iaas/user/%s/security-group/%s/rule", s.client.Username, securityGroupID), req.ToAPIPayload())
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &apiResp, nil
}

// FindRuleByID finds a rule by ID in a security group.
func (s *SecurityGroupService) FindRuleByID(ctx context.Context, securityGroupID, ruleID string) (*SecurityGroupRule, error) {
	rules, err := s.ListRules(ctx, securityGroupID)
	if err != nil {
		return nil, err
	}

	for _, rule := range rules {
		if rule.ID == ruleID {
			return &rule, nil
		}
	}

	return nil, nil
}
