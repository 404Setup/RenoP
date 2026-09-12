---
title: API インデックス
order: 1
category: API リファレンス
description: RenoP の HTTP、REST、RPC API の概要
---

# RenoP HTTP API

RenoP は、管理自動化、クライアント統合、監視向けの HTTP API を提供します。既定の待受先は
`http://localhost:3000` です。


[API リファレンス](/api) では、システム管理、クライアント統合、状態監視に関する各エンドポイントを提供します。仕様の確認やクライアント生成には、生の [OpenAPI ファイル](/assets/openapi.yaml) をダウンロードして利用できます。

## ルート構成

| プレフィックス                      | 用途                                                   |
|:------------------------------------|:-------------------------------------------------------|
| `/api/*`                            | 認証、アカウント、設定、状態、メッセージなどの管理 API |
| `/{repo}/*`                         | リポジトリ形式に応じたアップロード、ダウンロード、削除 |
| `/{npm-repo}/*`                     | npm packument、tarball、publication、dist-tag、search  |
| `/index/*` または `/{repo}/index/*` | Cargo Sparse Index                                     |
| `/v2/*`                             | Docker / OCI Distribution v2                           |
| `/javadoc/*`                        | サンドボックス化された Javadoc ビューア                |
| `/cargodoc/*`                       | サンドボックス化された Cargodoc ビューア               |

## ワイヤ形式と Protobuf

スキーマに基づく管理 API はバイナリ protobuf を使用します。`Content-Type: application/x-protobuf` を指定してください。
要求は `application/protobuf` と `application/octet-stream` も受け付け、Content-Type が未指定の場合は protobuf です。
JSON 本文は拒否され、エンドポイントに応じて `400` または `415` となります。応答は常に `application/x-protobuf` で、`Accept` で JSON に
切り替えることはできません。稼働バージョンの `proto/api/v1/api.proto` を使用してください。

制御要求の上限は 1 MiB で、各エンドポイントのより小さい上限も維持します。protobuf メッセージの JSON 例は
デコード後のフィールドを示し、JSON 転送形式ではありません。JSON 専用のエンドポイント、パッケージのネイティブ
プロトコル、アップロード部分、ヘルステキスト、個別エラーは宣言された形式を維持します。

## 認証方式

- **ブラウザ Cookie**: `renop_session=<session_id>`。HttpOnly セッション秘密値はヘッダーや URL では
  受け付けません。
- **Bearer API Token**: `Authorization: Bearer <token>`。Token の能力は常にアカウントの現在の権限と
  組み合わせて評価されます。
- **パッケージプロトコル用 Basic Auth**: `Authorization: Basic <base64(user:password_or_token)>`。

Basic Auth で管理 API を呼び出すことはできません。URL クエリの資格情報と `Authorization: Session` は
拒否されます。

## 主な HTTP ステータス

| コード                    | 意味             | 用途                                   |
|:--------------------------|:-----------------|:---------------------------------------|
| `200 OK`                  | 成功             | レスポンス本文を伴う成功               |
| `201 Created`             | 作成済み         | リソースまたはアップロードを初期化     |
| `204 No Content`          | 成功             | 本文を伴わない成功                     |
| `400 Bad Request`         | 不正なリクエスト | パラメーターまたは本文が不正           |
| `401 Unauthorized`        | 未認証           | 資格情報がない、または不正             |
| `403 Forbidden`           | 許可なし         | 権限不足、または一時的な IP ブロック   |
| `404 Not Found`           | 見つからない     | 対象リソースが存在しない               |
| `409 Conflict`            | 競合             | 状態が競合、またはリソースが既に存在   |
| `429 Too Many Requests`   | レート制限       | 許可された要求頻度を超過               |
| `503 Service Unavailable` | 一時利用不可     | 過負荷、または依存先が一時的に利用不可 |

## API リファレンス

- [認証 API](./authentication.md)
- [API Token とユーザー](./tokens.md)
- [Maven API](./maven.md)
- [Cargo API](./cargo.md)
- [Docker / OCI API](./docker.md)
- [npm Registry API](./npm.md)
- [グローバルチーム API](./global-teams.md)
- [公開クォータ API](./publication-quotas.md)
- [レビュー API](./reviews.md)
- [メッセージセンター API](./messages.md)
- [ストレージとアップロード API](./storage.md)
- [設定 API](./settings.md)
- [状態とテレメトリ API](./status.md)
- [GPG 暗号 API](./gpg.md)
- [レート制限](./rate-limit.md)
- [アップデーター API](./updater.md)
