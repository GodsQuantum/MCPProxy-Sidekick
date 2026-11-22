//go:build windows

package runtimepriv

import "errors"

// Windows has no POSIX UID/GID privilege-drop API. Fail closed if requested.
func Drop(t *Target) error {
	if t == nil {
		return nil
	}
	return errors.New("SIDEKICK_DROP_UID/GID is not supported on Windows")
}
