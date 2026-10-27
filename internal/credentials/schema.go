package credentials

import (
	"sort"
	"strings"
)

type FormField struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Secret   bool   `json:"secret"`
	Required bool   `json:"required"`
}
type FormSchema struct {
	Kind          string      `json:"kind"`
	DefaultMode   string      `json:"default_mode,omitempty"`
	DefaultHeader string      `json:"default_header,omitempty"`
	Hint          string      `json:"hint,omitempty"`
	Fields        []FormField `json:"fields,omitempty"`
}

// Describe emits only field names/types; stored credential values must never leave the server.
func Describe(name string, config map[string]any) FormSchema {
	if name == "cryptocom-app" || name == "cryptocom-exchange" || name == "openalex-github" || name == "postiz" || name == "paperless" || strings.HasPrefix(name, "paperless-") || name == "immich" || strings.HasPrefix(name, "immich-") {
		return FormSchema{Kind: "adapter"}
	}
	entry := serverEntry(name, config)
	if len(entry) == 0 {
		return FormSchema{Kind: "unavailable", Hint: "Upstream not found"}
	}
	if auth, ok := entry["oauth"].(map[string]any); ok && len(auth) > 0 {
		return FormSchema{Kind: "oauth", Hint: "Use native MCPProxy OAuth sign-in"}
	}
	if name == "ebay-api" {
		return FormSchema{Kind: "env", Hint: "eBay Developer credentials (this MCP supports read-only production APIs).", Fields: []FormField{{Name: "EBAY_CLIENT_ID", Label: "Client ID", Required: true}, {Name: "EBAY_CLIENT_SECRET", Label: "Client Secret", Secret: true, Required: true}}}
	}
	protocol, _ := entry["protocol"].(string)
	if protocol == "stdio" {
		keys := map[string]bool{}
		if raw, ok := entry["env"].(map[string]any); ok {
			for k := range raw {
				if isSecretEnv(k) {
					keys[k] = true
				}
			}
		}
		if args, ok := entry["args"].([]any); ok {
			for i, a := range args {
				if arg, ok := a.(string); ok && (arg == "-e" || arg == "--env") && i+1 < len(args) {
					if next, ok := args[i+1].(string); ok {
						key := strings.SplitN(next, "=", 2)[0]
						if isSecretEnv(key) {
							keys[key] = true
						}
					}
				}
			}
		}
		if len(keys) == 0 {
			return FormSchema{Kind: "none", Hint: "No credential fields configured for this local MCP"}
		}
		names := make([]string, 0, len(keys))
		for n := range keys {
			names = append(names, n)
		}
		sort.Strings(names)
		fields := make([]FormField, 0, len(names))
		for _, n := range names {
			fields = append(fields, FormField{Name: n, Label: strings.ReplaceAll(n, "_", " "), Secret: !strings.HasSuffix(n, "_CLIENT_ID"), Required: true})
		}
		return FormSchema{Kind: "env", Fields: fields, Hint: "Credentials injected into the MCP subprocess"}
	}
	mode := "bearer"
	header := ""
	if hs, ok := entry["headers"].(map[string]any); ok {
		for k := range hs {
			if strings.EqualFold(k, "Authorization") {
				mode = "bearer"
				header = ""
				break
			}
			if strings.EqualFold(k, "X-API-Key") {
				mode = "x-api-key"
				header = ""
				continue
			}
			if mode == "bearer" && header == "" {
				mode = "custom-header"
				header = k
			}
		}
	}
	return FormSchema{Kind: "header", DefaultMode: mode, DefaultHeader: header, Hint: "HTTP credential for this upstream"}
}
func isSecretEnv(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	for _, suffix := range []string{"_API_KEY", "_CLIENT_ID", "_CLIENT_SECRET", "_SECRET", "_TOKEN", "_PASSWORD", "_REFRESH_TOKEN", "_ACCESS_KEY"} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}
func serverEntry(name string, config map[string]any) map[string]any {
	servers, ok := config["mcpServers"].([]any)
	if !ok {
		return nil
	}
	for _, raw := range servers {
		if s, ok := raw.(map[string]any); ok && s["name"] == name {
			return s
		}
	}
	return nil
}
