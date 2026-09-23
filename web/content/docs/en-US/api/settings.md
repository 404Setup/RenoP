---
title: Settings API
order: 8
category: API Reference
description: Domain-based service settings, repository management, and index rebuilds
---

# Settings API

All settings routes require a manager account or an API token with `admin:settings` or `admin:repositories`, according
to the operation. Responses use protobuf where defined in `proto/api/v1/api.proto`.

## Discover setting domains

- **Path**: `GET /api/settings/domains`
- **Response**: Stable domain names currently supported by the server, including `server`, `proxy`, `storage`,
  `updater`, and `frontend`.

## Settings pages in the browser

The settings interface provides a separate page for each of the 13 advertised domains. Desktop navigation lists
the sections beside the form; smaller screens use a section selector. Previous and next controls follow the same
ordered pages. Opening a page fetches its configuration only, rather than fetching every service configuration.

Each page keeps its unsaved draft while moving between sections or changing language. Saving updates only the active
page and temporarily prevents editing or switching pages. Failed saves retain the draft; discard reloads that page
after confirmation. Pending changes are marked in the navigation. Browser reload/close warns about unsaved changes,
but drafts are held only in memory and are cleared at logout or account change. Stored write-only credentials stay
hidden, and entered secrets are cleared from a draft after a successful save.

GPG remains part of the service configuration. Global-team limits, publication quotas, registration, cache, email, OAuth
providers, and publishing-domain security have separate pages. Legal and unified OAuth settings use binary protobuf;
other pages retain their documented formats. Labels and hints remain associated with controls, and navigation moves
focus to the page heading.

Selected sections, unsaved markers, and muted text use the shared theme colors in both light and dark modes.

## Read and update one domain

- **Read**: `GET /api/settings/domain/:name`
- **Update**: `PUT /api/settings/domain/:name`
- **Behavior**: The request and response schema depends on `:name`. Unknown fields and invalid values are rejected.
  Host, port, TLS, database, and selected runtime changes may require a service restart.

**Third-party login**: `GET /api/settings/oauth-providers` returns binary protobuf `OAuthSettings` with the built-in
GitHub entry, redacted clients, and presets. `PUT /api/settings/oauth-providers` requires `replace_providers: true` and
a `providers` list, saving both groups atomically within 128 KiB. Up to 12 other clients plus GitHub are supported.
Omitting GitHub preserves it; disable its entry and use `clear_client_secret` to remove credentials. An empty list
removes only other clients. `revocation_secret` is write-only; blank retains it for the same client, and
`clear_revocation_secret` erases it. Compatibility endpoints `GET /api/settings/github-oauth` and
`PUT /api/settings/github-oauth` retain JSON and manage the same GitHub configuration.

[OAuth](../security/oauth-login.md)

## Repository settings

The generic `/api/settings/repositories` routes are preferred. Maven-prefixed aliases remain for compatibility.

Repository changes commit to the database before the live configuration is replaced. Deleting the last repository
persists an empty set across restarts; legacy YAML is only an initial migration source.

### List repositories

- **Path**: `GET /api/settings/repositories`
- **Alias**: `GET /api/settings/maven/repositories`

### Create, update, delete, or migrate

- **Create or update**: `PUT /api/settings/repositories/:name`
- **Delete**: `DELETE /api/settings/repositories/:name`
- **Migrate Maven/files**: `POST /api/settings/repositories/:name/migrate/:target`, where `:target` is `maven` or
  `files`. Stored objects remain in place while the Maven catalog is rebuilt when returning to Maven.

## Rebuild the search index

- **Path**: `POST /api/settings/index/rebuild`
- **Behavior**: Submits a coalesced background rebuild. A concurrent rebuild is not started twice.

## Publishing-domain reservation

`GET /api/settings/maven-domains` and `PUT /api/settings/maven-domains` use JSON.
Discovery includes `maven_domains`. The default is:

```json
{"release_value":2,"release_unit":"year"}
```

`release_value` is an integer from 1 to 100. `release_unit` accepts `month` or `year`, using UTC calendar arithmetic.
The settings database stores the same fields under `maven_domains`. A saved change applies to newly created security
locks without changing existing release dates or the separate 31-day voluntary closure period.
See [Maven domain health](maven.md).

[Legal documents and cookie choices](../configuration/legal.md)

Settings navigation, provider editors, and dependent fields use cancellable transitions and respect reduced motion.
Empty provider/account lists use the shared notice style. Closed dropdowns do not keep their option menus or document
listeners alive; opening a dropdown creates them on demand.

Embedded assets stream from the executable. RenoP caches their content type, length, and ETag without retaining another
copy of every bundle and compression variant in the Go heap. Precompressed negotiation and conditional requests remain
supported.

[Security verification](../security/captcha.md)

`capacity_limit_bytes` is the per-repository installed-byte limit; `0` means unlimited. The UI edits it in MiB. Disk and
S3 commits count artifacts, generated checksums, pending-review objects, and cached mirror content, excluding temporary
staging copies. Capacity is reserved before commits so concurrent uploads share the same limit. Excess writes return
`507` with `repository_capacity_exceeded`; a valid mirror response can still stream without being cached. Existing files
remain readable when the limit is lowered below usage. After out-of-band storage changes, rebuild the index or restart
to remeasure usage. Omitting the optional field preserves the existing limit for older clients.

Legal documents are edited under Frontend and saved atomically with branding; omitted `legal` preserves the prior
documents. Index controls are under Storage. The compatibility legal and index endpoints remain available. Server
settings expose listener IP/port, TLS, database and performance controls; listener and database changes require restart.
