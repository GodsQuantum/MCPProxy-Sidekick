# CloakBrowser profile-owner — independently licensed, production on Cloud9

Operational state verified 2026-10-08 23:40+ CEST. Current live state always
takes precedence over this guide.

## Runtime and access

- developer manager: cloakbrowser-manager, own Free key, own browser profile.
- profile-owner manager: cloakbrowser-manager-profile-owner, distinct Free key, own native
  profile (ID f59b12fa-7533-4825-a9ba-c81717e3924b).
- Both Managers are healthy and use CloakBrowser Chromium 154 stable.
- Separate native Compose project: cloakbrowser-profile-owner.
- Its Compose checkpoint: deploy/cloakbrowser/compose.profile-owner.yaml.
- Its active Compose: /srv/lxc/ia-studio/compose/cloakbrowser-profile-owner/compose.yaml.
- Its persistent data, settings, browser profile, cookies and cache live only
  under /srv/lxc/ia-studio/appdata/cloakbrowser-manager-profile-owner/.
- Shares read-only Windows fonts, NOT license files, cookies or browser state.
- Its human admin UI is bound to CT400 loopback 127.0.0.1:18089.
  Reach it only through an authenticated SSH tunnel until a dedicated
  HTTPS/SSO Pangolin route is configured. Never publicly expose raw Manager.
- Free key was entered in native Manager Settings; license_tier=free verified.
  Persisted locally in settings.json, NEVER present in Git, MCPProxy or Sidekick.

## MCPProxy integration

- Upstream browser-cloak-profile-owner is stdio via docker exec into
  cloakbrowser-manager-profile-owner (NOT cloakbrowser-manager).
- Target wrapper: /data/bin/browser-cloak-agent-mcp-wrapper.py.
- Target profile ID: f59b12fa-7533-4825-a9ba-c81717e3924b.
- Session/namespace: profile-owner / cloak-profile-owner.
- Both browser agents remain independent; developer upstream is unchanged.
- Declarative routing is persisted by native MCPProxy REST PATCH in the
  CT400 persistent mcp_config.json. Safe snapshot of old config:
  /srv/lxc/ia-studio/backups/20261008-profile-owner-manager-route/.
- Existing agent-browser binary and tracked wrapper were copied into profile-owner
  independent appdata/bin; they persist across container recreation.
- Never paste a CloakBrowser license key as a Sidekick/MCPProxy credential:
  the generic 'Credential not configured' UI label is NOT license state.

## Validation performed

- Both native Managers: healthy, Free license, Chromium 154 stable.
- Both profiles concurrently running, no HTTP 402 at launch.
- Both MCPProxy upstreams: READY, 71 tools each.
- Actual MCPProxy read-only call agent_browser_get_url: isError=false
  for browser-cloak-profile-owner and browser-cloak-developer.
- Source profile-owner profile filesystem was migrated from its previously stopped
  old-manager profile, checksum comparison 0 differences. Original was left
  in place for rollback and must remain stopped; no data deleted.
- No full CT400/host restart or HTTPS/SSO profile-owner human clipboard test was
  performed as part of this activation.

## Future maintenance

- To verify status: use native Manager GET /api/status and /api/profiles;
  MCPProxy upstream list then actual read-only MCP tool call.
- Restart only browser-cloak-profile-owner upstream when necessary; do not interrupt
  developer's human session.
- If updating the native wrapper: change it on CT700 NVMe, commit and push,
  mirror to HDD after validation, copy the verified binary to both managers'
  persistent appdata/bin. Do not create custom polling/watcher daemons.
- Do not change mounts or paths; treat two distinct licenses as individual
  accounts and comply with CloakHQ licensing rules.
