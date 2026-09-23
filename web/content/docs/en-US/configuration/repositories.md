---
title: Repositories & Mirrors
order: 2
category: Configuration
description: Repository engines, visibility, upstream mirrors, migration, and S3 storage
---

# Repositories & Mirrors

Repository definitions are stored in the database and edited through repository management. On the first upgrade,
RenoP imports `repositories.yaml` (or `RENOP_REPOSITORIES`) only if no database snapshot exists, then archives the
source as `repositories.yaml.migrated.<id>`. Invalid input stops startup without replacing the source. An existing
database snapshot always wins, including an empty repository set. New installations leave the repository set empty and
do not create a YAML file. Back up the database and protect the archived file, which can contain
S3 and mirror credentials. A repository name is an immutable lowercase slug and the first URL segment.

Fresh installations start with an empty repository list. Create repositories explicitly in the administration page;
existing database snapshots and one-time legacy imports retain their configured repositories.

## Legacy migration example

```yaml
repositories:
  releases:
    name: releases
    format: maven
    visibility: PUBLIC
    allow_redeployment: false
    require_gpg_signature: true
    publication_review: every_version
    download_statistics: true
    mirrors: []
  crates:
    name: crates
    format: cargo
    visibility: PUBLIC
    mirrors: []
  containers:
    name: containers
    format: docker
    visibility: PRIVATE
    allow_redeployment: false
    mirrors: []
```

## Repository fields

| Field                   | Default        | Description                                                                                                               |
|:------------------------|:---------------|:--------------------------------------------------------------------------------------------------------------------------|
| `name`                  | Required       | Immutable repository slug and URL prefix                                                                                  |
| `format`                | `maven`        | `maven`, `maven-classic`, `files`, `npm`, `cargo`, `docker`, `conan`, `conda`, `conda-native`, `apk`, `apt`, `rpm`, `yum` |
| `visibility`            | `PUBLIC`       | `PUBLIC`, `HIDDEN`, or `PRIVATE`                                                                                          |
| `allow_redeployment`    | `false`        | Maven version redeployment or replacement in files/Docker, when supported                                                 |
| `require_gpg_signature` | `false`        | Require detached OpenPGP validation for Maven publication                                                                 |
| `publication_review`    | `off`          | Maven/npm/Cargo/Docker review policy: `off`, `new_packages`, or `every_version`                                           |
| `download_statistics`   | Engine default | Enabled for Maven/npm/Cargo/Docker; unstructured `files` opts in                                                          |
| `mirrors`               | `[]`           | Ordered upstream definitions                                                                                              |
| `s3`                    | omitted        | Repository-specific S3-compatible storage                                                                                 |

For npm and Docker, `new_packages` reviews the explicit creation request before reserving the name; `every_version`
also reviews every later version or manifest. Maven and Cargo have no empty-package creation step, so their
`new_packages` policy reviews the first publication. Mirror imports bypass review for every engine.

`maven-classic` changes only the frontend layout and retains Maven publication rules. `files` is unstructured and does
not generate checksums, POM files, or signature validation. Maven repositories can migrate to `files` and back without
moving objects; returning to Maven rebuilds the catalog and restores saved Maven policy. Migration preserves the
repository's effective download-statistics switch.

Uploads and mirror downloads in `files` repositories preserve neighboring files even when paths contain `SNAPSHOT`
or names end in `.md5`, `.asc`, or `-javadoc.jar`. Maven version cleanup and Javadoc extraction apply only to Maven.

Publication review supports Maven, npm, Cargo, Docker, and managed native resources. Maven review forces
`allow_redeployment` to `false`; npm
keeps its immutable
version and dist-tag transaction. Local files remain hidden until a repository moderator or system administrator
approves them, and mirror content is never reviewed. Pending reviews prevent repository reconfiguration, deletion, or
engine migration.

An `npm` repository requires package reservation before publication, stores immutable semantic versions and dist-tags,
supports scoped private packages and L0-L4 teams, and can mirror exact package names or `@scope/*` patterns.

### Visibility

- **PUBLIC**: Anonymous reads and discovery are allowed.
- **HIDDEN**: Anonymous and unprivileged catalogs omit the repository, and profile memberships remain unlisted.
  Managers and viewers with explicit repository-browser permission can discover it. Exact known file paths remain
  readable.
- **PRIVATE**: Reads, listings, and writes require explicit authorization. Private Docker images additionally enforce
  image-level L0-L4 membership.

## Upstream mirrors

When a local object is missing, RenoP may stream it from an ordered enabled mirror. Successful fetches can be persisted
without buffering the whole body. Cargo and Docker prevent local creation when an applicable upstream name exists.

```yaml
mirrors:
  - name: "central"
    url: "https://repo1.maven.org/maven2"
    persist: true
    cache_ttl_secs: 86400
    negative_cache: true
    timeout_secs: 30
    proxy: ""
    allow_artifacts: []
    deny_artifacts: []
```

| Field             | Default  | Description                                          |
|:------------------|:---------|:-----------------------------------------------------|
| `name`            | Required | Unique mirror name within the repository             |
| `url`             | Required | Upstream base URL                                    |
| `persist`         | `true`   | Store successful responses in the repository backend |
| `cache_ttl_secs`  | `86400`  | Positive-cache lifetime                              |
| `negative_cache`  | `true`   | Cache supported upstream misses                      |
| `timeout_secs`    | `30`     | Per-request upstream timeout                         |
| `proxy`           | `""`     | Global route; `direct`; or an exact named proxy      |
| `allow_artifacts` | `[]`     | Format-aware allow rules                             |
| `deny_artifacts`  | `[]`     | Format-aware deny rules; deny wins                   |

Mirror credentials, when required, use the structured authorization fields. Do not embed secrets in `url`.

## S3-compatible storage

Each repository may use Disk or an independent S3-compatible backend. Changing the storage or engine is serialized
with active uploads, deletes, GPG commits, and mirror writes by the repository gate.

```yaml
s3:
  enabled: true
  endpoint: "https://s3.us-east-1.amazonaws.com"
  bucket: "my-renop-bucket"
  key_prefix: "releases/"
  region: "us-east-1"
  access_key_id: "YOUR_ACCESS_KEY"
  secret_access_key: "YOUR_SECRET_KEY"
  force_path_style: false
  redirect_downloads: false
```

`force_path_style` is commonly required by MinIO. With `redirect_downloads: true`, RenoP authorizes the request and
returns a short-lived presigned redirect; otherwise it streams the object through the server.

`capacity_limit_bytes` is the per-repository installed-byte limit; `0` means unlimited. The UI edits it in MiB. Disk and
S3 commits count artifacts, generated checksums, pending-review objects, and cached mirror content, excluding temporary
staging copies. Capacity is reserved before commits so concurrent uploads share the same limit. Excess writes return
`507` with `repository_capacity_exceeded`; a valid mirror response can still stream without being cached. Existing files
remain readable when the limit is lowered below usage. After out-of-band storage changes, rebuild the index or restart
to remeasure usage. Omitting the optional field preserves the existing limit for older clients.

## Native package repositories

Conan supports recipe and binary revisions through the native client. Conda and Conda native accept `.conda` and
`.tar.bz2` packages in platform directories such as `noarch/` and generate `repodata.json`. APK generates
`APKINDEX.tar.gz` beside packages in architecture directories such as `x86_64/`. apt accepts `.deb` files, generates
`Packages`, `Packages.gz`, and `dists/<suite>/Release`, and maps `pool/<component>/` to that component. rpm/yum
generates `repodata/repomd.xml` and its referenced metadata.

Reserve each hosted native resource in the repository browser before uploading. Resource identities come from package
metadata or Conan recipe references and do not require a global-team prefix. L0 reads, L1 publishes, L2 replaces or
deletes versions, L3 manages settings and members, and L4 owns the resource. At least one L4 owner must remain.
Repository write permission alone does not grant access to another resource. Native mirror repositories remain
pull-only.

`new_packages` reviews the first publication after reservation; `every_version` reviews each native version or Conan
revision. Incomplete signatures and pending reviews stay out of downloads and generated indexes, including after
restart. APK requires an RSA PEM publisher key and a valid native signature covering the control and payload; RPM
requires a trusted OpenPGP signature covering the payload directly or through a signed payload digest. Configure the
public key in resource settings. APT uses signed repository indexes rather than a detached signature for each `.deb`;
Conda uses its native package hashes and has no extra `.asc` upload requirement.

New uploads cannot replace generated repository indexes. Existing legacy metadata is preserved until an administrator
removes it. An automatic index is limited to 10,000 packages; split larger collections into repositories or native
subdirectories. Package uploads are bounded to 8 GiB, with at most 256 unpublished files per resource and 16,384 across
the instance. Existing uncatalogued artifacts remain readable; an administrator must handle them before a conflicting
managed upload can replace them.

Generated APK, apt, and rpm metadata is signed with independent persistent keys in the private settings database. Public
keys are served at `renop.rsa.pub`, `renop.asc`, and `repodata/repomd.xml.key`, respectively. apt also serves
`InRelease` and `Release.gpg`; rpm serves `repomd.xml.asc`. Import the key before using the client commands shown in the
repository browser. rpm package signatures still belong to their publishers and require the corresponding publisher
keys.

Conan requires its official signing-extension manifest and an OpenPGP signature. Install `scripts/conan/sign.py` at
`<CONAN_HOME>/extensions/plugins/sign/sign.py`, register the public key on the resource, and set `RENOP_CONAN_GPG_KEY`
to the private-key fingerprint and `RENOP_CONAN_GPG_KEYRING` to a trusted dearmored public keyring for `gpgv`. Uploads
remain hidden until every manifest file and signature arrives. Byte-identical retries of published Conan files are
idempotent.

```sh
conan cache sign "PACKAGE/*"
conan upload "PACKAGE/*" --remote "REPOSITORY" --confirm
```

## Shared file contents

Identical Disk files share immutable contents through hard links while keeping their logical paths. Writes replace a
file atomically, and deleting one path preserves other references. One background task scans existing files in batches,
skips busy repositories, and pauses between batches. Mutable indexes, staging files, and expiring Disk mirror caches
keep independent files.

S3 payloads of at least 64 KiB use logical references to shared content; smaller objects remain direct objects. Hashes
and references are persisted in the private file index. Known records avoid repeated metadata probes. Legacy objects
without a trusted hash are left untouched until an ordinary upload or complete download through RenoP supplies one;
background deduplication does not download them just to compare contents. Index rebuilding may use LIST pages and verify
missing reference metadata once. Regular collection processes the indexed reclamation queue rather than repeatedly
scanning the bucket.

A complete S3 backup includes the private `.renop-content-v1` namespace alongside repository objects and the private
index. Independent RenoP deployments should use separate key prefixes. Unreferenced contents are reclaimed after a grace
period. Repository capacity and download accounting continue to use logical file sizes, regardless of physical sharing.
