---
title: 设置 API
order: 8
category: API 参考
description: 按域管理服务设置、存储库与索引重建
---

# 设置 API

设置接口要求管理员账号，或根据操作提供带有 `admin:settings`、`admin:repositories` 的 API Token。
`proto/api/v1/api.proto` 中定义的响应使用 protobuf。

## 查询设置域

- **路径**：`GET /api/settings/domains`
- **响应**：服务端当前支持的稳定域名，包括 `server`、`proxy`、`storage`、`updater` 与 `frontend`。

## 浏览器设置分页

设置中心将系统配置按功能划分为独立的设置域，支持按需独立拉取与更新，避免全量拉取服务配置。
在前端界面中，分类切换支持本地草稿保护，提交操作仅保存并校验当前分类的配置项。

已配置的敏感密钥（如数据库密码、S3 密钥、OAuth 客户端密钥）在界面中保持脱敏，
保存成功后自动清除前端缓存的临时输入。
在未保存状态下离开或切换页面时，界面提供防丢失提示。

GPG 密钥属于核心服务配置；全局团队配额、注册、缓存、邮件、OAuth 服务商及发布域安全策略均有独立页面。法律文档及统一 OAuth 设置使用二进制 protobuf，其他页面保留其文档规定的格式。标签与提示仍关联到控件，页面导航会将焦点移至标题。

## 读取与更新设置域

- **读取**：`GET /api/settings/domain/:name`
- **更新**：`PUT /api/settings/domain/:name`
- **行为**：请求与响应结构取决于 `:name`。未知字段和无效值会被拒绝。主机、端口、TLS、数据库及部分运行时
  参数变更可能要求重启服务。

**第三方登录**：`GET /api/settings/oauth-providers` 返回二进制 protobuf `OAuthSettings`，包含内置 GitHub 条目、脱敏客户端和预设。`PUT /api/settings/oauth-providers` 必须包含 `replace_providers: true` 及 `providers` 列表，在 128 KiB 内原子保存两组配置。支持 GitHub 加最多 12 个其他客户端。省略 GitHub 会保留其配置；关闭该条目并使用 `clear_client_secret` 清除凭据。空列表仅移除其他客户端。`revocation_secret` 为只写，同一客户端留空时保留，使用 `clear_revocation_secret` 清除。兼容端点 `GET /api/settings/github-oauth` 和 `PUT /api/settings/github-oauth` 保留 JSON，管理同一份 GitHub 配置。

[OAuth](../security/oauth-login.md)

## 存储库设置

优先使用 `/api/settings/repositories`。带 Maven 前缀的旧接口继续用于兼容。

仓库变更先提交数据库，再替换运行中的配置。删除最后一个仓库后，重启仍保留空集合；旧 YAML 仅作为首次迁移的来源。

### 查询存储库

- **路径**：`GET /api/settings/repositories`
- **兼容别名**：`GET /api/settings/maven/repositories`

### 创建、更新、删除与迁移

- **创建或更新**：`PUT /api/settings/repositories/:name`
- **删除**：`DELETE /api/settings/repositories/:name`
- **Maven/files 迁移**：`POST /api/settings/repositories/:name/migrate/:target`，`:target` 为 `maven` 或
  `files`。存储对象保持原位，切回 Maven 时重建目录。

## 重建搜索索引

- **路径**：`POST /api/settings/index/rebuild`
- **行为**：提交可合并的后台重建任务，不会并发启动重复任务。

## 发布域保留周期

`GET /api/settings/maven-domains` 和 `PUT /api/settings/maven-domains` 使用 JSON。
设置发现列表包含 `maven_domains`，默认值为：

```json
{"release_value":2,"release_unit":"year"}
```

`release_value` 是 1–100 的整数；`release_unit` 支持 `month` 或 `year`，按 UTC 日历计算。
设置数据库在 `maven_domains` 下保存相同字段。保存后仅影响新建安全锁，
不会改变已有释放日期，也不会改变主动关闭的独立 31 天保留期。
参见 [Maven 发布域状态](maven.md)。

[法律文档与 Cookie 偏好](../configuration/legal.md)

设置分页、服务提供方编辑器和关联字段使用可取消的过渡动画，并尊重减少动态效果偏好。未配置提供方或邮箱时采用统一提示样式。关闭的下拉控件不会保留选项菜单或文档监听器，打开时才按需创建。

内置资源从可执行文件流式发送。RenoP 缓存资源类型、长度和 ETag，不再在 Go 堆中额外保留每个脚本包与压缩版本的完整副本，仍支持预压缩协商和条件请求。

[安全验证](../security/captcha.md)

`capacity_limit_bytes` 是单个存储库的已安装字节上限，`0` 表示不限；界面使用 MiB。Disk 和 S3 的软件包、生成的校验文件、待审核对象及镜像缓存均计入，临时暂存副本不计入。提交前预留容量，并发上传共享同一上限。超限写入返回 `507` 和 `repository_capacity_exceeded`；有效的镜像响应仍可直接传输，但不写入缓存。上限降至已有用量以下不会删除文件或阻止读取。通过 RenoP 之外的方式修改存储后，请重建索引或重启以重新统计。旧客户端省略该可选字段时保留原上限。

法律文档合并到前端设置并与品牌设置原子保存；省略 `legal` 时保留原文档。索引控制合并到存储设置，原法律与索引兼容接口仍可使用。服务设置展示监听 IP、端口、TLS、数据库和性能选项；监听与数据库变更需重启。
