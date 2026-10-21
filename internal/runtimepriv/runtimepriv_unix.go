//go:build !windows

package runtimepriv

import (
	"fmt"
	"os"
	"syscall"
)

// Drop switches to the requested non-root identity on Unix hosts.
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
