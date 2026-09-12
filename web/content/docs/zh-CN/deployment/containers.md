---
title: 容器部署
order: 7
category: 部署
description: 使用 Docker、Podman 或 Kubernetes 运行 RenoP
---

# 容器部署

## 运行

Release 构建为 Linux amd64 和 arm64 发布 `mvnc.pkg.one/oci/renop:<version>` 和 `:latest`。镜像复用发行版可执行文件，以 UID/GID 65532 运行。

```bash
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
docker logs renop
```

打开 `http://localhost:3000`。首次启动时，若未提供 `RENOP_DEFAULT_ADMIN_PASSWORD`，初始管理员密码会在日志中打印一次。使用 Podman 时，将命令中的 `docker` 替换为 `podman`。

macOS 上可通过 Docker Desktop、Podman machine、Colima、OrbStack、Rancher Desktop 或 Apple container 运行 Linux 镜像。镜像设置了 `RENOP_CONTAINER=1`，即使运行时隐藏常见容器标记，也能识别容器环境。

## 持久化数据

将可写卷挂载到 `/data`。此目录保存 `renop-settings.db`、SQLite 数据库、索引、软件包和其他相对存储路径。绑定挂载须允许 UID/GID 65532 写入；在启用 SELinux 的主机上使用 Podman 时可能需要 `:Z`。请挂载整个目录，以便 SQLite 创建事务日志文件。

`RENOP_SETTINGS_DB` 和 `RENOP_INDEX` 默认指向 `/data` 中的文件。只读根文件系统还需要可写的 `/tmp`，例如 `--read-only --tmpfs /tmp`。替换容器时保留持久卷，并在升级前备份。

## 升级和退出

检测到容器后，会禁用自动检查、在线及离线可执行文件更新、服务安装和程序内重启。请通过容器运行时更新。SIGTERM 会等待 HTTP 请求、后台任务和数据库状态收尾。请预留 90 秒退出时间。

```bash
docker pull mvnc.pkg.one/oci/renop:latest
docker stop --time 90 renop
docker rm renop
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
```

需要固定部署版本时，使用明确的版本标签或镜像摘要。若配置已经迁移，回滚可能需要恢复对应的备份。

## Kubernetes

使用 SQLite 或本地存储时，采用单副本、持久卷和 `Recreate` 策略。以下 Deployment 片段假定已有 `renop-data` PVC；请补充常规元数据和选择器。修改端口或启用应用 TLS 后，应同步调整探针。

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

## 本地构建

使用仓库指定的 Go 运行时、Node.js 24、pnpm 和 protoc。共享源码只准备一次，再分别编译容器架构。Dockerfile 使用 `container-bin/<arch>/renop` 中的二进制文件，不下载其他可执行文件，也不更换 Go 工具链重新编译。

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
