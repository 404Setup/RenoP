---
title: 認証 API
order: 2
category: API リファレンス
description: ブラウザセッション、プロフィール、ログイン方式、復旧、セッション失効
---

# 認証 API

ブラウザ認証は HttpOnly Cookie `renop_session` を使用します。秘密値はプロフィールやセッション一覧に
返さず、ヘッダーや URL でも受理しません。非公開のセキュリティ API はブラウザセッション専用で、
パスワードや API Token では呼び出せません。

ブラウザーのログインページは `/account/login` です。パスワード、Passkey、外部サービスでのログイン後は、任意のクエリパラメーター
`return_to` で指定したサイト内のパスに戻ります。クエリとフラグメントは保持しません。外部アドレス、認証エンドポイント、1,024
文字を超える値は `/` に戻ります。セッションが期限切れの場合はログインページを表示し、ログイン済みユーザーの権限不足の場合はセッションを維持してホームに戻ります。

[2段階認証](../security/two-step-verification.md)では、認証アプリの設定、追加認証用 Passkey、保留中のログイン応答、復旧について説明します。追加認証用
Passkey は最初のログイン方法にはなりません。オフライン復旧は認証アプリを削除し、Passkey
の追加認証を無効にします。メールによるパスワード再設定は両方を維持します。

## パスワードまたはメールでログイン

- **パス**: `POST /api/auth/login`
- **認証**: なし。
- **本文**: protobuf `LoginRequest`。以下は JSON 名です。`name` はユーザー名または非公開メールです。

### リクエスト

```json
{
  "name": "admin",
  "secret": "your_password"
}
```

### セッション結果

成功時は `HttpOnly`、`SameSite=Lax`、HTTPS では `Secure` の `renop_session` を設定します。protobuf
`SessionDetails` は権限とルートを返しますが、`session_token` は空です。

## Passkey と GitHub ログイン

- **Passkey 開始**: `POST /api/auth/fido/login/begin`
- **Passkey 完了**: `POST /api/auth/fido/login/finish`
- **GitHub 開始**: `GET /api/auth/oauth/github/start` (`GET /api/auth/github/start`)
- **GitHub callback**: `GET /api/auth/github/callback`
- **GitHub 利用可否**: `GET /api/auth/github/status`

GitHub は管理者が OAuth を設定した場合だけ表示されます。ユーザーと Organization の読み取りを要求し、
不変 Provider ID と Principal のスナップショットを保存しますが、OAuth Access Token は保存しません。

未連携の GitHub ID は[アカウント登録](../security/registration.md)とパスワード設定が必要です。OAuth は
`read:user read:org user:email` を要求し、コールバックには認可を開始したブラウザーの Cookie が必要です。

Microsoft、Google、GitLab、Cloudflare、Stack Exchange、カスタム OAuth
クライアントには[外部ログイン API](../security/oauth-login.md)
を使用します。アカウント連携、登録時に必要なメール確認、任意のプロフィール取り込み、同じ二段階認証ポリシーに対応します。

## 現在のアカウントと公開プロフィール

- **現在のセッション**: `GET /api/auth/me`
- **非公開プロフィール**: `GET /api/auth/profile`
- **ユーザー名または表示名の更新**: `PUT /api/auth/profile`
- **パスワード更新**: `PUT /api/auth/profile/password`
- **ログアウト**: `POST /api/auth/logout`
- **公開プロフィール**: `GET /api/users/:username/profile`
- **パッケージ所属**: `GET /api/users/:username/memberships?format=cargo|docker|maven|npm`

二次認証が有効な場合、パスワード変更には新しい TOTP コードまたは二次認証 Passkey が必要です。メール送信が有効なら現在のメインアドレスへのコードも選べます。`PUT /api/auth/profile/password` はバイナリの `UpdatePasswordRequest` で `factor`（`totp`、`passkey`、`email`）と証明（`totp_code`、`challenge_id` と `passkey_credential`、または `email_code`）を受け取ります。Passkey は `POST /api/auth/profile/password/passkey/begin` で開始します。証明は現在のセッションと認証情報に結び付き、一度だけ使用可能です。成功すると他のセッションを失効させます。

公開ルートはユーザー名を使用し、不変 ID は内部に保ちます。`HIDDEN` の所属は除外し、非公開所属は許可された
閲覧者だけに返します。

## アカウントセキュリティ

これらのルートは現在のブラウザセッションを要求し、`Cache-Control: no-store` を返します。

### メールとパスワードログイン方針

- **状態取得**: `GET /api/auth/profile/security`
- **メール設定**: `PUT /api/auth/profile/email`
  。キュー経由の確認とGitHubによる確認は[セキュリティ用メールの確認](../security/email-verification.md)を参照してください。
- **パスワードログイン切替**: `PUT /api/auth/profile/password-login`
- 主要なログイン用 Passkey または外部アカウントが残る場合だけパスワードログインを無効化できます。有効化には設定済みパスワードが必要です。

### メールコードによるパスワード再設定

独立ページ `/account/forgot-password` では、アカウントに登録された非公開メールを確認します。メールサービスが無効の場合、このページと送信・確認
API は `404` を返し、ログインと復旧ページのリンクは非表示になります。オフライン復旧 `/account/recovery` は引き続き利用できます。

- **利用状態**：`GET /api/auth/password-reset/status` は `{"enabled":true}` または `{"enabled":false}` を返します。
- **コード送信**：`POST /api/auth/password-reset/request` は `{"email":"admin@example.com"}` を受け取り、永続キューへの登録後、送信前に
  `202` と `{id,status,ticket}` を返します。
- **配信状態**：`GET /api/auth/mail/:id` に `X-Renop-Mail-Ticket` ヘッダーで非公開チケットを渡します。ページは 3 秒ごとに最大
  10 分間確認し、ページを離れると停止します。失敗や配信未確認も表示します。サービスによる受理は受信者への配信を保証しません。チケットを
  URL やブラウザストレージに保存しないでください。
- **再設定**：`POST /api/auth/password-reset/confirm` は以下の JSON を受け取ります。両 POST は
  `Content-Type: application/json` が必須で、本文は 4,096 バイトまで、パスワードは UTF-8 で 6–72 バイトです。

```json
{"email":"admin@example.com","code":"01234567","new_password":"new_secure_password"}
```

8 桁コードは 10 分間有効で、誤入力は 5 回までです。設定済み IP 送信制限に加え、同じアドレスへの発行には 60
秒の待機時間があります。新しいコードが正常にキュー登録されると旧コードが無効になり、登録失敗時は旧コードを保持します。キュー登録、コード保存、制限カウントは同一トランザクションです。確認記録は最大
2,048 件で、期限切れ記録は登録前と定期処理で削除します。

受信ポリシーで許可された有効なアドレスには、未登録アドレスを含め同じ所有確認メールを送ります。配信状態からアカウントの有無は判別できません。再設定時に発行時のアカウント
ID、メール、パスワード、安全状態の版を再確認します。後から登録されたアカウントは以前のコードを使用できず、退会後も旧コードは無効です。

成功するとコード消費、パスワード更新、パスワードログイン有効化、ブラウザセッション失効を原子的に実行します。
`{status:"success",username}` を返して Cookie を削除し、ページはログインに戻ります。他の認証情報の紐付けと停止措置は維持します。
`ACCOUNT_EMAIL_CODE_INVALID` は無効、期限切れ、試行上限、使用済み、状態変更を示します。制限超過は `429`、キューやサービス障害は
`503` です。すべての応答に `Cache-Control: no-store` を設定します。

### 復旧コード

独立したアカウント復旧ページは `/account/recovery`
です。ログインページのアカウント復旧リンクから開くことができ、メール配信の有効化は不要です。復旧後はブラウザーの既存セッションを消去し、ユーザー名を入力したログインページに戻ります。有効なローカルの
`return_to` は保持されます。ページを離れると復旧コードとパスワードは消去され、ブラウザーの履歴には保存されません。

- **生成**: `POST /api/auth/profile/recovery-codes`
- **パスワード再設定**: `POST /api/auth/recovery/password`
- 12 個のコードは一度だけ表示され、保存するのは Argon2id verifier だけです。異なる未使用コード 4 個を
  原子的に消費し、既存セッションを失効してパスワードログインを再有効化します。

```json
{
  "identifier": "admin@example.com",
  "codes": ["CODE-ONE", "CODE-TWO", "CODE-THREE", "CODE-FOUR"],
  "new_password": "new_secure_password"
}
```

オフライン復旧には現在のメインアドレス、または変更後14日以内の旧メインアドレスと未使用コード4個が必要です。旧アドレスによる復旧はそのアドレスを復元し、置き換えられたメインアドレスを削除して復旧期間を終了します。ユーザー名や通常の副アドレスは使えません。通常のメインアドレス変更後14日間は復旧コードの再発行、Passkey の追加・削除、二次認証の変更を禁止します（`ACCOUNT_SECURITY_HOLD`）。既存の認証は利用でき、初回のコード生成も可能です。この期間は旧メインアドレスを削除できません。非公開の応答に `security_hold_until` と `previous_primary_emails`（`email`、`expires_at`）が含まれます。

## ログイン方式の管理

- **Passkey 一覧**: `GET /api/auth/profile/fido`
- **Passkey 登録**: `POST /api/auth/profile/fido/register/begin` の後に
  `POST /api/auth/profile/fido/register/finish`
- **Passkey 削除**: `DELETE /api/auth/profile/fido/:device_id`
- **GitHub ID 取得**: `GET /api/auth/profile/github`
- **GitHub 切断**: `DELETE /api/auth/profile/oauth/github` (`DELETE /api/auth/profile/github`)

最後の有効なログイン方式は削除または無効化できません。

## ブラウザセッション

- **一覧**: `GET /api/auth/profile/sessions`
- **1 件を失効**: `DELETE /api/auth/profile/sessions/:session_id`
- **他をすべて失効**: `POST /api/auth/profile/sessions/revoke-others`

一覧には公開 ID、ログイン方式、時刻、IP、User-Agent が含まれますが、Cookie の秘密値は含まれません。

## アカウントの永久閉鎖

`GET /api/auth/profile/retirement` は現在のアカウントの閉鎖条件を返します。このルートと
`DELETE /api/auth/profile/retirement` はブラウザーセッション専用です。閉鎖リクエストは JSON を使用します。

```json
{"confirmation":"alice"}
```

確認文字列は現在のユーザー名と完全に一致する必要があります。成功時は `204 No Content`、
不一致の場合は `400` と `ACCOUNT_RETIREMENT_CONFIRMATION` を返します。未解決の条件がある場合は
`409`、`ACCOUNT_RETIREMENT_BLOCKED` と最新の確認結果を返します。結果には `eligible`、
`protected_role`、`super_team_owner_count`、`maven_domain_owner_count`、`package_owner_count`、
`pending_review_count` が含まれます。

システム管理者とリポジトリのモデレーターは閉鎖できません。グローバルチームの T4、有効な Maven
公開ドメインの所有権、非推奨化されていないパッケージの L4、未処理の審査申請が残っていてはいけません。
先に所有権を譲渡し、公開ドメインを閉鎖するか、パッケージを永久に非推奨化してください。

閉鎖後はアカウントとユーザー名が永久に予約され、全チームから脱退し、すべての外部ログイン連携を解除します。
Passkey、セッション、API Token、写真、復旧コード、メッセージも削除されます。ログインは
`ACCOUNT_DELETED` を返します。非公開メールアドレスは 14 日、操作履歴は 30 日保持され、
期限後に定期処理で分割して削除されます。公開済みパッケージは引き続きダウンロードできます。

解除された外部 ID は、別の有効なアカウントに直ちに関連付けられます。予約済みユーザー名の解放、
メール保持期間の短縮、閉鎖済みリソースの所有権復元は行いません。

システム管理者は `GET /api/tokens/:name/retention` で期限を確認し、
`DELETE /api/tokens/:name/retention/email` でメールを早期解放、
`DELETE /api/tokens/:name/retention/audit` で履歴を早期削除できます。
`deleted_at`、`email_release_at`、`email_released_at`、`audit_purge_at`、`audit_purged_at`
は Unix ミリ秒です。管理者用 `DELETE /api/tokens/:name` にも同じ永久閉鎖条件が適用されます。

[法的文書と Cookie の設定](../configuration/legal.md)

[セキュリティ認証](../security/captcha.md)

## 公開プロフィールのリンク

PUT /api/auth/profile/links は、ウェブサイト、Discord、カスタムリンクと `visibility.github` / `visibility.gitlab` を含む JSON を受け取ります。両設定の既定値は false です。GitHub URL と `providers` は読み取り専用で、現在の連携済みアカウントから生成されます。GitLab は設定済みサービスの認証元との一致も必要です。表示設定は本人だけに返されます。グローバルチームの GitHub URL は引き続き編集できます。

```json
{"website":"https://example.com","visibility":{"github":true,"gitlab":false}}
```

## プロバイダーのログアウトと認可取り消し

`POST /api/auth/logout` はログイン先プロバイダーに接続する前にローカルのブラウザーセッションを失効させます。外部認可のないセッションは `204`、それ以外はバイナリ protobuf `LogoutResponse` を返します。フィールドは `local_revoked`、`provider`、`provider_status`（`revoked`、`failed`、`unavailable`、`unsupported`）です。外部処理が失敗してもローカルセッションは復元されません。`POST /api/auth/oauth/:provider/revoke` は Cookie 認証された現在のセッションのプロバイダーを確認して同じ処理を行います。API トークンからは呼び出せません。

`POST /api/auth/oauth/:provider/backchannel-logout` は標準フォームの `logout_token` を受け取ります。署名付き OIDC JWT には設定に一致する発行者と対象、`iat`、`exp`、`jti`、バックチャネルログアウトイベントが必要です。`sub`、`sid`、または両方を含め、`nonce` は含めてはいけません。10 分より古いイベントは拒否されます。

`POST /api/auth/oauth/:provider/revoked` はバイナリ protobuf `ProviderRevocationRequest` を受け取ります。32 バイト以上の書き込み専用 `revocation_secret` を設定し、リクエストの元バイト列に対する HMAC-SHA256 の16進値を `X-Renop-Signature-256: sha256=<署名>` で送ります。メッセージは `event_id`、`subject`、`session_id`、Unix ミリ秒の `issued_at` を含み、OIDC では検証済み `issuer` も指定します。OAuth のみの場合は発行者を空にします。異なる Webhook 形式には信頼できるアダプターが必要で、元の本文は直接受け付けません。

有効なイベントと重複は `200`、無効または古いクレームは `400`、無効な HMAC は `401`、受付制限や保存処理の障害は `429`/`503` です。設定された認可元で一致し、イベントより古いセッションだけを失効させます。ID の関連付けと API トークンは保持します。リプレイや遅れて完了した MFA 証明では失効したセッションを再作成できません。

## アカウントの利用停止

管理者は公開ユーザーページから利用停止と解除を操作できます。`GET /api/tokens/:name/ban` は状態と保護を確認し、`PUT /api/tokens/:name/ban` は JSON の `reason_code`、自由記述 `reason`、任意の Unix ミリ秒 `expires_at`、`ban_ip` を受け取ります。`DELETE /api/tokens/:name/ban` は解除します。管理者・モデレーターは保護対象の役割を先に解除する必要があります。以下のコードは翻訳された定型理由に対応し、空の `reason_code` は翻訳しない自由記述を使います。

`harassment_abuse`, `spam_misleading`, `automation`, `alternate_accounts`, `security_rules`, `harmful_content`, `terms_violation`, `impersonation`, `copyright`.
