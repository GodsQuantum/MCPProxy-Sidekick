#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request

MANAGER = "http://127.0.0.1:8080"
AGENT_BROWSER = "/data/bin/agent-browser"


def request_json(method: str, path: str, timeout: float = 5.0):
    req = urllib.request.Request(MANAGER + path, method=method)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.load(resp)


def ensure_profile(profile_id: str, timeout: float) -> str:
    version_path = f"/api/profiles/{profile_id}/cdp/json/version"
    try:
        version = request_json("GET", version_path, 3.0)
        ws = str(version.get("webSocketDebuggerUrl") or "").strip()
        if ws:
            return ws
    except Exception:
        pass

    try:
        request_json("POST", f"/api/profiles/{profile_id}/launch", 20.0)
    except urllib.error.HTTPError as exc:
        # Some Manager versions answer conflict/already-running while the CDP is
        # becoming available. Polling below is the source of truth.
        if exc.code not in (400, 409, 423):
            raise

    deadline = time.monotonic() + timeout
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            version = request_json("GET", version_path, 3.0)
            ws = str(version.get("webSocketDebuggerUrl") or "").strip()
            if ws:
                return ws
        except Exception as exc:
            last_error = exc
        time.sleep(0.4)
    raise RuntimeError(f"CloakBrowser profile {profile_id} CDP did not become ready: {last_error}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--profile-id", required=True)
    parser.add_argument("--session", required=True)
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--startup-timeout", type=float, default=45.0)
    args = parser.parse_args()

    if not os.path.isfile(AGENT_BROWSER):
        raise RuntimeError(f"agent-browser missing: {AGENT_BROWSER}")

    ws = ensure_profile(args.profile_id, args.startup_timeout)
    cmd = [
        AGENT_BROWSER,
        "--session", args.session,
        "--namespace", args.namespace,
        "--cdp", ws,
        "--input-mode", "human",
        "mcp",
        "--tools", "core,network,state,tabs",
    ]
    env = os.environ.copy()
    env["AGENT_BROWSER_CDP"] = ws
    env["AGENT_BROWSER_SESSION"] = args.session
    env["AGENT_BROWSER_NAMESPACE"] = args.namespace
    os.execvpe(cmd[0], cmd, env)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"cloak-agent-mcp-wrapper: {exc}", file=sys.stderr)
        raise SystemExit(1)
