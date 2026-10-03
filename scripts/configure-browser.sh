#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROVIDER="${1:-}"
ENV_FILE="${SIDEKICK_BROWSER_ENV_FILE:-$ROOT/.env}"
COMPOSE_FILE="${SIDEKICK_BROWSER_COMPOSE_FILE:-$ROOT/compose.yaml}"
ATTEMPTS="${SIDEKICK_BROWSER_HEALTH_ATTEMPTS:-30}"

case "$PROVIDER" in
  chromium)
    IMAGE="lscr.io/linuxserver/chromium:latest"
    PROFILE_DIR="chromium-sidekick-oauth"
    ;;
  brave)
    IMAGE="lscr.io/linuxserver/brave:latest"
    PROFILE_DIR="brave-sidekick-oauth"
    ;;
  *)
    echo "usage: $0 chromium|brave" >&2
    exit 2
    ;;
esac

mkdir -p "$(dirname "$ENV_FILE")"
ORIGINAL_EXISTS=0
if [[ -f "$ENV_FILE" ]]; then
  ORIGINAL_EXISTS=1
fi
BACKUP="$(mktemp)"
trap 'rm -f "$BACKUP"' EXIT
if [[ "$ORIGINAL_EXISTS" == "1" ]]; then
  cp "$ENV_FILE" "$BACKUP"
else
  : >"$ENV_FILE"
fi

set_env() {
  local key="$1" value="$2" tmp
  tmp="$(mktemp)"
  awk -v key="$key" -v value="$value" '
    BEGIN { found=0 }
    index($0, key "=") == 1 { print key "=" value; found=1; next }
    { print }
    END { if (!found) print key "=" value }
  ' "$ENV_FILE" >"$tmp"
  mv "$tmp" "$ENV_FILE"
}

restore_env() {
  if [[ "$ORIGINAL_EXISTS" == "1" ]]; then
    cp "$BACKUP" "$ENV_FILE"
  else
    rm -f "$ENV_FILE"
  fi
}

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

browser_healthy() {
  local cid status
  cid="$(compose ps -q oauth-browser)"
  [[ -n "$cid" ]] || return 1
  for _ in $(seq 1 "$ATTEMPTS"); do
    status="$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null || true)"
    if [[ "$status" == "healthy" ]] &&
       docker exec "$cid" curl -fsS http://127.0.0.1:3000/ >/dev/null 2>&1 &&
       docker exec "$cid" curl -fsS http://127.0.0.1:9222/json/version >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

set_env SIDEKICK_BROWSER_PROVIDER "$PROVIDER"
set_env SIDEKICK_BROWSER_IMAGE "$IMAGE"
set_env SIDEKICK_BROWSER_PROFILE_DIR "$PROFILE_DIR"

if ! compose pull oauth-browser ||
   ! compose up -d --no-deps --force-recreate oauth-browser ||
   ! browser_healthy; then
  echo "browser switch to $PROVIDER failed; restoring previous provider" >&2
  restore_env
  if [[ -f "$ENV_FILE" ]]; then
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-deps --force-recreate oauth-browser >/dev/null 2>&1 || true
  else
    # No env file existed before the switch. Recreate with Compose defaults
    # so the failed target provider is not left running on a fresh install.
    docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate oauth-browser >/dev/null 2>&1 || true
  fi
  exit 1
fi

echo "Human Auth Browser: $PROVIDER"
