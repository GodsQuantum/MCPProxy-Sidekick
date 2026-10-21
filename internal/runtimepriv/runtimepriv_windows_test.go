//go:build windows

package runtimepriv

import "testing"

func TestWindowsDropFailsClosed(t *testing.T) {
	if err := Drop(nil); err != nil {
		t.Fatalf("nil target: %v", err)
	}
	if err := Drop(&Target{UID: 65532, GID: 65532}); err == nil {
		t.Fatal("Windows must reject Unix privilege-drop configuration")
	}
}
