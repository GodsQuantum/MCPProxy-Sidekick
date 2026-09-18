package omniroute

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	_ "modernc.org/sqlite"
)

type fakeEditor struct {
	patch   mcpproxy.ServerPatch
	enabled string
}

func (f *fakeEditor) PatchServer(_ context.Context, _ string, patch mcpproxy.ServerPatch) error {
	f.patch = patch
	return nil
}
func (f *fakeEditor) EnableServer(_ context.Context, name string) error { f.enabled = name; return nil }

func TestRestoreMasterReadsActiveKeyAndPatchesMCPProxy(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "omniroute.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("create table api_keys (name text,key text,is_active integer,revoked_at text); insert into api_keys values (?,?,1,null)", "Omniroute Master", "master-secret")
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	f := &fakeEditor{}
	a := Adapter{Editor: f, DBPath: dbPath, ServerName: "omniroute"}
	if err := a.RestoreMaster(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.patch.Headers["Authorization"]; got != "Bearer master-secret" {
		t.Fatalf("Authorization=%q", got)
	}
	if f.enabled != "omniroute" {
		t.Fatalf("enabled=%q", f.enabled)
	}
}
