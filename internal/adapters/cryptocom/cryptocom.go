package cryptocom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ToolCaller interface {
	CallTool(context.Context, string, map[string]interface{}) (json.RawMessage, error)
}

type Adapter struct {
	Proxy            ToolCaller
	PendingDir       string
	PendingHostDir   string
	SSHTransferRoot  string
	RemoteEnvPath    string
	RemoteComposeDir string
	RemoteUser       string
	SourceSSHProfile string
	RemoteSSHProfile string
}

type Result struct {
	RotateBy string
}

type vaultSearchResult struct {
	Results []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"results"`
}

func (a Adapter) Configured() bool {
	return a.Proxy != nil &&
		strings.TrimSpace(a.PendingDir) != "" &&
		strings.TrimSpace(a.PendingHostDir) != "" &&
		strings.TrimSpace(a.SSHTransferRoot) != "" &&
		strings.TrimSpace(a.RemoteEnvPath) != "" &&
		strings.TrimSpace(a.RemoteComposeDir) != "" &&
		strings.TrimSpace(a.RemoteUser) != "" &&
		strings.TrimSpace(a.SourceSSHProfile) != "" &&
		strings.TrimSpace(a.RemoteSSHProfile) != ""
}

func (a Adapter) Apply(ctx context.Context, apiKey, apiSecret string, expiresDays int) (Result, error) {
	apiKey = strings.TrimSpace(apiKey)
	apiSecret = strings.TrimSpace(apiSecret)
	if !a.Configured() {
		return Result{}, errors.New("Crypto.com credential adapter is not configured")
	}
	if apiKey == "" || apiSecret == "" {
		return Result{}, errors.New("Crypto.com API key and API secret are both required")
	}
	if strings.ContainsAny(apiKey, "\r\n") || strings.ContainsAny(apiSecret, "\r\n") {
		return Result{}, errors.New("Crypto.com credentials must be single-line values")
	}
	if expiresDays <= 0 {
		expiresDays = 90
	}
	if expiresDays > 365 {
		return Result{}, errors.New("Crypto.com expiry must be at most 365 days")
	}

	if err := os.MkdirAll(a.PendingDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create credential inbox: %w", err)
	}
	pendingName := "cryptocom-live.env"
	pendingPath := filepath.Join(a.PendingDir, pendingName)
	tmp, err := os.CreateTemp(a.PendingDir, ".cryptocom-live-*.tmp")
	if err != nil {
		return Result{}, fmt.Errorf("create pending credential: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	_ = tmp.Chmod(0o600)
	body := "CDCX_API_KEY=" + apiKey + "\nCDCX_API_SECRET=" + apiSecret + "\nCDCX_PROFILE=live\n"
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		return Result{}, fmt.Errorf("write pending credential: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return Result{}, fmt.Errorf("sync pending credential: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return Result{}, fmt.Errorf("close pending credential: %w", err)
	}
	if err := os.Rename(tmpName, pendingPath); err != nil {
		return Result{}, fmt.Errorf("publish pending credential: %w", err)
	}

	hostPending := filepath.Join(a.PendingHostDir, pendingName)
	transferFile := filepath.Join(a.SSHTransferRoot, pendingName)
	remoteTmp := "/tmp/cryptocom-live.env.sidekick"

	cleanupPending := func() { _ = os.Remove(pendingPath) }
	cleanupTransfer := func() {
		_, _ = a.callDestructive(ctx, "ssh-actions:privileged-command", map[string]interface{}{
			"profile": a.SourceSSHProfile,
			"command": "rm -f -- " + shellQuote(transferFile),
		}, "Remove transient Crypto.com credential transfer file")
	}
	defer cleanupPending()
	defer cleanupTransfer()

	if _, err := a.callDestructive(ctx, "ssh-actions:privileged-command", map[string]interface{}{
		"profile": a.SourceSSHProfile,
		"command": "install -m 0600 -o ssh-mcp -g ssh-mcp -- " + shellQuote(hostPending) + " " + shellQuote(transferFile),
	}, "Stage Crypto.com credential file inside the SSH MCP transfer root"); err != nil {
		return Result{}, fmt.Errorf("stage Crypto.com credential: %w", err)
	}

	if _, err := a.callDestructive(ctx, "ssh-actions:sftp-upload-file", map[string]interface{}{
		"profile":    a.RemoteSSHProfile,
		"localPath":  transferFile,
		"remotePath": remoteTmp,
		"overwrite":  true,
		"mode":       384,
	}, "Stream Crypto.com credential file to the finance container without exposing its contents"); err != nil {
		return Result{}, fmt.Errorf("transfer Crypto.com credential: %w", err)
	}

	remoteCommand := "cat " + shellQuote(remoteTmp) + " | sudo -u " + shellQuote(a.RemoteUser) + " tee " + shellQuote(a.RemoteEnvPath) + " >/dev/null" +
		" && sudo -u " + shellQuote(a.RemoteUser) + " chmod 0600 " + shellQuote(a.RemoteEnvPath) +
		" && cd " + shellQuote(a.RemoteComposeDir) +
		" && docker compose up -d cdcx-live-mcp" +
		" && rm -f -- " + shellQuote(remoteTmp)
	if _, err := a.callDestructive(ctx, "ssh-actions:privileged-command", map[string]interface{}{
		"profile": a.RemoteSSHProfile,
		"command": remoteCommand,
	}, "Install rotated Crypto.com credentials and restart only the existing live gateway"); err != nil {
		return Result{}, fmt.Errorf("activate Crypto.com credential: %w", err)
	}

	return Result{RotateBy: time.Now().UTC().AddDate(0, 0, expiresDays).Format("2006-01-02")}, nil
}

func (a Adapter) ApplyFromVaultwarden(ctx context.Context, itemName string, expiresDays int) (Result, error) {
	itemName = strings.TrimSpace(itemName)
	if itemName == "" {
		return Result{}, errors.New("Vaultwarden item name is required")
	}
	if _, err := a.callRead(ctx, "vaultwarden:keychain_sync", map[string]interface{}{}, "Sync Vaultwarden before Crypto.com credential rotation"); err != nil {
		return Result{}, fmt.Errorf("sync Vaultwarden: %w", err)
	}
	raw, err := a.callRead(ctx, "vaultwarden:keychain_search_items", map[string]interface{}{
		"text":  itemName,
		"type":  "login",
		"limit": 25,
	}, "Locate the exact Crypto.com rotation credential in Vaultwarden")
	if err != nil {
		return Result{}, fmt.Errorf("search Vaultwarden: %w", err)
	}
	payload, err := mcpTextPayload(raw)
	if err != nil {
		return Result{}, fmt.Errorf("parse Vaultwarden search result: %w", err)
	}
	var search vaultSearchResult
	if err := json.Unmarshal([]byte(payload), &search); err != nil {
		return Result{}, fmt.Errorf("decode Vaultwarden search result: %w", err)
	}
	matches := make([]string, 0, 1)
	for _, item := range search.Results {
		if strings.EqualFold(strings.TrimSpace(item.Name), itemName) && strings.EqualFold(strings.TrimSpace(item.Type), "login") {
			matches = append(matches, strings.TrimSpace(item.ID))
		}
	}
	if len(matches) != 1 || matches[0] == "" {
		return Result{}, fmt.Errorf("Vaultwarden must contain exactly one login named %q; found %d", itemName, len(matches))
	}
	itemID := matches[0]

	itemRaw, err := a.callRead(ctx, "vaultwarden:keychain_get_item", map[string]interface{}{"id": itemID, "reveal": true}, "Read Crypto.com custom credential fields from the exact Vaultwarden item")
	if err != nil {
		return Result{}, fmt.Errorf("read Crypto.com Vaultwarden item: %w", err)
	}
	payload, err = mcpTextPayload(itemRaw)
	if err != nil {
		return Result{}, fmt.Errorf("parse Crypto.com Vaultwarden item: %w", err)
	}
	var itemEnvelope struct {
		Item struct {
			Fields []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"fields"`
		} `json:"item"`
	}
	if err := json.Unmarshal([]byte(payload), &itemEnvelope); err != nil {
		return Result{}, fmt.Errorf("decode Crypto.com Vaultwarden item: %w", err)
	}
	var apiKey, apiSecret string
	for _, field := range itemEnvelope.Item.Fields {
		name := strings.ToLower(strings.TrimSpace(field.Name))
		switch name {
		case "clé api", "cle api", "api key", "apikey":
			apiKey = strings.TrimSpace(field.Value)
		case "clé secrète", "cle secrete", "client secret", "api secret", "secret":
			apiSecret = strings.TrimSpace(field.Value)
		}
	}
	if apiKey == "" || apiSecret == "" {
		return Result{}, errors.New("Crypto.com Vaultwarden login must contain custom fields 'Clé API' and 'Clé secrète' (or Client secret)")
	}
	return a.Apply(ctx, apiKey, apiSecret, expiresDays)
}

func (a Adapter) callRead(ctx context.Context, name string, args map[string]interface{}, reason string) (json.RawMessage, error) {
	return a.Proxy.CallTool(ctx, "call_tool_read", map[string]interface{}{
		"name":                    name,
		"args":                    args,
		"intent_reason":           reason,
		"intent_data_sensitivity": "private",
	})
}

func mcpTextPayload(raw json.RawMessage) (string, error) {
	type textPart struct {
		Text string `json:"text"`
	}
	var result struct {
		Content []textPart `json:"content"`
		IsError bool       `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		var parts []textPart
		if err2 := json.Unmarshal(raw, &parts); err2 != nil {
			return "", err
		}
		result.Content = parts
	}
	if result.IsError {
		return "", errors.New("MCP tool returned an error")
	}
	for _, part := range result.Content {
		t := strings.TrimSpace(part.Text)
		if t == "" {
			continue
		}
		const prefix = "[map[text:"
		const suffix = " type:text]]"
		if strings.HasPrefix(t, prefix) {
			if i := strings.LastIndex(t, suffix); i > len(prefix) {
				t = t[len(prefix):i]
			}
		}
		if strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t), nil
		}
	}
	return "", errors.New("MCP tool returned no text payload")
}

func mcpScalarValue(raw json.RawMessage) (string, error) {
	payload, err := mcpTextPayload(raw)
	if err != nil {
		return "", err
	}
	var wrapped struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if strings.HasPrefix(strings.TrimSpace(payload), "{") && json.Unmarshal([]byte(payload), &wrapped) == nil {
		if v := strings.TrimSpace(wrapped.Result.Value); v != "" {
			return v, nil
		}
	}
	return strings.TrimSpace(payload), nil
}

func (a Adapter) callDestructive(ctx context.Context, name string, args map[string]interface{}, reason string) (json.RawMessage, error) {
	return a.Proxy.CallTool(ctx, "call_tool_destructive", map[string]interface{}{
		"name":                    name,
		"args":                    args,
		"intent_reason":           reason,
		"intent_data_sensitivity": "private",
	})
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
