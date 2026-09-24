---
title: Cargo (Rust) Registry
order: 2
category: ガイド
description: Cargo リポジトリ、Sparse Index、公開、所有権管理、Cargodoc
---

# Cargo (Rust) Registry ガイド

クライアント設定を行う前に、形式 `cargo` のリポジトリを作成します（例: `crates`）。RenoP は Cargo Sparse
Index を実装しており、Git リポジトリ全体のクローンを行うことなく、crate アーカイブをストリーミング転送できます。

## Cargo 設定 (`.cargo/config.toml`)

```toml
[registries.renop]
index = "sparse+http://localhost:3000/crates/"

# 任意: 既定の crates.io 上流を置き換える場合
# [source.crates-io]
# replace-with = "renop"
# [source.renop]
# registry = "sparse+http://localhost:3000/crates/"
```

本番環境では HTTPS を使用してください。リポジトリの `config.json` がダウンロードや API のエンドポイントを通知します。非公開リポジトリでは
`auth-required` を設定し、インデックスと crate の読み取りに認証情報が求められます。

## 認証

専用の有効期限付き API トークンを作成します。初回の公開には通常 `repository:read`、`repository:publish`、
`package:create` 権限を使用します。アーカイブや取り下げ（yank）には `package:lifecycle`、所有者管理には `team:manage` 権限を追加してください。

```bash
cargo login --registry renop
# プロンプトが表示されたら RenoP のトークンを貼り付けます
```

Cargo は認証情報を `~/.cargo/credentials.toml` に保存します。

```toml
[registries.renop]
token = "your_renop_token"
```

トークンは `Authorization` ヘッダー値として検証されます。RenoP はトークンのスコープや対象制限を、現在の
アカウント権限、リポジトリ権限、パッケージチーム権限と突き合わせて厳密に評価します。

## 依存関係と公開

### 依存関係の追加 (`Cargo.toml`)

```toml
[dependencies]
my-crate = { version = "0.1.0", registry = "renop" }
```

### crate の公開

```bash
cargo publish --registry renop
```

初回の公開成功時に正規化された名前を予約し、公開者に L4（所有者）権限を付与します。ローカルまたは適用ミラーに同名が存在する場合は拒否されます。
上流の確認が完了しない場合は `503` で安全に失敗し、パッケージ名は予約されません。後続のバージョン公開にはチームの公開権限が
必要です。

公開審査を有効にすると、アーカイブの保存後に `cargo publish` は `202 Accepted` を返します。リポジトリの
審査者またはシステム管理者が承認するまで、crate は sparse インデックスおよび公開カタログには表示されません。
`new_packages` では最初の公開バージョンが承認されるまで審査が適用されます。ミラー由来の crate は審査対象外です。

### 検索、取り下げ（yank）、復元（unyank）

```bash
# crate の検索
cargo search --registry renop my-crate

# 特定バージョンの取り下げ
cargo yank --registry renop --version 0.1.0 my-crate

# 取り下げの解除（復元）
cargo yank --registry renop --undo --version 0.1.0 my-crate
```

所有者はパッケージ詳細画面から L0-L4 の共同作業者や招待を管理できます。ミラー由来の crate は上流由来として表示され、
ローカルの所有者は存在せず読み取り専用となります。

## Cargodoc

RenoP は rustdoc の出力を検証し、安全なサンドボックスビューアに展開します。システム設定で Cargodoc とファイルサイズ上限を有効化してください。

URL: `http://localhost:3000/cargodoc/{repo}/{crate}/{version}/index.html`

## リソースのロック

管理者とリポジトリ審査者は、パッケージ詳細画面からパッケージ全体または個別バージョンをロックし、公開理由を設定できます。
書き込み禁止にすると更新や削除が停止されます。読み取り禁止にするとメタデータの表示も担当者と共同作業者のみに制限され、
すべてのファイルダウンロードと Cargodoc プレビューが遮断されます。チームメンバーはメタデータを閲覧できますが、
ロックされた内容を変更することはできません。手動ロックを解除した後も、システムロックが優先して適用されます。
ロックされていない他のバージョンは通常どおり公開できますが、パッケージ全体を書き換える操作は禁止されます。

リクエスト形式やエラーコードの詳細は [Cargo API](/api/cargo) を参照してください。
