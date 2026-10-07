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
	"github.com/gorilla/websocket"
)

type OAuthStarter interface {
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

type cdpVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cdpResponse struct {
	ID     int             `json:"id"`
	Error  *cdpError       `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

func (b Browser) clearPageTargets(ctx context.Context, client *http.Client, base string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/list", nil)
	if err != nil {
		return err
	}
	req.Close = true
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
		closeReq.Close = true
		closeResp, err := client.Do(closeReq)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(closeResp.Body, 2048))
		closeResp.Body.Close()
		// CloakBrowser's CDP relay intentionally exposes only the read-side
		// /json endpoints. Failure to close a stale tab must never block OAuth.
	}
	return nil
}

func (b Browser) openViaHTTPNew(ctx context.Context, client *http.Client, base, authURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, base+"/json/new?"+url.QueryEscape(authURL), nil)
	if err != nil {
		return err
	}
	req.Close = true
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

func (b Browser) openViaWebSocket(ctx context.Context, client *http.Client, base, authURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/version", nil)
	if err != nil {
		return err
	}
	req.Close = true
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("CDP version: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return fmt.Errorf("CDP version: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var version cdpVersion
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		resp.Body.Close()
		return fmt.Errorf("decode CDP version: %w", err)
	}
	resp.Body.Close()
	if strings.TrimSpace(version.WebSocketDebuggerURL) == "" {
		return errors.New("CDP version response has no webSocketDebuggerUrl")
	}

	wsURL, err := url.Parse(version.WebSocketDebuggerURL)
	if err != nil {
		return fmt.Errorf("parse CDP websocket URL: %w", err)
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse CDP base URL: %w", err)
	}
	// The relay may advertise its externally published host. Sidekick lives on
	// the internal Docker network, so preserve the websocket path but use the
	// same reachable host as CDPBaseURL.
	wsURL.Host = baseURL.Host
	if baseURL.Scheme == "https" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, wsResp, err := dialer.DialContext(ctx, wsURL.String(), nil)
	if err != nil {
		detail := ""
		if wsResp != nil && wsResp.Body != nil {
			body, _ := io.ReadAll(io.LimitReader(wsResp.Body, 2048))
			_ = wsResp.Body.Close()
			detail = strings.TrimSpace(string(body))
		}
		if detail != "" {
			return fmt.Errorf("CDP websocket connect: %w: %s", err, detail)
		}
		return fmt.Errorf("CDP websocket connect: %w", err)
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteJSON(map[string]any{
		"id":     1,
		"method": "Target.createTarget",
		"params": map[string]any{"url": authURL},
	}); err != nil {
		return fmt.Errorf("CDP Target.createTarget write: %w", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		var response cdpResponse
		if err := conn.ReadJSON(&response); err != nil {
			return fmt.Errorf("CDP Target.createTarget read: %w", err)
		}
		if response.ID != 1 {
			continue
		}
		if response.Error != nil {
			return fmt.Errorf("CDP Target.createTarget error %d: %s", response.Error.Code, response.Error.Message)
		}
		return nil
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
		client = &http.Client{Timeout: 12 * time.Second}
	}

	// Stale-tab cleanup is best-effort. It must never turn a recoverable OAuth
	// flow into a failure, especially with relays that expose only /json/list.
	_ = b.clearPageTargets(ctx, client, base)

	var lastErr error
	delays := []time.Duration{0, 500 * time.Millisecond, 1500 * time.Millisecond}
	for _, delay := range delays {
		if delay > 0 {
			if err := sleepContext(ctx, delay); err != nil {
				return err
			}
		}

		// Direct Chromium exposes PUT /json/new. Keep that fast path.
		if err := b.openViaHTTPNew(ctx, client, base, authURL); err == nil {
			return nil
		} else {
			lastErr = err
		}

		// CloakBrowser exposes a browser-level CDP websocket but intentionally
		// does not proxy /json/new. Target.createTarget is the portable fallback.
		if err := b.openViaWebSocket(ctx, client, base, authURL); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("%v; websocket fallback: %w", lastErr, err)
		}
	}
	return fmt.Errorf("open OAuth tab after retries: %w", lastErr)
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

func isTransientOAuthStartError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"http 500", "http 502", "http 503", "http 504",
		"timeout", "deadline exceeded", "connection refused", "connection reset",
		"temporary", "eof", "network is unreachable",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func startOAuthWithRetry(ctx context.Context, starter OAuthStarter, server string) (mcpproxy.OAuthStart, error) {
	var lastErr error
	delays := []time.Duration{0, 750 * time.Millisecond, 2 * time.Second}
	for i, delay := range delays {
		if delay > 0 {
			if err := sleepContext(ctx, delay); err != nil {
				return mcpproxy.OAuthStart{}, err
			}
		}
		start, err := starter.StartOAuth(ctx, server)
		if err == nil {
			return start, nil
		}
		lastErr = err
		if i == len(delays)-1 || !isTransientOAuthStartError(err) {
			break
		}
	}
	return mcpproxy.OAuthStart{}, lastErr
}

func (s Service) Start(ctx context.Context, server string) (StartResult, error) {
	return s.StartWithBrowser(ctx, server, s.Browser)
}

func (s Service) StartWithBrowser(ctx context.Context, server string, browser Browser) (StartResult, error) {
	// Never revoke the currently stored token before a replacement OAuth flow
	// succeeds. A network or browser failure must leave the previous refresh
	// state intact so a simple upstream restart can recover after connectivity
	// returns.
	start, err := startOAuthWithRetry(ctx, s.Starter, server)
	if err != nil {
		return StartResult{}, fmt.Errorf("start OAuth session: %w", err)
	}
	if err := browser.Open(ctx, start.AuthURL); err != nil {
		return StartResult{}, fmt.Errorf("open OAuth browser: %w", err)
	}
	browserURL := browser.SessionURL()
	if browserURL == "" {
		return StartResult{}, errors.New("OAuth browser has no public session URL")
	}
	return StartResult{
		BrowserURL:    browserURL,
		CorrelationID: start.CorrelationID,
	}, nil
}
