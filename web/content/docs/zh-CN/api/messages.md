---
title: 消息中心 API
order: 7
category: API 参考
description: 账号通知、未读数量、工作流操作与管理员公告
---

# 消息中心 API

所有接口均要求认证。响应默认使用 protobuf，且不会缓存。API Token 需要 `messages:read`；管理员发送公告还
要求 `admin:notifications`，并且所属账号当前具有管理员权限。

## 查询或清理消息

- **查询**：`GET /api/messages?limit=30&cursor=...`
- **清理已完成消息**：`DELETE /api/messages`
- `limit` 范围为 1 至 100；`cursor` 使用上一页返回的不透明 `next_cursor`。
- 工作流操作仍为 `pending` 的消息不会被批量清理。

### 解码后的响应示例

```json
{
  "messages": [
    {
      "id": "00000000-0000-4000-8000-000000000001",
      "kind": "announcement",
      "severity": "info",
      "title": "Maintenance",
      "body": "Maintenance starts at 02:00 UTC.",
      "action_status": "",
      "created_at": 1787731200000,
      "read_at": 0
    }
  ],
  "unread_count": 1,
  "next_cursor": ""
}
```

## 查询未读数量

- **路径**：`GET /api/messages/unread-count`
- **解码后的响应**：`{"unread_count":3}`

## 标记已读或删除消息

### 单条消息

- **标记已读**：`POST /api/messages/:id/read`
- **删除**：`DELETE /api/messages/:id`
- 删除其他账号的消息返回 `404`；删除工作流仍未完成的消息返回 `409`。

### 全部消息

- **全部标记已读**：`POST /api/messages/read-all`
- 响应包含实际更新数量。

## 发送管理员公告

- **搜索接收者**：`GET /api/messages/admin/users?q=alice`，最多返回 8 个用户名。
- **发送**：`POST /api/messages/admin`
- 向全部账号发送时设置 `all: true`；否则提供精确 `recipients`。标题、正文、严重级别和接收者数量均受
  服务端限制。

```json
{
  "recipients": ["alice", "bob"],
  "all": false,
  "severity": "warning",
  "title": "Scheduled maintenance",
  "body": "The service will restart at 02:00 UTC."
}
```

工作流邀请与通知由对应业务服务触发创建。团队成员变动通知仅提示关联资源，不泄露具体操作者身份。
发布与转让工单会向有权审批的人员发送待办通知；任一审批人做出决定后，系统自动同步通知状态，并向申请人推送审批结果。

## 定向会话通知

将 `session_id` 设置为浏览器会话的公开 ID，并指定一个收件人及 `all: false`。ID 为空时仍向该账号的全部凭据开放通知。服务端会重新检查会话是否活跃且属于该用户；目标已撤销或过期时返回 `409`。

`GET /api/messages/admin/sessions?username=alice&cursor=...` 每页返回最多 100 个活跃会话，通过 `next_cursor` 读取后续页。响应只包含公开 ID、设备信息和时间，不包含会话密钥。调用者需要管理权限，API 令牌还需要 `admin:notifications`。

列表、未读数、单条和批量已读/删除操作都按当前认证会话隔离。其他会话及 API 令牌即使知道通知 ID，或请求同时携带目标 Cookie，也无法获取或修改定向通知。定向通知不会转发到邮件。前端在仅有一个收件人时以动画显示自定义下拉框，默认全量推送。

```json
{"recipients":["alice"],"all":false,"session_id":"00000000-0000-4000-8000-000000000001","severity":"info","title":"Session notice","body":"Only this browser session can read this notification."}
```
