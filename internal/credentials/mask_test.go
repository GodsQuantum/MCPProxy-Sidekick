package credentials

import "testing"

func TestMask(t *testing.T) {
	if got := Mask("abcdefghijklmnop"); got != "abcd••••mnop" {
		t.Fatalf("Mask() = %q", got)
	}
	if got := Mask("short"); got != "sh••••rt" {
		t.Fatalf("short Mask() = %q", got)
	}
}

func TestFingerprintIsStableAndDoesNotContainSecret(t *testing.T) {
	const secret = "super-secret-value"
	a := Fingerprint(secret)
	b := Fingerprint(secret)
	if a != b || a == "" {
		t.Fatalf("fingerprints differ: %q %q", a, b)
	}
	if a == secret {
		t.Fatal("fingerprint exposed secret")
	}
}
