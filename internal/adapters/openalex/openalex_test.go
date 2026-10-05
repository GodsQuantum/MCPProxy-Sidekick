package openalex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyWritesTrimmedSecret0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "openalex_api_key")
	if err := (Adapter{KeyFile: path}).Apply("  test-key  "); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "test-key\n" {
		t.Fatalf("content=%q", string(b))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}
