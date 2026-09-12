---
title: API 索引
order: 1
category: API 参考
description: RenoP HTTP、REST 与 RPC API 概览
---

# RenoP HTTP API

RenoP 提供用于管理自动化、客户端集成与健康监控的完整 HTTP API。服务默认监听
`http://localhost:3000`。


[API 文档](/api) 提供系统管理、客户端集成与监控等接口说明。你也可以直接查阅或下载完整的原始 [OpenAPI 文件](/assets/openapi.yaml)。

## 路由结构

| 路由前缀                        | 用途                                         |
|:--------------------------------|:---------------------------------------------|
| `/api/*`                        | 认证、账号、设置、状态与消息等管理 API       |
| `/{repo}/*`                     | 按仓库引擎执行上传、下载与删除               |
| `/{npm-repo}/*`                 | npm packument、tarball、发布、发布标签与搜索 |
| `/index/*` 或 `/{repo}/index/*` | Cargo Sparse Index                           |
| `/v2/*`                         | Docker 与 OCI Distribution v2                |
| `/javadoc/*`                    | 沙箱化 Javadoc 在线预览                      |
| `/cargodoc/*`                   | 沙箱化 Cargodoc 在线预览                     |

## 传输格式与 Protobuf

基于 schema 的管理 API 使用二进制 protobuf。请求请发送 `Content-Type: application/x-protobuf`；也接受
`application/protobuf` 和 `application/octet-stream`，未指定 Content-Type 时默认采用 protobuf。JSON 请求体
会被拒绝，端点返回 `400` 或 `415` 错误。响应固定使用 `application/x-protobuf`，`Accept` 不会启用 JSON。
请使用与部署版本对应的 `proto/api/v1/api.proto` 消息定义。

控制请求上限仍为 1 MiB，并保留端点的更小限制。protobuf 消息旁的 JSON 示例仅展示解码后的字段，
不表示支持 JSON 传输。仅支持 JSON 的端点、包管理器原生协议、上传二进制部分、健康检查文本与
各端点的错误响应保留各自声明的格式。

## 认证方式

- **浏览器 Cookie**：使用 HttpOnly 的 `renop_session=<session_id>`，仅限浏览器交互，不支持通过 Header 或 URL 参数传递。
- **Bearer API Token**：在请求头中传入 `Authorization: Bearer <token>`。Token 的实际可用权限受账号自身权限约束，二者取交集。
- **包客户端 Basic Auth**：在请求头中传入 `Authorization: Basic <base64(user:password_or_token)>`。

Basic Auth 仅适用于包客户端操作，不可用于管理 API。系统不接受在 URL 查询参数或使用 `Authorization: Session` 传递凭据。

## 常用 HTTP 状态码

| 状态码                    | 含义       | 说明                           |
|:--------------------------|:-----------|:-------------------------------|
| `200 OK`                  | 成功       | 请求成功并返回响应正文         |
| `201 Created`             | 已创建     | 资源或上传任务初始化成功       |
| `204 No Content`          | 成功       | 请求成功且无响应正文           |
| `400 Bad Request`         | 请求错误   | 参数或请求正文无效             |
| `401 Unauthorized`        | 未认证     | 缺少认证信息或凭据无效         |
| `403 Forbidden`           | 无权限     | 权限不足或 IP 被临时封禁       |
| `404 Not Found`           | 未找到     | 目标资源不存在                 |
| `409 Conflict`            | 冲突       | 当前状态不允许操作或资源已存在 |
| `429 Too Many Requests`   | 请求过多   | 超出允许的请求速率             |
| `503 Service Unavailable` | 服务不可用 | 服务过载或依赖暂时不可用       |

## API 参考目录

- [认证 API](./authentication.md)
- [API Token 与用户](./tokens.md)
- [Maven API](./maven.md)
- [Cargo API](./cargo.md)
- [Docker / OCI API](./docker.md)
- [npm 存储库 API](./npm.md)
- [超级团队 API](./global-teams.md)
- [发布配额 API](./publication-quotas.md)
- [审核 API](./reviews.md)
- [消息中心 API](./messages.md)
- [存储与上传 API](./storage.md)
- [设置 API](./settings.md)
- [状态与遥测 API](./status.md)
- [GPG 加密 API](./gpg.md)
- [速率限制](./rate-limit.md)
- [更新 API](./updater.md)
