#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
cd "$ROOT"

fail=0
check() {
  local label="$1" pattern="$2"
  local out
  out="$(grep -RInE --binary-files=without-match --exclude-dir=.git --exclude=.git --exclude=dashboard.png --exclude=privacy-scan.sh -- "$pattern" . 2>/dev/null || true)"
  if [[ -n "$out" ]]; then
    printf "\n[privacy] %s\n%s\n" "$label" "$out" >&2
    fail=1
  fi
}

# Deployment-specific identifiers that must never appear in the public project.
check "private names/hosts" "(arezki|fella|cloud9|celestra|pegasus|galactica)"
check "private domain" "(arezkichougar\.com|mespodcasts\.com)"
check "private absolute paths" "(/home/[^ <\"\x27]+|/srv/lxc/[^ <\"\x27]+)"
check "RFC1918 IPv4" "(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})"

# Email addresses are allowed only under reserved example domains.
emails="$(grep -RInE --binary-files=without-match --exclude-dir=.git --exclude=.git --exclude=dashboard.png --exclude=privacy-scan.sh -- "[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}" . 2>/dev/null || true)"
if [[ -n "$emails" ]]; then
  bad="$(printf "%s\n" "$emails" | grep -Ev "@(example\.com|example\.org|example\.net)([^A-Za-z0-9.-]|$)" || true)"
  if [[ -n "$bad" ]]; then
    printf "\n[privacy] non-example email\n%s\n" "$bad" >&2
    fail=1
  fi
fi

# Common secret/token shapes. Gitleaks remains the deeper second line in CI.
check "credential-like values" "(ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9_-]{20,}|mcp_agt_[A-Za-z0-9_-]{16,}|eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,})"

if (( fail )); then
  echo "[privacy] FAILED" >&2
  exit 1
fi
echo "[privacy] OK"
