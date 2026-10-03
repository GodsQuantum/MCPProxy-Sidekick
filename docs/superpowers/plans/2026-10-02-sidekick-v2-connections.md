# Sidekick V2 Connections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Sidekick into a capability-aware connection manager with Chromium/Brave Human Auth Browser support, clearer login/OAuth UX, unified Connections, richer Profiles/Agents, safe scope editing, and event-driven state.

**Architecture:** MCPProxy remains authoritative. Sidekick adds normalized safe DTOs and capability detection around stable v0.69.0 while gating newer features at runtime. Browser selection is configuration-driven and switched by a constrained reconfigure script with health-check rollback; Sidekick itself never gets the Docker socket.

**Tech Stack:** Go 1.27, net/http, embedded vanilla JavaScript/CSS, SQLite metadata, Docker Compose, LinuxServer Chromium/Brave, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-02-sidekick-v2-connections-design.md`

## Global Constraints

- MCPProxy stable v0.69.0 is the compatibility floor.
- MCPProxy remains source of truth for servers, profiles, tokens, OAuth, trust and quarantine.
- Sidekick never exposes admin key, full config, persisted Agent Token values, CDP, or Bitwarden vault state to the browser.
- No AI agent gets generic Human Auth Browser access.
- No npm production dependency or frontend framework.
- Browser profiles are not synced with Syncthing.
- Sidekick container gets no Docker socket.
- The production environment remains unchanged until local and GitHub CI are green.
- All behavior changes follow RED -> GREEN TDD.
- Browser reconfiguration must roll back on failed GUI/CDP health.

## Review Focus

- Missing/differently wrapped optional MCPProxy endpoints degrade capabilities without breaking baseline state. Task 1.
- Masked secret/config fields never persist as credentials. Tasks 3 and 4.
- Invalid/unhealthy browser switch restores previous provider/env. Task 2.
- OAuth scope changes validate exact diff and require re-consent. Task 4.
- SSE disconnects/duplicates/unknown events reconnect or fall back without duplicate destructive actions. Task 8.

---

### Task 1: MCPProxy capabilities and version-aware client

**Files:**
- Create: `internal/capabilities/capabilities.go`
- Create: `internal/capabilities/capabilities_test.go`
- Create: `internal/mcpproxy/client_info.go`
- Create: `internal/mcpproxy/client_info_test.go`
- Modify: `internal/mcpproxy/client.go`
- Modify: `internal/mcpproxy/types.go`

**Interfaces:**
- Produces: `mcpproxy.Info(ctx context.Context) (Info, error)`
- Produces: `capabilities.Detect(ctx context.Context, probe Probe) State`
- Produces: `State{Version string; ProfileV3, Clients, Attention, AccessExplain bool}`
- Consumed by Tasks 3, 7 and 8.

- [ ] Write failing tests for wrapped/unwrapped `/api/v1/info`, v0.69.0 baseline, optional 404/405, and transport errors retaining baseline.
- [ ] Run `go test ./internal/mcpproxy ./internal/capabilities`; verify RED.
- [ ] Implement `Info` plus short-timeout endpoint probing.
- [ ] Run targeted tests and `go test ./...`; verify GREEN.
- [ ] Commit: `feat: detect MCPProxy capabilities`.

### Task 2: Human Auth Browser provider configuration and safe reconfigure script

**Files:**
- Create: `internal/browser/provider.go`
- Create: `internal/browser/provider_test.go`
- Create: `scripts/configure-browser.sh`
- Create: `scripts/configure-browser-test.sh`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `compose.yaml`
- Modify: `.env.example`

**Interfaces:**
- Produces: `browser.Provider{Name, Image, CLIEnv, ProfileDir}`
- Produces: `browser.Resolve(name string) (Provider, error)`
- Adds config fields `BrowserProvider`, `BitwardenMode`, `BitwardenBaseURL`.
- Script accepts exactly `chromium|brave`, changes only browser env keys, recreates only `oauth-browser`, probes GUI/CDP, and restores env/container on failure.
- Consumed by Tasks 6 and 9.

- [ ] Write failing Go tests for manifests, invalid provider, defaults and Bitwarden mode validation.
- [ ] Write failing shell test with fake Docker proving invalid provider is a no-op and health failure restores original env.
- [ ] Run targeted Go + shell tests; verify RED.
- [ ] Implement registry/config/script. Compose uses `${SIDEKICK_BROWSER_IMAGE}`, sets both `CHROME_CLI` and `BRAVE_CLI`, and uses separate profile directories.
- [ ] Run tests plus `docker compose config -q`; verify GREEN.
- [ ] Commit: `feat: add configurable auth browser providers`.

### Task 3: Normalized Connections backend

**Files:**
- Create: `internal/connections/model.go`
- Create: `internal/connections/model_test.go`
- Create: `internal/mcpproxy/client_server_detail.go`
- Create: `internal/mcpproxy/client_server_detail_test.go`
- Modify: `internal/web/handlers.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Produces `connections.Build(server mcpproxy.Server, profiles []mcpproxy.Profile, meta storage.CredentialMeta, hasMeta bool) Detail`.
- Produces `GET /api/connections`, `GET /api/connections/{name}`, `PATCH /api/connections/{name}`.
- DTO includes masked credential metadata, OAuth/scopes, readiness, profile memberships and supported actions; never raw secrets.
- Consumed by Tasks 6 and 7.

- [ ] Write failing model tests for OAuth, header credentials, profile membership, quarantine/actions and redaction.
- [ ] Run `go test ./internal/connections ./internal/web`; verify RED.
- [ ] Implement normalized model and minimal-diff mutation mapping.
- [ ] Add handler tests for CSRF/Origin and no fixture-secret/mask-placeholder leakage.
- [ ] Run targeted + full Go suite; verify GREEN.
- [ ] Commit: `feat: add normalized Connections API`.

### Task 4: OAuth scope inspection, preview and apply

**Files:**
- Create: `internal/oauthconfig/service.go`
- Create: `internal/oauthconfig/service_test.go`
- Create: `internal/mcpproxy/client_config.go`
- Create: `internal/mcpproxy/client_config_test.go`
- Modify: `internal/web/handlers.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Produces: `oauthconfig.Preview(ctx, server string, desired []string) (Diff, error)`
- Produces: `oauthconfig.Apply(ctx, server string, desired []string) (Result, error)`
- Produces: `GET /api/connections/{name}/oauth-scopes`, `POST .../oauth-scopes/preview`, `POST .../oauth-scopes/apply`.
- Full MCPProxy config remains server-side.
- `ReauthRequired=true` whenever normalized scopes change.

- [ ] Write failing service tests for no-op diff, add/remove, dedupe, unknown server, masked-secret preservation and reauth.
- [ ] Run `go test ./internal/oauthconfig ./internal/mcpproxy`; verify RED.
- [ ] Implement redacted config read, minimal scope patch, MCPProxy validation/apply and post-apply verification.
- [ ] Add failing then passing web tests for session/CSRF/Origin, unknown fields, body size and no full-config leakage.
- [ ] Run targeted packages plus `go test ./...`; verify GREEN.
- [ ] Commit: `feat: manage OAuth scopes safely`.

### Task 5: Login and OAuth operation feedback contract

**Files:**
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`
- Modify: `internal/web/assets/index.html`
- Modify: `internal/web/assets/app.js`
- Modify: `internal/web/assets/app.css`
- Modify: `scripts/ui-smoke.mjs`

**Interfaces:**
- Login success JSON adds safe `mcpproxy_version`.
- Login grants a cookie only after authenticated MCPProxy info probe succeeds.
- Frontend login states: `idle|checking|success|error`.
- OAuth-start UI states: `preparing|browser-opening|waiting|error`.

- [ ] Write failing Go tests: MCPProxy unavailable -> no cookie + actionable 502; success -> version; wrong key -> 401.
- [ ] Run `go test ./internal/web`; verify RED.
- [ ] Implement backend login verification with no key logging.
- [ ] Update smoke assertions first for disabled submit, `aria-busy`, live status text and success; run and verify RED.
- [ ] Implement minimal JS/CSS login/OAuth states, preserving field on failure and clearing on success.
- [ ] Run JS syntax, full Go suite and browser smoke; verify GREEN.
- [ ] Commit: `feat: improve login and OAuth feedback`.

### Task 6: Connections-first information architecture and Settings browser card

**Files:**
- Modify: `internal/web/assets/index.html`
- Modify: `internal/web/assets/app.js`
- Modify: `internal/web/assets/app.css`
- Modify: `scripts/ui-smoke.mjs`
- Modify: `internal/web/server_test.go`

**Interfaces:**
- Navigation becomes `Overview - Connections - Profiles - Agents - Activity & Security - Settings`.
- Connections consumes Task 3 endpoints.
- Settings shows current Browser Provider, Bitwarden mode, health/capability notes and the safe reconfigure command when host-side switching is required.
- Legacy upstream/credential/OAuth actions remain reachable from Connection detail.

- [ ] Extend UI smoke first for six-nav structure, connection cards/detail, Advanced disclosure, Settings browser card and keyboard-visible controls; verify RED.
- [ ] Implement semantic HTML/navigation and connection-detail rendering without a framework.
- [ ] Add credential editor, OAuth reconnect, connection actions and scope preview/apply on the same detail surface.
- [ ] Add accessibility assertions for dialog focus return, `aria-live`, non-color-only status and reduced-motion-safe loading.
- [ ] Run JS syntax, Go suite and browser smoke; verify GREEN.
- [ ] Commit: `feat: redesign Sidekick around Connections`.

### Task 7: Capability-aware Profiles and Agents

**Files:**
- Modify: `internal/mcpproxy/client_profiles.go`
- Modify: `internal/mcpproxy/client_profiles_test.go`
- Create: `internal/mcpproxy/client_profile_v3.go`
- Create: `internal/mcpproxy/client_profile_v3_test.go`
- Modify: `internal/web/handlers.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`
- Modify: `internal/web/assets/app.js`
- Modify: `scripts/ui-smoke.mjs`

**Interfaces:**
- Stable v0.69.0 retains current profile CRUD/profile-pinned tokens.
- Task 1 capabilities gate Profile-v3 fields/endpoints.
- Produces `POST /api/agents/onboard` returning one-time token material plus safe endpoint/snippet metadata.
- Stored token values are never returned again after creation/regeneration.

- [ ] Write failing client/web tests for stable fallback, Profile-v3 path, optional endpoint absence and one-time token response.
- [ ] Run targeted tests; verify RED.
- [ ] Implement optional Profile-v3 calls and normalized agent-onboarding handler.
- [ ] Extend UI smoke first for Give to an agent wizard, profile assignment and one-time secret dismissal; verify RED.
- [ ] Implement capability-aware Profile editor and Agents page.
- [ ] Run full Go + UI smoke; verify GREEN.
- [ ] Commit: `feat: add profile-aware agent onboarding`.

### Task 8: Event-driven Activity & Security

**Files:**
- Create: `internal/events/relay.go`
- Create: `internal/events/relay_test.go`
- Modify: `internal/mcpproxy/client.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`
- Modify: `internal/web/assets/app.js`
- Modify: `scripts/ui-smoke.mjs`

**Interfaces:**
- Produces `GET /api/events` authenticated SSE.
- Relay emits only normalized invalidation/attention events, never raw secret-bearing MCPProxy payloads.
- Frontend prefers EventSource, deduplicates refreshes, reconnects with backoff and falls back to visibility-aware polling.

- [ ] Write failing relay tests for known normalization, unknown drop, secret-looking payload non-forwarding, disconnect and duplicate coalescing.
- [ ] Run `go test ./internal/events ./internal/web`; verify RED.
- [ ] Implement backend relay and authenticated SSE handler.
- [ ] Extend smoke first for EventSource activation and polling fallback on forced disconnect; verify RED.
- [ ] Implement frontend invalidation, Activity & Security rendering and hidden-page fallback.
- [ ] Run full suite + smoke; verify GREEN.
- [ ] Commit: `feat: add event-driven Sidekick state`.

### Task 9: Brave/Bitwarden deployment artifacts and CI matrix

**Files:**
- Create: `deploy/oauth-browser/policies/bitwarden.json.tmpl`
- Create: `scripts/render-browser-policy.py`
- Create: `scripts/test-browser-provider.sh`
- Modify: `compose.yaml`
- Modify: `.env.example`
- Modify: `.github/workflows/ci.yml`
- Modify: `scripts/privacy-scan.sh`

**Interfaces:**
- Managed Bitwarden uses official extension ID `nngceckbapebfimnlniiiahkandclblb` and official Chrome update URL.
- Policy renderer accepts only `off|assist|managed-extension` and optional HTTPS base URL; output contains no vault credentials.
- CI exercises disposable Chromium and Brave providers with GUI/CDP probes and opens one page.

- [ ] Write failing renderer/provider tests for official extension ID, HTTPS URL validation, off/assist no-force-install behavior and no secret-like values.
- [ ] Run provider test scripts; verify RED.
- [ ] Implement policy template/renderer and compose mounts for Chromium/Brave without public CDP.
- [ ] Add CI provider matrix for Chromium and Brave; run local equivalents.
- [ ] Extend latest-stable MCPProxy contract job to probe `/info`, baseline APIs and tolerate optional capability 404s.
- [ ] Run script syntax, privacy scan, Go suite, container build and local provider tests; verify GREEN.
- [ ] Commit: `ci: validate Chromium and Brave auth browsers`.

### Task 10: Documentation, branch verification and pre-production package

**Files:**
- Modify: `README.md`
- Create: `docs/sidekick-v2-operations.md`
- Modify: `docs/assets/dashboard.png` only if a fresh verified deterministic screenshot is produced.
- Modify design spec only for implementation rulings discovered during execution.
- Modify current MCP handoff only after production deployment.

**Interfaces:**
- Operations doc records browser selection, Brave Sync guidance, Bitwarden modes, rollback, capability gating and zero-Docker-socket Sidekick posture.
- Pre-production report identifies verified commit/image and current rollback image without deploying.

- [ ] Update docs from verified behavior, not planned behavior.
- [ ] Run full verification: `go test -race ./...`, `go vet ./...`, JS checks, browser smoke, privacy scan, govulncheck, local secret scan, Docker build and Trivy HIGH/CRITICAL scan.
- [ ] Run whole-branch review package; fix Critical/Important findings with RED -> GREEN tests and ledger Minor findings.
- [ ] Commit: `docs: document Sidekick V2 operations`.
- [ ] Push the feature branch through the credentialed GitHub integration and wait for all CI jobs to pass. No production deployment before green CI.

### Task 11: Production deployment, smoke verification and cleanup

**Files:**
- Deployment only after Task 10 GitHub CI green.
- Update live CT400 compose/env from verified repository commit.
- Update MCP handoff/current documentation with final commit/image/provider/rollback.

**Interfaces:**
- Deploy immutable image tagged by verified Git commit.
- Keep current production image/config available for rollback until smoke is green.

- [ ] Record live rollback state: Sidekick image/tag, browser image/provider, compose checksum and health states.
- [ ] Apply verified Sidekick image/config and selected browser provider without upgrading MCPProxy.
- [ ] Verify login, Connections, Profiles, Agents, SSE fallback, browser GUI/CDP, OAuth reconnect only if needed, MCPProxy health, n8n/Archie/Fatma profile access and YouTube/Workspace MCP health.
- [ ] If any smoke fails, roll back immediately and verify restoration.
- [ ] If green, update handoff with commit, image, browser provider, Bitwarden mode, rollback and capability gates.
- [ ] Clean only task-created resources: disposable browser containers/images, test files and superseded unreferenced Sidekick images; do not remove unrelated user stacks.
- [ ] Final health audit confirms relevant production services healthy and repo/worktree clean.
