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

## Correctifs de stabilité, langue et performances — 12 octobre 2026

- CloakBrowser Manager upstream [#51](https://github.com/CloakHQ/CloakBrowser-Manager/issues/51) : WebSocket VNC du serveur Uvicorn expirait par défaut après 20 secondes si sa boucle Python était ralentie ; UI revenait au profil sans reconnect automatique. **Mitigation native persistante** appliquée aux DEUX gestionnaires : `--ws-ping-interval 20 --ws-ping-timeout 120`, en conservant la séquence intégrale de l'entrypoint upstream et son nettoyage des anciens locks. Configuration déclarative sous les deux Compose existants, sans nouveau volume/montage, sans daemon personnalisé.
- Compose Arezki : `/srv/lxc/ia-studio/compose/browser-profiles/compose.yaml` ; Compose Fella : `/srv/lxc/ia-studio/compose/cloakbrowser-fella/compose.yaml`, dont la copie source versionnée est `deploy/cloakbrowser/compose.fella.yaml`. Le sidecar OAuth loopback Arezki suit le re-create de son gestionnaire pour ne pas conserver un ancien namespace réseau.
- **Chromium UI français vérifié sur les deux** : `LANGUAGE: fr` dans environnement manager, en plus du profil `locale=fr-FR` déjà correct. Contrôle réel `chrome://settings/languages` : titre **« Paramètres – Langues »**. Voir [Chromium Linux i18n](https://chromium.googlesource.com/chromium/src/+/HEAD/docs/linux/debugging.md).
- **FPS** : KasmVNC possède `encoding.max_frame_rate=60` par défaut : plafond 60 FPS pour les deux, débit réel adaptatif. Ne pas forcer 60 FPS au repos ni activer un encodage agressif qui consomme davantage. Profils écran 1920×1080, même mémoire limite 3 Gio, SHM 1 Gio, GPU /dev/dri/renderD128 déjà en place, aucune duplication de profil ni changement de fingerprint.
- Contrôles réels : deux conteneurs healthy et deux profils `running` après recréation ; Uvicorn argv confirme 120 secondes chaque instance ; Google.com HTTP200 dans les DEUX Chromium, YouTube Music compte Arezki HTTP200 ; Fella aboutit à `consent.youtube.com` (Google pas encore connecté) ; `browser-fella` HTTPS anonyme HTTP401 attendu, backend interne HTTP200, sidecar OAuth Arezki relancé et actif.
- Music Assistant 2.10.6 : le fournisseur Fella a été modifié par l'utilisateur à 00:40, mais le backend a journalisé **échec cookie puis fallback anonyme**, tandis que le cookie Arezki est authentifié. Droits MA vérifiés : `arezki` a exclusivement `ytmusic_free--puRHxXoX` et `fella` exclusivement `ytmusic_free--Br5QuFxC`. Fella doit d'abord connecter Google dans son CloakBrowser (consentement), puis copier le vrai Request Headers → Cookie d'une requête `/youtubei/v1/browse`. AUCUN cookie ni secret copié dans un document ou modifié par ce correctif.
- **Limite résiduelle :** upstream #51 indique que la frontend ne reconnecte toujours pas automatiquement après une vraie déconnexion WebSocket ou suspension du client. Le timeout 120 s atténue les chutes liées à la boucle d'événements ; il ne prouve pas que les déconnexions sont définitivement impossibles. Aucune modification destructive de profil, de permissions SSO ou de montages.

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
