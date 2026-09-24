---
title: システムサービス管理
order: 1
category: デプロイ
description: --install と --uninstall による RenoP のネイティブサービス登録
---

# システムサービス管理

RenoP は外部ラッパーを必要とせず、自動起動する OS ネイティブサービスとして登録できます。

## コマンド

```bash
# システムサービスとして登録し起動
./renop --install

# ローカル Caddy リバースプロキシを自動設定
./renop --install-caddy --hostname renop.example.com

# システムサービスを停止し登録解除
./renop --uninstall

# CLI ヘルプの表示
./renop --help
```

`--install` はバイナリの絶対パスを記録し、そのディレクトリを作業ディレクトリにします。`/opt/renop` や
`C:\Program Files\RenoP` など、最終配置先から実行してください。

## 対応プラットフォーム

| OS                  | サービス管理機構 | 動作                                                         |
|:--------------------|:-----------------|:-------------------------------------------------------------|
| **Windows**         | SCM              | 自動起動の `RenoP` サービスを登録し、`services.msc` で管理   |
| **Linux (systemd)** | systemd          | `/etc/systemd/system/renop.service` を作成して有効化         |
| **Linux (OpenRC)**  | OpenRC           | `/etc/init.d/renop` を作成して既定のランレベルに追加         |
| **macOS**           | launchd          | `/Library/LaunchDaemons/one.pkg.renop.plist` を作成して読み込み |
| **BSD**             | rc.d             | OS に適した rc.d スクリプトを生成                            |

サービスの登録や解除には管理者権限が必要です。登録済みパスを維持するため、バイナリの移動や置換は正規の更新手順で
実施してください。

## 日常操作

### Linux (systemd)

```bash
systemctl status renop    # サービス状態の確認
systemctl restart renop   # サービスの再起動
journalctl -u renop -f    # リアルタイムログの追尾
```

### Windows (PowerShell)

```powershell
Get-Service RenoP         # サービス状態の確認
Restart-Service RenoP     # サービスの再起動
```
