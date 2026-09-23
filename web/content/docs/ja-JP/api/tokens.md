---
title: API トークンとユーザー
order: 3
category: API リファレンス
description: きめ細かな API トークンのライフサイクル、認証境界、および管理者ユーザー エンドポイント
---

# API トークンとユーザー

API トークンは、自動化タスクや各種クライアント向けの永続的な資格情報です。RenoP はシークレットの SHA-256
ハッシュのみを保存し、平文は作成時またはローテーション時に一度だけ返されます。

すべてのリクエストは次の 2 つの独立した検証に合格する必要があります。

- トークンにエンドポイントで要求されるスコープが含まれていること。
- トークンを所有するアカウントが、対象リソースに対してその操作を実行する権限を保持していること。

アカウントの役割やパッケージ チームの権限が変更された場合、トークンを再生成することなく即座に反映されます。

## API トークンの管理

トークン管理エンドポイントには HttpOnly の `renop_session` Cookie が必要です。API トークンやパスワード、クエリ パラメーターでトークン
シークレットを管理することはできません。

### 割り当て可能なスコープの一覧取得

`GET /api/auth/profile/api-tokens/scopes`

応答は現在のアカウント権限によって除外されます。一般ユーザーに管理者スコープが提示されることはありません。

```json
{
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "target_kinds": {
    "repository:read": "repository",
    "repository:publish": "repository",
    "package:metadata": "package"
  },
  "target_limit": 128
}
```

### トークンの作成

`POST /api/auth/profile/api-tokens`

```json
{
  "name": "CI publishing",
  "scopes": ["repository:read", "repository:publish"],
  "targets": {
    "repository:publish": ["releases"]
  },
  "expires_at": 1798761600000
}
```

`expires_at` は作成から 5 分から 5 年までの任意の Unix ミリ秒タイムスタンプです。省略または null
を指定すると有効期限のないトークンが作成されます。1 アカウントあたり最大 50 個まで保持できます。

```json
{
  "token": {
    "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
    "name": "CI publishing",
    "scopes": ["repository:publish", "repository:read"],
    "targets": {"repository:publish": ["releases"]},
    "created_at": 1787731200000,
    "expires_at": 1798761600000
  },
  "secret": "rnp_pat_EXAMPLE_REDACTED_COPY_THE_REAL_VALUE_ONCE"
}
```

### トークン メタデータの一覧取得

`GET /api/auth/profile/api-tokens`

応答には機密でないメタデータと上限数が含まれ、トークン シークレットは含まれません。

```json
{
  "tokens": [
    {
      "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
      "name": "CI publishing",
      "scopes": ["repository:publish", "repository:read"],
      "targets": {"repository:publish": ["releases"]},
      "created_at": 1787731200000,
      "expires_at": 1798761600000,
      "disabled": false
    }
  ],
  "limit": 50
}
```

### トークンの編集

`PUT /api/auth/profile/api-tokens/{token_id}`

シークレットを変更することなく、既存トークンの表示名、スコープ、またはターゲット制限を更新します。

```json
{
  "name": "CI publishing updated",
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "targets": {
    "repository:publish": ["releases"]
  }
}
```

エンドポイントは更新されたメタデータを返します。

```json
{
  "token": {
    "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
    "name": "CI publishing updated",
    "scopes": ["package:metadata", "repository:publish", "repository:read"],
    "targets": {"repository:publish": ["releases"]},
    "created_at": 1787731200000,
    "expires_at": 1798761600000
  }
}
```

### トークンのローテーション

`POST /api/auth/profile/api-tokens/{token_id}/rotate`

名前やスコープを維持したまま、指定したトークンのシークレットを再生成します。以前のシークレットは直ちに無効化されます。

```json
{
  "token": {
    "id": "07cdcf2e-0828-4a29-9817-cf771cc9fb0a",
    "name": "CI publishing updated",
    "scopes": ["package:metadata", "repository:publish", "repository:read"],
    "targets": {"repository:publish": ["releases"]},
    "created_at": 1787731200000,
    "expires_at": 1798761600000
  },
  "secret": "rnp_pat_NEW_REGENERATED_SECRET_VALUE_COPY_ONCE"
}
```

### トークン状態の更新

`PUT /api/auth/profile/api-tokens/{token_id}/state`

トークンを失効させることなく、一時的に無効化または再有効化します。

```json
{
  "disabled": true
}
```

エンドポイントは更新された状態を返します。

```json
{
  "disabled": true
}
```

### トークンの失効

`DELETE /api/auth/profile/api-tokens/{token_id}`

失効が成功すると HTTP 204 No Content が返され、認証キャッシュが直ちに無効化されます。

## アクティブ セッションと IP ブロックの管理

アクティブなブラウザー セッション、Basic 認証リクエスト、API トークン アクセスを確認します。最近の IP
アドレスを追跡し、信頼できないクライアントを手動でブロックします。

### アクティブ セッションの一覧取得

`GET /api/auth/profile/sessions`

デバイスごとに集約されたセッション一覧を返します。各セッションには最大 10 件の最近の IP アドレスが記録されます。

```json
{
  "sessions": [
    {
      "public_id": "a1b2c3d4",
      "username": "alice",
      "ip": "192.168.1.100",
      "user_agent": "Mozilla/5.0 (Windows NT 10.0 Win64 x64)",
      "created_at": 1787731200000,
      "last_active": 1787734800000,
      "expires_at": 1788940800000,
      "current": true,
      "login_method": "password+totp",
      "recent_ips": ["192.168.1.100", "192.168.1.101"]
    }
  ]
}
```

### アクティブ セッションの失効

`DELETE /api/auth/profile/sessions/{session_id}`

指定した単一のセッションを失効させます。現在のデバイスを維持しながら他のすべてのセッションを失効させるには次を呼び出します。

`POST /api/auth/profile/sessions/revoke-others`

どちらのエンドポイントも成功時に HTTP 200 と StatusOk を返します。

### ブロックされた IP アドレスの管理

アカウント レベルの IP ブロックにより、特定の IP アドレスがこのアカウントにログインすることを阻止できます。

ブロック中の IP 一覧を取得する。

`GET /api/auth/profile/ip-bans`

```json
{
  "ips": ["203.0.113.195"]
}
```

新しい IP アドレスをブロックする。

`POST /api/auth/profile/ip-bans`

```json
{
  "ip": "203.0.113.195"
}
```

IP アドレスのブロックを解除する。

`DELETE /api/auth/profile/ip-bans/{ip}`

## スコープ リファレンス

| スコープ              | 機能                                                                       |
|:----------------------|:---------------------------------------------------------------------------|
| `repository:read`     | リポジトリのカタログ、メタデータ、ファイル、イメージ、バージョンの読み取り |
| `repository:publish`  | Maven、npm、Cargo、Docker、ファイル、またはチャンク アップロード経由の公開 |
| `repository:delete`   | リポジトリ ファイル、パッケージ バージョン、タグ、イメージの削除           |
| `package:create`      | リポジトリ承認後の新しい npm/Cargo パッケージや Docker イメージの予約      |
| `package:metadata`    | パッケージの説明およびその他のメタデータの更新                             |
| `package:lifecycle`   | パッケージやバージョンのアーカイブ、復元、公開状態の変更                   |
| `team:manage`         | npm、Cargo、Docker、Maven ドメイン チームおよび招待の表示と管理            |
| `domain:read`         | Maven 公開ドメインのプライベート設定の読み取り                             |
| `domain:create`       | Maven 公開ドメインの作成                                                   |
| `domain:verify`       | Maven ドメイン所有権の検証リクエストまたは強制検証                         |
| `domain:lifecycle`    | Maven 公開ドメインの閉鎖または再申請                                       |
| `messages:read`       | アカウント メッセージの読み取り、既読設定、削除                            |
| `account:read`        | プライベート アカウント データおよび個人の監査ログの読み取り               |
| `account:write`       | API 経由での公開プロフィールの更新                                         |
| `statistics:read`     | アカウントが閲覧権限を持つダウンロード統計の照会                           |
| `admin:users`         | ユーザー アカウントおよびログイン デバイスの管理                           |
| `admin:repositories`  | リポジトリの管理とインデックスの再構築                                     |
| `admin:settings`      | システム設定および診断の管理                                               |
| `admin:audit`         | 管理者向け監査ログおよびステータス データの読み取りやパージ                |
| `admin:notifications` | 管理者通知の作成                                                           |
| `admin:updates`       | システム アップデートの確認、アップロード、インストール、および再起動      |
| `admin:statistics`    | システム全体のダウンロード統計の照会                                       |

`admin:*` スコープは管理者のみが作成可能であり、アカウントが管理者権限を失うと直ちに無効化されます。

## トークンの使用

API 自動化リクエストには、トークンを Bearer 資格情報として使用します。

```http
Authorization: Bearer rnp_pat_REDACTED
```

パッケージ クライアントで Basic 認証を使用する場合、パスワードとして API トークンを指定する必要があります。アカウントのログイン
パスワードは使用できません。権限はトークンに設定されたスコープに従います。

```http
Authorization: Basic YWxpY2U6cm5wX3BhdF9SRURBQ1RFRF9UT0tFTg==
```

npm クライアントは `_authToken` または Basic 認証で送信します。Cargo は Authorization ヘッダーとして送信します。Docker は
`GET /v2/token` で資格情報を交換します。

## 互換エンドポイント

管理者向けユーザー操作は `GET /api/tokens` にあります。レガシー エンドポイント `POST /api/auth/profile/token`
は下位互換性のために保持されています。新しい統合ではきめ細かなプロフィール エンドポイントを使用してください。
