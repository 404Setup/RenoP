---
title: API Token 与用户
order: 3
category: API 参考
description: 细粒度 API Token 生命周期、认证边界与管理员用户接口
---

# API Token 与用户

API Token 是用于自动化任务与客户端的长期凭证。系统仅存储密钥的 SHA-256 安全哈希，明文仅在创建或轮换时展示一次，后续无法找回。

每个请求都必须通过两项独立验证：

- Token 具备目标接口所要求的权限范围（Scope）。
- 签发该 Token 的账号当前对目标资源具备对应操作权限。

当账号角色或所属团队权限变更时，鉴权判定实时生效，无需重新生成 Token。

## 管理自己的 API Token

Token 管理接口仅接受 HttpOnly `renop_session` 浏览器 Cookie。API Token、密码、`Authorization: Session` 与 URL
查询参数均不可管理凭证密钥。

### 查询可分配权限

`GET /api/auth/profile/api-tokens/scopes`

响应按照账号当前权限过滤，普通账号不会获得管理员权限项。

```json
{
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "target_kinds": {
    "repository:read": "repository",
    "repository:publish": "repository",
    "package:metadata": "package"
  },
  "target_limit": 128
}
```

### 创建 Token

`POST /api/auth/profile/api-tokens`

```json
{
  "name": "CI publishing",
  "scopes": ["repository:read", "repository:publish"],
  "targets": {
    "repository:publish": ["releases"]
  },
  "expires_at": 1798761600000
}
```

`expires_at` 为可选 Unix 毫秒时间戳，范围为创建后 5 分钟至 5 年。省略或传入 null 表示 Token 永不过期。每个账号最多可创建 50
个 API Token。

```json
{
  "token": {
    "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
    "name": "CI publishing",
    "scopes": ["repository:publish", "repository:read"],
    "targets": {"repository:publish": ["releases"]},
    "created_at": 1787731200000,
    "expires_at": 1798761600000
  },
  "secret": "rnp_pat_EXAMPLE_REDACTED_COPY_THE_REAL_VALUE_ONCE"
}
```

### 查询 Token 元数据

`GET /api/auth/profile/api-tokens`

响应包含非敏感元数据与账号配额上限，不包含 Token 密钥明文。

```json
{
  "tokens": [
    {
      "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
      "name": "CI publishing",
      "scopes": ["repository:publish", "repository:read"],
      "targets": {"repository:publish": ["releases"]},
      "created_at": 1787731200000,
      "expires_at": 1798761600000,
      "disabled": false
    }
  ],
  "limit": 50
}
```

### 编辑 Token

`PUT /api/auth/profile/api-tokens/{token_id}`

更新已有 Token 的名称、权限范围或精确目标限制，该操作不会改变 Token 密钥。

```json
{
  "name": "CI publishing updated",
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "targets": {
    "repository:publish": ["releases"]
  }
}
```

接口返回更新后的 Token 元数据：

```json
{
  "token": {
    "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
    "name": "CI publishing updated",
    "scopes": ["package:metadata", "repository:publish", "repository:read"],
    "targets": {"repository:publish": ["releases"]},
    "created_at": 1787731200000,
    "expires_at": 1798761600000
  }
}
```

### 轮换 Token

`POST /api/auth/profile/api-tokens/{token_id}/rotate`

重新生成指定 Token 的密钥，保留其名称、权限范围与目标限制。原密钥立即失效。

```json
{
  "token": {
    "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
    "name": "CI publishing updated",
    "scopes": ["package:metadata", "repository:publish", "repository:read"],
    "targets": {"repository:publish": ["releases"]},
    "created_at": 1787731200000,
    "expires_at": 1798761600000
  },
  "secret": "rnp_pat_NEW_REGENERATED_SECRET_VALUE_COPY_ONCE"
}
```

### 更新 Token 状态

`PUT /api/auth/profile/api-tokens/{token_id}/state`

临时停用或重新启用指定的 API Token，无需将其撤销。

```json
{
  "disabled": true
}
```

接口返回更新后的状态确认：

```json
{
  "disabled": true
}
```

### 撤销 Token

`DELETE /api/auth/profile/api-tokens/{token_id}`

撤销成功返回 HTTP 204 No Content，并立即清除相关认证缓存。

## 管理活跃会话与 IP 黑名单

查看当前活跃的浏览器会话、Basic Auth 访问及 API Token 请求。查看最近访问 IP 并手动拉黑不受信任的客户端。

### 查看活跃会话

`GET /api/auth/profile/sessions`

返回按设备合并的活跃会话列表，每个会话最多记录最近 10 个访问 IP。

```json
{
  "sessions": [
    {
      "public_id": "a1b2c3d4",
      "username": "alice",
      "ip": "192.168.1.100",
      "user_agent": "Mozilla/5.0 (Windows NT 10.0 Win64 x64)",
      "created_at": 1787731200000,
      "last_active": 1787734800000,
      "expires_at": 1788940800000,
      "current": true,
      "login_method": "password+totp",
      "recent_ips": ["192.168.1.100", "192.168.1.101"]
    }
  ]
}
```

### 撤销活跃会话

`DELETE /api/auth/profile/sessions/{session_id}`

撤销指定的单个活跃会话。如需保留当前设备并撤销其他所有会话，可调用：

`POST /api/auth/profile/sessions/revoke-others`

两个接口成功时均返回 HTTP 200 及 StatusOk。

### 管理拉黑 IP 地址

账号级 IP 黑名单可阻止特定 IP 地址继续登录当前账号。

查询当前已拉黑的 IP 列表：

`GET /api/auth/profile/ip-bans`

```json
{
  "ips": ["203.0.113.195"]
}
```

拉黑指定 IP 地址：

`POST /api/auth/profile/ip-bans`

```json
{
  "ip": "203.0.113.195"
}
```

解除对指定 IP 的拉黑：

`DELETE /api/auth/profile/ip-bans/{ip}`

## 权限参考

| Scope                 | 能力                                                 |
|:----------------------|:-----------------------------------------------------|
| `repository:read`     | 读取存储库目录、元数据、文件、镜像与版本             |
| `repository:publish`  | 通过 Maven、npm、Cargo、Docker、files 或分块上传发布 |
| `repository:delete`   | 删除存储库文件、版本、标签或镜像                     |
| `package:create`      | 通过存储库授权后预留 npm/Cargo 软件包或 Docker 镜像  |
| `package:metadata`    | 更新包描述及其他元数据                               |
| `package:lifecycle`   | 归档、恢复、yank 或 unyank 包与版本                  |
| `team:manage`         | 查看和管理 npm、Cargo、Docker 与 Maven 域团队及邀请  |
| `domain:read`         | 读取 Maven 域私有配置                                |
| `domain:create`       | 创建 Maven 发布域                                    |
| `domain:verify`       | 验证或强制验证 Maven 发布域                          |
| `domain:lifecycle`    | 关闭或重新申请 Maven 发布域                          |
| `messages:read`       | 读取、标记和删除账号消息                             |
| `account:read`        | 读取账号私有数据与个人行为日志                       |
| `account:write`       | 通过 API 更新公开个人资料                            |
| `statistics:read`     | 查询账号有权查看的下载统计                           |
| `admin:users`         | 管理用户账号及登录设备                               |
| `admin:repositories`  | 管理存储库与重建索引                                 |
| `admin:settings`      | 管理系统设置与诊断                                   |
| `admin:audit`         | 读取或清理管理员行为与状态数据                       |
| `admin:notifications` | 编写管理员通知                                       |
| `admin:updates`       | 检查、上传、安装更新及重启系统                       |
| `admin:statistics`    | 查询系统级下载统计                                   |

管理员权限项 `admin:*` 仅可由管理员创建，当账号失去管理员角色时权限立即失效。

## 使用 Token

在调用管理 API 时，将 Token 作为 Bearer 凭证传入：

```http
Authorization: Bearer rnp_pat_REDACTED
```

包客户端使用 Basic Auth 时必须使用 API Token 作为密码，不允许使用账号登录密码。权限严格跟随该 Token 所配置的范围。

```http
Authorization: Basic YWxpY2U6cm5wX3BhdF9SRURBQ1RFRF9UT0tFTg==
```

npm 客户端可通过 `_authToken` 或 Basic Auth 发送 Token。Cargo 将其作为 Authorization 头发送。Docker 在 `GET /v2/token`
换取短期凭证。

## 兼容接口

管理员用户接口位于 `GET /api/tokens`。旧版端点 `POST /api/auth/profile/token` 仍保持兼容，新集成项目应使用细粒度的个人资料接口。
