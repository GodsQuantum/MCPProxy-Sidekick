#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GUARD="$ROOT/scripts/mcpproxy-netns-guard.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$TMP/bin" "$TMP/state"

cat >"$TMP/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="${FAKE_NETNS_STATE:?}"
printf '%s\n' "$*" >>"${FAKE_DOCKER_LOG:?}"

if [[ "$1" == "inspect" && "${2:-}" == "-f" ]]; then
  name="${4:-}"
  case "$name" in
    target) printf '101\n'; exit 0 ;;
    sidecar)
      if [[ -f "$state/stopped" && ! -f "$state/rebound" ]]; then
        printf '0\n'
      else
        printf '202\n'
      fi
      exit 0
      ;;
  esac
fi

if [[ "$1" == "inspect" ]]; then
  case "${2:-}" in
    target|sidecar) exit 0 ;;
  esac
fi

if [[ "$1" == "restart" && "${2:-}" == "sidecar" ]]; then
  touch "$state/rebound"
  rm -f "$state/stopped"
  printf 'sidecar\n'
  exit 0
fi

echo "unexpected fake docker call: $*" >&2
exit 2
EOF
chmod +x "$TMP/bin/docker"

cat >"$TMP/bin/readlink" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="${FAKE_NETNS_STATE:?}"
case "$1" in
  /proc/101/ns/net)
    printf 'net:[target]\n'
    ;;
  /proc/202/ns/net)
    if [[ -f "$state/rebound" || -f "$state/already" ]]; then
      printf 'net:[target]\n'
    else
      printf 'net:[old]\n'
    fi
    ;;
  *)
    echo "unexpected fake readlink path: $1" >&2
    exit 2
    ;;
esac
EOF
chmod +x "$TMP/bin/readlink"

export PATH="$TMP/bin:$PATH"
export FAKE_NETNS_STATE="$TMP/state"
export FAKE_DOCKER_LOG="$TMP/docker.log"
export MCPPROXY_CONTAINER_NAME=target
export MCPPROXY_SIDECARS=sidecar
export MCPPROXY_NETNS_SETTLE_SECONDS=0

run_case() {
  local mode="$1" expected_restarts="$2"
  rm -f "$TMP/state/"* "$TMP/docker.log"
  : >"$TMP/docker.log"
  case "$mode" in
    already) touch "$TMP/state/already" ;;
    stale) ;;
    stopped) touch "$TMP/state/stopped" ;;
    *) echo "unknown case $mode" >&2; exit 2 ;;
  esac

  bash "$GUARD" --once

  local restarts
  restarts="$(grep -c '^restart sidecar$' "$TMP/docker.log" || true)"
  if [[ "$restarts" != "$expected_restarts" ]]; then
    echo "$mode: expected $expected_restarts restart(s), got $restarts" >&2
    cat "$TMP/docker.log" >&2
    exit 1
  fi
}

run_case already 0
run_case stale 1
run_case stopped 1

printf 'netns guard unit tests: PASS\n'
