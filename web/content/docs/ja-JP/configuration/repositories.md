---
title: リポジトリとミラー
order: 2
category: 設定
description: エンジン、可視性、上流ミラー、移行、S3 ストレージ
---

# リポジトリとミラー

リポジトリ定義はデータベースに保存され、リポジトリ管理画面で編集します。初回アップグレード時、
データベースに設定がない場合のみ `repositories.yaml`（または `RENOP_REPOSITORIES`）をインポートし、
元ファイルを `repositories.yaml.migrated.<id>` として保存します。不正な入力では元ファイルを維持して起動を停止します。
空のリポジトリ集合を含め、既存のデータベース設定が常に優先されます。新規インストールでは YAML を作らず、
リポジトリ一覧は空のままです。データベースをバックアップし、S3 やミラーの認証情報を含む
移行元の保管ファイルを保護してください。名前は不変の小文字 slug で、URL の最初のセグメントです。

新規インストール時のリポジトリ一覧は空です。管理画面から明示的に作成してください。既存のデータベース設定と一度限りの旧設定の移行では、設定済みのリポジトリを保持します。

## 旧設定の移行例

```yaml
repositories:
  releases:
    name: releases
    format: maven
    visibility: PUBLIC
    allow_redeployment: false
    require_gpg_signature: true
    publication_review: every_version
    download_statistics: true
    mirrors: [ ]
  crates:
    name: crates
    format: cargo
    visibility: PUBLIC
    mirrors: [ ]
  containers:
    name: containers
    format: docker
    visibility: PRIVATE
    allow_redeployment: false
    mirrors: [ ]
```

## リポジトリ項目

| 項目                    | 既定         | 説明                                                                                                                      |
|:------------------------|:-------------|:--------------------------------------------------------------------------------------------------------------------------|
| `name`                  | 必須         | 不変 slug と URL prefix                                                                                                   |
| `format`                | `maven`      | `maven`, `maven-classic`, `files`, `npm`, `cargo`, `docker`, `conan`, `conda`, `conda-native`, `apk`, `apt`, `rpm`, `yum` |
| `visibility`            | `PUBLIC`     | `PUBLIC`、`HIDDEN`、`PRIVATE`                                                                                             |
| `allow_redeployment`    | `false`      | 対応形式で Maven 再公開または files/Docker 上書き                                                                         |
| `require_gpg_signature` | `false`      | Maven 公開時の OpenPGP 分離署名検証                                                                                       |
| `publication_review`    | `off`        | Maven/npm/Cargo/Docker 審査方針: `off`、`new_packages`、`every_version`                                                   |
| `download_statistics`   | エンジン既定 | Maven/npm/Cargo/Docker は有効、`files` は明示的に有効化                                                                   |
| `mirrors`               | `[]`         | 順序付き上流定義                                                                                                          |
| `s3`                    | 省略         | リポジトリ固有 S3 storage                                                                                                 |

npm と Docker の `new_packages` は、名前を予約する前に明示的な作成申請を審査します。`every_version` は
その後の各バージョンまたはマニフェストも審査します。Maven と Cargo には空パッケージの作成段階がないため、
`new_packages` は最初の公開を審査します。ミラーからの取り込みはすべてのエンジンで審査対象外です。

`maven-classic` は画面レイアウトだけを変え、Maven の公開規則を維持します。`files` は非構造化で、
チェックサムや POM の生成、署名検証を行いません。Maven と `files` の相互移行ではオブジェクトを移動せず、
Maven へ戻す際にカタログと保存済みの方針を復元します。移行前のダウンロード統計設定も維持されます。

`files` のアップロードとミラーダウンロードは、パスに `SNAPSHOT` を含む場合や、名前が `.md5`、`.asc`、
`-javadoc.jar` で終わる場合も隣接ファイルを保持します。旧バージョンの削除と Javadoc 展開は Maven のみに適用されます。

公開審査は Maven、npm、Cargo、Docker と管理対象ネイティブリソースに対応します。Maven では `allow_redeployment` を `false`
に固定し、npm では不変
バージョンと dist-tag のトランザクションを維持します。ローカルファイルはリポジトリ審査者またはシステム
管理者の承認まで非公開となり、ミラーは審査されません。保留中の審査があるリポジトリは設定変更、削除、
エンジン移行ができません。

`npm` リポジトリは公開前のパッケージ予約、不変のセマンティックバージョン（SemVer）、配布タグ、スコープ付き非公開パッケージ、L0-L4
チーム、
完全名または `@scope/*` 規則によるミラーを提供します。

### 可視性

- **PUBLIC**: 匿名の読み取りと一覧取得を許可します。
- **HIDDEN**: 匿名ユーザーや閲覧権限のないユーザーの一覧とプロフィールの所属情報には表示されません。
  管理者と明示的なリポジトリ閲覧権限を持つユーザーには表示されます。既知の正確なファイルパスは読み取れます。
- **PRIVATE**: 読み取り、一覧、書き込みに明示権限が必要です。非公開 Docker イメージは L0-L4 権限も確認します。

## 上流ミラー

ローカルに存在しないオブジェクトは有効なミラーからストリーミング転送し、本文全体をバッファすることなく保存できます。Cargo
と Docker は
適用対象の上流名が存在する場合、ローカルでの作成を拒否します。

```yaml
mirrors:
  - name: "central"
    url: "https://repo1.maven.org/maven2"
    persist: true
    cache_ttl_secs: 86400
    negative_cache: true
    timeout_secs: 30
    proxy: ""
    allow_artifacts: [ ]
    deny_artifacts: [ ]
```

| 項目              | 既定    | 説明                                         |
|:------------------|:--------|:---------------------------------------------|
| `name`            | 必須    | リポジトリ内で一意の名前                     |
| `url`             | 必須    | 上流のベース URL                             |
| `persist`         | `true`  | 成功レスポンスを保存                         |
| `cache_ttl_secs`  | `86400` | 成功レスポンスのキャッシュ有効期間（秒）     |
| `negative_cache`  | `true`  | 上流で見つからない場合のネガティブキャッシュ |
| `timeout_secs`    | `30`    | 上流への要求タイムアウト（秒）               |
| `proxy`           | `""`    | 全体設定に従う、直接接続、または指定プロキシ |
| `allow_artifacts` | `[]`    | 形式に応じた許可ルール                       |
| `deny_artifacts`  | `[]`    | 優先される拒否ルール                         |

資格情報は構造化 authorization 項目に置き、`url` に埋め込まないでください。

## S3 互換ストレージ

各リポジトリはローカルディスクまたは独立した S3 互換ストレージを使用できます。ストレージやエンジンの変更はリポジトリゲートによってアップロード、削除、
GPG コミット、ミラー書き込みと直列化されます。

```yaml
s3:
  enabled: true
  endpoint: "https://s3.us-east-1.amazonaws.com"
  bucket: "my-renop-bucket"
  key_prefix: "releases/"
  region: "us-east-1"
  access_key_id: "YOUR_ACCESS_KEY"
  secret_access_key: "YOUR_SECRET_KEY"
  force_path_style: false
  redirect_downloads: false
```

MinIO では通常 `force_path_style` が必要です。`redirect_downloads` 有効時は認可後に有効期限付き署名 URL へ
リダイレクトし、無効時は RenoP がストリーミング転送します。

`capacity_limit_bytes` はリポジトリごとの保存容量上限（バイト）です。`0` は無制限で、画面上では MiB 単位で設定します。ディスク/S3
のパッケージ、生成されたチェックサム、審査待ちオブジェクト、ミラーキャッシュを計上し、一時領域のコピーは除外します。コミット前に予約し、並行アップロードで上限を共有します。超過は
`507` と `repository_capacity_exceeded`
を返し、正常なミラー応答はキャッシュせず転送できます。上限を既存使用量以下にしても読み取りは可能です。外部からストレージを変更した場合は索引再構築か再起動で再計測します。旧クライアントが省略した場合は現在の上限を維持します。

## ネイティブパッケージのリポジトリ

Conan はネイティブクライアントによるレシピとバイナリのリビジョンをサポートします。Conda と Conda native は `noarch/`
などのプラットフォームディレクトリ内の `.conda` と `.tar.bz2` を受け付け、`repodata.json` を生成します。APK は `x86_64/`
などのアーキテクチャディレクトリでパッケージと同じ場所に `APKINDEX.tar.gz` を生成します。apt は `.deb` を受け付け、
`Packages`、`Packages.gz`、`dists/<suite>/Release` を生成し、`pool/<component>/` を対応するコンポーネントに割り当てます。rpm/yum
は `repodata/repomd.xml` と参照先のメタデータを生成します。

アップロード前にリポジトリ画面でネイティブリソースを登録してください。識別子はパッケージメタデータまたは Conan
レシピから取得し、グローバルチームの接頭辞は不要です。L0 は読み取り、L1 は公開、L2 はバージョンの置換・削除、L3 は設定・メンバー管理、L4
は所有者です。L4 所有者を最低1人残す必要があります。リポジトリへの書き込み権限だけでは他のリソースを変更できません。ネイティブミラーは読み取り専用です。

`new_packages` は登録後の最初の公開を、`every_version` は各バージョンや Conan
リビジョンを審査します。署名未完了や審査待ちのファイルは、再起動後もダウンロードや生成インデックスに公開されません。APK
は公開者の RSA PEM 公開鍵と、制御部分およびペイロードを保護するネイティブ署名を必要とします。RPM
はペイロードを直接、または署名済みダイジェストで保護する信頼された OpenPGP 署名を必要とします。公開鍵はリソース設定で登録します。APT
はリポジトリインデックスに署名するため各 `.deb` の分離署名は不要です。Conda はネイティブのハッシュを使用し、追加の `.asc`
は要求しません。

新しいアップロードで生成インデックスを置き換えることはできません。既存のメタデータは管理者が削除するまで保持されます。生成インデックスは最大10,000パッケージで、大きな集合はリポジトリやネイティブのサブディレクトリに分割します。アップロードは8
GiB、未公開ファイルはリソースごとに256、インスタンス全体で16,384までです。未登録の既存ファイルは引き続き読み取れますが、競合する管理対象アップロードによる置換には管理者の対応が必要です。

生成した APK、apt、rpm のメタデータは、非公開の設定データベースに保存する独立した鍵で署名します。公開鍵はそれぞれ
`renop.rsa.pub`、`renop.asc`、`repodata/repomd.xml.key` で取得できます。apt は `InRelease` と `Release.gpg`、rpm は
`repomd.xml.asc` も提供します。リポジトリブラウザーに表示されるコマンドを使用する前に公開鍵をインポートしてください。rpm
パッケージ自体の署名は発行者の責任であり、その発行者の鍵が別途必要です。

Conan は公式署名拡張のマニフェストと OpenPGP 署名を使用します。`scripts/conan/sign.py` を
`<CONAN_HOME>/extensions/plugins/sign/sign.py` に配置して公開鍵を登録し、`RENOP_CONAN_GPG_KEY` に秘密鍵のフィンガープリント、
`RENOP_CONAN_GPG_KEYRING` に `gpgv` 用の信頼されたバイナリ公開鍵リングを指定します。必要な全ファイルと署名が揃うまで非公開です。公開済み
Conan ファイルの同一内容による再送は冪等です。

```sh
conan cache sign "PACKAGE/*"
conan upload "PACKAGE/*" --remote "REPOSITORY" --confirm
```

## ファイル内容の共有

同一内容の Disk ファイルはハードリンクで不変の内容を共有し、各論理パスを維持します。書き込みはファイルを原子的に置き換え、1
つのパスを削除しても他の参照は残ります。単一のバックグラウンドタスクが既存ファイルをバッチ処理し、使用中のリポジトリをスキップしてバッチ間で休止します。更新可能なインデックス、一時ファイル、有効期限のある
Disk ミラーキャッシュは独立したファイルを維持します。

S3 は 64 KiB
以上の内容を共有オブジェクトへの論理参照として保存し、小さなオブジェクトは直接保存します。ハッシュと参照は非公開のファイルインデックスに永続化され、既知の記録ではメタデータの反復確認を省略できます。信頼できるハッシュがない既存オブジェクトは、通常のアップロードまたは
RenoP 経由の完全なダウンロードでハッシュを取得するまで変更しません。比較のためだけにバックグラウンドでダウンロードすることはありません。インデックス再構築では
LIST のページを取得し、欠落した参照メタデータを一度確認する場合があります。通常の回収はインデックス内の回収待ちキューを処理し、バケット全体を繰り返し走査しません。

完全な S3 バックアップには、リポジトリのオブジェクト、非公開の `.renop-content-v1` 名前空間、非公開のインデックスを含めます。独立した
RenoP デプロイには異なるキープレフィックスを使用してください。参照のない内容は猶予期間後に回収します。リポジトリ容量とダウンロード統計は、物理共有にかかわらず論理ファイルサイズを使用します。
