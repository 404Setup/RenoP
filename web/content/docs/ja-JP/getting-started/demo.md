---
title: デモモード
order: 5
category: はじめに
description: プリセットデータを読み取り専用で表示する
---

# デモモード

`--demo` の初回起動で独立したプリセット DB を作成し、以降は読み取り専用で表示します。ログインフォームは `admin` /
`12345678` を自動入力します。アカウント、セッション、チーム、Maven/Cargo/npm/Docker、ファイルメタデータ、審査、チケット、クォータ、通知、統計、監査ログ、メール履歴を含みます。

```bash
./renop --demo
./renop --demo --demo-temp
```

`--demo --demo-temp` ではシステム設定とリポジトリ定義を保存できます。変更は保持され、`--demo` のみで再起動すると読み取り専用になります。
`--demo-temp` 単独は拒否します。その他の変更とログ書き込みは両モードで禁止します。

既定のファイルは `renop-demo.db` と `renop-demo-settings.db` です。`RENOP_DEMO_DATABASE` と `RENOP_DEMO_SETTINGS_DB`
で変更でき、通常のデータと分離されています。空のリポジトリ一覧を含め、既存のデモデータは保持します。DB
以外のパッケージ、索引、ログファイルは作成せず、ダウンロードと文書プレビューも提供しません。実際のセッションは上限付きメモリに保持し、プリセットのセッションでは認証できません。

`GET /api/demo` はバイナリ protobuf `DemoInfo` で `enabled`、`temporary`、公開デモ認証情報を返します。通常モードは認証情報を返しません。変更の拒否は
`403` と `demo_read_only`、利用できないファイルは `demo_file_unavailable` を使用します。メール、ミラー、保守、更新のバックグラウンド処理は起動しません。
