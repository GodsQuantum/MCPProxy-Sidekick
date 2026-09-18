package oauth

import (
	"context"
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
	StartOAuth(context.Context, string) (mcpproxy.OAuthStart, error)
}

type Browser struct {
	CDPBaseURL       string
	PublicSessionURL string
	Client           *http.Client
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
