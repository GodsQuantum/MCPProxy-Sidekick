package difypin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ToolCaller interface {
	CallTool(context.Context, string, map[string]interface{}) (json.RawMessage, error)
}

type Adapter struct {
	Proxy            ToolCaller
	PendingDir       string
	PendingHostDir   string
	SSHTransferRoot  string
	SourceSSHProfile string
	RemoteSSHProfile string
	RemoteHelperPath string
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func (a Adapter) Apply(ctx context.Context, tokenName, providerID, profile, token string) error {
	tokenName = strings.TrimSpace(tokenName)
	providerID = strings.TrimSpace(providerID)
	profile = strings.TrimSpace(profile)
	token = strings.TrimSpace(token)
	if a.Proxy == nil || a.PendingDir == "" || a.PendingHostDir == "" || a.SSHTransferRoot == "" ||
		a.SourceSSHProfile == "" || a.RemoteSSHProfile == "" || a.RemoteHelperPath == "" {
		return errors.New("Dify pin adapter is not fully configured")
	}
	if !safeName.MatchString(tokenName) || providerID == "" || profile == "" {
		return errors.New("invalid Dify pin metadata")
	}
	if !strings.HasPrefix(token, "mcp_") || len(token) < 24 {
		return errors.New("invalid MCPProxy agent token")
	}

	if err := os.MkdirAll(a.PendingDir, 0o700); err != nil {
		return fmt.Errorf("create credential inbox: %w", err)
	}
	fileName := "dify-pin-" + tokenName + ".json"
	pendingPath := filepath.Join(a.PendingDir, fileName)
	hostPending := filepath.Join(a.PendingHostDir, fileName)
	transferFile := filepath.Join(a.SSHTransferRoot, fileName)
	remoteTmp := "/tmp/" + fileName

	raw, err := json.Marshal(map[string]string{
		"provider_id": providerID,
		"profile":     profile,
		"token":       token,
	})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(a.PendingDir, ".dify-pin-*.tmp")
	if err != nil {
		return fmt.Errorf("create pending Dify credential: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	_ = tmp.Chmod(0o600)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write pending Dify credential: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, pendingPath); err != nil {
		return fmt.Errorf("publish pending Dify credential: %w", err)
	}
	defer os.Remove(pendingPath)
	defer func() {
		_, _ = a.callDestructive(ctx, "ssh-actions:privileged-command", map[string]interface{}{
			"profile": a.SourceSSHProfile,
			"command": "rm -f -- " + shellQuote(transferFile),
		}, "Remove transient Dify credential transfer file")
	}()

	if _, err := a.callDestructive(ctx, "ssh-actions:privileged-command", map[string]interface{}{
		"profile": a.SourceSSHProfile,
		"command": "install -m 0600 -o ssh-mcp -g ssh-mcp -- " + shellQuote(hostPending) + " " + shellQuote(transferFile),
	}, "Stage Dify profile-pinned MCP credential inside the SSH MCP transfer root"); err != nil {
		return fmt.Errorf("stage Dify credential: %w", err)
	}

	if _, err := a.callDestructive(ctx, "ssh-actions:sftp-upload-file", map[string]interface{}{
		"profile":    a.RemoteSSHProfile,
		"localPath":  transferFile,
		"remotePath": remoteTmp,
		"overwrite":  true,
		"mode":       384,
	}, "Stream Dify MCP credential into the Dify host without exposing its value"); err != nil {
		return fmt.Errorf("transfer Dify credential: %w", err)
	}

	if _, err := a.callDestructive(ctx, "ssh-actions:privileged-command", map[string]interface{}{
		"profile": a.RemoteSSHProfile,
		"command": shellQuote(a.RemoteHelperPath) + " " + shellQuote(remoteTmp),
	}, "Apply profile-pinned MCP credential through Dify native MCP provider encryption"); err != nil {
		return fmt.Errorf("apply Dify credential: %w", err)
	}
	return nil
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
