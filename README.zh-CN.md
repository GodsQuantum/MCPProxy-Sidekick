<p align="center">
  <img src="docs/assets/logo.svg" width="120" alt="MCPProxy Sidekick 标志">
</p>

<h1 align="center">MCPProxy Sidekick</h1>

<p align="center"><strong>用于 MCPProxy 凭据、OAuth、配置档案和 Agent Token 的轻量控制平面。</strong><br>
让 MCPProxy 继续作为唯一网关，同时让日常管理和重建更简单。</p>

<p align="center">🇬🇧 <a href="README.md">English</a> · 🇫🇷 <a href="README.fr.md">Français</a> · 🇨🇳 简体中文</p>

<p align="center"><img src="docs/assets/dashboard.png" width="100%" alt="使用通用演示数据的 MCPProxy Sidekick 界面"></p>

Sidekick 部署在现有 MCPProxy 旁边，把原本分散在配置文件、终端和 OAuth 回调中的操作集中到一个网页界面：**凭据、OAuth、配置档案、上游状态和 Agent Token**。

Sidekick 不替代 MCPProxy。MCPProxy 仍然负责路由、工具发现、上游状态和访问范围。

## ✨ 为什么使用 Sidekick

- **动态上游发现** — MCPProxy 新增服务器后，Sidekick 自动显示。
- **清晰的凭据状态** — 仅显示类似 `abcd••••wxyz` 的掩码预览，不返回完整密钥。
- **通用凭据编辑** — 支持 Bearer、`X-API-Key` 和自定义 Header。
- **特殊适配器** — 支持 Postiz URL 密钥、Paperless 多身份和 Immich 独立密钥文件。
- **可见的 OAuth 流程** — 点击后立即打开可操作的新标签页；loopback 回调通过受保护的云端浏览器完成。
- **Profiles** — 按身份、角色或项目组织上游。
- **MCPProxy Agent Tokens** — 可限制服务器、`read / write / destructive` 权限、有效期和 `profile_pin`。
- **可重建** — Go + Docker Compose + OAuth Browser，不依赖某一台客户端电脑。
- **更小的信任面** — 无 Docker socket、无 CDN JavaScript、非 root、只读根文件系统。

## 🚀 快速开始

Sidekick 需要一个已经运行的 MCPProxy 容器，默认名称为 `mcpproxy`。

```bash
git clone https://github.com/GodsQuantum/mcpproxy-sidekick.git
cd mcpproxy-sidekick
cp .env.example .env
mkdir -p secrets/immich
printf '%s\n' 'YOUR_MCPPROXY_ADMIN_KEY' > secrets/mcpproxy_admin_key
chmod 600 secrets/mcpproxy_admin_key
docker compose up -d
```

推荐反向代理：

- `/control/` → Sidekick；
- `/oauth-browser/` → 由 Sidekick 会话保护的 Chromium/Selkies；
- 其他路径 → MCPProxy。

仓库中提供 [`Caddyfile.example`](Caddyfile.example)。

## 🔐 凭据与 OAuth

普通服务器可使用 Bearer、`X-API-Key` 或自定义 Header。

特殊情况：

- **Postiz** — 密钥位于 MCP URL 中；
- **Paperless MCP** — 多个 MCPProxy 别名可指向同一个 bridge，并使用不同用户 Token；
- **ImmichMCP** — 每个身份可以使用独立 API Key 文件和独立 MCP 进程。

当 MCPProxy 要求服务器本机 loopback 回调时，Sidekick 会在服务器网络环境中打开 Chromium，并把它显示在你的浏览器标签页里。客户端无需 SSH 隧道或本地回调程序。

## 👥 Profiles

Profiles 用于按身份或角色组织上游。真正的 Agent 隔离由 MCPProxy Agent Tokens 强制执行。

## 🪪 Agent Tokens

Sidekick 支持 MCPProxy 原生 Agent Tokens：`allowed_servers`、`read / write / destructive`、有效期以及 `profile_pin`。

授予 `destructive` 权限时必须显式确认。Token 完整值只显示一次，Sidekick 不保存可恢复副本。

## 🛡️ 安全模型

- Admin Key 仅由服务器端 Secret 文件读取；
- HttpOnly + Secure + SameSite 会话 Cookie；
- CSRF 与 Origin/Host 校验；
- 严格 CSP；
- Sidekick API 不返回完整凭据；
- SQLite 只保存元数据、掩码预览和 SHA-256 指纹；
- OAuth Browser 必须通过 Sidekick 会话验证；
- 不挂载 Docker socket；
- 非 root、只读根文件系统、删除 Linux capabilities、启用 `no-new-privileges`。

详见 [SECURITY.md](SECURITY.md)。

## ♻️ 恢复

Sidekick 故意不充当密码保险库。恢复 MCPProxy、克隆 Sidekick、重建 `secrets/mcpproxy_admin_key`，必要时恢复 `/data`，然后运行 `docker compose up -d`。

即使 Sidekick 的 SQLite 数据丢失，MCPProxy 仍然是上游配置的权威来源。

## 🧪 开发

```bash
go test ./...
go test -race ./...
go vet ./...
node --check internal/web/assets/app.js
podman build -t mcpproxy-sidekick:dev .
```

## 🧭 设计原则

> **MCPProxy 管理 MCP 状态，Sidekick 让它更易于操作。**

## 📄 许可证

[MIT](LICENSE)
