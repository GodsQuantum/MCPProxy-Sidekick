# CloakBrowser → MCPProxy agent-browser integration

The existing manager wrapper lives in this repo so releases are reproducible.
It checks the native CDP endpoint, launches the selected profile if necessary,
and starts upstream agent-browser mcp with native environment variables:
AGENT_BROWSER_CDP, AGENT_BROWSER_SESSION, AGENT_BROWSER_NAMESPACE.

Passing only --cdp to the initial MCP server was not sufficient for all
subprocess tool calls; those calls previously returned Chrome not found.

The wrapper is synced only after tests and commit to the canonical deployment directory and to the Manager persistent binary directory.
Never commit private cookies, license keys, or other credentials.

Free Cloak permits one active browser. Never close an active human session
implicitly to switch profiles. After a Manager stop, an initialized MCP
upstream can hold a stale CDP connection. Restart the exact MCPProxy upstream
natively after deliberate profile switching; do not create a polling daemon.

Keep instance-specific endpoints and operator handoffs in private infrastructure documentation, never in a public repository.
