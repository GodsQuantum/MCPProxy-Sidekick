package security

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/auth"
)

func RequireMutation(r *http.Request, session auth.Session, allowedHosts []string) error {
	csrf := r.Header.Get("X-CSRF-Token")
	if len(csrf) != len(session.CSRFToken) || subtle.ConstantTimeCompare([]byte(csrf), []byte(session.CSRFToken)) != 1 {
		return errors.New("invalid CSRF token")
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !hostAllowed(u.Host, allowedHosts) {
			return errors.New("untrusted origin")
		}
	}
	if len(allowedHosts) > 0 && !hostAllowed(r.Host, allowedHosts) {
		return errors.New("untrusted host")
	}
	return nil
}

func hostAllowed(hostport string, allowed []string) bool {
	host := strings.ToLower(strings.TrimSpace(hostport))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else if strings.Count(host, ":") == 1 {
		if h, p, ok := strings.Cut(host, ":"); ok && p != "" {
			host = h
		}
	}
	for _, item := range allowed {
		if strings.EqualFold(strings.TrimSpace(item), host) {
			return true
		}
	}
	return false
}
