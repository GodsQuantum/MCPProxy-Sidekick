//go:build windows

package runtimepriv

import "errors"

// Windows has no Unix UID/GID drop semantics. Fail closed if a Unix-only
// privilege-drop setting was requested instead of silently ignoring it.
func Drop(t *Target) error {
	if t == nil {
		return nil
	}
	return errors.New("SIDEKICK_DROP_UID/GID is unsupported on Windows; run Sidekick under the intended Windows account")
}
