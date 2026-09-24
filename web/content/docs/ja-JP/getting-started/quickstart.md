---
title: クイックスタート
order: 3
category: はじめに
description: 初回起動、管理者、稼働確認、リポジトリ作成
---

# クイックスタート

## サーバー起動

初回起動時、RenoP はデータベースに `admin` スーパー管理者を作成します。パスワードを明示してください。

```bash
# Linux / macOS
RENOP_DEFAULT_ADMIN_PASSWORD='your-admin-password' ./renop

# Windows (PowerShell)
$env:RENOP_DEFAULT_ADMIN_PASSWORD='your-admin-password'
.\renop.exe
```

未設定ならランダムパスワードを生成し標準出力に一度だけ表示します。直ちに保存して
`http://localhost:3000` を開きます。既定バインドは `0.0.0.0:3000` です。本番環境では TLS または信頼プロキシを
使用してください。

## 既定と新規リポジトリ

新しいデータベースには互換用 Maven リポジトリが 3 件作成されます。

| Path         | Visibility | Policy                |
|:-------------|:-----------|:----------------------|
| `/releases`  | `PUBLIC`   | Maven、再デプロイ無効 |
| `/snapshots` | `PUBLIC`   | Maven、再デプロイ有効 |
| `/private`   | `PRIVATE`  | Maven、認証必須       |

npm、Cargo、Docker、`files` は管理画面から明示的に作成します。Docker イメージと npm パッケージは各リポジトリ
画面で予約後に push できます。Cargo パッケージ名は上流確認後に作成します。Maven 公開には検証済みドメインが必要です。

## 稼働確認

```bash
curl -s http://localhost:3000/api/status/health
# Output: "UP"
```

Protobuf ランタイム指標は `/api/status/instance` です。稼働確認はプロセスが応答することだけを示すため、本番
トラフィック前に実際の認証操作でデータベースとストレージも検証してください。

## 主要な環境変数

| 変数                           | 既定                | 用途                                     |
|:-------------------------------|:--------------------|:-----------------------------------------|
| `RENOP_SETTINGS_DB`            | `renop-settings.db` | 主設定ファイルの保存パス                 |
| `RENOP_REPOSITORIES`           | `repositories.yaml` | 旧設定の移行パス。DB 初期化後は無視      |
| `RENOP_INDEX`                  | `index.json`        | ファイル索引スナップショットのパス       |
| `RENOP_DEFAULT_ADMIN_PASSWORD` | 1 回生成            | `admin` が存在しない場合の初期パスワード |

アカウント、セッション、チーム、API トークン、監査、メッセージはデータベースのデータであり、YAML パス変数はありません。

## 次の手順

- [設定概要](../configuration/overview.md) — TLS、データベース、プロキシ、プレビュー、自動更新
- [リポジトリとミラー](../configuration/repositories.md) — エンジン、公開範囲、上流ミラー、移行、S3
- [Maven / Gradle](../guides/maven-client.md) — ドメイン検証と JVM クライアント
- [Cargo Registry](../guides/cargo-registry.md) — リポジトリ作成と crate 公開
- [Docker Registry](../guides/docker-registry.md) — 事前イメージ作成とクライアント設定
- [npm Registry](../guides/npm-registry.md) — パッケージ予約と npm 互換クライアント設定
