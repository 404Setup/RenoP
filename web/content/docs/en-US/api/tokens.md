---
title: API Tokens & Users
order: 3
category: API Reference
description: Fine-grained API-token lifecycle, authentication boundaries, and administrator user endpoints
---

# API Tokens & Users

API tokens are durable machine credentials owned by one account. RenoP stores only a SHA-256 lookup digest of each
256-bit random secret. The plaintext value is returned once when the token is created or rotated and cannot be recovered
later.

Every request must pass two independent checks:

- The token must include the capability required by the endpoint.
- The owning account must still be allowed to perform that operation on the target resource.

Changing an account's role or package-team membership therefore takes effect without recreating its tokens.

## Manage your API tokens

Token-management endpoints require the `renop_session` HttpOnly browser cookie. API tokens, passwords,
`Authorization: Session`, and query-string credentials cannot manage token secrets.

### List assignable scopes

`GET /api/auth/profile/api-tokens/scopes`

The response is filtered by the current account. Administrator scopes are never offered to ordinary users.

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

### Create a token

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

`expires_at` is an optional Unix-millisecond timestamp between five minutes and five years after creation. A null or
omitted value creates a token without an expiration. Accounts may own at most 50 API tokens.

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

### List token metadata

`GET /api/auth/profile/api-tokens`

The response contains non-secret metadata and the account limit. It never contains a token secret.

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

### Edit a token

`PUT /api/auth/profile/api-tokens/{token_id}`

Update an existing token's display name, scopes, or target restrictions without modifying its secret.

```json
{
  "name": "CI publishing updated",
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "targets": {
    "repository:publish": ["releases"]
  }
}
```

The endpoint returns the updated token metadata:

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

### Rotate a token

`POST /api/auth/profile/api-tokens/{token_id}/rotate`

Regenerate the secret for a token while preserving its name, scopes, and target restrictions. The previous secret stops
working immediately.

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

### Update token state

`PUT /api/auth/profile/api-tokens/{token_id}/state`

Temporarily disable or re-enable an API token without revoking it.

```json
{
  "disabled": true
}
```

The endpoint returns the updated state confirmation:

```json
{
  "disabled": true
}
```

### Revoke a token

`DELETE /api/auth/profile/api-tokens/{token_id}`

Successful revocation returns HTTP 204 No Content and invalidates cached authentication immediately.

## Manage active sessions and IP bans

Review active browser sessions, Basic Auth requests, and API Token accesses. Track recent IP addresses and block
untrusted clients directly.

### List active sessions

`GET /api/auth/profile/sessions`

Returns active sessions coalesced by device. Each session includes up to ten recent IP addresses.

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

### Revoke active sessions

`DELETE /api/auth/profile/sessions/{session_id}`

Revoke an individual session by its identifier. To revoke all other sessions while keeping the current device active:

`POST /api/auth/profile/sessions/revoke-others`

Both endpoints return HTTP 200 with StatusOk on success.

### Manage blocked IP addresses

Account-level IP blocks prevent specific IP addresses from logging into the account.

List currently blocked IP addresses:

`GET /api/auth/profile/ip-bans`

```json
{
  "ips": ["203.0.113.195"]
}
```

Block a new IP address:

`POST /api/auth/profile/ip-bans`

```json
{
  "ip": "203.0.113.195"
}
```

Unblock an IP address:

`DELETE /api/auth/profile/ip-bans/{ip}`

## Scope reference

| Scope                 | Capability                                                                     |
|:----------------------|:-------------------------------------------------------------------------------|
| `repository:read`     | Read repository catalogs, metadata, files, images, and versions                |
| `repository:publish`  | Publish through Maven, npm, Cargo, Docker, files, or chunked-upload protocols  |
| `repository:delete`   | Delete repository files, package versions, tags, or images                     |
| `package:create`      | Reserve new npm/Cargo packages or Docker images after repository authorization |
| `package:metadata`    | Update package descriptions and other package metadata                         |
| `package:lifecycle`   | Archive, restore, yank, or unyank packages and versions                        |
| `team:manage`         | View and administer npm, Cargo, Docker, and Maven-domain teams and invitations |
| `domain:read`         | Read private Maven publishing-domain configuration                             |
| `domain:create`       | Create Maven publishing domains                                                |
| `domain:verify`       | Request or force Maven-domain ownership verification                           |
| `domain:lifecycle`    | Close or reclaim Maven publishing domains                                      |
| `messages:read`       | Read, mark, and remove the account's message-center entries                    |
| `account:read`        | Read private account data and personal audit history                           |
| `account:write`       | Update the account's public profile through the API                            |
| `statistics:read`     | Query download statistics available to the account                             |
| `admin:users`         | Administer user accounts and their login devices                               |
| `admin:repositories`  | Administer repositories and rebuild repository indexes                         |
| `admin:settings`      | Administer system settings and diagnostics                                     |
| `admin:audit`         | Read or clear administrator-visible audit and status data                      |
| `admin:notifications` | Compose administrator notifications                                            |
| `admin:updates`       | Check, upload, install, and restart system updates                             |
| `admin:statistics`    | Query system-wide download statistics                                          |

The `admin:*` scopes can be created only by an administrator and stop authorizing administrator operations as soon as
the owning account loses that role.

## Use a token

Use a bare token as a Bearer credential for scoped API automation:

```http
Authorization: Bearer rnp_pat_REDACTED
```

Package clients must use Basic Auth with an API Token as the password. User login passwords are not accepted for Basic
Auth. Permissions strictly follow the scopes configured on the token.

```http
Authorization: Basic YWxpY2U6cm5wX3BhdF9SRURBQ1RFRF9UT0tFTg==
```

An npm client sends the token through `_authToken` or Basic authentication. Cargo sends it as an Authorization header.
Docker exchanges credentials at `GET /v2/token` for short-lived access.

## Compatible endpoints

Administrator user operations are available at `GET /api/tokens`. The legacy endpoint `POST /api/auth/profile/token`
remains available for backward compatibility. New integrations should use the fine-grained profile endpoints.
