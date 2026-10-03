package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type ProfileToolRules struct {
	Allow    []string          `json:"allow,omitempty"`
	Deny     []string          `json:"deny,omitempty"`
	Classify map[string]string `json:"classify,omitempty"`
}

type ProfileV3 struct {
	Name            string            `json:"name"`
	Servers         []string          `json:"servers"`
	Title           string            `json:"title,omitempty"`
	Description     string            `json:"description,omitempty"`
	MaxTier         string            `json:"max_tier,omitempty"`
	Unannotated     string            `json:"unannotated,omitempty"`
	Tools           *ProfileToolRules `json:"tools,omitempty"`
	CodeExecution   *bool             `json:"code_execution,omitempty"`
	ManagementTools *bool             `json:"management_tools,omitempty"`
	SwitchableTo    *[]string         `json:"switchable_to,omitempty"`
}

type ProfileTryHidden struct {
	Server string `json:"server"`
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

type ProfileTryResult struct {
	Results         []map[string]any   `json:"results,omitempty"`
	HiddenByProfile int                `json:"hidden_by_profile"`
	Hidden          []ProfileTryHidden `json:"hidden,omitempty"`
	HiddenTruncated bool               `json:"hidden_truncated,omitempty"`
}

func (c *Client) GetProfileV3(ctx context.Context, name string) (ProfileV3, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProfileV3{}, errors.New("profile name is required")
	}
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/profiles/"+url.PathEscape(name), nil, &raw); err != nil {
		return ProfileV3{}, err
	}
	return decodeProfileV3(raw)
}

func (c *Client) UpdateProfileV3(ctx context.Context, name string, profile ProfileV3) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name is required")
	}
	if profile.Name == "" {
		profile.Name = name
	}
	if profile.Name != name {
		return errors.New("profile name must match update path")
	}
	return c.doJSON(ctx, http.MethodPut, "/api/v1/profiles/"+url.PathEscape(name), profile, nil)
}

func (c *Client) TryProfileV3(ctx context.Context, profile ProfileV3, query string, limit int) (ProfileTryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ProfileTryResult{}, errors.New("profile try query is required")
	}
	if limit < 0 || limit > 50 {
		return ProfileTryResult{}, errors.New("profile try limit must be between 0 and 50")
	}
	var raw json.RawMessage
	payload := map[string]any{"profile": profile, "query": query}
	if limit > 0 {
		payload["limit"] = limit
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/profiles/try", payload, &raw); err != nil {
		return ProfileTryResult{}, err
	}
	return decodeProfileTry(raw)
}

func decodeProfileV3(raw json.RawMessage) (ProfileV3, error) {
	var direct ProfileV3
	if err := json.Unmarshal(raw, &direct); err == nil && strings.TrimSpace(direct.Name) != "" {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ProfileV3{}, fmt.Errorf("decode Profile v3: %w", err)
	}
	for _, key := range []string{"profile", "data"} {
		if child, ok := obj[key]; ok {
			if profile, err := decodeProfileV3(child); err == nil {
				return profile, nil
			}
		}
	}
	return ProfileV3{}, errors.New("Profile v3 response did not contain a profile")
}

func decodeProfileTry(raw json.RawMessage) (ProfileTryResult, error) {
	var direct ProfileTryResult
	if err := json.Unmarshal(raw, &direct); err == nil {
		var marker map[string]json.RawMessage
		if json.Unmarshal(raw, &marker) == nil {
			if _, ok := marker["hidden_by_profile"]; ok {
				return direct, nil
			}
		}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ProfileTryResult{}, fmt.Errorf("decode Profile v3 try result: %w", err)
	}
	if child, ok := obj["data"]; ok {
		return decodeProfileTry(child)
	}
	return ProfileTryResult{}, errors.New("Profile v3 try response did not contain a result")
}
