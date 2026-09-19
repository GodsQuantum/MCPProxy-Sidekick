package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
)

type OAuthStarter interface {
	LogoutOAuth(context.Context, string) error
	StartOAuth(context.Context, string) (mcpproxy.OAuthStart, error)
}

type Browser struct {
	CDPBaseURL       string
	PublicSessionURL string
	Client           *http.Client
}

type cdpTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func (b Browser) clearPageTargets(ctx context.Context, client *http.Client, base string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/list", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return fmt.Errorf("CDP target list: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var targets []cdpTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		resp.Body.Close()
		return fmt.Errorf("decode CDP target list: %w", err)
	}
	resp.Body.Close()

	for _, target := range targets {
		if target.Type != "page" || strings.TrimSpace(target.ID) == "" {
			continue
		}
		closeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/close/"+url.PathEscape(target.ID), nil)
		if err != nil {
			return err
		}
		closeResp, err := client.Do(closeReq)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(closeResp.Body, 2048))
		closeResp.Body.Close()
		if closeResp.StatusCode < 200 || closeResp.StatusCode >= 300 {
			return fmt.Errorf("CDP close stale tab %q: HTTP %d", target.ID, closeResp.StatusCode)
		}
	}
	return nil
}

func (b Browser) Open(ctx context.Context, authURL string) error {
	if strings.TrimSpace(authURL) == "" {
		return errors.New("empty OAuth authorization URL")
	}
	base := strings.TrimRight(strings.TrimSpace(b.CDPBaseURL), "/")
	if base == "" {
		return errors.New("empty CDP base URL")
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if err := b.clearPageTargets(ctx, client, base); err != nil {
		return fmt.Errorf("clear stale OAuth tabs: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, base+"/json/new?"+url.QueryEscape(authURL), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("CDP new tab: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (b Browser) SessionURL() string {
	return strings.TrimSpace(b.PublicSessionURL)
}

type Service struct {
	Starter OAuthStarter
	Browser Browser
}

type StartResult struct {
	BrowserURL    string `json:"browser_url"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func (s Service) Start(ctx context.Context, server string) (StartResult, error) {
	if err := s.Starter.LogoutOAuth(ctx, server); err != nil {
		return StartResult{}, fmt.Errorf("reset OAuth session: %w", err)
	}
	start, err := s.Starter.StartOAuth(ctx, server)
	if err != nil {
		return StartResult{}, err
	}
	if err := s.Browser.Open(ctx, start.AuthURL); err != nil {
		return StartResult{}, err
	}
	browserURL := s.Browser.SessionURL()
	if browserURL == "" {
		return StartResult{}, errors.New("OAuth browser has no public session URL")
	}
	return StartResult{
		BrowserURL:    browserURL,
		CorrelationID: start.CorrelationID,
	}, nil
}
