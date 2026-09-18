package runtimepriv

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
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

func Drop(t *Target) error {
	if t == nil {
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("privilege drop requested but process is not root")
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("clear supplementary groups: %w", err)
	}
	if err := syscall.Setgid(t.GID); err != nil {
		return fmt.Errorf("setgid: %w", err)
	}
	if err := syscall.Setuid(t.UID); err != nil {
		return fmt.Errorf("setuid: %w", err)
	}
	if os.Geteuid() != t.UID || os.Getegid() != t.GID {
		return fmt.Errorf("privilege drop verification failed")
	}
	return nil
}
