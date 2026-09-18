# Security Policy

MCPProxy Sidekick is an administrative control plane. A user who can successfully authenticate to Sidekick can change MCPProxy upstream credentials and manage scoped Agent Tokens.

## Supported versions

Security fixes are made on the latest release line. Upgrade to the newest published container image before reporting an issue that has already been fixed upstream.

## Threat model

Sidekick is designed to run behind HTTPS and an authenticated/trusted reverse proxy boundary. It is not intended to be exposed as a naked HTTP service on the public internet.

The application assumes:

- MCPProxy itself is trusted;
- the mounted MCPProxy admin key file is readable only by the Sidekick container;
- the reverse proxy prevents direct access to the OAuth browser;
- the host/container runtime is not already compromised.

## Secret handling

- Full credentials are accepted only on explicit credential-update requests.
- Sidekick does not return submitted full credentials in later API responses.
- Sidekick stores only a masked preview and SHA-256 fingerprint in its SQLite metadata database.
- Agent Token secrets returned by MCPProxy are displayed once and are not persisted by Sidekick.
- Request bodies for credential endpoints must never be logged.
- OAuth codes, access tokens, refresh tokens, cookies and Authorization headers must never be logged.

## Browser protections

- Session cookies are HttpOnly, Secure and SameSite.
- State-changing requests require a valid CSRF token.
- Host and Origin are validated for mutations.
- Responses use a restrictive Content Security Policy and `Cache-Control: no-store`.
- The OAuth browser route must be protected with Sidekick `/auth/check` or an equivalent trusted reverse-proxy policy.

## Container posture

The reference Compose runs Sidekick:

- as a non-root user;
- with a read-only root filesystem;
- with all Linux capabilities dropped;
- with `no-new-privileges`;
- without the Docker socket.

## Reporting a vulnerability

Please use GitHub private vulnerability reporting for this repository when available. Do not open a public issue containing credentials, tokens, private URLs or exploit details.

Include:

- affected Sidekick version/commit;
- MCPProxy version;
- exact reproduction steps using redacted/example credentials;
- expected and observed behavior;
- impact assessment.

Do not send real production secrets as proof.
