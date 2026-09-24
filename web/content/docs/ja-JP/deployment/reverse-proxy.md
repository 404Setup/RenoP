---
title: リバースプロキシ
order: 2
category: デプロイ
description: Nginx と Caddy による TLS 終端、ストリーミング転送、信頼済みクライアント IP の設定
---

# リバースプロキシ

本番環境では TLS 終端、ルーティング、ネットワーク保護のため RenoP を Nginx、Caddy、ロードバランサーの背後に配置します。
大容量のアップロードやブロブをメモリやディスクに全量バッファせず、ストリーミング転送できる設定にしてください。

## Nginx

```nginx
server {
    listen 80;
    server_name renop.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name renop.example.com;

    ssl_certificate     /etc/ssl/certs/renop.example.com.crt;
    ssl_certificate_key /etc/ssl/private/renop.example.com.key;

    # 大容量成果物のアップロードを許可
    client_max_body_size 0;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;

        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # リアルタイムストリーミングのためバッファリングを無効化
        proxy_request_buffering off;
        proxy_buffering off;

        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

Docker の `Location`、`Range`、`Content-Range`、`Docker-Upload-UUID` ヘッダーを透過させます。大きな成果物を
公開する場合、プロキシ側のリクエスト本文サイズ制限（`client_max_body_size`）を RenoP より小さくしないでください。

## Caddy

```caddy
renop.example.com {
    reverse_proxy 127.0.0.1:3000 {
        flush_interval -1
    }
}
```

Caddy は TLS 証明書を自動管理します。`flush_interval -1` はストリーミング応答の遅延を防ぎます。

### 自動設定

RenoP の配置ディレクトリから自動構成コマンドを実行します。標準的な配置場所の Caddyfile を検出し、Caddy バイナリで
新しいサイト定義を検証してから、両方の設定を不可分（トランザクション）に更新して Caddy を再読み込みします。`renop-settings.db` には公開
ホスト名、ループバック待受設定、Caddy が管理する TLS 設定が同期されます。

```bash
./renop --install-caddy --hostname renop.example.com

# 明示的なパス指定またはオフライン準備
./renop --install-caddy --hostname renop.example.com \
  --caddyfile /etc/caddy/Caddyfile \
  --settings-db /opt/renop/renop-settings.db \
  --skip-reload
```

成功後に RenoP を再起動してください。通常は `--skip-reload` を指定しません。Caddy バイナリが存在しない環境での
オフライン準備時は、Caddy の検証と再読み込みを明示的に省略します。

## RenoP の信頼設定

公開ホスト名と、自身で管理するプロキシの CIDR のみを設定します。

```yaml
server:
  domains:
    - "renop.example.com"
  trusted_proxies:
    - "127.0.0.1"
    - "10.0.0.0/8"
  cdn_ip_header: "X-Forwarded-For" # または Cloudflare の場合は "CF-Connecting-IP"
```

直接の接続元が信頼されたプロキシでなければ、RenoP は転送元 IP ヘッダーを無視します。信頼する範囲を不必要に広げると、
監査ログやレート制限で使用するクライアント IP の偽装を許す原因になります。
