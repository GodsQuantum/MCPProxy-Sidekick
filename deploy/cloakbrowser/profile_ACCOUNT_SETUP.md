# profile-owner's own CloakBrowser instance (independent free license)

Separate genuine user, not a duplicate of developer's CloakBrowser account.

## Isolated runtime

- Native Manager: cloakbrowser-manager-profile-owner (same pinned upstream image)
- Independent Compose project: cloakbrowser-profile-owner
- Independent manager data and browser state: /srv/lxc/ia-studio/appdata/cloakbrowser-manager-profile-owner/
- Independent binary download/license cache in that appdata
- Reuse read-only OS Windows fonts only, never browser cookies or license keys
- Bound only to CT400 loopback 127.0.0.1:18089, not publicly exposed
- No profile-owner credentials/license in Git, logs or handoff
- Existing developer Manager, profiles, OAuth loopback and ports stay unchanged

## Activation prerequisites

CloakHQ issues Free keys via GitHub sign-in at https://cloakbrowser.dev/free.
profile-owner must authenticate with her own GitHub identity, if necessary created
with her own Hotmail email. Email alone is not a CloakHQ license credential.
Keep her license private; set through the native Manager Settings only after
an HTTPS/SSO route is set up. Her key cannot be shared with developer.

CloakHQ forbids multiple free-trial accounts for license circumvention. Two
genuine independent users may hold individual free keys, but the provider
has not explicitly guaranteed concurrent free-license instances on a single
shared host. Never present concurrent seats as proven before a live test.

After profile-owner's free GitHub key is available:
1. Provision private SSO-protected HTTPS access to her Manager.
2. Import her native Manager profile data without sharing disk browser
   directories with the original Manager or overwriting profile-owner's prior state.
3. Route only profile-owner's MCPProxy browser upstream to her own Manager/CDP.
4. Test two real concurrent launches and the HTTPS clipboard.
5. Do not silently stop either person's existing active browser session.
