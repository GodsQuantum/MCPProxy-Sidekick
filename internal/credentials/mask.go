package credentials

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Mask(secret string) string {
	s := strings.TrimSpace(secret)
	if len(s) <= 4 {
		return "••••"
	}
	if len(s) <= 8 {
		return s[:2] + "••••" + s[len(s)-2:]
	}
	return s[:4] + "••••" + s[len(s)-4:]
}

func Fingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
