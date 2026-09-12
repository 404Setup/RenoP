---
title: Storage & Upload API
order: 10
category: API Reference
description: Direct repository operations and bounded resumable uploads
---

# Storage & Upload API

Direct storage routes apply to Maven, `files`, and managed native repositories. npm, Cargo, and Docker use their native
protocol APIs.
Every
mutation is checked against API-token scope, repository permission, repository format, and Maven-domain policy.

## Direct repository operations

The canonical path is `/{repo}/{path...}`. Reads support HTTP validators and byte ranges. `HIDDEN` repositories are
unlisted but exact paths remain readable; `PRIVATE` repositories require authorization.

### Download

- **Request**: `GET /{repo}/{path}` or `HEAD /{repo}/{path}`
- Missing local files may be resolved through an enabled mirror and streamed into the configured cache policy.

### Upload

- **Request**: `PUT /{repo}/{path}`
- **Auth**: Password or API token with `repository:publish`, plus current write/domain permission.
- Maven accepts only valid coordinates and metadata under a verified domain. `files` accepts sanitized arbitrary paths
  and supports replacement.

### Delete

- **Request**: `DELETE /{repo}/{path}`
- **Auth**: API token with `repository:delete` or another allowed credential, plus current delete permission.

## Chunked resumable uploads

Chunked uploads use protobuf metadata and raw binary parts. The server owns the final destination, bounds part size and
session count, and deletes abandoned temporary files.

### Initialize

- **Path**: `POST /api/upload/chunked/`
- **Content-Type**: `application/x-protobuf` / `application/json` with `ChunkedUploadInitRequest`.
- `purpose` is `storage` or `updater`. Storage `path` includes the repository name.

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

### Upload a part

- **Path**: `PUT /api/upload/chunked/{upload_id}/{index}`
- **Content-Type**: `application/octet-stream`.
- Parts may run concurrently. Retrying an already accepted index is idempotent; a part with the wrong length is
  rejected.

### Complete or abort

- **Complete**: `POST /api/upload/chunked/{upload_id}/complete`
- **Abort**: `DELETE /api/upload/chunked/{upload_id}`
- Completion is single-winner. It verifies every part, rechecks authorization, and commits through the repository gate.

```json
{
  "status": "created",
  "message": "",
  "path": "releases/com/example/app/1.0.0/app-1.0.0.jar",
  "release_id": ""
}
```

When Maven requires GPG, completion may return `202 Accepted` with a `release_id` while publication remains quarantined.
For `purpose=updater`, success returns `ready_to_restart` instead of a repository path.

## Managed native resource API

These JSON endpoints manage APK, apt, Conan, Conda/Conda native, and rpm/yum publishing resources. Mutations require a
browser cookie session. Package clients still upload to their native paths, with repository token scopes intersected
with live resource permissions.

| Operation | Endpoint                                                                | Purpose                                                                                         |
|-----------|-------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------|
| `GET`     | `/api/native/repositories/{repo}/resources`                             | List resources; `name` selects details, with `limit` 1–100 and `offset` up to 10000.            |
| `POST`    | `/api/native/repositories/{repo}/resources`                             | Reserve a hosted resource with `{"name":"example"}`; requires repository publishing permission. |
| `PUT`     | `/api/native/repositories/{repo}/resources`                             | Update `name`, `description`, and public `signing_key`; requires L3.                            |
| `DELETE`  | `/api/native/repositories/{repo}/resources`                             | Release an empty resource identified by `name`; requires L4 and no pending reviews.             |
| `PUT`     | `/api/native/repositories/{repo}/resources/members`                     | Set `name`, `username`, and `level` (0–4, or -1 to remove); preserves the last L4 owner.        |
| `GET`     | `/api/native/repositories/{repo}/resources/key?name={name}`             | Download the publisher public key; unpublished-resource visibility still applies.               |
| `GET`     | `/api/native/repositories/{repo}/resources/users?name={name}&q={query}` | Search at most eight visible usernames for the L3 permission editor.                            |

`new_packages` reviews the first publication, and `every_version` reviews each version. `202` means files remain hidden
pending signature completion or review; a review supplies `X-RenoP-Review-ID`. Native package filenames must match their
metadata. Generated indexes cannot be uploaded. APK and RPM require valid native signatures against the configured
public key. Conan requires the signing manifest generated by `scripts/conan/sign.py`. APT signs repository indexes;
Conda does not require extra detached signatures. See [repository configuration](/docs/configuration/repositories).
