# MCPProxy Sidekick V2 — Connections, Human Auth Browser, Profiles & Agents

**Date:** 2026-10-02
**Status:** Design approved in chat; implementation not started
**Compatibility floor:** MCPProxy stable v0.69.0
**Production rule:** The production environment remains unchanged until the implementation branch is green locally and in GitHub CI.

## 1. Product intent

Sidekick should behave as a connection manager for AI agents rather than a thin frontend over MCPProxy primitives.

Primary journey:

1. Add or inspect a service connection.
2. Authenticate as naturally as in a normal browser.
3. Confirm the connection becomes Ready.
4. Assign it to one or more Profiles.
5. Give an agent a profile-pinned credential and endpoint.
6. Surface later authentication, tool, quarantine, or version problems automatically.

The default UI hides infrastructure detail until useful; advanced operator controls remain accessible.

## 2. Non-goals

- MCPProxy remains the source of truth.
- Sidekick does not store full upstream secrets in browser state or metadata.
- AI agents do not receive generic Human Auth Browser control.
- Sidekick does not bypass supported MCPProxy mutation surfaces with direct production-file edits.
- The redesign must not require an unreleased MCPProxy build.
- No heavy frontend framework is added solely for the redesign.
- Browser profile directories are not synced across machines with Syncthing.
- No production deployment happens before local verification and GitHub CI are green.

## 3. Architecture principles

### 3.1 MCPProxy remains authoritative

Sidekick reads live state from MCPProxy and performs mutations through authenticated MCPProxy management surfaces. Sidekick stores only Sidekick-specific metadata such as browser-provider selection, UI preferences, and masked credential metadata.

### 3.2 Capability-driven behavior

Sidekick discovers the connected MCPProxy version and API capabilities. Unsupported controls are omitted or shown read-only; there is no silent fallback to direct config-file editing.

Capability detection covers at least:
- stable v0.69.0 behavior;
- Profile v3;
- /api/v1/clients;
- /api/v1/attention;
- /api/v1/access/explain;
- richer server health/action vocabulary.

### 3.3 Human Auth Browser is separate from agent automation

The visible browser used for OAuth and passkeys is a privileged human interaction surface. Agents may trigger a connection request but never receive CDP access, browser cookies, Bitwarden vault access, or arbitrary tab control.

## 4. Human Auth Browser providers

### 4.1 Provider abstraction

Introduce a BrowserProvider abstraction with initial providers Chromium and Brave.

| Provider | Image | CLI env | GUI | CDP |
| --- | --- | --- | --- | --- |
| Chromium | existing LinuxServer Chromium image | CHROME_CLI | Selkies :3000 | 127.0.0.1:9222 |
| Brave | lscr.io/linuxserver/brave:latest | BRAVE_CLI | Selkies :3000 | 127.0.0.1:9222 |

Each provider defines its image, CLI environment variable, profile persistence rules, GUI/CDP probes, optional extension policy, display name, and version probe.

A disposable container audit on 2026-10-02 verified Brave 154.1.96.60, Selkies on port 3000, DevTools protocol 1.3 on port 9222 when launched through BRAVE_CLI, and persistent profile data below /config/.config/BraveSoftware/Brave-Browser.

### 4.2 Provider selection and rollback

Install and Settings expose:

Human Auth Browser: Chromium | Brave

Changing provider must:
1. validate the requested provider;
2. preserve the previous configuration;
3. recreate only the browser service;
4. run GUI and CDP health checks;
5. roll back automatically if health checks fail.

Changing browser provider must not alter MCPProxy, Profiles, Agent Tokens, or upstream configuration.

### 4.3 Brave profile and Sync

The server-side Brave profile persists under /config. Sidekick may guide Brave Sync onboarding but must not copy the raw profile to another host.

### 4.4 Bitwarden integration

Opt-in modes:
- off;
- assist: guide installation and sign-in;
- managed-extension: install the official Bitwarden Chromium-compatible extension by policy, with optional preconfigured self-hosted vault URL.

Sidekick never stores Bitwarden master passwords, recovery keys, passkeys, or unlocked sessions. Bitwarden-held passkeys are the preferred remote-browser passkey mechanism.

## 5. Navigation and information architecture

Replace:
Overview · Profiles · Upstreams · Credentials · OAuth · Agent Tokens · Security · Setup / Restore

with:
Overview · Connections · Profiles · Agents · Activity & Security · Settings
### 5.1 Overview

Show:
- MCPProxy connection and version;
- Human Auth Browser provider and health;
- ready and action-needed connection counts;
- tool count;
- profile count;
- active agent credentials;
- high-priority Needs Attention items;
- version and capability notices.

### 5.2 Connections

Connections unifies upstreams, credentials, and OAuth.

Connection cards show:
- canonical name;
- readiness/status;
- authentication type;
- tool count;
- profile membership;
- current required action.

Connection detail shows:
- endpoint/protocol;
- health and action hints;
- enabled/quarantine/trust state;
- masked headers/environment variables;
- OAuth configuration and configured scopes;
- token expiry/validity where MCPProxy exposes it;
- callback/redirect configuration;
- tool approval/quarantine state;
- profile memberships;
- diagnostics/logs;
- connect, reconnect, restart, enable, disable, edit, and convert-to-secret actions.

Advanced fields remain behind an Advanced section.
### 5.3 Profiles

On MCPProxy v0.69.0, Sidekick supports:
- create and safe delete;
- add/remove upstreams;
- tool count;
- pinned Agent Tokens.

When Profile v3 capabilities exist, Sidekick additionally supports:
- title and description;
- server membership;
- max tier;
- unannotated behavior;
- allow/deny/classify rules;
- code execution;
- management tools;
- switchable profiles;
- effective-tool preview;
- profile try;
- safe rename/delete;
- access explanation.

### 5.4 Agents

Agents represents credential consumers rather than raw tokens.

The page shows token status, expiry, last use, profile pin, permissions, regeneration, revoke, and delete.

A Give to an agent wizard follows:
Connection or Profile → Profile → Permission tier → Agent name/type → Create or reuse profile-pinned token → Connection snippet

Initial snippets support generic MCP HTTP, n8n, Dify, and deterministic Claude/Codex-compatible formats. Full token values appear only once after creation or regeneration.

### 5.5 Activity & Security

Combine operational attention and security:
- non-ready, disabled, or quarantined servers;
- tool changes and approvals;
- OAuth failures;
- relevant logs;
- security scan state;
- /attention, activity feed, access explain, and client bindings when supported.
## 6. MCPProxy admin-key login UX

States:
idle → checking → success → loading dashboard
plus error.

Requirements:
- disable duplicate submission while checking;
- visible spinner/progress indicator;
- inline status text;
- aria-busy;
- aria-live result announcement;
- preserve input on failure;
- clear the key immediately after success;
- never store the key in LocalStorage or SessionStorage;
- retain the current HttpOnly session model;
- show connected MCPProxy version after success;
- respect prefers-reduced-motion.

## 7. OAuth UX

### 7.1 Session state

OAuth state is shown as:
- preparing;
- browser opening;
- waiting for sign-in;
- callback received;
- reconnecting upstream;
- Ready;
- actionable error.

Provider, correlation, and request IDs are preserved when available. Errors must be actionable rather than collapsing every failure to HTTP 502.

### 7.2 Browser session

The selected Human Auth Browser receives OAuth URLs through private CDP. The frontend receives only the authenticated browser-session UI URL. CDP remains private to the backend/container network.
### 7.3 OAuth scopes

Sidekick shows configured scopes whenever available.

Scope mutation is backend-only and uses official MCPProxy mutation surfaces. For v0.69.0, Sidekick may use the supported config validate/apply path or the upstream_servers patch path with oauth_json. Direct production-file edits are not a normal mutation path.

Scope-change flow:
1. read redacted current configuration server-side;
2. build a minimal scope diff;
3. validate;
4. apply through MCPProxy;
5. verify the expected result;
6. require fresh OAuth when scopes changed.

The browser never receives the complete MCPProxy config document.

## 8. Credentials and secret handling

Generic credentials remain write-only.

Support:
- Bearer;
- X-API-Key;
- custom headers;
- supported environment variables;
- conversion of existing config values to MCPProxy secret/keyring references.

Masked values from MCPProxy must never be persisted back as literal credentials. All writes use minimal-diff semantics.

## 9. Real-time state

Replace unconditional 15-second full polling with:
1. MCPProxy SSE /events as primary invalidation source;
2. targeted state refetch;
3. exponential reconnect with jitter;
4. polling fallback;
5. reduced refresh while the page is hidden.
## 10. Backend decomposition

Keep Go plus embedded vanilla JS/CSS.

Proposed Go components:
- internal/capabilities
- internal/browser
- internal/browser/providers/chromium
- internal/browser/providers/brave
- internal/connections
- internal/oauthconfig
- extensions to internal/mcpproxy
- safe DTO routes in internal/web

Do not add npm production dependencies merely for components or animations.

## 11. Sidekick API additions

The design requires server-side endpoints for:
- capability state;
- browser status/configuration;
- browser-provider change;
- normalized connection detail;
- connection mutation;
- OAuth scope preview/apply;
- safe event/SSE relay when browser boundaries require it.

Every state-changing endpoint remains protected by:
- Sidekick session;
- CSRF;
- trusted Origin/Host;
- strict JSON size/schema validation.

## 12. Accessibility and interaction

Requirements:
- semantic headings and landmarks;
- visible keyboard focus;
- keyboard-operable navigation, dialogs, forms, and chips;
- dialog focus trap and return;
- field-associated inline validation;
- aria-live for asynchronous state;
- useful touch targets;
- reduced-motion support;
- no status conveyed only by color;
- responsive narrow-screen behavior;
- no large loading layout shifts.
## 13. Security requirements

Preserve:
- constant-time admin-key comparison;
- HttpOnly, Secure, SameSite session cookie;
- CSRF checks;
- trusted Origin/Host checks;
- CSP;
- no-store;
- hardened non-root/read-only Sidekick container posture;
- no Docker socket.

Additional requirements:
- Human Auth Browser CDP is never publicly routed.
- Public browser GUI remains behind the authenticated gateway.
- Admin API key and Agent Tokens are never logged.
- Sensitive OAuth URL/callback data is redacted.
- Full MCPProxy config never reaches the frontend.
- Browser-provider switching cannot mutate MCPProxy config.
- Managed Bitwarden mode installs only the official extension source/id.
- No AI-agent endpoint grants arbitrary browser/CDP control.
- Sidekick does not silently bypass MCPProxy trust/quarantine behavior.

## 14. MCPProxy compatibility

### 14.1 Stable baseline

MCPProxy v0.69.0 is the compatibility floor.

Verified against the production-compatible MCPProxy v0.69.0 environment:
- /api/v1/servers;
- /api/v1/profiles;
- /api/v1/tokens;
- server login/logout;
- supported server PATCH;
- enable/disable/restart/quarantine;
- tool approval APIs;
- config read/validate/apply;
- profile-pinned Agent Tokens.

### 14.2 Future capabilities

When present, Sidekick enables Profile v3, /clients, /attention, /access/explain, effective-tool inspection, and richer canonical statuses/actions. Their absence must not produce errors.
## 15. Testing strategy

All product-code changes use TDD.

Required Go tests:
- browser provider registry;
- Chromium/Brave CLI mapping;
- capability detection;
- safe normalized DTOs;
- scope mutation validation;
- redaction;
- provider rollback;
- MCPProxy version compatibility.

Required web/handler tests:
- login asynchronous-state contract;
- browser status/provider mutation;
- capability-gated APIs;
- unauthorized, CSRF, and Origin failures;
- scope preview/apply;
- no secret leakage.

Browser smoke/E2E:
- login progress/success/error;
- Connections navigation/detail;
- OAuth-start state;
- accessibility-critical flows.

Browser-provider integration:
- Chromium health/CDP;
- Brave health/CDP;
- disposable page opening;
- profile persistence across recreation where practical.

Full verification:
- go test -race ./...
- go vet ./...
- JS syntax/tests
- browser smoke
- privacy scan
- govulncheck
- gitleaks
- container build
- Trivy HIGH/CRITICAL gate
- MCPProxy latest-stable contract job
## 16. Git and production deployment

Implementation occurs outside main in an isolated worktree/feature branch.

Before production:
1. run full local verification;
2. create clean scoped commits;
3. push the feature branch;
4. wait for GitHub CI to be green;
5. integrate the target branch;
6. build an immutable image from the verified commit;
7. retain the current production image/config as rollback target;
8. deploy Sidekick/browser changes to CT400;
9. run health, UI, and OAuth-browser smoke tests;
10. verify MCPProxy, n8n, and agent profiles still operate;
11. remove temporary build/test resources not needed for rollback;
12. update handoff documentation.

MCPProxy itself is not upgraded merely to unlock Sidekick V2 features.

## 17. Setup / Restore

Move Setup / Restore into Settings and show:
- Sidekick version/commit;
- MCPProxy version;
- capability set;
- browser provider/health;
- persistent paths;
- endpoint health;
- restore notes;
- secret-free compose/env examples.

First-boot configuration supports browser selection.

## 18. Research basis — 2026-10-02

Primary sources reviewed:
- https://github.com/smart-mcp-proxy/mcpproxy-go
- https://github.com/smart-mcp-proxy/mcpproxy-go/releases/tag/v0.69.0
- https://docs.mcpproxy.app/api/rest-api/
- https://docs.mcpproxy.app/configuration/upstream-servers/
- https://docs.mcpproxy.app/features/agent-tokens/
- https://github.com/linuxserver/docker-brave
- https://support.brave.com/hc/en-us/articles/360017909112-How-can-I-add-extensions-to-Brave
- https://support.brave.com/hc/en-us/articles/29808985123085-Sensitive-data-storage
- https://bitwarden.com/help/browserext-deploy/
- https://bitwarden.com/help/storing-passkeys/
- https://blog.modelcontextprotocol.io/posts/2026-07-28/
- https://blog.modelcontextprotocol.io/posts/enterprise-managed-auth/

## 19. Acceptance criteria

The work is complete only when:
1. A fresh install can choose Chromium or Brave.
2. Brave passes GUI and CDP health checks.
3. Browser-provider switching has tested rollback.
4. Login visibly reports checking/success/error.
5. Upstreams, credentials, and OAuth work through one Connections UX.
6. Connection detail is useful without exposing secrets.
7. OAuth scopes can be inspected and safely changed through supported backend mutation with re-consent when required.
8. Profiles are editable to the connected MCPProxy capability level.
9. Agent onboarding creates or reuses profile-pinned credentials without later secret leakage.
10. Sidekick reacts through SSE with a safe fallback.
11. No agent gains generic access to the credentialed Human Auth Browser.
12. MCPProxy v0.69.0 remains supported.
13. Future controls appear only when capabilities exist.
14. Existing security tests remain green.
15. The local full suite is green.
16. GitHub CI is green before production deployment.
17. Production smoke checks pass after deployment.
18. Temporary audit/build/test resources are cleaned.
19. Handoff documentation records deployed commit, image, browser provider, and rollback procedure.
