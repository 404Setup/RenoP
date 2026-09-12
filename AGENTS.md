# AGENTS.md

RenoP is a Go package repository server with an embedded SPA and a separate static website.
Use this file as a work guide and source index, not a feature catalog.

## Working contract

- Follow the current task's scope, acceptance criteria, and stop/report-only instructions. Continue authorized work;
  ask only when missing information blocks a consequential decision.
- Start with `git status --short --branch` and the relevant diff. Preserve unrelated edits, local data, and secrets.
  Never reset, clean, overwrite, or stage someone else's work.
- For multi-step work, keep a short checklist of requirements, affected layers, verification, and remaining blockers.
  Update it when scope changes or work resumes; do not create planning files for trivial edits.
- Trace the affected flow before editing: entry point -> authorization -> service -> database/storage -> response/UI.
  Find every caller of a changed shared function; inspect actual types, signatures, existing helpers, and nearby tests.
- Prefer deletion or reuse, then stdlib/native features, installed dependencies, and minimal new code.
  Fix the shared root cause. Avoid speculative abstractions, unrelated refactors, and dependency churn.
- Finish each logical change across all affected layers and fix regressions it introduces. Report unrelated findings
  separately. Do not reduce explicitly requested behavior to make a smaller diff.
- Commit/push only within the active task's authorization. Use standard English commit messages; when per-item delivery
  is requested, verify and deliver each item before the next. After pushing, verify HEAD matches the intended remote
  branch.
- Update this file in the same turn when architecture, toolchains, build scripts, workflows, or directories change.
  Replace the affected entry; keep implementation details, API fields, and historical fixes in their owning source/docs.

## Locate the owner

Use `rg --files <directory>` to find files and `rg -n '<symbol>' <directory>` for definitions/callers.
Start with the smallest relevant directory; widen only when references cross its boundary. Batch independent reads,
limit output, and retain useful path/symbol findings instead of repeatedly dumping files or scanning the whole repo.
Skip dependencies, generated bundles, and local storage unless the task concerns them.

Service paths in this table are relative to `internal/service/`; all other paths explicitly start at the repo root.

| Area / symptom                                                         | Start here                                                                                                                                                                                                                                                   |
|------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Startup, shutdown, configuration, shared state                         | `server.go`, `internal/bootstrap/`, `internal/config/`, `internal/configstore/`, `internal/core/`                                                                                                                                                                                     |
| HTTP routing, search, middleware, public API                           | `internal/api/`, `internal/middleware/`, relevant service `routes.go`; bounded request gates in `internal/utils/ratelimit/`, download admission in `internal/middleware/download.go` |
| SQL, migrations, transactions, persistence caches                      | `internal/database/`; MySQL schema in `dialect_mysql.go` and `dialect.go`; native ClickHouse logic in `clickhouse*.go`                                                                                                                                                                                                      |
| Memory, Redis, Valkey cache backends | `internal/cache/` owns encrypted remote values; `internal/cache/ttl/` owns bounded generation-aware caches shared by database and index; binding in `internal/core/cache.go`, configuration in `settings/cache.go` |
| CAPTCHA verification | `internal/config/captcha.go`, `captcha/`, `settings/captcha.go`; browser transport and disposable widget in `js/captcha*.js` |
| Legal documents and browser consent | `internal/config/legal.go`, `legal/`, `settings/legal.go`; public pages and consent in frontend `js/legal-*.js`, `js/cookie-consent.js` |
| Login, sessions, Passkey, TOTP, OAuth, API tokens, profiles            | `auth/`; second factors in `mfa*.go`; email ownership in `internal/database/account_emails.go`, verification in `email_verification.go`, `github_email.go`; provider flows in `github_*.go`, `oauth_*.go`; encrypted session grants and revocation events in `internal/database/oauth_sessions.go`; OAuth configuration in `internal/config/oauth.go` |
| Registration, retirement, recovery, avatars                            | `auth/`, `internal/database/`, matching `registration*`, `account_retirement*`, `recovery_codes*`, `password_reset*`, `avatar*` files; registration policy in `internal/config/registration.go` and `settings/registration.go`                               |
| Account and IP suspensions                                             | `token/routes.go`, `internal/database/account_ban.go`, `internal/database/account_ip_ban.go`; request enforcement in `internal/middleware/anomaly.go`                                                                                                        |
| Cargo registry and documentation                                       | `cargo/`, `cargodocs/`                                                                                                                                                                                                                                       |
| Conan recipes, binary packages, revisions and native clients | `conan/` owns route parsing and bounded metadata; managed publication and signing manifests use `nativepkg/` and `storage/native_publication.go`; the Conan signing extension is `scripts/conan/sign.py`; `auth/protocol_grant.go` owns repository-bound protocol grants |
| Maven domains, verification, artifacts                                 | `maven/`                                                                                                                                                                                                                                                     |
| Docker Registry v2, blobs, manifests, mirrors                          | `docker/`                                                                                                                                                                                                                                                    |
| npm metadata, tarballs, versions, dist-tags                            | `npm/`                                                                                                                                                                                                                                                       |
| Native Conda, APK, apt, rpm/yum layouts | `nativepkg/` owns managed identities, upload validation and signature policy; `internal/database/native*.go` owns resources, immutable-ID memberships and publication records; management API in `internal/api/native.go`; codecs and native signature verification in `conda/`, `apk/`, `apt/`, and `rpm/`; storage adapters in `storage/engine_*.go`, publication in `storage/native_publication.go`, cached index records in `storage/native_index.go`; Debian XZ preflight in `apt/xz.go`; browser management in `js/browser/native.js` |
| Files, Disk/S3, quarantine, mirror completion                          | `storage/` (protocol composition in `engines.go` and `engine_*.go`, physical backends in `backend*.go`, immutable disk installation/deduplication in `internal/artifactstore/`, package adapter in `package_store.go`), `gpg/`, `packagestore/`; shared mirror hook in `storage/mirror.go`                                                                                                                                                                               |
| Missing/stale index entries, file-vs-directory errors | `index/` owns serialized index mutations and authoritative file/directory state; `index/cache.go` uses the shared cache backend for disposable negative lookups; HTTP classification in `storage/` |
| Content sharing, existing-file maintenance and private index persistence | `internal/artifactstore/` owns Disk hard links and S3 content references; `storage/dedup_maintenance.go`, `storage/s3_content.go`, and `storage/s3_index.go` schedule work and reuse index metadata; `index/content.go` owns private content records and `index/stream.go` uses versioned go-json record streams with legacy snapshot reads |
| Upload/configuration races                                             | `repositorygate/`, affected protocol, database transaction                                                                                                                                                                                                   |
| Global teams, ownership, public resources                              | `superteam/`, `ticket/`, `internal/database/super_team_resources.go`                                                                                                                                                                                         |
| Tickets, reports, publication review and notifications | `ticket/` (request admission in `admission.go`), `ticketnotify/`, `internal/database/ticket*.go`, `internal/database/review*.go`; cursor-based conversation reads in `ticket_messages.go` |
| Quota, statistics, global/activity logs, messages, periodic work       | `publicationquota/`, `statistics/`, `audit/`, `message/`, `tasks/`                                                                                                                                                                                           |
| Email transports, templates, durable queue, accounting                 | `internal/mail/`, `mailqueue/`, `internal/database/mail*.go`, `settings/mail.go`; disposable domain data in `internal/mail/data/`, refreshed by `scripts/update-disposable-domains.ps1`                                                                      |
| Outbound networking                                                    | `proxy/`, `outboundproxy/`                                                                                                                                                                                                                                   |
| Updates, container runtime, services, Caddy                                               | `updater/`, `internal/containerenv/`, `internal/daemon/`, `internal/caddy/`, `internal/version/`; container image in `Dockerfile`                                                                                                                                                                                       |
| Shared bounds, secret encryption, renames, memory tuning, test cleanup | `internal/utils/` (AES-GCM in `secretcipher/`), `internal/testutil/`                                                                                                                                                                                         |
| SPA and embedded assets                                                | `internal/service/frontend/`; sources in `renop-html/`, JS page components in `renop-html/js/views/`, embedding in `html.go`                                                                                                                                                                               |
| Shared UI / website / documentation                                    | `packages/renop-ui/`, `web/`, `web/content/docs/`; independent API views in `web/js/pages/api.js`                                                                                                                                                                                                            |
| Base32 codecs | `pkg/base32/`; SIMD backends, arm64 assembly fallback, and standard-library compatibility tests |
| Hex codecs | `pkg/hex/`; standard-library-compatible APIs, amd64/wasm SIMD, arm64 NEON assembly, and standard-library fallback |
| API/session schemas and generated Go bindings                          | `proto/`, `pkg/pb/`                                                                                                                                                                                                                                          |
| Build, compression, release publishing                                 | `build.ps1`, `scripts/`, `cmd/`, `.github/workflows/`, `.github/scripts/`                                                                                                                                                                                    |
| Source line and file counts | `count.ps1` counts Go, JS, CSS, and Markdown under the current directory; optional `rg` acceleration retains a .NET fallback, with dependency/output/storage directories excluded |

### Frontend reuse map

Paths below are relative to `internal/service/frontend/renop-html/`.
Check `packages/renop-ui/package.json` exports before building another shared control.

| Concern                                                               | Existing owner                                                                                                                                                                                                                                                     |
|-----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| History, sign-in page, protected routes, HTTP failures, offline state | `js/main.js`, `js/back-navigation.js`, `js/login-route.js`, `js/auth.js`, `css/account-pages.css`, `js/protected-route.js`, `js/api.js`, `js/response-errors.js`, `js/backend-availability.js`                                                                                              |
| Identity, profile photos/links, provider connections, profile cache   | `js/user-profiles.js`, `js/profile.js`, `js/profile-avatar.js`, `js/profile-links.js`, `js/account-language.js`, `js/oauth.js`, `js/components/user-avatar.js`                                                                                                     |
| Account security / recovery / retirement / tokens / administration    | `js/account-security.js`, `js/account-emails.js`, `js/fido-utils.js`, `js/mfa-login.js`, `js/profile-email-verification.js`, `js/account-recovery.js`, `js/password-recovery.js`, `js/login-route.js`, `js/account-retirement.js`, `js/api-tokens.js`, `js/api-token-targets.js`, `js/users/` |
| Email accounts, provider presets, billing, delivery | `js/settings.js` owns independent drafts and saves; category grouping in `js/settings/navigation.js`; forms in `js/settings/`, email in `js/settings/mail.js`, setup links in `js/settings/documentation.js` |
| Tickets / reports / messages / administrator composer                           | `js/tickets.js`, `js/ticket-report.js`, `js/ticket-messages.js`, `js/messages.js`, `js/notification-composer.js`, `js/notification-sessions.js`                                                                                                                                                                          |
| Teams and quota                                                       | `js/super-teams.js`, `js/super-team-resources.js`, `js/publication-quota.js`                                                                                                                                                                                       |
| Repository/package UI                                                 | `js/browser/`; protocol capabilities and client snippets in `js/repository-engines/`, catalog in `js/repository-formats.js`, view dispatch in `js/browser/engines.js`; reuse `repository-view.js`, `package-detail-tabs.js`, `copy-feedback.js`, `user-suggestions.js`; lifecycle in `js/package-deprecation.js`; lock controls in `js/resource-locks.js`                                                                                                           |
| Markdown, clipboard, timestamps, async buttons | `js/markdown.js`, `js/clipboard.js`, `js/time.js`, `js/components/button.js` (`runUIAction`, `createActionButton`); JSON endpoint clients use `createJSONClient` in `js/api.js` |
| Localization and API error codes                                      | `js/i18n/<locale>/`, root `scripts/i18n-catalog.mjs` and `scripts/i18n-protobuf.mjs`; shared binary loader in `packages/renop-ui/js/i18n-catalog.js`; `js/*-errors.js`                                                                                                                                                                                                  |
| Shared controls, animation, styling, jQuery runtime                   | `@renop/ui` exports and matching styles in `packages/renop-ui/`; field rows, badges, skeletons, meta grids, and code badges live there. Permission drafts share `js/permission-editor.js`; settings CSS is composed from `css/manager/settings/`. |
| Build and size budgets                                                | `build.mjs`, `rolldown.config.mjs`; frontend loading order in `js/bootstrap.js` and `js/shell.js`                                                                                                                                                                                                                                 |

## Preserve these contracts

Read the relevant implementation and tests for exact limits and exceptions before changing these areas.

- **Database:** Support SQLite, MySQL, PostgreSQL/pgx, and native ClickHouse. Use existing transaction/rebinding APIs.
  ClickHouse uses `clickhouse.Open` + EmbeddedRocksDB, never the `database/sql` adapter; preserve journal recovery and
  restart-safe copy/verify/rename schema migrations. Production SQL DELETE statements need a statically provable WHERE.
  Keep SQLite shutdown/WAL cleanup and use `internal/testutil.TempDir` for file-backed database tests.
- **Startup configuration:** `internal/configstore/` owns the private `renop-settings.db` SQLite snapshot, including
  application database connection settings and authenticator/mail encryption keys. Independent native repository
  signing keys live there too; `internal/service/nativesign/` initializes and signs generated APK, apt, and rpm indexes.
  `RENOP_SETTINGS_DB` selects its path.
  Import legacy `config.yaml` (`RENOP_CONFIG`) only before the first committed snapshot, then archive it. Never create
  YAML configuration files. Save before publishing immutable in-memory settings; offline Caddy changes use compare-and-swap.
- **Demo mode:** `internal/demo/` owns isolated database presets and bounded in-memory browser sessions. `--demo`
  uses read-only database connections, no artifact files, and no background workers or audit writes. `--demo-temp`
  requires `--demo` and permits persistent system settings and repository definitions only; all other records remain
  read-only. Preset session records are display data and must never authenticate requests.
- **Repository configuration:** `internal/database/repository_settings.go` persists complete snapshots; startup migration
  lives in `internal/bootstrap/repositories.go`. Import legacy `repositories.yaml` only before the initial database
  snapshot and archive it after commit. Fresh installations start with no repositories; never create implicit defaults.
- **Identity and authorization:** Ownership/membership uses immutable user IDs; public profiles use `/user/<username>`.
  Enforce live account, repository, package/team, and scoped-token permissions server-side. API tokens intersect current
  owner permissions; cookie-only sessions gate browser-only operations. Basic/password authentication is protocol-only.
  Moderator roles do not imply write or manager authority. Never expose secrets or private profile/team fields publicly.
  Private profiles are visible only to their owner, administrators, and moderators of any repository. Apply the shared
  profile privacy guard to details, batches, memberships, and avatars; autocomplete filters before pagination, while
  explicit invitations remain supported. Privacy persistence and session checks live in `profile_privacy.go` in database/auth.
  OAuth subjects are scoped to the provider configuration authority; never merge accounts by email.
  GitHub shares the third-party login settings and provider lists; preserve its legacy configuration, callback,
  identity bindings, organization authorization, and compatibility endpoints when changing the unified UI.
  User profile provider links come from current GitHub/GitLab bindings and owner-controlled visibility; GitLab links
  must match the configured authority. Global-team GitHub URLs remain editable. Derivation lives in `auth/profile_links.go`.
  Primary and provider email addresses share immutable account ownership; binding and email reservations commit
  together. Unverified provider addresses require existing ownership or email proof; retained aliases survive unlinking.
  Preserve PKCE, OIDC signature/issuer/audience/nonce verification, browser-bound state, and mandatory email proof
  during provider registration.
  Browser session issuance enforces live second-factor policy and credential snapshots. Secondary Passkeys cannot count
  as primary login methods; MFA accounts use API tokens for package clients. Startup persists the private authenticator
  encryption key before serving requests; preserve it across configuration updates.
  OAuth login grants are encrypted per browser session with that key and carried through MFA. Provider revocation
  removes local sessions before outbound requests. Signed callbacks use original authority and subject/session scope;
  persist replay protection and serialize callbacks with grant issuance so delayed login proofs cannot restore access.
- **Account lifecycle:** Preserve alternate-login and atomic recovery-consumption invariants, hashed credentials,
  immediate revocation, and targeted cache invalidation. All login methods honor bans and retirement.
  Recovery uses the primary email; `internal/database/primary_email.go` retains former primaries for 14-day recovery
  with offline codes and restores the selected address atomically. Normal primary changes hold recovery-code
  replacement, Passkey changes, and second-factor changes for 14 days. Password changes enforce a fresh configured
  second factor or a primary-email proof. Aliases cannot reset passwords or recover accounts.
  The existing account-retention sweep releases expired recovery-only addresses under the account lock while
  preserving the current primary, explicitly retained aliases, and independent retirement holds.
  Administrators and moderators must lose those roles before suspension; suspended accounts cannot gain those roles.
  Optional IP restrictions share the account ban's lifetime, use recorded login addresses, and commit with session
  revocation. Preserve trusted-proxy extraction, local cache invalidation, and overlapping bans on shared addresses.
  Registration is explicitly enabled; configured default permissions apply only at account creation across all providers.
  Confirmation, credentials, email, provider identity, and persistent IP accounting
  commit together. Pending provider registrations confer no account privileges. OAuth callbacks require the initiating
  browser cookie as well as a single-use server state.
  Retirement rechecks protected roles, ownership, and pending reviews; keeps permanent tombstones, reserves email for
  14 days, retains audit activity for 30 days, and rejects stale writes or delayed audit resurrection.
- **Teams and packages:** Preserve live T1-T4/L0-L4 permission mapping, the last owner, membership limits, and public
  visibility filters. Scoped npm and namespaced Docker require the matching global-team prefix. Reserve npm/Docker
  resources explicitly, check upstream conflicts, and keep mirrored packages pull-only. Permanent package deprecation
  blocks every mutation while retaining downloads; pending transfers/reviews prevent deprecation.
- **Managed native resources:** APK, apt, Conan, Conda/Conda native, and rpm/yum require explicit resource reservation
  and live L0-L4 permissions without a team-prefix requirement. Identity comes from validated native metadata or
  the Conan recipe reference. APK/RPM require trusted native package signatures; Conan uses its OpenPGP signing
  extension and waits for the complete signed manifest. APT authenticates server-generated repository metadata.
  Keep unsigned/incomplete and pending-review files hidden across restart, commit review visibility atomically,
  retain the last owner, and include native ownership in retirement and repository cleanup. New uploads cannot
  replace server-generated indexes. `new_packages` reviews the first publication after resource reservation.
- **Resource locks:** Shared records live in `internal/database/resource_lock.go`; protocol enforcement lives in
  `internal/service/cargo/locks.go`, `internal/service/npm/locks.go`, `internal/service/docker/locks.go`, and
  `internal/service/maven/locks.go`. Maven XML projections live in `internal/service/maven/metadata_locks.go`;
  preserve SNAPSHOT and companion-path coverage in `internal/database/maven_locks.go`.
  Docker index references are captured in `internal/database/docker_locks.go`; preserve source-specific inheritance
  and shared-blob protection. Cross-repository mounts acquire ordered gates in `repositorygate/`.
  Shared mirror authorization is wired in `internal/service/storage/mirror.go`.
  Global-team lock routes live in `internal/service/superteam/locks.go`; bindings inherit restrictions through
  `internal/database/resource_lock.go`. Maven domain/path inheritance lives in `internal/database/maven_domain_locks.go`.
  Global publishing-domain lock routes live in `internal/service/maven/locks.go`; repository changes also inspect
  uncatalogued domain namespaces through `EnsureRepositoryMutable`.
  Preserve retained memberships, independent verified child domains, and current-request moderator scopes.
  Store bounded custom reasons in `resource_locks.reason_text` and render them as plain text.
  Keep manual and system locks independent. Read locks freeze writes, restrict metadata to live staff/members, and deny
  files to everyone. Include index/search/profile visibility, cached files, mirror refreshes, and repository changes.
- **Maven lifecycle:** Domains are global across repositories. Closure blocks mutations, preserves downloads, and holds
  the name for 31 days; a later claimant must verify ownership and obtain administrator approval before publication.
  Security monitoring in `maven/domain_health.go` uses RDAP or pinned provider account IDs. Preserve explicit redemption,
  configurable calendar reservations (two years by default), and moderator-approved restoration of reclaimed artifacts;
  persistence lives in `internal/database/maven_health.go` and `maven_restore.go`.
  Preserve classic/catalog layouts and Maven/files migration without moving stored objects.
- **Review:** Decisions and metadata changes commit once, atomically, with live authority rechecked. Pending
  publications
  stay out of public protocol responses and index paths, including after restart/GPG completion. Creation uses
  `@create`;
  T2 requests pass team approval before any repository stage. Virtual Docker manifests are not filesystem objects;
  rejection must not delete shared blobs. Block repository reconfiguration/migration/deletion while publication reviews
  are pending. Notify after durable decisions, deduplicate recipients, and expose handling staff identity only to authorized staff inside tickets, never to the requester.
  Reports hide reporter identity from targets; assignment and escalation commit atomically with live authority.
- **Storage and concurrency:** Reuse `packagestore/` for bounded staging, validation, atomic Disk/S3 commit, and
  rollback;
  Keep S3 deduplication driven by persisted hashes and reference indexes. Learn unknown hashes during ordinary
  transfers; never download old objects solely for a background comparison. Preserve indexed collection queues,
  reference guards, and shared-object backup coverage described in the repository configuration documentation.
  use `repositorygate/` around mutations that race configuration, review, or retirement. Stream large bodies/archives,
  validate actual bytes as well as advertised sizes, verify hashes, bound extraction paths and work queues.
  Replace files atomically; failed staging/rename must preserve the installed destination and recoverable source.
  Installed Disk artifacts may share hard-linked contents; all writers, including generated checksums and mirror caches,
  must replace files through `internal/artifactstore/`, never modify installed inodes in place.
  `internal/repositorycapacity/` reserves per-repository committed bytes; `storage/capacity.go` measures Disk/S3
  artifacts, including checksums and cached mirrors. Reserve before commit and invalidate after failed writes,
  deletion, or reconfiguration. Exceeding a limit returns `repository_capacity_exceeded`; mirror responses may still
  stream without being cached. Temporary staging copies do not count as installed bytes.
- **Visibility and resource bounds:** A normalized index path is a file or directory, never both; known artifacts never
  receive the SPA shell. Bound cache size/lifetime, request and response reads, pagination, batches, and background
  work.
  Invalidate affected caches after successful mutations and guard against stale in-flight fills. Avoid N+1 queries,
  redundant hot-path copies, and whole-object buffering.
  External cache values are encrypted with process-private keys; keep bounded local invalidation indexes so cache
  outages cannot undo credential revocation. Backend changes require a restart; never use Redis as authoritative
  storage.
- **Notifications:** `user_messages.session_id` is an optional public browser-session ID. Empty grants account-wide
  visibility; derive non-empty scope only from the authenticated cookie session. Apply scope to list/count/read/delete,
  including bulk operations, and exclude targeted messages from email. Validate the target during transactional delivery.
  Workflow action messages remain account-wide. Manager discovery uses bounded active-session cursor pages.
- **Quota and events:** Reserve/commit/release quota transactionally; team-owned resources charge only the team and
  mirrors are exempt. Keep download-count exclusions and pending-plus-persisted resets in `statistics/`.
  Use `tasks/` for coalescible periodic work; preserve dedicated serial workers where event order matters.
  `audit/` captures process/HTTP diagnostics, filters activity and global logs, and drains its shared serial writer on
  shutdown.
  Preserve hidden-operator filtering, credential redaction, and separate activity/system retention budgets.
- **Email:** `mailqueue/` owns the durable serial worker, rate limits, credit reservations, and status checks.
  `internal/database/mail_rate.go` debits manual IP, IPv6 network, account, and recipient budgets with queue insertion.
  Preserve encrypted queue payloads, persistent OAuth rotation, bounded history, and private status capabilities.
  An interrupted submission has an unknown outcome; never resend it automatically or treat acceptance as delivery.
- **CAPTCHA:** Browser actions consume bounded, single-use approvals bound to their cookies, scope, and current policy.
  Verified API tokens and protocol-password clients remain exempt. Manual-mail scopes take precedence on overlapping
  email actions. Enforce before mutation/staging; retry only the explicit precondition denial. Provider code lives in
  the disposable `captcha-widget` bundle and requires optional-service consent.
- **Legal documents:** The configuration database owns the three Markdown documents; no local policy file or external legal URL is read.
  Keep bounded safe rendering and public access with expired credentials. Cache legal metadata and document ETags per
  immutable configuration snapshot; public binary metadata and plain-text documents require HTTP revalidation. Account entry requires the current privacy/terms
  revision; optional browser services require explicit category consent, with preferences available from the footer.
- **Frontend:** Reuse the shared UI, jQuery runtime, error, identity, clipboard, time, and animation helpers.
  Both `index.html` files contain metadata and a mount point. `js/shell.js` mounts `js/views/` components.
  The frontend bootstrap awaits shell creation before loading behavior modules; keep the application entry hashed,
  preserve bundle budgets, and include view components in i18n and DOM contract checks.
  Embedded static assets stream from `embed.FS`; cache representation metadata rather than duplicate payload bytes.
  Shared custom selects create menus and global listeners only while open; preserve detach/remount and keyboard behavior.
  Scope target dialogs commit only on confirmation; stacked dialogs retain keyboard focus and dismiss only the top dialog.
  Keep streaming/observers/native APIs where appropriate. Preserve keyboard/focus behavior, responsive layouts,
  viewport-bounded dialogs, and loading/empty/error states. A valid authenticated 403 must not log out the user.
  Render untrusted Markdown through the inert allowlist; never show raw backend errors or runtime exceptions.
- **API encoding:** `internal/utils/protohttp/` encodes and decodes binary protobuf for schema-backed control APIs.
  ProtoJSON fallback is removed; schema-backed APIs accept and serve binary protobuf (`application/x-protobuf`).
  Preserve request bounds, Content-Type verification, and Vary headers. Native package protocols and binary uploads
  retain their own wire formats.
- **Localization/docs:** Add stable errors and audit actions to every supported locale. English is canonical;
  `internal/locale/` matches account and request languages; private preferences live in `user_profiles.locale`.
  `internal/mail/template_locales.go` covers every frontend language. Mail captures the recipient language when queued;
  browser language synchronization binds pending changes to immutable user IDs and preserves credential revisions.
  preserve keys/placeholders, lazy locale loading, and bundle budgets. Website `/api` serves Markdown; `/docs` excludes API references.
  Both browser consumers validate source keys, fragments, placeholders, and references before merging translations.
  Build one hashed protobuf key index and one language values file from `proto/i18n/v1/catalog.proto`; preserve lazy
  loading, English fallback, a visible startup retry, byte bounds, schema revision checks, and stale language-request rejection.
  Keep locale protobuf bindings in their own `renop_i18n` root so control API generation cannot overwrite their types. `.pb` locale
  assets use `application/x-protobuf` and inline disposition, remain immutable, and use normal embedded precompression
  negotiation. Website response rules live in `web/_headers`. Avoid generic `.bin` names that download integrations
  can intercept. Generated catalogs and decoders are ignored.
  Markdown sources remain in `web/content/docs/<locale>/api/` and `web/assets/openapi.yaml` owns the specification.
  Website translations must retain canonical
  files, heading outlines, examples, endpoints, and links; see `web/test/docs-parity.test.mjs`.
- **Release:** Frontend sidecars must not be recompressed; serve them with correct negotiation, ETags, and Vary.
  Update payloads contain only raw `.br` executables and `manifest.json`; release docs attach separately to GitHub.
  Preserve SHA-256 checks, legacy ZIP decoding, current/previous commit ordering, and bounded nightly retention.
  Nightly metadata is rebuilt by `.github/scripts/nightly-info.ps1`; publishing requires PowerShell 7.5 or later
  to preserve JSON date strings. `cmd/renop-release-index` reads the actual protobuf directory inventory;
  `.github/scripts/package-retention.ps1` protects retained SHA aliases and discovers orphaned package trees.
  Cleanup runs after durable publication, verifies directory removal, preserves five-delete/100-probe bounds,
  skips missing directories without consuming the deletion budget, and surfaces HTTP failures.
  The custom linker receives `-o3` and `-fmth` through `-ldflags`; preserve the shared flags in `build.ps1`.
  `scripts/build-target.ps1` defaults to `GOEXPERIMENT=simd` on target amd64/wasm and `nosimd` elsewhere,
  selecting arm64 NEON assembly independently of the build host. Preserve other experiments and explicit
  `simd`, `nosimd`, or `none` from the environment or `go env -w`; validate with `scripts/test-build-target.ps1`.
  `pkg/base32/` uses standard-library SIMD, arm64 assembly without the experiment,
  and the original scalar implementation elsewhere or with `-tags=purego`.
  `pkg/hex/` uses Go SIMD on amd64 with AVX2 and on wasm with SIMD128, handwritten NEON on arm64
  regardless of the experiment, and `encoding/hex` elsewhere or with `-tags=purego`.
  Actions share the workflow-level `renop-actions` FIFO group with `queue: max`. Within each run, prepare shared
  inputs once, compile the `scripts/build-targets.psd1` matrix concurrently, then package the complete build matrix
  concurrently and assemble a verified full manifest before publishing. No per-workflow matrix parallelism cap.
  Local compile/compression pools remain independently bounded; CI selects one target per runner with `-Target`
  and `-SkipPreparation`. Packaging and assembly live in `.github/scripts/package-matrix-target.ps1` and
  `assemble-matrix.ps1`; only the final `dist-artifacts` artifact is publishable to the update repository. Pin the prepared Go runtime version
  across the matrix, share module caches, keep target build caches separate, and run obsolete-cache cleanup once.
  Release-only container publication reuses Linux amd64/arm64 executable artifacts with the root `Dockerfile`;
  `.github/scripts/prepare-container-publish.ps1` reserves `oci/renop` and uses the existing publishing token.
  Container detection in `internal/containerenv/` disables executable updates, process restarts, and service setup;
  `/data` is the image's writable working directory. Preserve SIGTERM draining and database shutdown in `server.go`.

## Implementation standards

- Keep comments, docstrings, and engineering documentation in English; translated product content uses its locale.
  Use standard Go doc comments for exported declarations and concise JSDoc for handwritten JS/TS functions.
  Comment non-obvious intent, safety constraints, or tradeoffs; do not narrate statements.
- Preserve validation, error propagation, cleanup, rollback, and accessibility. Do not silence failures, return dummy
  success, weaken assertions, or bypass security checks to make validation pass.
- Modify source, then regenerate affected outputs. Do not hand-edit generated protobuf, locale catalogs, or bundles.
  Inspect generated diffs and preserve license headers. Update lockfiles/notices when dependencies actually change.
- Check affected API/schema compatibility, all callers, frontend wiring, permissions, i18n, migration/rollback,
  cache invalidation, and docs as part of the change; irrelevant layers require no work.

## Verification by impact

Choose checks from the changed behavior and its callers. The table is a selection guide, not a mandatory full checklist.
Run existing checks first. Non-trivial behavior needs a runnable check of its contract; reuse sufficient coverage.
Add or extend the smallest regression test only when existing checks would miss the changed behavior or defect.
Use the current Go/Node test setup; avoid duplicate or implementation-mirroring tests, broad fixtures, new frameworks,
and test files for prose/trivial edits.

| Change                                                      | Required relevant evidence                                                                                                             |
|-------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| Prose-only engineering docs/comments (no directives)        | Review accuracy, paths/commands, and `git diff --check`; no application build/test                                                     |
| Website documentation                                       | `pnpm run test:web`; add `pnpm run build:web` when rendering/build behavior is affected                                                |
| Local Go behavior                                           | Affected package tests and caller tests; targeted regression/boundary cases for changed behavior                                       |
| Shared backend contracts, Go dependencies, broad Go changes | `go test -count=1 -p 1 ./...` and `go vet ./...`; local build when startup/toolchain/embedding is affected                             |
| Shared SQL/schema/transaction semantics                     | Database + affected service tests; isolated contract runs on all four drivers; report unavailable drivers explicitly                   |
| SPA JS/CSS/HTML                                             | Relevant existing frontend tests for behavior, `pnpm run build:frontend`; inspect changed UI when browser access is available          |
| Shared UI                                                   | Relevant tests in both consumers; `pnpm run build:ui-consumers`                                                                        |
| Locale keys/errors/audit actions                            | `pnpm run check:i18n` and relevant coverage tests; frontend build already includes i18n validation                                     |
| Protobuf/API contract                                       | Regenerate affected Go/JS bindings, test affected producers/consumers, verify compatibility; local build for integration               |
| Build/workflow/packaging                                    | Script syntax and affected build path; validate release payload for packaging changes; full target matrix only when affected/requested |
| Security/concurrency/performance                            | Reproduce the issue; check denial/boundary/rollback cases; race detector or focused benchmark only when that risk is changed           |

- Start focused, then expand for shared contracts or unresolved risk. A build proves compilation, not behavior.
  Do not rerun overlapping full suites after successful final checks unless code, failures, or relevant evidence
  changes.
- Validate the final edited state; earlier results do not cover later relevant edits. Track passed, failed, and unrun
  checks. If tooling/data/browser access blocks a required check, report the gap instead of claiming full verification.
- Prefer serial Go package tests (`-p 1`) on Windows; avoid competing builds over shared generated files/caches.
  Diagnose locks, permissions, network, and toolchain failures before attributing them to code; never weaken tests.
- `renop-dbtest` is destructive: use a fresh, empty, disposable DSN per run with `-confirm-isolated`; its temporary
  mail encryption keys cannot decode queue data from earlier runs. Never use local/live
  application data or print credentials. Benchmark only when needed to validate a performance claim.

## Commands and toolchain

Run from the repository root using PowerShell 7. Toolchain sources of truth: `go.mod` and
`.github/actions/setup-go-runtime/` (404Setup/go), `package.json` (Node minimum and pnpm pin),
`.github/workflows/build.yml`
(Node/protoc).
Install missing prerequisites; do not silently replace the custom Go runtime or upgrade pins to bypass a failure.

| Task                                                  | Command                                                                            |
|-------------------------------------------------------|------------------------------------------------------------------------------------|
| Restore JS dependencies when missing/lockfile changed | `pnpm install --frozen-lockfile`                                                   |
| Focused Go tests (substitute package and test name)   | `go test -count=1 -p 1 ./internal/service/<module> -run '<TestName>'`              |
| Frontend / website tests                              | `pnpm run test:frontend` / `pnpm run test:web`                                     |
| One frontend test (substitute file)                   | `node --test internal/service/frontend/renop-html/test/<name>.test.mjs`            |
| Frontend / website build                              | `pnpm run build:frontend` / `pnpm run build:web`                                   |
| Both UI consumers                                     | `pnpm run build:ui-consumers`                                                      |
| Go protobuf generation                                | `go generate ./pkg/pb`                                                             |
| Browser protobuf generation                           | `pnpm --filter renop-html run proto`                                               |
| Local integrated build, unpackaged                    | `pwsh ./build.ps1 c nb`                                                            |
| Current-platform packaged / full release matrix       | `pwsh ./build.ps1 c` / `pwsh ./build.ps1`                                          |
| Release payload check                                 | `pwsh ./.github/scripts/test-release-payload.ps1 -DistDir ./dist`                  |
| Disposable database contract                          | `go run ./cmd/renop-dbtest -driver <driver> -dsn <isolated-dsn> -confirm-isolated` |

MySQL integration tests opt in with `RENOP_TEST_MYSQL_DSN` pointing to a disposable test server.
`go test -count=1 -p 1 ./internal/database -run '^TestMySQL'` creates fresh utf8mb4 schemas, runs the full driver
contract and boundary/restart checks, and drops only those generated schemas. The test account needs CREATE/DROP DATABASE.

The optional browser contract in `test/custom-select-browser.test.mjs` uses an already running browser.
Set `RENOP_TEST_BROWSER_CDP` to its debugging origin; `RENOP_TEST_BROWSER_URL` and
`RENOP_TEST_SELECT_SELECTOR` select the loaded page and control. It changes and restores one draft selection.
`test/api-token-browser.test.mjs` uses the same connection with `RENOP_TEST_TOKEN_DIALOG=1` and an open token creation
dialog; it verifies target confirmation, cancellation, and stacked focus behavior, then restores the draft.

`build.ps1` generates Go schemas and builds the frontend (i18n validation and binary assets, JS protobuf, bundles, precompression).
Both browser builds share `scripts/i18n-catalog.mjs`, `scripts/i18n-protobuf.mjs`, and the `@renop/ui` locale loader;
`pnpm run check:i18n` validates and regenerates catalogs for both consumers. Shared locale bindings use the installed
protobufjs toolchain, while control API bindings continue to use `proto/api/v1/api.proto`.
Full frontend builds use PowerShell 7 to prepare the ignored `internal/mail/data/` before Go compilation, including CI
builds.
`scripts/update-disposable-domains.ps1` pins source revisions and checksums, reuses verified local data, and fails on
download or integrity errors. Keep generated mail data out of Git; update source pins and notices when refreshing it.
`go generate ./...` also installs/builds the frontend; do not stack it with an equivalent completed frontend build.
Generation requires `protoc`/`protoc-gen-go`; frontend precompression also requires Go.
Builds regenerate assets and may replace `dist/`; inspect the final diff and output, not just the exit code.

## Before delivery

- Recheck each acceptance criterion and the final diff; resolve TODOs introduced by this task or report a concrete
  blocker.
- Confirm coupled code, permissions, i18n, docs, and generated outputs are consistent; run `git diff --check`.
- Remove only temporary artifacts created by this task; preserve unrelated changes, ignored local data, and secrets.
- Report the result, actual verification, and material remaining gaps concisely; include commit/push status only when
  relevant.
  Never call a plan, partial implementation, failed check, or unpushed change a completed delivery when more was
  requested.
