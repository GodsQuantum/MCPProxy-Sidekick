<p align="center">
  <img src="docs/assets/logo.svg" width="120" alt="Logo MCPProxy Sidekick">
</p>

<h1 align="center">MCPProxy Sidekick</h1>

<p align="center"><strong>Un petit control plane pour les credentials, OAuth, profils et accès agents de MCPProxy.</strong><br>
MCPProxy reste la passerelle ; Sidekick simplifie les opérations humaines et la reconstruction du setup.</p>

<p align="center">🇬🇧 <a href="README.md">English</a> · 🇫🇷 Français · 🇨🇳 <a href="README.zh-CN.md">简体中文</a></p>

<p align="center"><img src="docs/assets/dashboard.png" width="100%" alt="Interface MCPProxy Sidekick avec données de démonstration"></p>

Sidekick se place à côté d’une instance MCPProxy existante et centralise dans le navigateur ce qui finit sinon dispersé entre fichiers de config, terminaux et callbacks OAuth : **credentials, OAuth, profils, santé des upstreams et Agent Tokens**.

Sidekick ne remplace pas MCPProxy. MCPProxy reste la source de vérité pour le routage, la découverte des tools, l’état des upstreams et les accès scoppés.

## ✨ Pourquoi Sidekick

- **Inventaire dynamique** — un nouveau serveur ajouté à MCPProxy apparaît automatiquement.
- **État des credentials lisible** — aperçu masqué comme `abcd••••wxyz`, jamais le secret complet.
- **Credentials génériques** — Bearer, `X-API-Key` ou header personnalisé.
- **Adapters spéciaux** — Postiz, Paperless multi-identité et Immich par clé/processus.
- **OAuth visible** — le bouton ouvre réellement un onglet utilisable ; les callbacks loopback passent par le navigateur Cloud protégé.
- **Profils** — groupe les upstreams par identité, rôle ou projet.
- **Agent Tokens MCPProxy** — serveurs autorisés, permissions `read / write / destructive`, expiration et `profile_pin`.
- **Reconstructible** — Go + Docker Compose + navigateur OAuth, sans helper installé sur le poste client.
- **Surface de confiance réduite** — pas de Docker socket, pas de JavaScript CDN, conteneur non-root et rootfs read-only.

## 🚀 Installation rapide

Sidekick attend un conteneur **MCPProxy v0.67+** déjà actif, nommé `mcpproxy` par défaut.

```bash
git clone https://github.com/GodsQuantum/mcpproxy-sidekick.git
cd mcpproxy-sidekick
cp .env.example .env
mkdir -p secrets/immich
printf '%s\n' 'YOUR_MCPPROXY_ADMIN_KEY' > secrets/mcpproxy_admin_key
chmod 600 secrets/mcpproxy_admin_key
docker compose up -d
```

Routes recommandées :

- `/control/` → Sidekick ;
- `/control/oauth-browser/` (ou le chemin équivalent sous votre mount Sidekick) → Chromium/Selkies protégé par Sidekick ;
- le reste → MCPProxy.

Un exemple Caddy est fourni dans [Caddyfile.example](Caddyfile.example).

## 🔐 Credentials et OAuth

Pour les serveurs ordinaires, Sidekick sait écrire Bearer, `X-API-Key` ou un header personnalisé.

Pour les cas spéciaux :

- **Postiz** : clé intégrée à l’URL MCP ;
- **Paperless MCP** : plusieurs alias MCPProxy peuvent utiliser le même bridge avec des tokens utilisateurs distincts ;
- **ImmichMCP** : chaque identité peut avoir son fichier de clé et son processus MCP séparé.

Pour OAuth, Sidekick ouvre immédiatement un nouvel onglet. Quand MCPProxy exige un callback loopback sur la machine serveur, cet onglet affiche le Chromium exécuté dans le même namespace réseau que MCPProxy.

Aucun tunnel SSH ni daemon de callback n’est requis sur le poste client.

## 👥 Profils

Les profils sont les **Profiles natifs de MCPProxy v0.67+**. Sidekick les lit via `GET /api/v1/profiles` et modifie leurs memberships via l’API de configuration MCPProxy ; SQLite ne contient plus de second catalogue de profils. Chaque profil est accessible via `/mcp/p/<nom>`.

**Mise à niveau depuis Sidekick ≤ v0.1.8 :** les anciennes tables SQLite de profils ne sont plus utilisées au runtime et ne sont pas supprimées silencieusement. Recréez dans MCPProxy tout profil qui n’existerait encore que dans l’ancienne base avant de supprimer cette base.

## 🪪 Agent Tokens

Sidekick gère les Agent Tokens natifs de MCPProxy avec `allowed_servers`, permissions `read`, `write`, `destructive`, expiration et `profile_pin`.

Le niveau `destructive` demande une confirmation explicite. Le secret du token est affiché une seule fois et Sidekick n’en conserve pas de copie récupérable.

## 🛡️ Sécurité

- clé admin lue côté serveur depuis un fichier secret ;
- cookie de session HttpOnly + Secure + SameSite ;
- CSRF + validation Origin/Host ;
- CSP stricte ;
- secrets complets jamais renvoyés par l’API Sidekick ;
- SQLite limité aux métadonnées, previews masquées et fingerprints SHA-256 ;
- navigateur OAuth protégé par la session Sidekick ;
- aucun Docker socket ;
- runtime non-root, root filesystem read-only, capabilities supprimées, `no-new-privileges`.

Voir [SECURITY.md](SECURITY.md).

## 🐳 Architecture Docker

```text
namespace réseau du conteneur MCPProxy existant
│
├── :8080  MCPProxy
├── :8081  MCPProxy Sidekick
├── :3000  UI navigateur OAuth
└── :9222  Chromium CDP, loopback uniquement
```

## ♻️ Restauration

Sidekick n’est volontairement pas un coffre-fort. Restaurer MCPProxy, cloner Sidekick, recréer `secrets/mcpproxy_admin_key`, restaurer éventuellement le volume `/data`, puis lancer `docker compose up -d`.

MCPProxy reste la source de vérité pour les upstreams **et les Profiles** même si la base SQLite de Sidekick est perdue.

## 🧪 Développement

```bash
go test ./...
go test -race ./...
go vet ./...
node --check internal/web/assets/app.js
podman build -t mcpproxy-sidekick:dev .
```

## 🧭 Principe

> **MCPProxy possède l’état MCP. Sidekick le rend opérable.**

## 📄 Licence

[MIT](LICENSE)
