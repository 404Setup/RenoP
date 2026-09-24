---
title: システムアーキテクチャ
order: 4
category: はじめに
description: 構成モジュール、認可、ストリーミングストレージ、非同期処理
---

# システムアーキテクチャ

RenoP は通信層、パッケージプロトコル、認可、永続化、バックグラウンド保守の境界を持つ単一の Go プロセス
です。内蔵フロントエンドも外部クライアントと同様に、制限付き API を呼び出します。

## モジュール境界

```text
Browser and package clients
        |
HTTP routing, rate limits, authentication, API-token policy
        |
Maven | npm | Cargo | Docker | Files | Management services
        |
Repository gate and publication workflows
        |
Disk or S3 storage          SQL database
        |                       |
File index and mirrors      Identity, teams, audit, messages
```

- `internal/api` とミドルウェアは、一般的な HTTP 規約、検索、異常検知、認証境界を担います。
- 各形式のサービスは、Maven ドメイン/カタログ、npm packument、Cargo Sparse Index、Docker Distribution v2、ドキュメント表示機能を管理します。
- データベース層は、SQLite、MySQL、PostgreSQL、ClickHouse の方言に対応したトランザクション処理を提供します。
- ローカルディスクや S3 は大容量の本文をストリーミング転送し、ファイル索引は上限付きのメタデータ走査を提供します。

## リクエスト処理の流れ

### ストリーミングと整合性

アップロードおよびダウンロードは、クライアントとディスク/S3 の間でストリーミング転送されます。ハッシュ計算、
Brotli/ZIP 展開、ミラーキャッシュ、GPG 処理には、上限付きリーダーと一時ファイルを使用します。
ストライプ状のリポジトリゲートにより、ストレージ/エンジンの変更と、アップロード、削除、ミラーコミット、
公開処理の競合を防ぎます。

### 認証と認可

ブラウザーセッションは Cookie のみを使用し、Basic 認証は標準のパッケージプロトコル専用です。
Bearer API トークンのスコープと対象制限は、実行時に現在のリポジトリ権限および L0-L4 チーム権限と突き合わせて
検証されます。ユーザー名を変更した場合でも、不変の内部ユーザー ID によって所有権が維持されます。

### 非同期処理

プロセス全体で再入不可能なスケジューラが、スナップショット作成、不要ファイル整理、索引更新、ダウンロード計数、
更新確認を統括します。順序保証が必要な監査ログ、GPG 検証、トークン変更、ファイル監視は、専用の直列ワーカーで
実行されます。永続的な処理結果はメッセージセンターに通知され、一時的な進捗は画面状態またはトースト通知として
表示されます。
