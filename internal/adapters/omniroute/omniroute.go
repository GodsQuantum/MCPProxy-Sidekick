package omniroute

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/GodsQuantum/mcpproxy-sidekick/internal/credentials"
	"github.com/GodsQuantum/mcpproxy-sidekick/internal/mcpproxy"
	_ "modernc.org/sqlite"
)

type Adapter struct {
	Editor     credentials.ServerEditor
	DBPath     string
	ServerName string
}

func (a Adapter) RestoreMaster(ctx context.Context) error {
	if strings.TrimSpace(a.DBPath) == "" {
		return errors.New("OmniRoute database is not configured")
	}
	server := strings.TrimSpace(a.ServerName)
	if server == "" {
		server = "omniroute"
	}
	db, err := sql.Open("sqlite", "file:"+a.DBPath+"?mode=ro&immutable=1")
	if err != nil {
		return err
	}
	defer db.Close()
	var key string
	err = db.QueryRow("select key from api_keys where name=? and is_active=1 and revoked_at is null limit 1", "Omniroute Master").Scan(&key)
	if err == sql.ErrNoRows || strings.TrimSpace(key) == "" {
		return errors.New("active OmniRoute Master key not found")
	}
	if err != nil {
		return err
	}
	if err = a.Editor.PatchServer(ctx, server, mcpproxy.ServerPatch{Headers: map[string]string{"Authorization": "Bearer " + strings.TrimSpace(key)}}); err != nil {
		return err
	}
	return a.Editor.EnableServer(ctx, server)
}
