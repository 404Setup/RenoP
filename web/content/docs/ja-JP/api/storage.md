---
title: ストレージとアップロード API
order: 10
category: API リファレンス
description: リポジトリ直接操作と制限付き再開可能アップロード
---

# ストレージとアップロード API

直接ストレージルートは Maven、`files`、管理対象のネイティブリポジトリ用です。npm、Cargo、Docker はネイティブプロトコルを使用します。
変更操作では API Token scope、リポジトリ権限、形式、Maven ドメインポリシーをすべて確認します。

## リポジトリ直接操作

正規パスは `/{repo}/{path...}` です。読み取りは HTTP validator と byte range に対応します。`HIDDEN` は
一覧に出ませんが正確なパスで読めます。`PRIVATE` は認可が必要です。

### ダウンロード

- **要求**: `GET /{repo}/{path}` または `HEAD /{repo}/{path}`
- ローカルにないファイルは有効ミラーから解決し、設定済みポリシーに従ってキャッシュできます。

### アップロード

- **要求**: `PUT /{repo}/{path}`
- **認証**: パスワード、または `repository:publish` を持つ API Token と現在の書き込み/ドメイン権限。
- Maven は検証済みドメイン下の有効な座標とメタデータのみ受理します。`files` は安全化した任意パスと
  上書きを許可します。

### 削除

- **要求**: `DELETE /{repo}/{path}`
- **認証**: `repository:delete` を持つ API Token または許可された資格情報と、現在の削除権限。

## 分割再開可能アップロード

メタデータは protobuf、各 part は生バイナリです。サーバーが最終保存先を所有し、part サイズと session 数を
制限し、放棄された一時ファイルを削除します。

### 初期化

- **パス**: `POST /api/upload/chunked/`
- **Content-Type**: `ChunkedUploadInitRequest` の `application/x-protobuf` / `application/json`。
- `purpose` は `storage` または `updater`。storage の `path` はリポジトリ名から始めます。

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

### part のアップロード

- **パス**: `PUT /api/upload/chunked/{upload_id}/{index}`
- **Content-Type**: `application/octet-stream`。
- part は並列送信できます。受理済み index の再送は冪等で、長さが違う part は拒否されます。

### 完了または中止

- **完了**: `POST /api/upload/chunked/{upload_id}/complete`
- **中止**: `DELETE /api/upload/chunked/{upload_id}`
- 完了処理は 1 件だけ成功し、全 part と権限を再確認してリポジトリゲート経由で確定します。

```json
{
  "status": "created",
  "message": "",
  "path": "releases/com/example/app/1.0.0/app-1.0.0.jar",
  "release_id": ""
}
```

Maven で GPG が必須の場合、隔離中は `release_id` を含む `202 Accepted` になることがあります。
`purpose=updater` の成功はリポジトリパスではなく `ready_to_restart` を返します。

## 管理対象ネイティブリソース API

APK、apt、Conan、Conda/Conda native、rpm/yum の公開リソースを管理する JSON API です。変更にはブラウザー Cookie
セッションが必要です。クライアントは引き続きネイティブパスを使用し、Token のリポジトリ scope と現在のリソース権限の両方を確認します。

| 操作     | エンドポイント                                                          | 用途                                                                                     |
|----------|-------------------------------------------------------------------------|------------------------------------------------------------------------------------------|
| `GET`    | `/api/native/repositories/{repo}/resources`                             | リソース一覧。`name` で詳細を選び、`limit` は1–100、`offset` は最大10000です。           |
| `POST`   | `/api/native/repositories/{repo}/resources`                             | `{"name":"example"}` でリソースを登録します。リポジトリの公開権限が必要です。            |
| `PUT`    | `/api/native/repositories/{repo}/resources`                             | `name`、`description`、公開 `signing_key` を更新します。L3 が必要です。                  |
| `DELETE` | `/api/native/repositories/{repo}/resources`                             | `name` で指定した空のリソースを解除します。L4 が必要で、審査待ちは許可されません。       |
| `PUT`    | `/api/native/repositories/{repo}/resources/members`                     | `name`、`username`、`level`（0–4、削除は-1）を指定します。最後の L4 所有者は保持します。 |
| `GET`    | `/api/native/repositories/{repo}/resources/key?name={name}`             | 公開者の公開鍵を取得します。未公開リソースの可視性制限も適用します。                     |
| `GET`    | `/api/native/repositories/{repo}/resources/users?name={name}&q={query}` | L3 権限編集用に、閲覧可能なユーザー名を最大8件検索します。                               |

`new_packages` は最初の公開、`every_version` は各バージョンを審査します。`202` のファイルは署名完了または審査を待って非公開です。審査には
`X-RenoP-Review-ID` が返ります。ファイル名はメタデータと一致する必要があり、生成インデックスはアップロードできません。APK と
RPM は設定した公開鍵による署名検証が必須です。Conan は `scripts/conan/sign.py` の署名マニフェストを必要とします。APT
はリポジトリインデックスを署名し、Conda は追加の分離署名を要求しません。[リポジトリ設定](/docs/configuration/repositories)
を参照してください。
