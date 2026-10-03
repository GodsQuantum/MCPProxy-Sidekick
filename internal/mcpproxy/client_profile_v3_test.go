package mcpproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestProfileV3ReadUpdateAndTry(t *testing.T) {
	var updated ProfileV3
	var tried map[string]any
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/profiles/research":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"name": "research", "title": "Research", "servers": []string{"github"},
					"max_tier": "read", "unannotated": "deny", "is_legacy": false,
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/profiles/research":
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"profile": updated}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/profiles/try":
			if err := json.NewDecoder(r.Body).Decode(&tried); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"hidden_by_profile": 2}})
		default:
			http.NotFound(w, r)
		}
	})

	got, err := client.GetProfileV3(context.Background(), "research")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "research" || got.MaxTier != "read" || got.Title != "Research" {
		t.Fatalf("profile=%#v", got)
	}

	off := false
	want := ProfileV3{
		Name: "research", Title: "Research", Servers: []string{"github"},
		MaxTier: "write", Unannotated: "deny", CodeExecution: &off,
	}
	if err := client.UpdateProfileV3(context.Background(), "research", want); err != nil {
		t.Fatal(err)
	}
	if updated.MaxTier != "write" || updated.CodeExecution == nil || *updated.CodeExecution {
		t.Fatalf("updated=%#v", updated)
	}

	try, err := client.TryProfileV3(context.Background(), want, "issue", 10)
	if err != nil {
		t.Fatal(err)
	}
	if try.HiddenByProfile != 2 {
		t.Fatalf("try=%#v", try)
	}
	profileBody := tried["profile"].(map[string]any)
	if !reflect.DeepEqual(profileBody["servers"], []any{"github"}) {
		t.Fatalf("try profile=%#v", tried)
	}
}

func TestProfileV3ResponseCanBeNestedUnderProfile(t *testing.T) {
	client := infoClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"profile": map[string]any{"name": "p", "servers": []string{}}},
		})
	})
	got, err := client.GetProfileV3(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "p" {
		t.Fatalf("profile=%#v", got)
	}
}
