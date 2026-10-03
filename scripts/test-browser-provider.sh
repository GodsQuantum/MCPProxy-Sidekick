#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROVIDER="${1:-}"
case "$PROVIDER" in
  chromium)
    IMAGE="${SIDEKICK_CHROMIUM_TEST_IMAGE:-lscr.io/linuxserver/chromium:latest}"
    PROFILE="chromium-sidekick-provider-test"
    POLICY="/etc/chromium/policies/managed/sidekick-bitwarden.json"
    VERSION_CMD="chromium --version"
    WANT_VERSION="Chromium"
    ;;
  brave)
    IMAGE="${SIDEKICK_BRAVE_TEST_IMAGE:-lscr.io/linuxserver/brave:latest}"
    PROFILE="brave-sidekick-provider-test"
    POLICY="/etc/brave/policies/managed/sidekick-bitwarden.json"
    VERSION_CMD="brave-browser --version"
    WANT_VERSION="Brave"
    ;;
  *)
    echo "usage: $0 chromium|brave" >&2
    exit 2
    ;;
esac

NAME="sidekick-provider-${PROVIDER}-$$"
CFG="$(mktemp -d)"
cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  rm -rf "$CFG"
}
trap cleanup EXIT

start_browser() {
  docker run -d --name "$NAME" \
    --shm-size=768m \
    -e PUID=1000 -e PGID=1000 -e TZ=UTC \
    -e SIDEKICK_BROWSER_PROVIDER="$PROVIDER" \
    -e SIDEKICK_BROWSER_PROFILE_DIR="$PROFILE" \
    -e SIDEKICK_BITWARDEN_MODE=managed-extension \
    -e SIDEKICK_BITWARDEN_BASE_URL=https://vault.example.com \
    -e CHROME_CLI="--user-data-dir=/config/$PROFILE --remote-debugging-port=9222 --remote-debugging-address=127.0.0.1 --no-first-run --no-default-browser-check about:blank" \
    -e BRAVE_CLI="--user-data-dir=/config/$PROFILE --remote-debugging-port=9222 --remote-debugging-address=127.0.0.1 --no-first-run --no-default-browser-check about:blank" \
    -v "$CFG:/config" \
    -v "$ROOT/deploy/oauth-browser/90-sidekick-chromium:/custom-cont-init.d/90-sidekick-chromium:ro" \
    -v "$ROOT/scripts/render-browser-policy.py:/opt/sidekick/render-browser-policy.py:ro" \
    -v "$ROOT/deploy/oauth-browser/policies/bitwarden.json.tmpl:/opt/sidekick/policies/bitwarden.json.tmpl:ro" \
    "$IMAGE" >/dev/null

  for _ in $(seq 1 90); do
    if docker exec "$NAME" curl -fsS http://127.0.0.1:3000/ >/dev/null 2>&1 &&
       docker exec "$NAME" curl -fsS http://127.0.0.1:9222/json/version >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  docker logs "$NAME" >&2 || true
  return 1
}

start_browser

PROVIDER_VERSION="$(docker exec "$NAME" sh -lc "$VERSION_CMD")"
case "$PROVIDER_VERSION" in
  *"$WANT_VERSION"*) ;;
  *) echo "unexpected provider version: $PROVIDER_VERSION" >&2; exit 1 ;;
esac
echo "provider: $PROVIDER_VERSION"

VERSION="$(docker exec "$NAME" curl -fsS http://127.0.0.1:9222/json/version)"
python3 - "$VERSION" <<'PY'
import json, sys
data = json.loads(sys.argv[1])
assert data.get("Browser"), data
assert data.get("webSocketDebuggerUrl"), data
assert data.get("Protocol-Version"), data
print("CDP:", data["Browser"], "protocol", data["Protocol-Version"])
PY

POLICY_JSON="$(docker exec "$NAME" cat "$POLICY")"
python3 - "$POLICY_JSON" <<'PY'
import json, sys
p=json.loads(sys.argv[1])
eid="nngceckbapebfimnlniiiahkandclblb"
entry=p["ExtensionSettings"][eid]
assert entry["installation_mode"]=="force_installed"
assert entry["update_url"]=="https://clients2.google.com/service/update2/crx"
assert p["3rdparty"]["extensions"][eid]["environment"]["base"]=="https://vault.example.com"
print("policy: OK")
PY

for _ in $(seq 1 45); do
  if docker exec "$NAME" test -d "/config/$PROFILE/Default/Extensions/nngceckbapebfimnlniiiahkandclblb"; then
    break
  fi
  sleep 1
done
if ! docker exec "$NAME" test -d "/config/$PROFILE/Default/Extensions/nngceckbapebfimnlniiiahkandclblb"; then
  echo "Bitwarden force-install policy was present but the extension was not installed" >&2
  docker exec "$NAME" sh -lc "find '/config/$PROFILE/Default/Extensions' -maxdepth 2 -type d 2>/dev/null | head -80" >&2 || true
  exit 1
fi
echo "Bitwarden extension: installed"

TARGET="$(docker exec "$NAME" curl -fsS -X PUT 'http://127.0.0.1:9222/json/new?https%3A%2F%2Fexample.com%2F')"
python3 - "$TARGET" <<'PY'
import json, sys
p=json.loads(sys.argv[1])
assert p.get("type")=="page", p
print("CDP open page: OK")
PY

for _ in $(seq 1 20); do
  [[ -f "$CFG/$PROFILE/Local State" ]] && break
  sleep .5
done
test -s "$CFG/$PROFILE/Local State"

docker rm -f "$NAME" >/dev/null
start_browser
test -s "$CFG/$PROFILE/Local State"
docker exec "$NAME" curl -fsS http://127.0.0.1:9222/json/version >/dev/null

echo "browser provider $PROVIDER: PASS"
