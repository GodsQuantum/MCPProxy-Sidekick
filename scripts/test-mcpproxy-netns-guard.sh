#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="sidekick-netns-guard-target-$$"
sidecar="sidekick-netns-guard-sidecar-$$"

cleanup() { docker rm -f "$sidecar" "$target" >/dev/null 2>&1 || true; }
trap cleanup EXIT
cleanup

docker run -d --name "$target" alpine:latest sleep 120 >/dev/null
docker run -d --name "$sidecar" --network "container:$target" alpine:latest sleep 120 >/dev/null

inode() {
  local pid
  pid="$(docker inspect -f '{{.State.Pid}}' "$1")"
  readlink "/proc/$pid/ns/net"
}

initial_target="$(inode "$target")"
initial_sidecar="$(inode "$sidecar")"
[[ "$initial_target" == "$initial_sidecar" ]]

docker restart "$target" >/dev/null
restarted_target="$(inode "$target")"
sidecar_after_target_restart="$(inode "$sidecar")"

if [[ "$restarted_target" != "$sidecar_after_target_restart" ]]; then
  MCPPROXY_CONTAINER_NAME="$target" \
  MCPPROXY_SIDECARS="$sidecar" \
  MCPPROXY_NETNS_SETTLE_SECONDS=0 \
    bash "$root/scripts/mcpproxy-netns-guard.sh" --once
fi

rebound_sidecar="$(inode "$sidecar")"
[[ "$restarted_target" == "$rebound_sidecar" ]]
printf 'netns guard OK\n'
