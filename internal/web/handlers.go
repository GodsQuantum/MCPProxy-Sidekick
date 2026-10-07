package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	cryptocomadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/cryptocom"
	difypinadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/difypin"
	immichadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/immich"
	omnirouteadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/omniroute"
	openalexadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/openalex"
	paperlessadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/paperless"
	postizadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/postiz"
	youtubeoauthadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/youtubeoauth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/capabilities"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/connections"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/oauth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/oauthconfig"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/storage"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/tokens"
)

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.DemoMode {
		s.handleDemoState(w)
		return
	}
	warnings := []string{}
	servers, err := s.Proxy.ListServers(r.Context())
	if err != nil {
		log.Printf("sidekick state: MCPProxy server inventory unavailable: %v", err)
		warnings = append(warnings, "MCPProxy server inventory is temporarily unavailable")
		servers = nil
	}
	ps, err := s.Proxy.ListProfiles(r.Context())
	if err != nil {
		log.Printf("sidekick state: MCPProxy profile inventory unavailable: %v", err)
		warnings = append(warnings, "MCPProxy profile inventory is temporarily unavailable")
		ps = nil
	}
	toks, err := s.Tokens.List(r.Context())
	if err != nil {
		log.Printf("sidekick state: MCPProxy Agent Token inventory unavailable: %v", err)
		warnings = append(warnings, "MCPProxy Agent Tokens are temporarily unavailable")
		toks = nil
	}
	pm := profileMap(ps)
	var youtubeOAuth youtubeoauthadapter.Status
	var youtubeOAuthErr error
	if strings.TrimSpace(s.Cfg.YouTubeOAuthControlURL) != "" {
		youtubeOAuth, youtubeOAuthErr = (youtubeoauthadapter.Adapter{BaseURL: s.Cfg.YouTubeOAuthControlURL}).Status(r.Context())
		if youtubeOAuthErr != nil {
			log.Printf("sidekick state: YouTube OAuth helper unavailable: %v", youtubeOAuthErr)
			warnings = append(warnings, "YouTube OAuth helper is temporarily unavailable")
		}
	}
	safe := make([]safeUpstream, 0, len(servers))
	connected, tools, authNeeded, quarantined := 0, 0, 0, 0
	for _, srv := range servers {
		meta, ok, merr := s.Store.CredentialMeta(srv.Name)
		if merr != nil {
			writeError(w, 500, merr.Error())
			return
		}
		configured, preview := credentialView(srv, meta, ok)
		st := srv.Status
		if st == "" {
			st = srv.Health.Summary
		}
		oauthEnabled := len(srv.OAuth) > 0
		authenticated := srv.Authenticated
		if strings.TrimSpace(s.Cfg.YouTubeOAuthControlURL) != "" && youtubeoauthadapter.MatchesServer(srv.Name) {
			oauthEnabled = true
			if youtubeOAuthErr == nil {
				connected := youtubeoauthadapter.ConnectedForServer(srv.Name, youtubeOAuth)
				configured = connected
				authenticated = connected
				if connected {
					preview = "OAuth connected"
					if strings.TrimSpace(youtubeOAuth.Handle) != "" {
						preview += " · " + youtubeOAuth.Handle
					}
					st = "ready"
				} else {
					preview = "OAuth not connected"
					st = "auth required"
				}
			} else {
				configured = false
				authenticated = false
				preview = "OAuth helper unavailable"
				st = "auth helper unavailable"
			}
		}
		if strings.Contains(strings.ToLower(st), "ready") || strings.Contains(strings.ToLower(st), "connected") {
			connected++
		}
		if strings.Contains(strings.ToLower(st), "auth") && !configured {
			authNeeded++
		}
		if srv.Quarantined {
			quarantined++
		}
		tools += srv.ToolCount
		safe = append(safe, safeUpstream{Name: srv.Name, Enabled: srv.Enabled, Status: st, Protocol: srv.Protocol, ToolCount: srv.ToolCount, Authenticated: authenticated, Quarantined: srv.Quarantined, OAuth: oauthEnabled, CredentialConfigured: configured, CredentialPreview: preview, Profiles: pm[srv.Name]})
	}
	mcpCaps := capabilities.Detect(r.Context(), s.Proxy)
	writeJSON(w, 200, map[string]any{
		"summary":   map[string]int{"total": len(safe), "connected": connected, "tools": tools, "auth_required": authNeeded, "quarantined": quarantined},
		"upstreams": safe, "profiles": ps, "tokens": toks, "warnings": warnings, "csrf": r.Header.Get("X-Sidekick-CSRF-Expected"),
		"capabilities": map[string]bool{"omniroute_restore_master": strings.TrimSpace(s.Cfg.OmniRouteDB) != ""},
		"mcpproxy":     mcpCaps,
		"settings":     s.safeSettings(),
	})
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	servers, err := s.Proxy.ListServers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	profiles, err := s.Proxy.ListProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := make([]connections.Detail, 0, len(servers))
	for _, server := range servers {
		meta, ok, err := s.Store.CredentialMeta(server.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, connections.Build(server, profiles, meta, ok))
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": out})
}

func (s *Server) handleConnectionDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	server, err := s.Proxy.GetServer(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	profiles, err := s.Proxy.ListProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	meta, ok, err := s.Store.CredentialMeta(server.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connections.Build(server, profiles, meta, ok))
}

func (s *Server) handleConnectionPatch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing server name")
		return
	}
	var req struct {
		URL      string `json:"url"`
		Protocol string `json:"protocol"`
		Enabled  *bool  `json:"enabled"`
		Action   string `json:"action"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "":
		if req.URL == "" && req.Protocol == "" && req.Enabled == nil {
			writeError(w, http.StatusBadRequest, "no connection changes requested")
			return
		}
		if err := s.Proxy.PatchServer(r.Context(), name, mcpproxy.ServerPatch{
			URL: req.URL, Protocol: req.Protocol, Enabled: req.Enabled,
		}); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "enable":
		if err := s.Proxy.EnableServer(r.Context(), name); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	case "disable":
		if err := s.Proxy.DisableServer(r.Context(), name); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	case "restart":
		if err := s.Proxy.RestartServer(r.Context(), name); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported connection action")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOAuthScopesGet(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	scopes, err := (oauthconfig.Service{Backend: s.Proxy}).Current(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scopes": scopes})
}

func (s *Server) handleOAuthScopesPreview(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	var req struct {
		Scopes []string `json:"scopes"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	diff, err := (oauthconfig.Service{Backend: s.Proxy}).Preview(r.Context(), name, req.Scopes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

func (s *Server) handleOAuthScopesApply(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	var req struct {
		Scopes []string `json:"scopes"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	result, err := (oauthconfig.Service{Backend: s.Proxy}).Apply(r.Context(), name, req.Scopes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) cryptoComCredentialAdapter(name string) (cryptocomadapter.Adapter, bool) {
	adapter := cryptocomadapter.Adapter{
		Proxy:            s.Proxy,
		PendingDir:       filepath.Join(filepath.Dir(s.Cfg.DBPath), "credential-inbox"),
		PendingHostDir:   s.Cfg.CredentialPendingHostDir,
		SSHTransferRoot:  s.Cfg.CredentialSSHTransferRoot,
		RemoteUser:       s.Cfg.CryptoComRemoteUser,
		SourceSSHProfile: s.Cfg.CredentialSourceSSHProfile,
		RemoteSSHProfile: s.Cfg.CryptoComRemoteSSHProfile,
	}
	switch name {
	case "cryptocom-app":
		adapter.Kind = "app"
		adapter.RemoteEnvPath = s.Cfg.CryptoComAppRemoteEnvPath
		adapter.RemoteComposeDir = s.Cfg.CryptoComAppRemoteComposeDir
		adapter.RemoteService = s.Cfg.CryptoComAppRemoteService
		return adapter, true
	case "cryptocom-exchange":
		adapter.Kind = "exchange"
		adapter.RemoteEnvPath = s.Cfg.CryptoComExchangeRemoteEnvPath
		adapter.RemoteComposeDir = s.Cfg.CryptoComExchangeRemoteComposeDir
		adapter.RemoteService = s.Cfg.CryptoComExchangeRemoteService
		return adapter, true
	default:
		return cryptocomadapter.Adapter{}, false
	}
}

func (s *Server) handleCredential(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeError(w, 400, "missing server name")
		return
	}
	var req struct {
		Mode        string `json:"mode"`
		HeaderName  string `json:"header_name"`
		Value       string `json:"value"`
		Secret      string `json:"secret"`
		VaultItem   string `json:"vault_item"`
		ExpiresDays int    `json:"expires_days"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}

	preview := credentials.Mask(req.Value)
	fingerprintInput := req.Value
	var err error
	switch {
	case (name == "cryptocom-app" || name == "cryptocom-exchange") && req.Mode == "cryptocom-api-pair":
		adapter, _ := s.cryptoComCredentialAdapter(name)
		var result cryptocomadapter.Result
		result, err = adapter.Apply(r.Context(), req.Value, req.Secret, req.ExpiresDays)
		if err == nil {
			label := "App"
			if name == "cryptocom-exchange" {
				label = "Exchange"
			}
			preview = "Crypto.com " + label + " key " + credentials.Mask(req.Value) + " + secret set · valid until " + result.RotateBy
			fingerprintInput = name + "|" + req.Value + "|" + req.Secret
		}
	case (name == "cryptocom-app" || name == "cryptocom-exchange") && req.Mode == "cryptocom-vaultwarden":
		adapter, _ := s.cryptoComCredentialAdapter(name)
		var result cryptocomadapter.Result
		result, err = adapter.ApplyFromVaultwarden(r.Context(), req.VaultItem, req.ExpiresDays)
		if err == nil {
			label := "App"
			if name == "cryptocom-exchange" {
				label = "Exchange"
			}
			preview = "Crypto.com " + label + " from Vaultwarden " + strings.TrimSpace(req.VaultItem) + " · valid until " + result.RotateBy
			fingerprintInput = name + "|vaultwarden|" + strings.TrimSpace(req.VaultItem) + "|" + result.RotateBy
		}
	case name == "openalex-github" && s.Cfg.OpenAlexKeyFile != "":
		err = (openalexadapter.Adapter{KeyFile: s.Cfg.OpenAlexKeyFile}).Apply(req.Value)
	case name == "postiz" && s.Cfg.PostizBaseURL != "":
		err = postizadapter.Adapter{Editor: s.Proxy, BaseURL: s.Cfg.PostizBaseURL, ServerName: name}.Apply(r.Context(), req.Value)
	case (name == "paperless" || strings.HasPrefix(name, "paperless-")) && s.Cfg.PaperlessEndpoint != "":
		err = paperlessadapter.Adapter{Editor: s.Proxy, Endpoint: s.Cfg.PaperlessEndpoint}.Apply(r.Context(), name, req.Value)
	case (name == "immich" || strings.HasPrefix(name, "immich-")) && s.Cfg.ImmichKeyDir != "":
		if filepath.Base(name) != name {
			writeError(w, 400, "invalid Immich server name")
			return
		}
		err = immichadapter.Adapter{Editor: s.Proxy, KeyFile: filepath.Join(s.Cfg.ImmichKeyDir, name), ServerName: name}.Apply(r.Context(), req.Value)
	default:
		err = s.Credentials.SetGeneric(r.Context(), name, req.Mode, req.HeaderName, req.Value)
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	_ = s.Store.UpsertCredentialMeta(storage.CredentialMeta{ServerName: name, MaskedPreview: preview, Fingerprint: credentials.Fingerprint(fingerprintInput), UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
	writeJSON(w, 200, map[string]any{"ok": true, "preview": preview})
}

func (s *Server) handleOmniRouteRestoreMaster(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.Cfg.OmniRouteDB) == "" {
		writeError(w, 400, "OmniRoute database is not configured")
		return
	}
	adapter := omnirouteadapter.Adapter{Editor: s.Proxy, DBPath: s.Cfg.OmniRouteDB, ServerName: "omniroute"}
	if err := adapter.RestoreMaster(r.Context()); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeError(w, 400, "missing server name")
		return
	}
	instance, err := s.ensureActiveBrowser(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	browser := oauth.Browser{CDPBaseURL: instance.CDPURL, PublicSessionURL: instance.BrowserURL}
	if strings.TrimSpace(s.Cfg.YouTubeOAuthControlURL) != "" && youtubeoauthadapter.MatchesServer(name) {
		start, err := (youtubeoauthadapter.Adapter{BaseURL: s.Cfg.YouTubeOAuthControlURL}).Start(r.Context())
		if err != nil {
			writeError(w, 502, err.Error())
			return
		}
		if err := browser.Open(r.Context(), start.AuthURL); err != nil {
			writeError(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"browser_url": browser.SessionURL(), "provider": "youtube", "profile": "full"})
		return
	}
	result, err := s.OAuth.StartWithBrowser(r.Context(), name, browser)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) profileV3Available(ctx context.Context) bool {
	return capabilities.Detect(ctx, s.Proxy).ProfileV3
}

func (s *Server) handleProfileAdvanced(w http.ResponseWriter, r *http.Request) {
	if !s.profileV3Available(r.Context()) {
		writeError(w, http.StatusNotFound, "Profiles v3 is not supported by connected MCPProxy")
		return
	}
	name := strings.TrimSpace(r.PathValue("id"))
	profile, err := s.Proxy.GetProfileV3(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleProfileAdvancedUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.profileV3Available(r.Context()) {
		writeError(w, http.StatusNotFound, "Profiles v3 is not supported by connected MCPProxy")
		return
	}
	name := strings.TrimSpace(r.PathValue("id"))
	var profile mcpproxy.ProfileV3
	if decodeJSON(w, r, &profile) != nil {
		return
	}
	if profile.Name == "" {
		profile.Name = name
	}
	if profile.Name != name {
		writeError(w, http.StatusBadRequest, "profile name must match path")
		return
	}
	if err := s.Proxy.UpdateProfileV3(r.Context(), name, profile); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfileTry(w http.ResponseWriter, r *http.Request) {
	if !s.profileV3Available(r.Context()) {
		writeError(w, http.StatusNotFound, "Profiles v3 is not supported by connected MCPProxy")
		return
	}
	var req struct {
		Profile mcpproxy.ProfileV3 `json:"profile"`
		Query   string             `json:"query"`
		Limit   int                `json:"limit,omitempty"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	result, err := s.Proxy.TryProfileV3(r.Context(), req.Profile, req.Query, req.Limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.ID)
	}
	if err := s.Proxy.CreateProfile(r.Context(), name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": name})
}

func (s *Server) handleProfileServer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var req struct {
		Server string `json:"server"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	req.Server = strings.TrimSpace(req.Server)
	if id == "" || req.Server == "" {
		writeError(w, 400, "profile id and server are required")
		return
	}
	if err := s.Proxy.AssignProfileServer(r.Context(), id, req.Server); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfileServerDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	server := strings.TrimSpace(r.PathValue("server"))
	if id == "" || server == "" {
		writeError(w, 400, "profile and upstream are required")
		return
	}
	if err := s.Proxy.RemoveProfileServer(r.Context(), id, server); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfileDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("id"))
	if name == "" {
		writeError(w, 400, "profile is required")
		return
	}
	toks, err := s.Proxy.ListAgentTokens(r.Context())
	if err != nil {
		writeError(w, 502, "cannot verify Agent Token profile pins: "+err.Error())
		return
	}
	var pinnedBy []string
	now := time.Now()
	for _, tok := range toks {
		active := !tok.Revoked && (tok.ExpiresAt.IsZero() || tok.ExpiresAt.After(now))
		if active && tok.ProfilePin == name {
			pinnedBy = append(pinnedBy, tok.Name)
		}
	}
	if len(pinnedBy) > 0 {
		writeError(w, 409, "profile is pinned by Agent Tokens: "+strings.Join(pinnedBy, ", "))
		return
	}
	if err := s.Proxy.DeleteProfile(r.Context(), name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAgentOnboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name               string   `json:"name"`
		Profile            string   `json:"profile"`
		Permissions        []string `json:"permissions"`
		ExpiresIn          string   `json:"expires_in"`
		Target             string   `json:"target"`
		ConfirmDestructive bool     `json:"confirm_destructive"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Profile = strings.TrimSpace(req.Profile)
	req.Target = strings.ToLower(strings.TrimSpace(req.Target))
	if req.Target == "" {
		req.Target = "generic"
	}
	switch req.Target {
	case "generic", "n8n", "dify", "claude", "codex":
	default:
		writeError(w, http.StatusBadRequest, "unsupported agent target")
		return
	}
	if req.Profile == "" {
		writeError(w, http.StatusBadRequest, "profile is required")
		return
	}
	profiles, err := s.Proxy.ListProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	found := false
	for _, p := range profiles {
		if p.Name == req.Profile {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusBadRequest, "profile does not exist")
		return
	}

	// Optional deployment-defined Dify bindings are applied server-side so the
	// one-time MCPProxy token never leaves Sidekick. The configured permission
	// set is authoritative and prevents the browser from widening privileges.
	difyProviderID := ""
	if req.Target == "dify" {
		if binding, ok := s.Cfg.DifyBindings[req.Profile]; ok {
			difyProviderID = binding.ProviderID
			req.Permissions = append([]string(nil), binding.Permissions...)
		}
	}

	created, err := s.Tokens.Create(r.Context(), tokens.CreateRequest{
		Name: req.Name, AllowedServers: []string{"*"}, Permissions: req.Permissions,
		ExpiresIn: req.ExpiresIn, ProfilePin: req.Profile, ConfirmDestructive: req.ConfirmDestructive,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if difyProviderID != "" {
		adapter := difypinadapter.Adapter{
			Proxy:            s.Proxy,
			PendingDir:       filepath.Join(filepath.Dir(s.Cfg.DBPath), "credential-inbox"),
			PendingHostDir:   s.Cfg.CredentialPendingHostDir,
			SSHTransferRoot:  s.Cfg.CredentialSSHTransferRoot,
			SourceSSHProfile: s.Cfg.CredentialSourceSSHProfile,
			RemoteSSHProfile: s.Cfg.DifyRemoteSSHProfile,
			RemoteHelperPath: s.Cfg.DifyApplyHelperPath,
		}
		if err := adapter.Apply(r.Context(), created.Name, difyProviderID, req.Profile, created.Token); err != nil {
			_ = s.Tokens.Delete(context.Background(), created.Name)
			writeError(w, http.StatusBadGateway, "Dify credential apply failed; new token removed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"name": created.Name, "profile": req.Profile, "selected_target": req.Target,
			"expires_at": created.ExpiresAt, "applied": true, "provider_id": difyProviderID,
		})
		return
	}

	endpoint := s.agentProfileEndpoint(req.Profile)
	bearer := "Bearer " + created.Token
	snippets := map[string]string{
		"generic": "URL: " + endpoint + "\nAuthorization: " + bearer,
		"n8n":     "MCP URL: " + endpoint + "\nAuthorization header: " + bearer,
		"dify":    "MCP server URL: " + endpoint + "\nAuthorization: " + bearer,
		"claude":  "URL: " + endpoint + "\nAuthorization: " + bearer,
		"codex":   "URL: " + endpoint + "\nAuthorization: " + bearer,
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"name": created.Name, "token": created.Token, "profile": req.Profile,
		"endpoint": endpoint, "selected_target": req.Target, "snippet": snippets[req.Target],
		"expires_at": created.ExpiresAt,
	})
}

func (s *Server) agentProfileEndpoint(profile string) string {
	path := "/mcp/p/" + url.PathEscape(profile)
	raw := strings.TrimSpace(s.Cfg.PublicBaseURL)
	if raw == "" {
		return path
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return path
	}
	u.Path = path
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (s *Server) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name               string   `json:"name"`
		AllowedServers     []string `json:"allowed_servers"`
		Permissions        []string `json:"permissions"`
		ExpiresIn          string   `json:"expires_in"`
		ProfilePin         string   `json:"profile_pin"`
		ConfirmDestructive bool     `json:"confirm_destructive"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	created, err := s.Tokens.Create(r.Context(), tokens.CreateRequest{Name: req.Name, AllowedServers: req.AllowedServers, Permissions: req.Permissions, ExpiresIn: req.ExpiresIn, ProfilePin: req.ProfilePin, ConfirmDestructive: req.ConfirmDestructive})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleTokenRegenerate(w http.ResponseWriter, r *http.Request) {
	got, err := s.Tokens.Regenerate(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, got)
}
func (s *Server) handleTokenRevoke(w http.ResponseWriter, r *http.Request) {
	if err := s.Tokens.Revoke(r.Context(), r.PathValue("name")); err != nil {
		writeError(w, 502, err.Error())
		return
	}
	w.WriteHeader(204)
}
func (s *Server) handleTokenDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.Tokens.Delete(r.Context(), r.PathValue("name")); err != nil {
		writeError(w, 502, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) activeBrowserInstance() config.BrowserInstance {
	id := s.Cfg.BrowserInstance
	if s.Store != nil {
		if stored, ok, err := s.Store.Setting("browser_instance"); err == nil && ok {
			if _, valid := s.Cfg.BrowserInstanceByID(stored); valid {
				id = stored
			}
		}
	}
	if instance, ok := s.Cfg.BrowserInstanceByID(id); ok {
		return instance
	}
	if len(s.Cfg.BrowserInstances) > 0 {
		return s.Cfg.BrowserInstances[0]
	}
	return config.BrowserInstance{ID: "default", Label: "Default", CDPURL: s.Cfg.OAuthCDPURL, BrowserURL: s.Cfg.OAuthBrowserURL}
}

func (s *Server) activeBrowser() oauth.Browser {
	instance := s.activeBrowserInstance()
	return oauth.Browser{CDPBaseURL: instance.CDPURL, PublicSessionURL: instance.BrowserURL}
}

func (s *Server) ensureActiveBrowser(ctx context.Context) (config.BrowserInstance, error) {
	instance := s.activeBrowserInstance()
	if strings.TrimSpace(instance.LaunchURL) == "" {
		return instance, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, instance.LaunchURL, nil)
	if err != nil {
		return instance, fmt.Errorf("prepare browser launch: %w", err)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return instance, fmt.Errorf("launch browser instance %q: %w", instance.ID, err)
	}
	defer resp.Body.Close()
	if (resp.StatusCode >= 200 && resp.StatusCode < 300) || resp.StatusCode == http.StatusConflict {
		return instance, nil
	}
	return instance, fmt.Errorf("launch browser instance %q: HTTP %s", instance.ID, resp.Status)
}

func (s *Server) handleBrowserOpen(w http.ResponseWriter, r *http.Request) {
	instance, err := s.ensureActiveBrowser(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if strings.TrimSpace(instance.BrowserURL) == "" {
		writeError(w, http.StatusServiceUnavailable, "browser instance has no visible session URL")
		return
	}
	http.Redirect(w, r, instance.BrowserURL, http.StatusFound)
}

func (s *Server) handleBrowserInstanceUpdate(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "settings store unavailable")
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.ID = strings.TrimSpace(body.ID)
	if _, ok := s.Cfg.BrowserInstanceByID(body.ID); !ok {
		writeError(w, http.StatusBadRequest, "unknown browser instance")
		return
	}
	if err := s.Store.SetSetting("browser_instance", body.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "save browser instance")
		return
	}
	writeJSON(w, http.StatusOK, s.safeSettings())
}

func (s *Server) safeSettings() map[string]any {
	provider := strings.TrimSpace(s.Cfg.BrowserProvider)
	if provider == "" {
		provider = "chromium"
	}
	bitwarden := strings.TrimSpace(s.Cfg.BitwardenMode)
	if bitwarden == "" {
		bitwarden = "off"
	}
	active := s.activeBrowserInstance()
	instances := make([]map[string]string, 0, len(s.Cfg.BrowserInstances))
	for _, instance := range s.Cfg.BrowserInstances {
		instances = append(instances, map[string]string{"id": instance.ID, "label": instance.Label})
	}
	return map[string]any{
		"browser_provider":              provider,
		"browser_instance":              active.ID,
		"browser_instances":             instances,
		"browser_open_supported":        strings.TrimSpace(active.BrowserURL) != "",
		"bitwarden_mode":                bitwarden,
		"bitwarden_base_url_configured": strings.TrimSpace(s.Cfg.BitwardenBaseURL) != "",
		"browser_switch_host_side":      true,
	}
}

func (s *Server) handleDemoState(w http.ResponseWriter) {
	writeJSON(w, 200, map[string]any{
		"summary": map[string]int{"total": 8, "connected": 7, "tools": 284, "auth_required": 1, "quarantined": 0},
		"profiles": []map[string]any{
			{"name": "personal", "servers": []string{"google-workspace", "paperless", "immich", "microsoft"}, "tool_count": 269},
			{"name": "creator", "servers": []string{"google-workspace-work", "postiz"}, "tool_count": 96},
			{"name": "family", "servers": []string{"paperless-family", "immich-family"}, "tool_count": 120},
		},
		"upstreams": []safeUpstream{
			{Name: "google-workspace", Enabled: true, Status: "ready", ToolCount: 87, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected", Profiles: []string{"personal"}},
			{Name: "microsoft", Enabled: true, Status: "ready", ToolCount: 62, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected", Profiles: []string{"personal"}},
			{Name: "paperless", Enabled: true, Status: "ready", ToolCount: 119, CredentialConfigured: true, CredentialPreview: "tok_••••93fa", Profiles: []string{"personal"}},
			{Name: "immich", Enabled: true, Status: "ready", ToolCount: 1, CredentialConfigured: true, CredentialPreview: "imm_••••4d1c", Profiles: []string{"personal"}},
			{Name: "github", Enabled: true, Status: "ready", ToolCount: 94, CredentialConfigured: true, CredentialPreview: "ghp_••••7f2a"},
			{Name: "canva", Enabled: true, Status: "ready", ToolCount: 21, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected"},
			{Name: "notion", Enabled: true, Status: "ready", ToolCount: 22, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected"},
			{Name: "example-new-mcp", Enabled: false, Status: "authentication required", ToolCount: 0, CredentialConfigured: false},
		},
		"tokens": []map[string]any{
			{"name": "owner-agent", "token_prefix": "mcp_agt_demo", "allowed_servers": []string{"*"}, "permissions": []string{"read", "write"}, "profile_pin": ""},
			{"name": "family-agent", "token_prefix": "mcp_agt_fami", "allowed_servers": []string{"paperless-family", "immich-family"}, "permissions": []string{"read", "write"}, "profile_pin": "family"},
		},
		"settings": s.safeSettings(),
	})
}
