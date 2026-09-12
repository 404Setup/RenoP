<p align="center">
  <img src="assets/banner.svg" alt="RenoP" width="720">
</p>

# RenoP

A self-hosted package repository for Maven, Cargo, npm, Docker/OCI, and generic files, with a web management interface
embedded in one executable.

[Website](https://renop.pkg.one/) ·
[Documentation](https://renop.pkg.one/docs) ·
[Downloads](https://renop.pkg.one/download) ·
[Releases](https://github.com/404Setup/RenoP/releases)

## Features

- Local disk or S3-compatible storage; upstream mirrors with per-mirror proxy routing.
- SQLite, PostgreSQL, MySQL, and native ClickHouse databases.
- Password, Passkey, GitHub OAuth, and scoped API tokens.
- Package and global teams, publication reviews, ownership transfers, and quotas.
- GPG verification, audit logs, messages, download statistics, and online updates.
- System service management on Windows, Linux, macOS, and BSD, plus Caddy integration.

## Quick start

[Download a build](https://renop.pkg.one/download) for your platform and extract it to `renop` or `renop.exe`.
The download page supports ZIP conversion for raw Brotli packages.

Linux / macOS:

```bash
chmod +x ./renop
RENOP_DEFAULT_ADMIN_PASSWORD='replace-this-password' ./renop
```

Windows PowerShell:

```powershell
$env:RENOP_DEFAULT_ADMIN_PASSWORD = 'replace-this-password'
.\renop.exe
```

Open `http://localhost:3000` and sign in as `admin`. If the password variable is omitted, the initial password is
generated and printed once to the server log. Configuration is created on first start; manage repositories and
settings through the web interface.

The default listener is `0.0.0.0:3000`. Follow the
[production checklist](https://renop.pkg.one/docs/deployment/production-checklist) before exposing the instance.
See [Quickstart](https://renop.pkg.one/docs/getting-started/quickstart) for repository setup.

## Documentation

| Topic           | Guides                                                                                                                                                                                                                                                                           |
|-----------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Package clients | [Maven](https://renop.pkg.one/docs/guides/maven-client), [Cargo](https://renop.pkg.one/docs/guides/cargo-registry), [npm](https://renop.pkg.one/docs/guides/npm-registry), [Docker/OCI](https://renop.pkg.one/docs/guides/docker-registry)                                       |
| Configuration   | [Overview](https://renop.pkg.one/docs/configuration/overview), [repositories](https://renop.pkg.one/docs/configuration/repositories), [databases](https://renop.pkg.one/docs/configuration/database)                                                                             |
| Deployment      | [Containers](https://renop.pkg.one/docs/deployment/containers), [services](https://renop.pkg.one/docs/deployment/daemon), [reverse proxy](https://renop.pkg.one/docs/deployment/reverse-proxy), [backup and recovery](https://renop.pkg.one/docs/deployment/backup-and-recovery) |
| Security        | [Security model](https://renop.pkg.one/docs/security/security-model), [tokens and keys](https://renop.pkg.one/docs/security/tokens-and-keys)                                                                                                                                     |
| API             | [Reference](https://renop.pkg.one/api/README), [OpenAPI](https://renop.pkg.one/assets/openapi.yaml)                                                                                                                                                                              |
| Support         | [Troubleshooting](https://renop.pkg.one/docs/guides/troubleshooting), [issues](https://github.com/404Setup/RenoP/issues)                                                                                                                                                         |

## Building from source

Use the [Illium Go](https://github.com/404Setup/go/releases) matching [go.mod](go.mod), PowerShell 7, Node.js
24+,
the pnpm version pinned in [package.json](package.json), `protoc`, and `protoc-gen-go`.

```powershell
pnpm install --frozen-lockfile
pwsh ./build.ps1 c nb
```

This generates protobuf bindings, builds the embedded frontend, and compiles an unpackaged binary for the current
platform. Use `pwsh ./build.ps1 c` for a raw Brotli release package or `pwsh ./build.ps1` for all targets.
See [AGENTS.md](AGENTS.md) for contribution and verification guidance.

## License

[Mozilla Public License 2.0](LICENSE), incompatible with secondary licenses.
See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for dependency licenses.
