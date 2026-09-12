---
title: 配置概览
order: 1
category: 配置
description: 配置文件、服务设置、存储、代理、品牌与更新策略
---

# 配置概览

RenoP 将系统设置保存在私有 SQLite 数据库 `renop-settings.db`，通过 `RENOP_SETTINGS_DB` 指定路径。请在管理员设置页面调整监听
IP、端口和应用数据库连接；监听配置变更需重启。法律文档合并到前端设置，索引控制合并到存储设置。下文示例仅描述保存的字段，不要创建或编辑
YAML 配置文件。

设置导航合并为外观与政策、账户与登录、发布管理、存储与缓存、服务与网络以及邮件分类；各子页仍独立保留草稿和保存。

仅首次启动时导入已有的 `config.yaml`（或 `RENOP_CONFIG` 指定的文件），提交后归档。之后只使用数据库快照。请同时备份设置数据库和应用数据库；加密密钥保存在设置数据库中。

## 配置文件

| 文件                | 覆盖变量            | 用途                                             |
|:--------------------|:--------------------|:-------------------------------------------------|
| `renop-settings.db` | `RENOP_SETTINGS_DB` | 服务、数据库、文档预览、代理、前端、审计与更新器 |
| 数据库              | 数据库 DSN          | 仓库引擎、可见性、镜像、Maven 策略与 S3          |
| `index.json`        | `RENOP_INDEX`       | 持久化文件索引快照，必要时可从存储重建           |

账号、API Token、会话、团队、行为日志与消息均存储在数据库中，不通过 YAML 配置。配置文件和数据库备份可能
包含凭据，应只允许服务账号读取。

仓库定义包含在数据库备份中。只有数据库尚无仓库配置时才导入旧 YAML，已有配置不会被覆盖。
如需回退到旧版 RenoP，请另外保留迁移归档。

## 保存的设置字段

### 存储与文档预览

```yaml
storage_path: "storage"
enable_javadoc_preview: true
javadoc_extract_path: ""
max_javadoc_size_mb: 48
enable_cargodoc_preview: true
cargodoc_extract_path: ""
max_cargodoc_size_mb: 128
```

提取路径留空时使用平台缓存目录。通过 `/javadoc` 或 `/cargodoc` 暴露内容前，会校验归档路径与大小限制。

### `server` 网络与安全

```yaml
server:
  host: "0.0.0.0"
  port: 3000
  ssl_enabled: false
  ssl_cert_path: ""
  ssl_key_path: ""
  domains: ["localhost"]
  cors_origins: []
  enable_compression: false
  file_cache_size_mb: 16
  max_active_requests: 512
  trusted_proxies: []
  cdn_ip_header: "X-Forwarded-For"
  debug_mode: false
  gpg:
    key_servers: ["https://keys.openpgp.org", "https://keyserver.ubuntu.com"]
```

`domains` 提供公开主机名与默认 CORS 主机。`cors_origins` 可增加精确 Origin、主机或通配主机，`*` 表示允许
全部 Origin。只有直接连接来源匹配 `trusted_proxies` 时才信任转发客户端 IP 请求头。主机、端口、TLS、压缩、
调试模式及部分缓存设置变更要求重启。

GitHub OAuth 同样存储在 `server.github_oauth` 下；应通过界面配置 Client ID 与只写 Secret。

其他 [OAuth 服务](../security/oauth-login.md)通过 `server.oauth_providers` 配置。后台编辑器提供官方预设，保存但不回传凭据，并立即将新配置应用于新的授权。

### `database` 数据库连接

```yaml
database:
  driver: "sqlite3"
  dsn: "renop.db"
  max_open_conns: 25
  max_idle_conns: 25
  conn_max_lifetime_sec: 300
```

支持 `sqlite3`（或 `sqlite`）、`mysql`、`postgres` 与原生 `clickhouse`。详见[数据库配置](./database.md)。

### `proxy` 出站路由

```yaml
proxy:
  selected: ""
  proxies:
    - name: "corp_proxy"
      url: "http://proxy.internal:8080"
      username: ""
      password: ""
```

最多配置 16 个 HTTP、HTTPS 或 SOCKS5 代理。详见[出站代理配置](./outbound-proxy.md)。

### `frontend` 品牌设置

```yaml
frontend:
  id: "renop"
  title: "RenoP Package Registry"
  description: "Self-hosted package repository"
  organization_website: ""
  organization_logo: "/svg/logo.svg"
  background_url: ""
  font_preset: "system"
  font_url: ""
  icp_license: ""
  public_security_filing: ""
```

品牌 URL 使用前会被校验。背景图必须满足 WebP 格式与大小策略。
`font_preset` 支持 `system`、`inter`、`noto_sans`、`open_sans`、`source_sans` 和 `custom`。预设使用本机
已安装字体；自定义值可使用 WOFF2、WOFF、TTF 文件直链或 Google Fonts CSS URL。资源在后台加载，主要字体
完整可用后才会启用，因此不会阻塞首次渲染。

### `updater` 更新策略

```yaml
updater:
  channel: "release"
  mode: "manual"
```

`channel` 为 `release` 或 `nightly`；`mode` 为 `manual`、`auto_check` 或 `auto_install`。自动检查由进程级调度器
合并执行，结果通过消息中心发送给管理员。

[法律文档与 Cookie 偏好](./legal.md)
