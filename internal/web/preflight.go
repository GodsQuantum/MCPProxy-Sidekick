package web

import (
	"errors"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	"net/http"
	"strings"
	"unicode"
)

// Only an authenticated Sidekick operator may inspect the native readiness
// evaluator. This endpoint never calls upstream tools or grants new access.
func (s *Server) handleToolPreflight(w http.ResponseWriter, r *http.Request) {
	var request mcpproxy.PreflightRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	if len(request.Tools) < 1 || len(request.Tools) > 16 {
		writeError(w, http.StatusBadRequest, "provide 1 to 16 tool IDs")
		return
	}
	request.Profile = strings.TrimSpace(request.Profile)
	if len(request.Profile) > 128 {
		writeError(w, http.StatusBadRequest, "invalid profile")
		return
	}
	for _, ch := range request.Profile {
		if unicode.IsControl(ch) || unicode.IsSpace(ch) {
			writeError(w, http.StatusBadRequest, "invalid profile")
			return
		}
	}
	seen := make(map[string]struct{}, len(request.Tools))
	for i, t := range request.Tools {
		id := strings.TrimSpace(t.ID)
		lhs, rhs, ok := strings.Cut(id, ":")
		if !ok || lhs == "" || rhs == "" || len(id) > 256 || len(t.PinHash) > 128 {
			writeError(w, http.StatusBadRequest, "invalid tool ID or pin")
			return
		}
		for _, ch := range id {
			if unicode.IsControl(ch) || unicode.IsSpace(ch) {
				writeError(w, http.StatusBadRequest, "invalid tool ID")
				return
			}
		}
		if _, found := seen[id]; found {
			writeError(w, http.StatusBadRequest, "duplicate tool ID")
			return
		}
		seen[id] = struct{}{}
		request.Tools[i].ID = id
	}
	result, err := s.Proxy.Preflight(r.Context(), request)
	if err != nil {
		if errors.Is(err, mcpproxy.ErrPreflightUnavailable) {
			writeError(w, http.StatusNotImplemented, "this MCPProxy version does not support tool preflight")
			return
		}
		writeError(w, http.StatusBadGateway, "MCPProxy tool preflight is temporarily unavailable")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
