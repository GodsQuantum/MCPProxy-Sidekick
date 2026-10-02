package youtubeoauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Status struct {
	Configured         bool     `json:"configured"`
	CompleteConfigured bool     `json:"complete_configured"`
	Profile            string   `json:"profile,omitempty"`
	Scopes             []string `json:"scopes,omitempty"`
	AuthorizedAt       string   `json:"authorized_at,omitempty"`
	Handle             string   `json:"handle,omitempty"`
	ChannelID          string   `json:"channel_id,omitempty"`
	AuthRunning        bool     `json:"auth_running"`
	LastError          string   `json:"last_error,omitempty"`
	LastSync           string   `json:"last_sync,omitempty"`
}

type StartResult struct {
	AuthURL string `json:"auth_url"`
	Status
}

type Adapter struct {
	BaseURL string
	Client  *http.Client
}

func (a Adapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return &http.Client{Timeout: 12 * time.Second}
}

func (a Adapter) endpoint(p string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	if base == "" {
		return "", errors.New("YouTube OAuth control URL is not configured")
	}
	return base + p, nil
}

func (a Adapter) Status(ctx context.Context) (Status, error) {
	u, err := a.endpoint("/status")
	if err != nil {
		return Status{}, err
	}
	var out Status
	if err := a.do(ctx, http.MethodGet, u, nil, &out); err != nil {
		return Status{}, err
	}
	return out, nil
}

func (a Adapter) Start(ctx context.Context) (StartResult, error) {
	u, err := a.endpoint("/start")
	if err != nil {
		return StartResult{}, err
	}
	var out StartResult
	if err := a.do(ctx, http.MethodPost, u, map[string]any{}, &out); err != nil {
		return StartResult{}, err
	}
	if strings.TrimSpace(out.AuthURL) == "" {
		return StartResult{}, errors.New("YouTube OAuth helper returned no authorization URL")
	}
	return out, nil
}

func (a Adapter) do(ctx context.Context, method, u string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("YouTube OAuth helper: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func MatchesServer(name string) bool {
	switch strings.TrimSpace(name) {
	case "youtube-arezki", "youtube-creator-arezki":
		return true
	default:
		return false
	}
}

func ConnectedForServer(name string, st Status) bool {
	switch strings.TrimSpace(name) {
	case "youtube-creator-arezki":
		return st.Configured
	case "youtube-arezki":
		return st.CompleteConfigured
	default:
		return false
	}
}
