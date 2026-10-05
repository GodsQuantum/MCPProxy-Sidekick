package openalex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Adapter struct {
	KeyFile string
}

func (a Adapter) Apply(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("empty OpenAlex API key")
	}
	keyFile := strings.TrimSpace(a.KeyFile)
	if keyFile == "" {
		return errors.New("OpenAlex API key file is not configured")
	}
	dir := filepath.Dir(keyFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".openalex-key-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(value + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, keyFile)
}
