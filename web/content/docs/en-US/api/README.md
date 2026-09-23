---
title: API Index
order: 1
category: API Reference
description: RenoP HTTP RESTful and RPC API overview and endpoints
---

# RenoP HTTP API

RenoP provides a complete HTTP API for administrative automation, client integrations, and health monitoring. The server
listens on `http://localhost:3000` by default.

The [API reference](/api) documents management endpoints, client integration flows, and health monitoring. The
raw [OpenAPI specification](/assets/openapi.yaml) is also available for direct reference and client generation.

## API Route Structure

| Route Prefix                    | Purpose                                                              |
|:--------------------------------|:---------------------------------------------------------------------|
| `/api/*`                        | Management APIs (authentication, tokens, settings, status, messages) |
| `/{repo}/*`                     | Maven/files storage or format-specific package protocol              |
| `/{npm-repo}/*`                 | npm packuments, tarballs, publication, dist-tags, and search         |
| `/index/*` or `/{repo}/index/*` | Cargo Sparse Index endpoints                                         |
| `/v2/*`                         | Docker & OCI Distribution Spec v2 endpoints                          |
| `/javadoc/*`                    | Javadoc online HTML viewer                                           |
| `/cargodoc/*`                   | Cargodoc online HTML viewer                                          |

## Wire Formats & Protobuf

Schema-backed management APIs use binary protobuf. Send `Content-Type: application/x-protobuf`; requests also accept
`application/protobuf` and `application/octet-stream`, and a missing Content-Type defaults to protobuf. JSON request
bodies are rejected with endpoint-specific `400` or `415` errors. Responses always use `application/x-protobuf`;
`Accept` does not enable JSON.
Use message definitions from `proto/api/v1/api.proto` for the deployed release.

Control requests remain bounded to 1 MiB, with smaller endpoint limits retained. JSON examples accompanying protobuf
messages show decoded fields, not a JSON wire format. JSON-only endpoints, native registry protocols, raw upload parts,
health text, and endpoint-specific errors retain their declared representations.

## Authentication Transports

- **Browser cookie**: `renop_session=<session_id>`; the HttpOnly session secret is not accepted in headers or URLs.
- **Bearer API token**: `Authorization: Bearer <token>`; endpoint scopes are intersected with account permissions.
- **Basic Auth for package protocols**: `Authorization: Basic <base64(user:password_or_token)>`.

Basic credentials cannot call management APIs. Query-string credentials and `Authorization: Session` are rejected.

## HTTP Status Codes

| Code                      | Meaning      | Description                                       |
|:--------------------------|:-------------|:--------------------------------------------------|
| `200 OK`                  | Success      | Request succeeded with response body              |
| `201 Created`             | Created      | Resource or upload task successfully initialized  |
| `204 No Content`          | Success      | Request succeeded with no response body           |
| `400 Bad Request`         | Bad Request  | Invalid parameters or body format                 |
| `401 Unauthorized`        | Unauthorized | Missing or invalid authentication credentials     |
| `403 Forbidden`           | Forbidden    | Insufficient permissions or IP temporarily banned |
| `404 Not Found`           | Not Found    | Resource not found                                |
| `409 Conflict`            | Conflict     | Artifact already exists and cannot be overwritten |
| `429 Too Many Requests`   | Rate Limited | Request rate exceeds configured limits            |
| `503 Service Unavailable` | Overloaded   | Maximum active concurrent requests reached        |

## API Reference Index

- [Authentication API](./authentication.md)
- [Tokens & Users API](./tokens.md)
- [Maven Metadata API](./maven.md)
- [Cargo Registry API](./cargo.md)
- [Docker / OCI Registry API](./docker.md)
- [npm Registry API](./npm.md)
- [Global Teams API](./global-teams.md)
- [Publication Quotas API](./publication-quotas.md)
- [Review API](./reviews.md)
- [Message Center API](./messages.md)
- [Storage & Upload API](./storage.md)
- [Settings API](./settings.md)
- [Status & Telemetry API](./status.md)
- [GPG Cryptography API](./gpg.md)
- [Rate Limiting Reference](./rate-limit.md)
- [Updater API](./updater.md)
