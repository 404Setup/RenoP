---
title: Message Center API
order: 7
category: API Reference
description: Account notifications, unread counts, workflow actions, and administrator announcements
---

# Message Center API

Message routes require authentication. Responses use protobuf by default and are never cached. An API token needs
`messages:read`; administrator composition additionally needs `admin:notifications` and the account's manager role.

## List or clear messages

- **List**: `GET /api/messages?limit=30&cursor=...`
- **Clear resolved messages**: `DELETE /api/messages`
- `limit` is 1-100. `cursor` is the opaque `next_cursor` from the preceding page.
- Clearing does not delete a message whose workflow action is still `pending`.

### Decoded response example

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

## Read the unread count

- **Path**: `GET /api/messages/unread-count`
- **Decoded response**: `{"unread_count":3}`

## Mark or delete messages

### One message

- **Mark read**: `POST /api/messages/:id/read`
- **Delete**: `DELETE /api/messages/:id`
- Deleting another account's message returns `404`. Deleting a pending workflow message returns `409`.

### Every message

- **Mark all read**: `POST /api/messages/read-all`
- The response includes the number of rows updated.

## Send an administrator announcement

- **Find recipients**: `GET /api/messages/admin/users?q=alice` returns at most eight names.
- **Send**: `POST /api/messages/admin`
- Set `all` to `true` for every account, or provide one or more exact `recipients`. Title, body, severity, and recipient
  counts are bounded by the server.

```json
{
  "recipients": ["alice", "bob"],
  "all": false,
  "severity": "warning",
  "title": "Scheduled maintenance",
  "body": "The service will restart at 02:00 UTC."
}
```

Workflow invitations and system results are created by their owning services. Package-team removal messages identify
the repository and package or Maven domain, but deliberately do not disclose which member performed the removal.
Review tasks send deduplicated `review_pending` messages to eligible reviewers. The first decision removes all remaining
reviewer copies and sends one localized `review_result` to the requester without the reviewer’s identity.

## Session-targeted notifications

Set `session_id` to a public browser-session ID and provide exactly one recipient with `all: false`. An empty ID preserves delivery to all of that account's credentials. The server rechecks that the session is still active and belongs to the recipient; a revoked or expired target returns `409`.

`GET /api/messages/admin/sessions?username=alice&cursor=...` lists active sessions in pages of at most 100. Follow `next_cursor` to obtain the remaining sessions. The response contains public IDs, device information and timestamps, never session secrets. Manager authority and `admin:notifications` are required for API-token callers.

Lists, unread counts, individual and bulk read/delete operations all use the authenticated session's scope. Other sessions and API tokens cannot retrieve or modify a targeted notification, even when its ID is known or an API request also carries the target cookie. Targeted notifications are excluded from email delivery. The browser composer shows an animated custom selector for a single recipient and defaults to all sessions.

```json
{"recipients":["alice"],"all":false,"session_id":"00000000-0000-4000-8000-000000000001","severity":"info","title":"Session notice","body":"Only this browser session can read this notification."}
```
