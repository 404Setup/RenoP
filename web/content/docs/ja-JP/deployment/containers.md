---
title: コンテナ
order: 7
category: デプロイ
description: Docker、Podman、Kubernetes で RenoP を実行する
---

# コンテナ

## 実行

リリースビルドは Linux amd64 と arm64 向けに `mvnc.pkg.one/oci/renop:<version>` と `:latest`
を公開します。イメージはリリースの実行ファイルを再利用し、UID/GID 65532 で動作します。

```bash
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
docker logs renop
```

`http://localhost:3000` を開きます。初回起動で `RENOP_DEFAULT_ADMIN_PASSWORD` を指定しなければ、初期管理者パスワードがログに一度表示されます。Podman
では `docker` を `podman` に置き換えてください。

macOS では Docker Desktop、Podman machine、Colima、OrbStack、Rancher Desktop、Apple container で Linux
イメージを実行します。通常のコンテナ検出マーカーが隠される場合も、イメージの `RENOP_CONTAINER=1` で判定できます。

## 永続データ

書き込み可能なボリュームを `/data` にマウントしてください。`renop-settings.db`、SQLite
データベース、インデックス、パッケージ、その他の相対パスを保存します。バインドマウントは UID/GID 65532 に書き込みを許可し、SELinux
環境の Podman では必要に応じて `:Z` を指定します。SQLite がジャーナルファイルを作成できるよう、ディレクトリ全体をマウントしてください。

`RENOP_SETTINGS_DB` と `RENOP_INDEX` は既定で `/data` 内のファイルを参照します。読み取り専用ルートには
`--read-only --tmpfs /tmp` などで書き込み可能な `/tmp` も必要です。コンテナ置換時は永続ボリュームを保持し、更新前にバックアップしてください。

## 更新と終了

コンテナでは自動確認、オンライン／オフラインの実行ファイル更新、サービス登録、プロセス内再起動が無効になります。更新はランタイムで行います。SIGTERM
は HTTP リクエスト、バックグラウンド処理、データベース書き込みを終了させます。終了に90秒を確保してください。

```bash
docker pull mvnc.pkg.one/oci/renop:latest
docker stop --time 90 renop
docker rm renop
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
```

バージョンを固定するには明示的なタグまたはイメージダイジェストを指定します。設定が移行された場合、ロールバックには対応するバックアップの復元が必要になることがあります。

## Kubernetes

SQLite やローカルストレージでは、単一レプリカ、永続ボリューム、`Recreate` を使用します。この Deployment 断片は既存の
`renop-data` PVC を前提とします。通常のメタデータとセレクターを追加してください。ポート変更やアプリケーション TLS
の有効化時はプローブも変更します。

```yaml
spec:
  replicas: 1
  strategy:
    type: Recreate
  template:
    spec:
      terminationGracePeriodSeconds: 90
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
      containers:
        - name: renop
          image: mvnc.pkg.one/oci/renop:latest
          ports:
            - containerPort: 3000
          readinessProbe:
            httpGet:
              path: /api/status/hash
              port: 3000
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: renop-data
```

## ローカルビルド

リポジトリ指定の Go、Node.js 24、pnpm、protoc を使用します。共有ソースを一度準備し、各アーキテクチャをコンパイルします。Dockerfile
は `container-bin/<arch>/renop` を利用し、別の実行ファイルをダウンロードしたり異なる Go ツールチェーンで再ビルドしたりしません。

```powershell
./build.ps1 -PrepareOnly
foreach ($arch in 'amd64', 'arm64') {
    New-Item -ItemType Directory -Path "container-bin/$arch" -Force | Out-Null
    Push-Location "container-bin/$arch"
    try { ../../build.ps1 -Target "linux/$arch" -SkipPreparation -nb }
    finally { Pop-Location }
}
docker buildx build --platform linux/amd64 --load -t renop:local .
```
