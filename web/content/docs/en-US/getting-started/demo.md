---
title: Demo mode
order: 5
category: Getting Started
description: Explore preset data without changing the demonstration dataset
---

# Demo mode

Run `--demo` to create an isolated preset dataset on the first start, then serve it read-only. The sign-in form prefills
`admin` / `12345678`. Presets cover accounts, sessions, teams, Maven/Cargo/npm/Docker packages, files metadata, reviews,
tickets, quotas, notifications, statistics, audit logs, and mail history.

```bash
./renop --demo
./renop --demo --demo-temp
```

Use `--demo --demo-temp` to save system settings and repository definitions. These edits persist; restart with only
`--demo` to present them read-only. `--demo-temp` alone is rejected. Other account/content mutations and log writes
remain disabled in both modes.

The default files are `renop-demo.db` and `renop-demo-settings.db`, overridden by `RENOP_DEMO_DATABASE` and
`RENOP_DEMO_SETTINGS_DB`. Normal installation data is separate. Existing demo datasets are retained, including an empty
repository set. Only databases are created: artifact downloads and documentation previews are unavailable, and no
artifact, index, or log files are written. Authentication sessions remain in bounded process memory; preset session rows
cannot sign in.

`GET /api/demo` returns binary protobuf `DemoInfo` with `enabled`, `temporary`, and the public demonstration
credentials. In normal mode it contains no credentials. Rejected mutations use `403` and `demo_read_only`; unavailable
artifacts use `demo_file_unavailable`. Background mail, mirror, maintenance, and update workers do not run.
