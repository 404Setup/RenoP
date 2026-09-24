---
title: 設定の概要
order: 1
category: 設定
description: 設定ファイル、サーバー、ストレージ、プロキシ、ブランド、更新
---

# 設定の概要

システム設定は専用の SQLite データベース `renop-settings.db` に保存され、`RENOP_SETTINGS_DB` でパスを指定します。管理画面で待受
IP・ポートやアプリケーションデータベース接続を変更できます。待受設定の反映には再起動が必要です。法的文書はフロントエンド、索引操作は
ストレージに統合されています。以下は保存フィールドの例であり、YAML 設定ファイルを作成・編集する必要はありません。

設定は外観とポリシー、アカウントとログイン、公開管理、ストレージとキャッシュ、サービスとネットワーク、メールに分類されます。各ページの下書きと保存は独立しています。

初回起動時のみ既存の `config.yaml`（または `RENOP_CONFIG`）を取り込み、コミット後にアーカイブします。以降はデータベース
の設定だけを使用します。暗号鍵を含む設定データベースとアプリケーションデータベースの両方をバックアップしてください。

## 設定ファイル

| ファイル            | 上書き              | 用途                                                                   |
|:--------------------|:--------------------|:-----------------------------------------------------------------------|
| `renop-settings.db` | `RENOP_SETTINGS_DB` | サーバー、データベース、事前確認、プロキシ、フロントエンド、監査、更新 |
| データベース        | データベース DSN    | エンジン、公開範囲、ミラー、Maven 方針、S3                             |
| `index.json`        | `RENOP_INDEX`       | ストレージから再構築できるファイル索引スナップショット                 |

アカウント、API トークン、セッション、チーム、監査、メッセージはデータベースに保存し、YAML では設定しません。認証情報を含む
場合があるため、設定ファイルはサービス実行アカウントのみが読めるように権限を設定してください。

リポジトリ定義はデータベースのバックアップに含まれます。旧 YAML はデータベースにリポジトリ設定がない場合のみ
インポートされ、既存の設定を上書きしません。古い RenoP へのロールバックが必要な場合に備え、
移行元の保管ファイルを別途保存してください。

## 保存される設定フィールド

### ストレージとドキュメント事前確認

```yaml
storage_path: "storage"
enable_javadoc_preview: true
javadoc_extract_path: ""
max_javadoc_size_mb: 48
enable_cargodoc_preview: true
cargodoc_extract_path: ""
max_cargodoc_size_mb: 128
```

抽出先が空の場合はシステム一時領域を使用します。`/javadoc` または `/cargodoc` で公開する前にパスとファイルサイズを
検証します。

### `server` のネットワークとセキュリティ

```yaml
server:
  host: "0.0.0.0"
  port: 3000
  ssl_enabled: false
  ssl_cert_path: ""
  ssl_key_path: ""
  domains: [ "localhost" ]
  cors_origins: [ ]
  enable_compression: false
  file_cache_size_mb: 16
  max_active_requests: 512
  trusted_proxies: [ ]
  cdn_ip_header: "X-Forwarded-For"
  debug_mode: false
  gpg:
    key_servers: [ "https://keys.openpgp.org", "https://keyserver.ubuntu.com" ]
```

`domains` は公開ホスト名と既定の CORS ホストです。`cors_origins` には完全一致の Origin、ホスト、ワイルドカードを指定でき、
`*` は
すべてを許可します。転送元 IP ヘッダーは接続元が `trusted_proxies` に一致する場合のみ信頼します。ホスト、ポート、
TLS、圧縮、デバッグ、一部キャッシュの変更には再起動が必要です。

GitHub OAuth は `server.github_oauth` に保存し、クライアント ID と書き込み専用シークレットは管理画面で設定します。

その他の [OAuth プロバイダー](../security/oauth-login.md)は `server.oauth_providers`
で設定します。管理画面には公式プリセットがあり、認証情報を返さずに保存し、新しい認可設定を即座に適用します。

### `database` 接続

```yaml
database:
  driver: "sqlite3"
  dsn: "renop.db"
  max_open_conns: 25
  max_idle_conns: 25
  conn_max_lifetime_sec: 300
```

`sqlite3`（または `sqlite`）、`mysql`、`postgres`、ネイティブ `clickhouse` に対応します。
[データベース設定](./database.md)を参照してください。

### `proxy` 送信ルート

```yaml
proxy:
  selected: ""
  proxies:
    - name: "corp_proxy"
      url: "http://proxy.internal:8080"
      username: ""
      password: ""
```

HTTP、HTTPS、SOCKS5 プロキシを最大 16 件設定できます。[送信プロキシ](./outbound-proxy.md)を参照してください。

### `frontend` ブランド設定

```yaml
frontend:
  id: "renop"
  title: "RenoP Package Registry"
  description: "Self-hosted package repository"
  organization_website: ""
  organization_logo: "/svg/logo.svg"
  background_url: ""
  font_preset: "system"
  font_url: ""
  icp_license: ""
  public_security_filing: ""
```

URL は使用前に検証します。背景画像は WebP 形式と容量制限を満たす必要があります。
`font_preset` には `system`、`inter`、`noto_sans`、`open_sans`、`source_sans`、`custom` を指定できます。
プリセットはローカルにインストールされたフォントを使用します。カスタム値には WOFF2、WOFF、TTF の直接 URL または
Google Fonts の CSS URL を指定できます。バックグラウンドで取得し、主要フォントの読み込み完了後に有効になります。

### `updater` 更新方針

```yaml
updater:
  channel: "release"
  mode: "manual"
```

`channel` は `release` または `nightly`、`mode` は `manual`、`auto_check`、`auto_install` です。自動確認は
プロセススケジューラが統括し、結果を管理者へ通知します。

[法的文書と Cookie の設定](./legal.md)
