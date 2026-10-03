package browser

import "testing"

func TestResolveKnownProviders(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		cliEnv     string
		profileDir string
	}{
		{"chromium", "lscr.io/linuxserver/chromium:latest", "CHROME_CLI", "chromium-sidekick-oauth"},
		{"brave", "lscr.io/linuxserver/brave:latest", "BRAVE_CLI", "brave-sidekick-oauth"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Resolve(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if p.Name != tc.name || p.Image != tc.image || p.CLIEnv != tc.cliEnv || p.ProfileDir != tc.profileDir {
				t.Fatalf("provider=%#v", p)
			}
		})
	}
}

func TestResolveRejectsUnknownProvider(t *testing.T) {
	if _, err := Resolve("firefox"); err == nil {
		t.Fatal("expected unknown provider to fail")
	}
}
