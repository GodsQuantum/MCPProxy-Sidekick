package oauthconfig

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type Backend interface {
	GetConfig(context.Context) (map[string]any, error)
	ValidateConfig(context.Context, map[string]any) error
	ApplyConfig(context.Context, map[string]any) error
}

type Service struct {
	Backend Backend
}

type Diff struct {
	Current        []string `json:"current"`
	Desired        []string `json:"desired"`
	Added          []string `json:"added,omitempty"`
	Removed        []string `json:"removed,omitempty"`
	ReauthRequired bool     `json:"reauth_required"`
}

type Result = Diff

func (s Service) EnsureRedirectURI(ctx context.Context, server, desired string) (bool, error) {
	if s.Backend == nil {
		return false, errors.New("OAuth config backend is not configured")
	}
	desired = strings.TrimSpace(desired)
	if desired == "" {
		return false, errors.New("OAuth redirect URI is required")
	}
	doc, err := s.Backend.GetConfig(ctx)
	if err != nil {
		return false, err
	}
	_, oauth, err := locateOAuth(doc, server)
	if err != nil {
		return false, err
	}
	if current, _ := oauth["redirect_uri"].(string); strings.TrimSpace(current) == desired {
		return false, nil
	}
	oauth["redirect_uri"] = desired
	if err := s.Backend.ValidateConfig(ctx, doc); err != nil {
		return false, fmt.Errorf("validate OAuth redirect URI: %w", err)
	}
	if err := s.Backend.ApplyConfig(ctx, doc); err != nil {
		return false, fmt.Errorf("apply OAuth redirect URI: %w", err)
	}
	verified, err := s.Backend.GetConfig(ctx)
	if err != nil {
		return false, fmt.Errorf("verify OAuth redirect URI: %w", err)
	}
	_, gotOAuth, err := locateOAuth(verified, server)
	if err != nil {
		return false, fmt.Errorf("verify OAuth redirect URI: %w", err)
	}
	got, _ := gotOAuth["redirect_uri"].(string)
	if strings.TrimSpace(got) != desired {
		return false, fmt.Errorf("verify OAuth redirect URI: got %q", got)
	}
	return true, nil
}

func (s Service) Current(ctx context.Context, server string) ([]string, error) {
	if s.Backend == nil {
		return nil, errors.New("OAuth config backend is not configured")
	}
	doc, err := s.Backend.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	_, oauth, err := locateOAuth(doc, server)
	if err != nil {
		return nil, err
	}
	return normalizeScopes(oauth["scopes"]), nil
}

func (s Service) Preview(ctx context.Context, server string, desired []string) (Diff, error) {
	current, err := s.Current(ctx, server)
	if err != nil {
		return Diff{}, err
	}
	want := normalizeStrings(desired)
	return makeDiff(current, want), nil
}

func (s Service) Apply(ctx context.Context, server string, desired []string) (Result, error) {
	if s.Backend == nil {
		return Result{}, errors.New("OAuth config backend is not configured")
	}
	doc, err := s.Backend.GetConfig(ctx)
	if err != nil {
		return Result{}, err
	}
	_, oauth, err := locateOAuth(doc, server)
	if err != nil {
		return Result{}, err
	}
	current := normalizeScopes(oauth["scopes"])
	want := normalizeStrings(desired)
	diff := makeDiff(current, want)
	if !diff.ReauthRequired {
		return diff, nil
	}

	oauth["scopes"] = append([]string(nil), want...)
	if err := s.Backend.ValidateConfig(ctx, doc); err != nil {
		return Result{}, fmt.Errorf("validate OAuth scopes: %w", err)
	}
	if err := s.Backend.ApplyConfig(ctx, doc); err != nil {
		return Result{}, fmt.Errorf("apply OAuth scopes: %w", err)
	}
	got, err := s.Current(ctx, server)
	if err != nil {
		return Result{}, fmt.Errorf("verify OAuth scopes: %w", err)
	}
	if !reflect.DeepEqual(got, want) {
		return Result{}, fmt.Errorf("verify OAuth scopes: got %v, want %v", got, want)
	}
	return diff, nil
}

func locateOAuth(doc map[string]any, name string) (map[string]any, map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil, errors.New("server name is required")
	}
	rawServers, ok := doc["mcpServers"]
	if !ok {
		return nil, nil, errors.New("MCPProxy config has no mcpServers")
	}
	servers, ok := rawServers.(map[string]any)
	if !ok {
		return nil, nil, errors.New("MCPProxy mcpServers has unsupported shape")
	}
	rawServer, ok := servers[name]
	if !ok {
		return nil, nil, fmt.Errorf("server %q not found", name)
	}
	server, ok := rawServer.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("server %q has unsupported config shape", name)
	}
	rawOAuth, ok := server["oauth"]
	if !ok || rawOAuth == nil {
		oauth := map[string]any{}
		server["oauth"] = oauth
		return server, oauth, nil
	}
	oauth, ok := rawOAuth.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("server %q OAuth config has unsupported shape", name)
	}
	return server, oauth, nil
}

func normalizeScopes(v any) []string {
	switch x := v.(type) {
	case []string:
		return normalizeStrings(x)
	case []any:
		items := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
		return normalizeStrings(items)
	case string:
		return normalizeStrings(strings.Fields(x))
	default:
		return nil
	}
}

func normalizeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func makeDiff(current, desired []string) Diff {
	current = append([]string(nil), current...)
	desired = append([]string(nil), desired...)
	cur := make(map[string]struct{}, len(current))
	want := make(map[string]struct{}, len(desired))
	for _, item := range current {
		cur[item] = struct{}{}
	}
	for _, item := range desired {
		want[item] = struct{}{}
	}
	var added, removed []string
	for _, item := range desired {
		if _, ok := cur[item]; !ok {
			added = append(added, item)
		}
	}
	for _, item := range current {
		if _, ok := want[item]; !ok {
			removed = append(removed, item)
		}
	}
	return Diff{
		Current: current, Desired: desired, Added: added, Removed: removed,
		ReauthRequired: len(added) > 0 || len(removed) > 0,
	}
}
