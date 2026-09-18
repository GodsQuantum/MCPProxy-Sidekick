package web

import (
	"embed"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/config"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/oauth"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/profiles"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/security"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/tokens"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	Cfg         config.Config
	Auth        *auth.Manager
	Proxy       *mcpproxy.Client
	Profiles    *profiles.Store
	Credentials credentials.Service
	Tokens      tokens.Service
	OAuth       oauth.Service
}

type safeUpstream struct {
	Name                 string `json:"name"`
	Enabled              bool   `json:"enabled"`
	Status               string `json:"status"`
	Protocol             string `json:"protocol,omitempty"`
	ToolCount            int    `json:"tool_count"`
	Authenticated        bool   `json:"authenticated"`
	Quarantined          bool   `json:"quarantined"`
	OAuth                bool   `json:"oauth"`
	CredentialConfigured bool   `json:"credential_configured"`
	CredentialPreview    string `json:"credential_preview,omitempty"`
	Profile              string `json:"profile,omitempty"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	staticFS, _ := fs.Sub(assets, "assets")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.requireSession(s.handleLogout))
	mux.HandleFunc("GET /auth/check", s.requireSession(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	mux.HandleFunc("GET /api/state", s.requireSession(s.handleState))
	mux.HandleFunc("POST /api/upstreams/{name}/credential", s.requireSession(s.requireMutation(s.handleCredential)))
	mux.HandleFunc("POST /api/upstreams/{name}/oauth/start", s.requireSession(s.requireMutation(s.handleOAuthStart)))
	mux.HandleFunc("POST /api/adapters/omniroute/restore-master", s.requireSession(s.requireMutation(s.handleOmniRouteRestoreMaster)))
	mux.HandleFunc("POST /api/profiles", s.requireSession(s.requireMutation(s.handleProfile)))
	mux.HandleFunc("POST /api/profiles/{id}/servers", s.requireSession(s.requireMutation(s.handleProfileServer)))
	mux.HandleFunc("POST /api/tokens", s.requireSession(s.requireMutation(s.handleTokenCreate)))
	mux.HandleFunc("POST /api/tokens/{name}/regenerate", s.requireSession(s.requireMutation(s.handleTokenRegenerate)))
	mux.HandleFunc("DELETE /api/tokens/{name}", s.requireSession(s.requireMutation(s.handleTokenRevoke)))
	mux.HandleFunc("DELETE /api/tokens/{name}/permanent", s.requireSession(s.requireMutation(s.handleTokenDelete)))
	return security.Headers(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	b, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "UI unavailable", 500)
		return
	}
	page := strings.ReplaceAll(string(b), "__SIDEKICK_MOUNT_PATH__", html.EscapeString(s.Cfg.MountPath))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.DemoMode {
		writeJSON(w, 200, map[string]any{"ok": true, "csrf": "demo"})
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if decodeJSON(w, r, &req) != nil {
		return
	}
	sess, err := s.Auth.Login(req.Key)
	if err != nil {
		writeError(w, 401, "invalid credentials")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "sidekick_session", Value: sess.ID, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: int(time.Until(sess.ExpiresAt).Seconds())})
	writeJSON(w, 200, map[string]any{"ok": true, "csrf": sess.CSRFToken})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("sidekick_session"); err == nil {
		s.Auth.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "sidekick_session", Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Cfg.DemoMode {
			next(w, r)
			return
		}
		c, err := r.Cookie("sidekick_session")
		if err != nil {
			writeError(w, 401, "authentication required")
			return
		}
		sess, ok := s.Auth.Validate(c.Value)
		if !ok {
			writeError(w, 401, "session expired")
			return
		}
		r.Header.Set("X-Sidekick-CSRF-Expected", sess.CSRFToken)
		next(w, r)
	}
}

func (s *Server) requireMutation(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Cfg.DemoMode {
			writeError(w, 403, "demo mode is read-only")
			return
		}
		c, _ := r.Cookie("sidekick_session")
		sess, ok := s.Auth.Validate(c.Value)
		if !ok {
			writeError(w, 401, "session expired")
			return
		}
		hosts := s.Cfg.AllowedHosts
		if len(hosts) == 0 {
			hosts = []string{strings.Split(r.Host, ":")[0]}
		}
		if err := security.RequireMutation(r, sess, hosts); err != nil {
			writeError(w, 403, err.Error())
			return
		}
		next(w, r)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, 400, "invalid JSON")
		return err
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func credentialView(server mcpproxy.Server, meta profiles.CredentialMeta, hasMeta bool) (bool, string) {
	if hasMeta {
		return true, meta.MaskedPreview
	}
	if server.Authenticated && len(server.OAuth) > 0 {
		return true, "OAuth connected"
	}
	for _, v := range server.Headers {
		t := strings.TrimSpace(v)
		bare := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "Bearer "), "bearer "))
		if strings.HasPrefix(bare, "${env:") || strings.HasPrefix(bare, "${keyring:") {
			return true, "Managed reference"
		}
		if bare != "" {
			return true, credentials.Mask(bare)
		}
	}
	return false, ""
}

func profileMap(ps []profiles.Profile) map[string]string {
	m := map[string]string{}
	for _, p := range ps {
		for _, name := range p.Servers {
			m[name] = p.ID
		}
	}
	return m
}
