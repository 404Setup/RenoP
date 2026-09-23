---
title: Containers
order: 7
category: Deployment
description: Run RenoP with Docker, Podman, or Kubernetes
---

# Containers

## Run

Release builds publish `mvnc.pkg.one/oci/renop:<version>` and `:latest` for Linux amd64 and arm64. The image reuses the
release executables and runs as UID/GID 65532.

```bash
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
docker logs renop
```

Open `http://localhost:3000`. The initial administrator password is printed once in the logs unless
`RENOP_DEFAULT_ADMIN_PASSWORD` is supplied on first start. With Podman, replace `docker` with `podman`.

On macOS, use the Linux image in Docker Desktop, Podman machine, Colima, OrbStack, Rancher Desktop, or Apple container.
The image sets `RENOP_CONTAINER=1`, including when the runtime hides standard container markers.

## Persistent data

Mount a writable volume at `/data`. It contains `renop-settings.db`, the SQLite database, index, packages, and other
relative storage paths. Bind mounts must be writable by UID/GID 65532; Podman on SELinux hosts may require `:Z`. Mount
the directory so SQLite can create its journal files.

`RENOP_SETTINGS_DB` and `RENOP_INDEX` default to files inside `/data`. A read-only root filesystem also needs writable
`/tmp`, for example `--read-only --tmpfs /tmp`. Keep the persistent volume when replacing the container; back it up
before upgrades.

## Upgrade and shutdown

Container detection disables automatic checks, online/offline executable updates, service installation, and in-process
restarts. Update through the runtime. SIGTERM drains HTTP requests, background work, and database state. Allow 90
seconds for shutdown.

```bash
docker pull mvnc.pkg.one/oci/renop:latest
docker stop --time 90 renop
docker rm renop
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
```

Use a specific version tag or image digest when deployment needs a fixed version. Configuration migrations may require
restoring the matching backup for rollback.

## Kubernetes

Use one replica with a persistent volume and `Recreate` when using SQLite/local storage. This Deployment fragment
assumes an existing `renop-data` PVC; add your normal Deployment metadata and selectors. Adapt the probe if you change
the port or enable application TLS.

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

## Build locally

Use the repository’s custom Go runtime, Node.js 24, pnpm, and protoc. Prepare sources once, then compile each container
architecture. The Dockerfile packages binaries from `container-bin/<arch>/renop`; it does not download an unrelated
executable or rebuild with a different Go toolchain.

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
