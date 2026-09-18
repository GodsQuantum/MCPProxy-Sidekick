package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/oauth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/profiles"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	app := &web.Server{Cfg: cfg}
	if !cfg.DemoMode {
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
		store, err := profiles.Open(cfg.DBPath)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		app.Auth = authManager
		app.Proxy = proxy
		app.Profiles = store
		app.Credentials = credentials.Service{Editor: proxy}
		app.Tokens = tokens.Service{Backend: proxy}
		app.OAuth = oauth.Service{Starter: proxy, Browser: oauth.Browser{CDPBaseURL: cfg.OAuthCDPURL, PublicSessionURL: cfg.OAuthBrowserURL}}
	}
	srv := &http.Server{Addr: cfg.ListenAddr, Handler: app.Handler(), ReadHeaderTimeout: 5_000_000_000, IdleTimeout: 60_000_000_000}
	log.Printf("MCPProxy Sidekick listening on %s", cfg.ListenAddr)
	log.Fatal(srv.ListenAndServe())
}
