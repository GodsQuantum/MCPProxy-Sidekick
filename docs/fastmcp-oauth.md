# FastMCP OAuth interoperability

Some FastMCP/OIDC-proxy based upstreams can add their own authorization-consent page in front of the provider. When that intermediate consent layer breaks an otherwise valid MCPProxy OAuth flow, FastMCP supports:

```python
require_authorization_consent="external"
```

This is **not** a generic “disable security” recommendation. In current FastMCP, `"external"` follows the same authorization path as `False`: the built-in consent screen and its transaction-binding protections are skipped. The value means the operator asserts that equivalent protections are enforced outside FastMCP.

Use it only when all of the following are true:

- the deployment has a trusted external authorization boundary;
- redirect and callback handling are constrained by that boundary;
- the change fixes a verified interoperability problem rather than hiding an unknown auth failure;
- the setting is kept in persistent deployment/config source, not patched into a running container.

Sidekick itself does not inject this setting into upstreams. It is an upstream-specific compatibility option.

Reference: FastMCP OAuth/OIDC proxy documentation for `require_authorization_consent`.
