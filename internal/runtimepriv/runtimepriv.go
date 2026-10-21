package runtimepriv

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Target struct{ UID, GID int }

func FromEnv() (*Target, error) {
	u := strings.TrimSpace(os.Getenv("SIDEKICK_DROP_UID"))
	g := strings.TrimSpace(os.Getenv("SIDEKICK_DROP_GID"))
	if u == "" && g == "" {
		return nil, nil
	}
	if u == "" || g == "" {
		return nil, fmt.Errorf("SIDEKICK_DROP_UID and SIDEKICK_DROP_GID must be set together")
	}
	uid, err := strconv.Atoi(u)
	if err != nil || uid < 1 {
		return nil, fmt.Errorf("invalid SIDEKICK_DROP_UID")
	}
	gid, err := strconv.Atoi(g)
	if err != nil || gid < 1 {
		return nil, fmt.Errorf("invalid SIDEKICK_DROP_GID")
	}
	return &Target{UID: uid, GID: gid}, nil
}
