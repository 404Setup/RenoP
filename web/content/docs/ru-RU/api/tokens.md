---
title: API-токены и пользователи
order: 3
category: Справочник API
description: Жизненный цикл API-токенов, границы аутентификации и эндпоинты управления пользователями
---

# API-токены и пользователи

API-токены представляют собой долговечные учетные данные для автоматизации и клиентов. RenoP сохраняет только SHA-256
хеш каждого секрета. Открытое значение возвращается только один раз при создании или ротации.

Каждый запрос проходит две независимые проверки:

- Токен должен содержать требуемую область разрешений (Scope).
- Аккаунт владельца должен сохранять право выполнять данную операцию с целевым ресурсом.

Изменение роли аккаунта или прав в команде пакета применяется немедленно без необходимости повторного создания токенов.

## Управление вашими API-токенами

Эндпоинты управления токенами требуют cookie браузера HttpOnly `renop_session`. API-токены, пароли и параметры запроса
не могут управлять секретами.

### Список доступных областей

`GET /api/auth/profile/api-tokens/scopes`

Ответ фильтруется для текущего аккаунта. Области администратора никогда не предлагаются обычным пользователям.

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

### Создать токен

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

`expires_at` представляет собой необязательную метку времени в миллисекундах от 5 минут до 5 лет. Значение null или его
отсутствие создает токен без ограничения срока действия. Аккаунт может иметь до 50 токенов.

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

### Список метаданных токенов

`GET /api/auth/profile/api-tokens`

Ответ содержит несекретные метаданные и лимит аккаунта, никогда не раскрывая секреты токенов.

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

### Редактировать токен

`PUT /api/auth/profile/api-tokens/{token_id}`

Обновите имя, области разрешений или целевые ограничения токена без изменения его секрета.

```json
{
  "name": "CI publishing updated",
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "targets": {
    "repository:publish": ["releases"]
  }
}
```

Эндпоинт возвращает обновленные метаданные токена:

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

### Ротировать токен

`POST /api/auth/profile/api-tokens/{token_id}/rotate`

Перевыпустите секрет для указанного токена с сохранением имени и областей разрешений. Прежний секрет аннулируется
немедленно.

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

### Обновить состояние токена

`PUT /api/auth/profile/api-tokens/{token_id}/state`

Временно отключите или включите токен без его отзыва.

```json
{
  "disabled": true
}
```

Эндпоинт возвращает подтверждение обновленного состояния:

```json
{
  "disabled": true
}
```

### Отозвать токен

`DELETE /api/auth/profile/api-tokens/{token_id}`

При успешном отзыве возвращается HTTP 204 No Content и немедленно сбрасывается кэш аутентификации.

## Управление активными сессиями и блокировкой IP

Просматривайте активные сессии браузера, запросы Basic Auth и обращения по API-токенам. Отслеживайте недавние IP-адреса
и блокируйте подозрительных клиентов.

### Список активных сессий

`GET /api/auth/profile/sessions`

Возвращает список активных сессий, объединенных по устройствам. Каждая запись содержит до десяти недавних IP-адресов.

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

### Отозвать активные сессии

`DELETE /api/auth/profile/sessions/{session_id}`

Отозвать отдельную сессию по ее идентификатору. Чтобы отозвать все остальные сессии и сохранить текущее устройство:

`POST /api/auth/profile/sessions/revoke-others`

Оба эндпоинта возвращают HTTP 200 с объектом StatusOk при успешном выполнении.

### Управление заблокированными IP-адресами

Блокировка IP на уровне аккаунта предотвращает вход с указанных адресов.

Получить список заблокированных IP-адресов:

`GET /api/auth/profile/ip-bans`

```json
{
  "ips": ["203.0.113.195"]
}
```

Заблокировать IP-адрес:

`POST /api/auth/profile/ip-bans`

```json
{
  "ip": "203.0.113.195"
}
```

Разблокировать IP-адрес:

`DELETE /api/auth/profile/ip-bans/{ip}`

## Справочник областей разрешений

| Область               | Возможности                                                                 |
|:----------------------|:----------------------------------------------------------------------------|
| `repository:read`     | Чтение каталогов репозитория, метаданных, файлов, образов и версий          |
| `repository:publish`  | Публикация через Maven, npm, Cargo, Docker, файлы или поблочную загрузку    |
| `repository:delete`   | Удаление файлов репозитория, версий пакетов, тегов или образов              |
| `package:create`      | Резервирование новых пакетов npm/Cargo или образов Docker после авторизации |
| `package:metadata`    | Обновление описаний пакетов и других метаданных                             |
| `package:lifecycle`   | Архивирование, восстановление или изменение видимости пакетов и версий      |
| `team:manage`         | Просмотр и управление командами и приглашениями npm, Cargo, Docker и Maven  |
| `domain:read`         | Чтение конфигурации доменов публикации Maven                                |
| `domain:create`       | Создание доменов публикации Maven                                           |
| `domain:verify`       | Запрос или принудительная проверка владения доменом Maven                   |
| `domain:lifecycle`    | Закрытие или повторный запрос доменов публикации Maven                      |
| `messages:read`       | Чтение, отметка и удаление сообщений аккаунта                               |
| `account:read`        | Чтение личных данных аккаунта и личного журнала аудита                      |
| `account:write`       | Обновление публичного профиля аккаунта через API                            |
| `statistics:read`     | Запрос статистики скачиваний, доступной аккаунту                            |
| `admin:users`         | Управление аккаунтами пользователей и устройствами входа                    |
| `admin:repositories`  | Управление репозиториями и перестроение индексов                            |
| `admin:settings`      | Управление системными настройками и диагностикой                            |
| `admin:audit`         | Чтение или очистка журналов аудита администратора                           |
| `admin:notifications` | Составление уведомлений администратора                                      |
| `admin:updates`       | Проверка, загрузка, установка обновлений и перезапуск системы               |
| `admin:statistics`    | Запрос общесистемной статистики скачиваний                                  |

Области `admin:*` могут создаваться только администратором и перестают действовать сразу после утраты этой роли.

## Использование токена

Используйте токен в заголовке Bearer для автоматизации запросов к API:

```http
Authorization: Bearer rnp_pat_REDACTED
```

Клиенты пакетных менеджеров при Basic Auth должны передавать API-токен в качестве пароля. Пароль от аккаунта
использовать запрещено. Права строго определяются областями токена.

```http
Authorization: Basic YWxpY2U6cm5wX3BhdF9SRURBQ1RFRF9UT0tFTg==
```

Клиент npm отправляет токен через `_authToken` или Basic Auth. Cargo передает его в заголовке Authorization. Docker
запрашивает краткосрочный доступ через `GET /v2/token`.

## Эндпоинты совместимости

Операции управления пользователями администратором расположены на `GET /api/tokens`. Устаревший эндпоинт
`POST /api/auth/profile/token` сохранен для обратной совместимости. Для новых интеграций используйте эндпоинты профиля.
