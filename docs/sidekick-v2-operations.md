# Sidekick V2 Operations

This document covers the verified V2 operating model: Connections-first UI, Human Auth Browser providers, OAuth scope management, capability-gated Profiles, Agent onboarding, and event-driven state.

## Compatibility

- MCPProxy v0.69.0 is the compatibility floor.
- Optional newer capabilities are detected at runtime and hidden when unsupported.
- MCPProxy remains authoritative for servers, Profiles, Agent Tokens, trust/quarantine state, and OAuth configuration.

## Human Auth Browser

Supported providers:

| Provider | Image | CLI variable | Persistent profile |
| --- | --- | --- | --- |
| Chromium | lscr.io/linuxserver/chromium:latest | CHROME_CLI | /config/chromium-sidekick-oauth |
| Brave | lscr.io/linuxserver/brave:latest | BRAVE_CLI | /config/brave-sidekick-oauth |

Both providers use Selkies for the visible browser and private CDP on loopback.

A deployment may declare multiple persistent browser instances (for example separate primary/secondary profiles) and reuse them for both Playwright automation and human OAuth. When those browsers share `network_mode: container:<mcpproxy>`, assign each instance distinct GUI, Selkies control, browser CDP and optional CDP-relay ports. The provider OAuth callback to `127.0.0.1:<dynamic-port>` then reaches MCPProxy naturally because the browser and MCPProxy share the same loopback. Add every shared-netns browser container to `MCPPROXY_SIDECARS` so the namespace guard rebinds it after an MCPProxy restart.

Example Brave settings:

    SIDEKICK_BROWSER_PROVIDER=brave
    SIDEKICK_BROWSER_IMAGE=lscr.io/linuxserver/brave:latest
    SIDEKICK_BROWSER_PROFILE_DIR=brave-sidekick-oauth

For an existing installation:

    ./scripts/configure-browser.sh brave

The guarded switch backs up browser settings, recreates only oauth-browser, verifies GUI and CDP health, and automatically restores the previous browser if verification fails. On a fresh install where no `.env` existed, failure restores that absence and recreates the browser from Compose defaults. It never changes MCPProxy, Profiles, Agent Tokens, or upstream configuration.

## Brave Sync

Use Brave Sync inside the Human Auth Browser for supported synchronized browser data. Do not copy or synchronize the raw browser profile directory between hosts. Chromium and Brave use separate persistent profile directories so switching providers preserves rollback.

## Bitwarden

SIDEKICK_BITWARDEN_MODE accepts:

- off: no Bitwarden browser policy.
- assist: optional self-hosted vault URL configuration, no forced extension install.
- managed-extension: force-installs the official Bitwarden Chromium extension by managed browser policy.

Optional self-hosted configuration:

    SIDEKICK_BITWARDEN_MODE=managed-extension
    SIDEKICK_BITWARDEN_BASE_URL=https://vault.example.com

The base URL must use HTTPS and cannot contain user credentials, query parameters, or fragments.

Sidekick never stores a Bitwarden master password, recovery key, vault session, access/refresh token, or passkey material. Unlock/sign in to Bitwarden manually in the Human Auth Browser. Agents never receive generic access to that browser or vault.

## Connections and OAuth scopes

Connections combines upstream state, credentials, OAuth, tools, and Profile membership.

The frontend receives normalized safe metadata only. Raw secrets and the complete MCPProxy configuration never reach it. Credential writes use minimal diffs and never write masked placeholders back as real credentials. Connection endpoint display is also sanitized: userinfo, query strings, fragments, and potentially secret path segments are never returned to the UI.

OAuth scope changes follow this backend-only flow:

1. Read redacted MCPProxy config server-side.
2. Change only the selected upstream scope list.
3. Validate through MCPProxy.
4. Apply through MCPProxy.
5. Re-read and verify the persisted scope set.
6. Require OAuth re-consent when the set changed.

## Profiles and Agents

On v0.69.0 Sidekick supports native Profiles and profile-pinned Agent Tokens. Newer policy controls appear only when their capabilities are detected.

Give to an agent returns the full token only in the creation/regeneration response. Sidekick does not keep a recoverable token copy.

## Live state

Sidekick consumes MCPProxy /events through an authenticated relay. The relay discards upstream event payloads and sends only normalized event types to the browser.

On an unexpected disconnect or EOF, Sidekick falls back to visibility-aware polling, reconnects with exponential backoff plus jitter, and returns to SSE after recovery. Live updates pause while the page is hidden.

## Security boundaries

- No Docker socket in Sidekick.
- CDP is never publicly routed.
- Visible browser UI stays behind Sidekick authentication.
- AI agents never get generic Human Auth Browser/CDP access.
- Admin-key login becomes a server-side HttpOnly session; the key is not stored in browser storage.
- Mutations require CSRF and trusted Origin/Host.
- Full MCPProxy config and credentials remain server-side.
- Sidekick keeps read-only root filesystem, dropped capabilities, and no-new-privileges.

## Browser validation

Run:

    ./scripts/test-browser-provider.sh chromium
    ./scripts/test-browser-provider.sh brave

Each test verifies browser identity, Selkies GUI, CDP, managed Bitwarden policy, actual extension installation, CDP page creation, and profile persistence across container recreation.

## Restore and rollback

Restore MCPProxy first, then Sidekick configuration and the admin-key secret mount. Restore the optional Sidekick metadata volume if desired. Browser Sync and Bitwarden login remain human actions.

Before upgrades record the Sidekick image/tag, browser image/provider, browser environment settings, and Compose checksum.

Browser rollback:

    ./scripts/configure-browser.sh chromium

Application rollback restores the previous immutable Sidekick image and Compose configuration, then verifies health, login, Connections, and the Human Auth Browser.

## Pre-release verification

    go test -race ./...
    go vet ./...
    node --check internal/web/assets/app.js
    bash scripts/privacy-scan.sh .
    python3 scripts/test_render_browser_policy.py
    ./scripts/configure-browser-test.sh
    ./scripts/test-browser-provider.sh chromium
    ./scripts/test-browser-provider.sh brave

CI additionally runs CodeQL, govulncheck, gitleaks, container build, Trivy HIGH/CRITICAL scanning, and a contract test against the latest stable MCPProxy release.

## Shared network namespace resilience

When Sidekick and the Human Auth Browser use `network_mode: container:<mcpproxy>`, a runtime restart of the MCPProxy container creates a new network namespace. Existing sidecars can remain attached to the old namespace even though their Docker status is still `healthy`.

Sidekick ships `scripts/mcpproxy-netns-guard.sh` and an optional systemd unit template at `deploy/systemd/mcpproxy-netns-guard.service`.

The guard:
- compares the network namespace inode of MCPProxy and the configured sidecars;
- does nothing when namespaces already match;
- restarts only stale sidecars;
- listens for future Docker `start` events for MCPProxy;
- does not require the Docker socket inside Sidekick itself.

Install the helper on a Docker host only when shared-container networking is used. Override `MCPPROXY_SIDECARS` in the systemd unit for local container names.
