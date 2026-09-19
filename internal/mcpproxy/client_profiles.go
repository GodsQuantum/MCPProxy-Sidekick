package mcpproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type Profile struct {
	Name      string   `json:"name"`
	Servers   []string `json:"servers"`
	ToolCount int      `json:"tool_count,omitempty"`
}

type profilePatch struct {
	Name    string   `json:"name"`
	Servers []string `json:"servers"`
}

var profileSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

var reservedProfileSlugs = map[string]struct{}{
	"all": {}, "code": {}, "call": {}, "p": {},
}

func ValidateProfileName(name string) error {
	if !profileSlugPattern.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use 1-63 lowercase letters, digits, '-' or '_', starting with a letter or digit", name)
	}
	if _, reserved := reservedProfileSlugs[name]; reserved {
		return fmt.Errorf("profile name %q is reserved by MCPProxy", name)
	}
	return nil
}

func (c *Client) ListProfiles(ctx context.Context) ([]Profile, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/profiles", nil, &raw); err != nil {
		return nil, err
	}
	return decodeProfiles(raw)
}

func decodeProfiles(raw json.RawMessage) ([]Profile, error) {
	var direct []Profile
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode profile inventory: %w", err)
	}
	for _, key := range []string{"profiles", "data"} {
		if child, ok := obj[key]; ok {
			if profiles, err := decodeProfiles(child); err == nil {
				return profiles, nil
			}
		}
	}
	return nil, errors.New("profile inventory did not contain a profile array")
}

func (c *Client) CreateProfile(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	c.profileMu.Lock()
	defer c.profileMu.Unlock()

	profiles, err := c.ListProfiles(ctx)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.Name == name {
			return fmt.Errorf("profile %q already exists", name)
		}
	}
	return c.patchProfiles(ctx, append(profiles, Profile{Name: name, Servers: []string{}}))
}

func (c *Client) AssignProfileServer(ctx context.Context, profileName, serverName string) error {
	profileName, serverName = strings.TrimSpace(profileName), strings.TrimSpace(serverName)
	if profileName == "" || serverName == "" {
		return errors.New("profile and upstream are required")
	}
	c.profileMu.Lock()
	defer c.profileMu.Unlock()

	profiles, err := c.ListProfiles(ctx)
	if err != nil {
		return err
	}
	if err := ensureServerExists(ctx, c, serverName); err != nil {
		return err
	}
	found := false
	for i := range profiles {
		if profiles[i].Name != profileName {
			continue
		}
		found = true
		if !containsString(profiles[i].Servers, serverName) {
			profiles[i].Servers = append(profiles[i].Servers, serverName)
		}
	}
	if !found {
		return fmt.Errorf("profile %q does not exist", profileName)
	}
	return c.patchProfiles(ctx, profiles)
}

func (c *Client) RemoveProfileServer(ctx context.Context, profileName, serverName string) error {
	c.profileMu.Lock()
	defer c.profileMu.Unlock()

	profiles, err := c.ListProfiles(ctx)
	if err != nil {
		return err
	}
	found := false
	for i := range profiles {
		if profiles[i].Name != profileName {
			continue
		}
		found = true
		profiles[i].Servers = removeString(profiles[i].Servers, serverName)
	}
	if !found {
		return fmt.Errorf("profile %q does not exist", profileName)
	}
	return c.patchProfiles(ctx, profiles)
}

func (c *Client) DeleteProfile(ctx context.Context, profileName string) error {
	c.profileMu.Lock()
	defer c.profileMu.Unlock()

	profiles, err := c.ListProfiles(ctx)
	if err != nil {
		return err
	}
	next := make([]Profile, 0, len(profiles))
	found := false
	for _, p := range profiles {
		if p.Name == profileName {
			found = true
			continue
		}
		next = append(next, p)
	}
	if !found {
		return fmt.Errorf("profile %q does not exist", profileName)
	}
	return c.patchProfiles(ctx, next)
}

func (c *Client) patchProfiles(ctx context.Context, profiles []Profile) error {
	payload := make([]profilePatch, 0, len(profiles))
	for _, p := range profiles {
		servers := append([]string(nil), p.Servers...)
		payload = append(payload, profilePatch{Name: p.Name, Servers: servers})
	}
	return c.doJSON(ctx, http.MethodPatch, "/api/v1/config", map[string]any{"profiles": payload}, nil)
}

func ensureServerExists(ctx context.Context, c *Client, name string) error {
	servers, err := c.ListServers(ctx)
	if err != nil {
		return err
	}
	for _, srv := range servers {
		if srv.Name == name {
			return nil
		}
	}
	return fmt.Errorf("upstream %q does not exist in MCPProxy", name)
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func removeString(xs []string, want string) []string {
	out := xs[:0]
	for _, x := range xs {
		if x != want {
			out = append(out, x)
		}
	}
	return out
}
