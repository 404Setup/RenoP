---
title: 存储与上传 API
order: 10
category: API 参考
description: 仓库直接操作与有界可恢复分块上传
---

# 存储与上传 API

直接存储接口用于 Maven、`files` 及受管理的原生仓库。npm、Cargo 和 Docker 使用各自原生协议。所有修改操作都会同时检查
API Token 权限、仓库权限、仓库引擎及 Maven 域策略。

## 仓库直接操作

标准路径为 `/{repo}/{path...}`。读取支持 HTTP 条件请求与字节范围。`HIDDEN` 不参与列表发现，但精确路径
仍可读取。`PRIVATE` 要求授权。

### 下载

- **请求**：`GET /{repo}/{path}` 或 `HEAD /{repo}/{path}`
- 本地缺失文件可通过已启用镜像解析，并按照配置的缓存策略写入本地。

### 上传

- **请求**：`PUT /{repo}/{path}`
- **认证**：密码，或带有 `repository:publish` 的 API Token，并要求当前账号具有写入/域权限。
- Maven 仅接受已验证域下的有效坐标与元数据。`files` 接受清理后的任意路径并支持覆盖。

### 删除

- **请求**：`DELETE /{repo}/{path}`
- **认证**：带有 `repository:delete` 的 API Token 或其他允许的凭据，并要求当前删除权限。

## 可恢复分块上传

元数据使用 protobuf，分块使用原始二进制。服务端控制最终路径，限制分块大小和会话数量，并清理废弃的
临时文件。

### 初始化

- **路径**：`POST /api/upload/chunked/`
- **Content-Type**：`application/x-protobuf` / `application/json`，正文为 `ChunkedUploadInitRequest`。
- `purpose` 为 `storage` 或 `updater`。storage 的 `path` 以仓库名称开头。

```json
{
  "purpose": "storage",
  "filename": "app-1.0.0.jar",
  "size": 524288000,
  "path": "releases/com/example/app/1.0.0/app-1.0.0.jar",
  "generate_checksums": true,
  "chunk_size": 4194304,
  "gpg_signature_expected": false
}
```

### 上传分块

- **路径**：`PUT /api/upload/chunked/{upload_id}/{index}`
- **Content-Type**：`application/octet-stream`。
- 分块可并发上传。重试已接收的 index 是幂等操作，长度不符的分块会被拒绝。

### 完成或中止

- **完成**：`POST /api/upload/chunked/{upload_id}/complete`
- **中止**：`DELETE /api/upload/chunked/{upload_id}`
- 完成操作只允许一个调用成功，会重新检查全部分块与权限，并通过仓库门控提交。

```json
{
  "status": "created",
  "message": "",
  "path": "releases/com/example/app/1.0.0/app-1.0.0.jar",
  "release_id": ""
}
```

Maven 强制 GPG 时，隔离阶段可返回带 `release_id` 的 `202 Accepted`。`purpose=updater` 成功时返回
`ready_to_restart`，而不是仓库路径。

## 受管理的原生资源 API

这些 JSON 接口管理 APK、apt、Conan、Conda/Conda native 及 rpm/yum 发布资源。修改必须使用浏览器 Cookie 会话。包客户端仍上传到原生路径，Token
的仓库权限与实时资源权限取交集。

| 操作     | 接口                                                                    | 用途                                                                             |
|----------|-------------------------------------------------------------------------|----------------------------------------------------------------------------------|
| `GET`    | `/api/native/repositories/{repo}/resources`                             | 列出资源，`name` 选择详情，`limit` 为 1–100，`offset` 最大 10000。               |
| `POST`   | `/api/native/repositories/{repo}/resources`                             | 用 `{"name":"example"}` 登记托管资源，要求仓库发布权限。                         |
| `PUT`    | `/api/native/repositories/{repo}/resources`                             | 更新 `name`、`description` 和公开的 `signing_key`，要求 L3。                     |
| `DELETE` | `/api/native/repositories/{repo}/resources`                             | 按 `name` 注销空资源，要求 L4 且没有待处理审核。                                 |
| `PUT`    | `/api/native/repositories/{repo}/resources/members`                     | 提交 `name`、`username` 和 `level`（0–4，-1 表示移除），保留最后一名 L4 所有者。 |
| `GET`    | `/api/native/repositories/{repo}/resources/key?name={name}`             | 下载发布者公钥，仍检查未发布资源的可见性。                                       |
| `GET`    | `/api/native/repositories/{repo}/resources/users?name={name}&q={query}` | 供 L3 权限编辑器搜索最多八个可见用户名。                                         |

`new_packages` 审核首次发布，`every_version` 审核每个版本。`202` 表示文件仍因签名未收齐或等待审核而隐藏。审核会返回
`X-RenoP-Review-ID`。原生包文件名须匹配元数据，不能上传自动生成的索引。APK、RPM 必须通过已配置公钥的原生签名验证。Conan 必须提供
`scripts/conan/sign.py` 生成的签名清单。APT 签名仓库索引，Conda
不要求额外分离签名。详见[仓库配置](/docs/configuration/repositories)。
