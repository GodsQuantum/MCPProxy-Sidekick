package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	cryptocomadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/cryptocom"
	difypinadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/difypin"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/oauth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/runtimepriv"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/storage"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/tokens"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8081/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "dify-pin" {
		runDifyPin(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "cryptocom-set-vault" {
		runCryptoComSetVault()
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	app := &web.Server{Cfg: cfg}
	if !cfg.DemoMode {
		target, err := runtimepriv.FromEnv()
		if err != nil {
			log.Fatal(err)
		}
		if target != nil {
			if cfg.MCPProxyAdminKey == "" {
				log.Fatal("privilege drop requires SIDEKICK_MCPPROXY_CONFIG_FILE so the admin key is loaded before dropping privileges")
			}
			if err := runtimepriv.Drop(target); err != nil {
				log.Fatal(err)
			}
			log.Printf("Dropped runtime privileges to uid=%d gid=%d", target.UID, target.GID)
		}
		var authManager *auth.Manager
		var proxy *mcpproxy.Client
		if cfg.MCPProxyAdminKey != "" {
			authManager, err = auth.NewManagerFromKey(cfg.MCPProxyAdminKey, cfg.SessionLifetime)
			if err == nil {
				proxy, err = mcpproxy.NewClientWithKey(cfg.MCPProxyBaseURL, cfg.MCPProxyAdminKey)
			}
		} else {
			authManager, err = auth.NewManager(cfg.MCPProxyAdminKeyFile, cfg.SessionLifetime)
			if err == nil {
				proxy, err = mcpproxy.NewClient(cfg.MCPProxyBaseURL, cfg.MCPProxyAdminKeyFile)
			}
		}
		if err != nil {
			log.Fatal(err)
		}
		store, err := storage.Open(cfg.DBPath)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		app.Auth = authManager
		app.Proxy = proxy
		app.Store = store
		app.Credentials = credentials.Service{Editor: proxy}
		app.Tokens = tokens.Service{Backend: proxy}
		app.OAuth = oauth.Service{Starter: proxy, Browser: oauth.Browser{CDPBaseURL: cfg.OAuthCDPURL, PublicSessionURL: cfg.OAuthBrowserURL}}
	}
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: app.Handler(), ReadHeaderTimeout: 5_000_000_000, IdleTimeout: 60_000_000_000}
	log.Printf("MCPProxy Sidekick listening on %s", cfg.ListenAddr)
	log.Fatal(srv.ListenAndServe())
}

func runCryptoComSetVault() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.DemoMode {
		log.Fatal("cryptocom-set-vault is unavailable in demo mode")
	}
	target, err := runtimepriv.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if target != nil {
		if cfg.MCPProxyAdminKey == "" {
			log.Fatal("cryptocom-set-vault requires the MCPProxy admin key to be loaded before privilege drop")
		}
		if err := runtimepriv.Drop(target); err != nil {
			log.Fatal(err)
		}
	}
	var proxy *mcpproxy.Client
	if cfg.MCPProxyAdminKey != "" {
		proxy, err = mcpproxy.NewClientWithKey(cfg.MCPProxyBaseURL, cfg.MCPProxyAdminKey)
	} else {
		proxy, err = mcpproxy.NewClient(cfg.MCPProxyBaseURL, cfg.MCPProxyAdminKeyFile)
	}
	if err != nil {
		log.Fatal(err)
	}
	adapter := cryptocomadapter.Adapter{
		Proxy:            proxy,
		PendingDir:       filepath.Join(filepath.Dir(cfg.DBPath), "credential-inbox"),
		PendingHostDir:   cfg.CredentialPendingHostDir,
		SSHTransferRoot:  cfg.CredentialSSHTransferRoot,
		RemoteEnvPath:    cfg.CryptoComRemoteEnvPath,
		RemoteComposeDir: cfg.CryptoComRemoteComposeDir,
		RemoteService:    cfg.CryptoComRemoteService,
		RemoteUser:       cfg.CryptoComRemoteUser,
		SourceSSHProfile: cfg.CredentialSourceSSHProfile,
		RemoteSSHProfile: cfg.CryptoComRemoteSSHProfile,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := adapter.ApplyFromVaultwarden(ctx, "Crypto.com", 90)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("cryptocom credentials set from Vaultwarden; valid_until=%s\n", result.RotateBy)
}

func runDifyPin(args []string) {
	if len(args) < 4 || len(args) > 5 {
		log.Fatal("usage: sidekick dify-pin <token-name> <profile> <provider-id> <permissions-csv> [expiry]")
	}
	name := strings.TrimSpace(args[0])
	profile := strings.TrimSpace(args[1])
	providerID := strings.TrimSpace(args[2])
	permissions := splitNonEmpty(args[3])
	expiry := "365d"
	if len(args) == 5 && strings.TrimSpace(args[4]) != "" {
		expiry = strings.TrimSpace(args[4])
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.DemoMode {
		log.Fatal("dify-pin is unavailable in demo mode")
	}
	target, err := runtimepriv.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if target != nil {
		if cfg.MCPProxyAdminKey == "" {
			log.Fatal("dify-pin requires the MCPProxy admin key to be loaded before privilege drop")
		}
		if err := runtimepriv.Drop(target); err != nil {
			log.Fatal(err)
		}
	}

	var proxy *mcpproxy.Client
	if cfg.MCPProxyAdminKey != "" {
		proxy, err = mcpproxy.NewClientWithKey(cfg.MCPProxyBaseURL, cfg.MCPProxyAdminKey)
	} else {
		proxy, err = mcpproxy.NewClient(cfg.MCPProxyBaseURL, cfg.MCPProxyAdminKeyFile)
	}
	if err != nil {
		log.Fatal(err)
	}

	tokenService := tokens.Service{Backend: proxy}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	existing, err := tokenService.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range existing {
		if item.Name == name && !item.Revoked {
			log.Fatalf("token %s already exists; refusing to overwrite an active credential", name)
		}
	}

	created, err := tokenService.Create(ctx, tokens.CreateRequest{
		Name:           name,
		AllowedServers: []string{"*"},
		Permissions:    permissions,
		ExpiresIn:      expiry,
		ProfilePin:     profile,
	})
	if err != nil {
		log.Fatal(err)
	}

	adapter := difypinadapter.Adapter{
		Proxy:            proxy,
		PendingDir:       filepath.Join(filepath.Dir(cfg.DBPath), "credential-inbox"),
		PendingHostDir:   cfg.CredentialPendingHostDir,
		SSHTransferRoot:  cfg.CredentialSSHTransferRoot,
		SourceSSHProfile: cfg.CredentialSourceSSHProfile,
		RemoteSSHProfile: cfg.DifyRemoteSSHProfile,
		RemoteHelperPath: cfg.DifyApplyHelperPath,
	}
	if err := adapter.Apply(ctx, name, providerID, profile, created.Token); err != nil {
		_ = tokenService.Delete(context.Background(), name)
		log.Fatalf("dify-pin failed and the newly created token was removed: %v", err)
	}
	fmt.Printf("dify-pin ok: name=%s profile=%s permissions=%s provider_id=%s expires_at=%s\n",
		name, profile, strings.Join(permissions, ","), providerID, created.ExpiresAt.Format(time.RFC3339))
}

func splitNonEmpty(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
