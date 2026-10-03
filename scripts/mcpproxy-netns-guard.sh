#!/usr/bin/env bash
set -euo pipefail

target="${MCPPROXY_CONTAINER_NAME:-mcpproxy}"
sidecars_raw="${MCPPROXY_SIDECARS:-mcpproxy-sidekick-oauth-browser mcpproxy-sidekick}"
settle_seconds="${MCPPROXY_NETNS_SETTLE_SECONDS:-2}"

log() { printf '%s %s\n' "$(date -Is)" "$*" >&2; }

container_pid() { docker inspect -f '{{.State.Pid}}' "$1" 2>/dev/null; }

netns_inode() {
  local pid
  pid="$(container_pid "$1")" || return 1
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  (( pid > 0 )) || return 1
  readlink "/proc/$pid/ns/net"
}

rebind_once() {
  local target_ns sidecar sidecar_ns after_ns
  target_ns="$(netns_inode "$target")" || {
    log "target $target is not running; nothing to rebind"
    return 0
  }

  for sidecar in $sidecars_raw; do
    docker inspect "$sidecar" >/dev/null 2>&1 || continue
    sidecar_ns="$(netns_inode "$sidecar" 2>/dev/null || true)"
    if [[ -n "$sidecar_ns" && "$sidecar_ns" == "$target_ns" ]]; then
      continue
    fi

    log "rebind $sidecar to current network namespace of $target"
    docker restart "$sidecar" >/dev/null
    after_ns="$(netns_inode "$sidecar" 2>/dev/null || true)"
    if [[ -z "$after_ns" || "$after_ns" != "$target_ns" ]]; then
      log "failed to rebind $sidecar: target=$target_ns sidecar=${after_ns:-unavailable}"
      return 1
    fi
  done
}

case "${1:-watch}" in
  --once|once)
    rebind_once
    ;;
  watch|"")
    rebind_once
    docker events --filter type=container --filter event=start --format '{{.Actor.Attributes.name}}' |
      while IFS= read -r name; do
        [[ "$name" == "$target" ]] || continue
        sleep "$settle_seconds"
        rebind_once || log "rebind after $target start failed"
      done
    ;;
  *)
    echo "usage: $0 [--once]" >&2
    exit 2
    ;;
esac
