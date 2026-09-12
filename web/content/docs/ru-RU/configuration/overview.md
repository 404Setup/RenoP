---
title: Обзор конфигурации
order: 1
category: Конфигурация
description: Файлы, сервер, хранилище, прокси, оформление и обновления
---

# Обзор конфигурации

RenoP хранит настройки в отдельной базе SQLite `renop-settings.db`, путь задаёт `RENOP_SETTINGS_DB`. Изменяйте IP/порт и
подключение к основной базе через страницы администратора. Изменения listener требуют перезапуска. Правовые документы
объединены с Frontend, управление индексом — с Storage. Примеры ниже описывают сохранённые поля; создавать или
редактировать YAML не нужно.

Настройки объединены в разделы оформления и политик, аккаунтов и входа, публикации, хранилища и кеша, сервиса и сети,
почты. У каждой страницы остаются отдельные черновики и сохранение.

Только при первом запуске существующий `config.yaml` (или `RENOP_CONFIG`) импортируется и архивируется после фиксации.
Далее используется только снимок в базе. Сохраняйте резервные копии обеих баз: ключи шифрования находятся в базе
настроек.

## Файлы конфигурации

| Файл                | Переопределение     | Назначение                                               |
|:--------------------|:--------------------|:---------------------------------------------------------|
| `renop-settings.db` | `RENOP_SETTINGS_DB` | Сервер, база, previews, proxy, frontend, аудит и updater |
| База данных         | DSN базы            | Движки, видимость, зеркала, политика Maven и S3          |
| `index.json`        | `RENOP_INDEX`       | Снимок файлового индекса, восстанавливаемый из хранилища |

Аккаунты, API Token, сессии, команды, аудит и сообщения находятся в базе, а не в YAML. Дайте доступ к конфигурации
только сервисному аккаунту, поскольку файлы могут содержать секреты.

Определения репозиториев входят в резервную копию базы. Старый YAML импортируется только при отсутствии
настроек репозиториев в базе и не заменяет существующее состояние. Сохраняйте архив миграции отдельно,
если потребуется откат к старой версии RenoP.

## Поля сохранённых настроек

### Хранилище и preview документации

```yaml
storage_path: "storage"
enable_javadoc_preview: true
javadoc_extract_path: ""
max_javadoc_size_mb: 48
enable_cargodoc_preview: true
cargodoc_extract_path: ""
max_cargodoc_size_mb: 128
```

Пустой путь использует кеш платформы. Перед публикацией через `/javadoc` или `/cargodoc` проверяются пути и размеры.

### Сеть и безопасность `server`

```yaml
server:
  host: "0.0.0.0"
  port: 3000
  ssl_enabled: false
  ssl_cert_path: ""
  ssl_key_path: ""
  domains: ["localhost"]
  cors_origins: []
  enable_compression: false
  file_cache_size_mb: 16
  max_active_requests: 512
  trusted_proxies: []
  cdn_ip_header: "X-Forwarded-For"
  debug_mode: false
  gpg:
    key_servers: ["https://keys.openpgp.org", "https://keyserver.ubuntu.com"]
```

`domains` задаёт публичные host и CORS по умолчанию. `cors_origins` добавляет exact origins, host или wildcard; `*`
разрешает всё. Перенаправленный IP доверяется только от соединения из `trusted_proxies`. Изменения host, port, TLS,
compression, debug и части cache требуют перезапуск.

GitHub OAuth хранится в `server.github_oauth`; Client ID и секрет только для записи задаются в интерфейсе.

Другие [провайдеры OAuth](../security/oauth-login.md) настраиваются в `server.oauth_providers`. Редактор администратора
предлагает официальные шаблоны, сохраняет учётные данные без их возврата и сразу применяет изменения к новой
авторизации.

### Подключение `database`

```yaml
database:
  driver: "sqlite3"
  dsn: "renop.db"
  max_open_conns: 25
  max_idle_conns: 25
  conn_max_lifetime_sec: 300
```

Поддерживаются `sqlite3` (или `sqlite`), `mysql`, `postgres` и нативный `clickhouse`. См.
[Настройка базы](./database.md).

### Исходящий маршрут `proxy`

```yaml
proxy:
  selected: ""
  proxies:
    - name: "corp_proxy"
      url: "http://proxy.internal:8080"
      username: ""
      password: ""
```

Можно задать до 16 HTTP, HTTPS или SOCKS5 прокси. См. [Настройка прокси](./outbound-proxy.md).

### Оформление `frontend`

```yaml
frontend:
  id: "renop"
  title: "RenoP Package Registry"
  description: "Self-hosted package repository"
  organization_website: ""
  organization_logo: "/svg/logo.svg"
  background_url: ""
  font_preset: "system"
  font_url: ""
  icp_license: ""
  public_security_filing: ""
```

URL проверяются до использования. Фон должен соответствовать политике WebP и размера.
`font_preset` принимает `system`, `inter`, `noto_sans`, `open_sans`, `source_sans` или `custom`. Предустановки
используют локальные шрифты. Пользовательское значение может быть прямой ссылкой на WOFF2, WOFF или TTF либо CSS-ссылкой
Google Fonts. Оно загружается в фоне и применяется после загрузки основной гарнитуры.

### Политика `updater`

```yaml
updater:
  channel: "release"
  mode: "manual"
```

`channel` — `release` или `nightly`; `mode` — `manual`, `auto_check` или `auto_install`. Автопроверки объединяются
планировщиком процесса, а результаты отправляются администраторам.

[Правовые документы и cookie](./legal.md)
