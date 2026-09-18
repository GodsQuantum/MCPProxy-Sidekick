package immich

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
)

type Adapter struct {
	Editor     credentials.ServerEditor
	KeyFile    string
	ServerName string
}

func (a Adapter) Apply(ctx context.Context, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("empty Immich API key")
	}
	f, err := os.OpenFile(a.KeyFile, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.WriteString(value + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return a.Editor.EnableServer(ctx, a.ServerName)
}
