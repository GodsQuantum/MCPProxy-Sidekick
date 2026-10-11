# CloakBrowser Fella — independently licensed, production on Cloud9

Operational state verified 2026-10-08 23:40+ CEST. Current live state always
takes precedence over this guide.

## Mise en production HTTPS — 12 octobre 2026

- **URL Fella : `https://browser-fella.arezkichougar.com/` (ACTIVE).**
- Ressource Pangolin HTTP **49**, domaine `browser-fella.arezkichougar.com`, `enabled=true`, `sso=true`, `blockAccess=false`; règle d'accès **rôle Admin (id=1) uniquement**, identique à Arezki.
- Target Pangolin : site **Cloud 9 (2)** vers `192.168.1.218:18080` en HTTP. Le proxy Caddy `mcpproxy-gateway` sur CT400 fait le routage `Host: browser-fella.arezkichougar.com` vers `cloakbrowser-manager-fella:8080` par Docker network `ct400-services`.
- **Le port Docker Fella reste à `127.0.0.1:18089:8080`** ; aucun port de Manager Fella ouvert au LAN. Aucune modification de montage ou de profil.
- Tests réalisés : Caddy `validate` puis `reload` OK ; `/api/health` et `/api/profiles` via Caddy HTTP 200 ; handshake WebSocket CDP via Caddy OK ; HTTPS public retourne **401 anonyme (SSO)**, comme browser Arezki ; le raccourci `/control/browser/fella` redirige vers le nouveau domaine.
- L'ouverture de Google sous l'identité Fella et un vrai parcours humain Pangolin SSO restent à effectuer ; ne jamais prétendre que les cookies Google ont déjà été renouvelés. L'historique du 9 octobre ci-dessous décrit l'état **antérieur** à cette publication.

## Runtime and access

- Arezki manager: cloakbrowser-manager, own Free key, own browser profile.
- Fella manager: cloakbrowser-manager-fella, distinct Free key, own native
  profile (ID f59b12fa-7533-4825-a9ba-c81717e3924b).
- Both Managers are healthy and use CloakBrowser Chromium 154 stable.
- Separate native Compose project: cloakbrowser-fella.
- Its Compose checkpoint: deploy/cloakbrowser/compose.fella.yaml.
- Its active Compose: /srv/lxc/ia-studio/compose/cloakbrowser-fella/compose.yaml.
- Its persistent data, settings, browser profile, cookies and cache live only
  under /srv/lxc/ia-studio/appdata/cloakbrowser-manager-fella/.
- Shares read-only Windows fonts, NOT license files, cookies or browser state.
- Its human admin UI is bound to CT400 loopback 127.0.0.1:18089.
  Reach it only through an authenticated SSH tunnel until a dedicated
  HTTPS/SSO Pangolin route is configured. Never publicly expose raw Manager.
- Free key was entered in native Manager Settings; license_tier=free verified.
  Persisted locally in settings.json, NEVER present in Git, MCPProxy or Sidekick.

## MCPProxy integration

- Upstream browser-cloak-fella is stdio via docker exec into
  cloakbrowser-manager-fella (NOT cloakbrowser-manager).
- Target wrapper: /data/bin/browser-cloak-agent-mcp-wrapper.py.
- Target profile ID: f59b12fa-7533-4825-a9ba-c81717e3924b.
- Session/namespace: fella / cloak-fella.
- Both browser agents remain independent; Arezki upstream is unchanged.
- Declarative routing is persisted by native MCPProxy REST PATCH in the
  CT400 persistent mcp_config.json. Safe snapshot of old config:
  /srv/lxc/ia-studio/backups/20261008-fella-manager-route/.
- Existing agent-browser binary and tracked wrapper were copied into Fella
  independent appdata/bin; they persist across container recreation.
- Never paste a CloakBrowser license key as a Sidekick/MCPProxy credential:
  the generic 'Credential not configured' UI label is NOT license state.

## Validation performed

- Both native Managers: healthy, Free license, Chromium 154 stable.
- Both profiles concurrently running, no HTTP 402 at launch.
- Both MCPProxy upstreams: READY, 71 tools each.
- Actual MCPProxy read-only call agent_browser_get_url: isError=false
  for browser-cloak-fella and browser-cloak-arezki.
- Source Fella profile filesystem was migrated from its previously stopped
  old-manager profile, checksum comparison 0 differences. Original was left
  in place for rollback and must remain stopped; no data deleted.
- Follow-up smoke 2026-10-09 00:40 CEST: BOTH direct KasmVNC WebSocket
  connections succeeded (binary subprotocol); clipboard POST + GET text
  round-trip matched on BOTH live browsers and original clipboard text was
  restored and verified. BOTH MCPProxy real read-only calls returned
  isError=false. MCP profile discovery/isolation smoke: 9/9 PASS.
- Browser Arezki Pangolin HTTPS correctly returns 401 anonymously.
  browser-fella.arezkichougar.com is still 404 (no Pangolin resource); an
  experimental native Caddy route was validated, but fully ROLLED BACK
  after Pangolin resource creation was blocked. Direct Fella Manager access
  remains localhost-only via SSH tunnel. Never claim Fella HTTPS is live.
- No full CT400/host restart or human clipboard action through Pangolin
  HTTPS/SSO was performed; neither is proven end-to-end.

## Future maintenance

- To verify status: use native Manager GET /api/status and /api/profiles;
  MCPProxy upstream list then actual read-only MCP tool call.
- Restart only browser-cloak-fella upstream when necessary; do not interrupt
  Arezki's human session.
- If updating the native wrapper: change it on CT700 NVMe, commit and push,
  mirror to HDD after validation, copy the verified binary to both managers'
  persistent appdata/bin. Do not create custom polling/watcher daemons.
- Do not change mounts or paths; treat two distinct licenses as individual
  accounts and comply with CloakHQ licensing rules.
