#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/scripts/configure-browser.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ENVFILE="$TMP/.env"
cat >"$ENVFILE" <<'EOF'
SIDEKICK_BROWSER_PROVIDER=chromium
SIDEKICK_BROWSER_IMAGE=lscr.io/linuxserver/chromium:latest
SIDEKICK_BROWSER_PROFILE_DIR=chromium-sidekick-oauth
UNRELATED_SETTING=keep-me
EOF
cp "$ENVFILE" "$TMP/original.env"

mkdir -p "$TMP/bin"
cat >"$TMP/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_DOCKER_LOG:?}"
if [[ "$*" == *"compose"*"ps -q oauth-browser"* ]]; then
  printf 'fake-browser\n'
  exit 0
fi
if [[ "$*" == *"inspect"*"State.Health.Status"* ]]; then
  printf 'healthy\n'
  exit 0
fi
if [[ "$*" == *"exec fake-browser curl"* ]] && [[ "${FAKE_HEALTH_FAIL:-0}" == "1" ]]; then
  exit 22
fi
exit 0
EOF
chmod +x "$TMP/bin/docker"

export PATH="$TMP/bin:$PATH"
export FAKE_DOCKER_LOG="$TMP/docker.log"
export SIDEKICK_BROWSER_ENV_FILE="$ENVFILE"
export SIDEKICK_BROWSER_COMPOSE_FILE="$ROOT/compose.yaml"
export SIDEKICK_BROWSER_HEALTH_ATTEMPTS=1

if "$SCRIPT" firefox >/dev/null 2>&1; then
  echo "unknown provider unexpectedly succeeded" >&2
  exit 1
fi
cmp -s "$ENVFILE" "$TMP/original.env" || {
  echo "invalid provider changed env" >&2
  exit 1
}

export FAKE_HEALTH_FAIL=1
if "$SCRIPT" brave >/dev/null 2>&1; then
  echo "unhealthy provider unexpectedly succeeded" >&2
  exit 1
fi
cmp -s "$ENVFILE" "$TMP/original.env" || {
  echo "failed switch did not restore env" >&2
  exit 1
}
ups="$(grep -c 'compose.*up.*oauth-browser' "$FAKE_DOCKER_LOG" || true)"
if [[ "$ups" -lt 2 ]]; then
  echo "expected switch and rollback compose up calls, got $ups" >&2
  cat "$FAKE_DOCKER_LOG" >&2 || true
  exit 1
fi

# Fresh-install rollback: when no .env existed before the attempted switch,
# the failed target must be replaced by Compose defaults rather than left running.
rm -f "$ENVFILE"
: >"$FAKE_DOCKER_LOG"
export FAKE_HEALTH_FAIL=1
if "$SCRIPT" brave >/dev/null 2>&1; then
  echo "fresh-install unhealthy provider unexpectedly succeeded" >&2
  exit 1
fi
if [[ -e "$ENVFILE" ]]; then
  echo "fresh-install rollback should restore missing env file" >&2
  exit 1
fi
ups="$(grep -c 'compose.*up.*oauth-browser' "$FAKE_DOCKER_LOG" || true)"
if [[ "$ups" -lt 2 ]]; then
  echo "fresh-install rollback did not recreate default browser; got $ups compose up calls" >&2
  cat "$FAKE_DOCKER_LOG" >&2 || true
  exit 1
fi

echo "configure-browser rollback tests: PASS"
