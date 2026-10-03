package browser

import (
	"errors"
	"strings"
)

type Provider struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Image       string `json:"image"`
	CLIEnv      string `json:"cli_env"`
	ProfileDir  string `json:"profile_dir"`
}

var providers = map[string]Provider{
	"chromium": {
		Name: "chromium", DisplayName: "Chromium",
		Image:  "lscr.io/linuxserver/chromium:latest",
		CLIEnv: "CHROME_CLI", ProfileDir: "chromium-sidekick-oauth",
	},
	"brave": {
		Name: "brave", DisplayName: "Brave",
		Image:  "lscr.io/linuxserver/brave:latest",
		CLIEnv: "BRAVE_CLI", ProfileDir: "brave-sidekick-oauth",
	},
}

func Resolve(name string) (Provider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	p, ok := providers[name]
	if !ok {
		return Provider{}, errors.New("unsupported browser provider")
	}
	return p, nil
}
