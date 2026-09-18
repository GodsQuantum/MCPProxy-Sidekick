package runtimepriv

import "testing"

func TestFromEnvDisabled(t *testing.T) {
	t.Setenv("SIDEKICK_DROP_UID", "")
	t.Setenv("SIDEKICK_DROP_GID", "")
	target, err := FromEnv()
	if err != nil || target != nil {
		t.Fatalf("target=%v err=%v", target, err)
	}
}

func TestFromEnvTarget(t *testing.T) {
	t.Setenv("SIDEKICK_DROP_UID", "65532")
	t.Setenv("SIDEKICK_DROP_GID", "65532")
	target, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if target.UID != 65532 || target.GID != 65532 {
		t.Fatalf("target=%+v", target)
	}
}

func TestFromEnvRejectsPartial(t *testing.T) {
	t.Setenv("SIDEKICK_DROP_UID", "65532")
	t.Setenv("SIDEKICK_DROP_GID", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error")
	}
}
