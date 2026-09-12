---
title: Контейнеры
order: 7
category: Развёртывание
description: Запуск RenoP в Docker, Podman или Kubernetes
---

# Контейнеры

## Запуск

Релизные сборки публикуют `mvnc.pkg.one/oci/renop:<version>` и `:latest` для Linux amd64 и arm64. Образ использует исполняемые файлы релиза и работает с UID/GID 65532.

```bash
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
docker logs renop
```

Откройте `http://localhost:3000`. Если при первом запуске не задана переменная `RENOP_DEFAULT_ADMIN_PASSWORD`, начальный пароль администратора один раз выводится в журнал. Для Podman замените `docker` на `podman`.

На macOS запускайте Linux-образ в Docker Desktop, Podman machine, Colima, OrbStack, Rancher Desktop или Apple container. В образе задано `RENOP_CONTAINER=1`, поэтому скрытые стандартные маркеры не мешают обнаружению контейнера.

## Постоянные данные

Подключите доступный для записи том к `/data`. В нём хранятся `renop-settings.db`, SQLite, индекс, пакеты и другие относительные пути. Каталоги хоста должны разрешать запись UID/GID 65532; для Podman с SELinux может потребоваться `:Z`. Подключайте весь каталог, чтобы SQLite мог создавать файлы журнала.

`RENOP_SETTINGS_DB` и `RENOP_INDEX` по умолчанию указывают на файлы в `/data`. Для корневой файловой системы только для чтения нужен записываемый `/tmp`, например `--read-only --tmpfs /tmp`. Сохраняйте том при пересоздании контейнера и делайте резервную копию перед обновлением.

## Обновление и остановка

В контейнере отключены автоматические проверки, онлайн- и офлайн-обновления исполняемого файла, установка системной службы и внутренние перезапуски. Обновляйте через среду контейнеров. SIGTERM завершает HTTP-запросы, фоновые операции и запись состояния БД. Выделите 90 секунд на остановку.

```bash
docker pull mvnc.pkg.one/oci/renop:latest
docker stop --time 90 renop
docker rm renop
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
```

Для фиксированной версии используйте конкретный тег или дайджест образа. После миграции конфигурации откат может потребовать восстановления соответствующей резервной копии.

## Kubernetes

Для SQLite и локального хранилища используйте одну реплику, постоянный том и `Recreate`. Фрагмент Deployment предполагает существующий PVC `renop-data`; добавьте обычные метаданные и селекторы. Измените пробу при смене порта или включении TLS приложения.

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

## Локальная сборка

Используйте специальный Go из проекта, Node.js 24, pnpm и protoc. Подготовьте исходники один раз, затем соберите каждую архитектуру. Dockerfile использует `container-bin/<arch>/renop`, не загружая другой исполняемый файл и не заменяя Go при сборке.

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
