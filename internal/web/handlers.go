package web

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	immichadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/immich"
	omnirouteadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/omniroute"
	paperlessadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/paperless"
	postizadapter "github.com/GodsQuantum/mcpproxy-sidekick/internal/adapters/postiz"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/profiles"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/tokens"
)

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.DemoMode {
		s.handleDemoState(w)
		return
	}
	servers, err := s.Proxy.ListServers(r.Context())
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	ps, err := s.Profiles.ListProfiles()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	toks, err := s.Tokens.List(r.Context())
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	pm := profileMap(ps)
	safe := make([]safeUpstream, 0, len(servers))
	connected, tools, authNeeded, quarantined := 0, 0, 0, 0
	for _, srv := range servers {
		meta, ok, merr := s.Profiles.CredentialMeta(srv.Name)
		if merr != nil {
			writeError(w, 500, merr.Error())
			return
		}
		configured, preview := credentialView(srv, meta, ok)
		st := srv.Status
		if st == "" {
			st = srv.Health.Summary
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
		safe = append(safe, safeUpstream{Name: srv.Name, Enabled: srv.Enabled, Status: st, Protocol: srv.Protocol, ToolCount: srv.ToolCount, Authenticated: srv.Authenticated, Quarantined: srv.Quarantined, OAuth: len(srv.OAuth) > 0, CredentialConfigured: configured, CredentialPreview: preview, Profile: pm[srv.Name]})
	}
	writeJSON(w, 200, map[string]any{
		"summary":   map[string]int{"total": len(safe), "connected": connected, "tools": tools, "auth_required": authNeeded, "quarantined": quarantined},
		"upstreams": safe, "profiles": ps, "tokens": toks, "csrf": r.Header.Get("X-Sidekick-CSRF-Expected"),
		"capabilities": map[string]bool{"omniroute_restore_master": strings.TrimSpace(s.Cfg.OmniRouteDB) != ""},
	})
}

func (s *Server) handleCredential(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeError(w, 400, "missing server name")
		return
	}
	var req struct {
		Mode       string `json:"mode"`
		HeaderName string `json:"header_name"`
		Value      string `json:"value"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	var err error
	switch {
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
	_ = s.Profiles.UpsertCredentialMeta(profiles.CredentialMeta{ServerName: name, MaskedPreview: credentials.Mask(req.Value), Fingerprint: credentials.Fingerprint(req.Value), UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
	writeJSON(w, 200, map[string]any{"ok": true, "preview": credentials.Mask(req.Value)})
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
	result, err := s.OAuth.Start(r.Context(), name)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	var p profiles.Profile
	if decodeJSON(w, r, &p) != nil {
		return
	}
	p.ID = strings.TrimSpace(p.ID)
	p.Label = strings.TrimSpace(p.Label)
	if p.ID == "" || p.Label == "" {
		writeError(w, 400, "profile id and label are required")
		return
	}
	if err := s.Profiles.UpsertProfile(p); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, p)
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
	if err := s.Profiles.AssignServer(id, req.Server); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (s *Server) handleDemoState(w http.ResponseWriter) {
	writeJSON(w, 200, map[string]any{
		"summary": map[string]int{"total": 8, "connected": 7, "tools": 284, "auth_required": 1, "quarantined": 0},
		"profiles": []map[string]any{
			{"id": "personal", "label": "Personal", "servers": []string{"google-workspace", "paperless", "immich", "microsoft"}},
			{"id": "creator", "label": "Creator", "servers": []string{"google-workspace-work", "postiz"}},
			{"id": "family", "label": "Family member", "servers": []string{"paperless-family", "immich-family"}},
		},
		"upstreams": []safeUpstream{
			{Name: "google-workspace", Enabled: true, Status: "ready", ToolCount: 87, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected", Profile: "personal"},
			{Name: "microsoft", Enabled: true, Status: "ready", ToolCount: 62, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected", Profile: "personal"},
			{Name: "paperless", Enabled: true, Status: "ready", ToolCount: 119, CredentialConfigured: true, CredentialPreview: "tok_••••93fa", Profile: "personal"},
			{Name: "immich", Enabled: true, Status: "ready", ToolCount: 1, CredentialConfigured: true, CredentialPreview: "imm_••••4d1c", Profile: "personal"},
			{Name: "github", Enabled: true, Status: "ready", ToolCount: 94, CredentialConfigured: true, CredentialPreview: "ghp_••••7f2a"},
			{Name: "canva", Enabled: true, Status: "ready", ToolCount: 21, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected"},
			{Name: "notion", Enabled: true, Status: "ready", ToolCount: 22, Authenticated: true, OAuth: true, CredentialConfigured: true, CredentialPreview: "OAuth connected"},
			{Name: "example-new-mcp", Enabled: false, Status: "authentication required", ToolCount: 0, CredentialConfigured: false},
		},
		"tokens": []map[string]any{
			{"name": "owner-agent", "token_prefix": "mcp_agt_demo", "allowed_servers": []string{"*"}, "permissions": []string{"read", "write"}, "profile_pin": ""},
			{"name": "family-agent", "token_prefix": "mcp_agt_fami", "allowed_servers": []string{"paperless-family", "immich-family"}, "permissions": []string{"read", "write"}, "profile_pin": "family"},
		},
	})
}
