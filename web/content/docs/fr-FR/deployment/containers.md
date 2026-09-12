---
title: Conteneurs
order: 7
category: Déploiement
description: Exécuter RenoP avec Docker, Podman ou Kubernetes
---

# Conteneurs

## Exécution

Les versions publient `mvnc.pkg.one/oci/renop:<version>` et `:latest` pour Linux amd64 et arm64. L’image réutilise les exécutables de la version et fonctionne avec UID/GID 65532.

```bash
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
docker logs renop
```

Ouvrez `http://localhost:3000`. Le mot de passe administrateur initial apparaît une fois dans les journaux si `RENOP_DEFAULT_ADMIN_PASSWORD` n’est pas fourni au premier démarrage. Pour Podman, remplacez `docker` par `podman`.

Sur macOS, utilisez l’image Linux avec Docker Desktop, Podman machine, Colima, OrbStack, Rancher Desktop ou Apple container. L’image définit `RENOP_CONTAINER=1`, même si le moteur masque les marqueurs habituels de conteneur.

## Données persistantes

Montez un volume accessible en écriture sur `/data`. Il contient `renop-settings.db`, la base SQLite, l’index, les paquets et les autres chemins relatifs. Les montages hôtes doivent autoriser UID/GID 65532 ; Podman avec SELinux peut nécessiter `:Z`. Montez le répertoire entier pour permettre la création des journaux SQLite.

`RENOP_SETTINGS_DB` et `RENOP_INDEX` désignent par défaut des fichiers dans `/data`. Une racine en lecture seule nécessite aussi `/tmp` accessible en écriture, par exemple `--read-only --tmpfs /tmp`. Conservez le volume lors du remplacement du conteneur et sauvegardez-le avant une mise à jour.

## Mise à jour et arrêt

La détection du conteneur désactive les vérifications automatiques, les mises à jour en ligne/hors ligne de l’exécutable, l’installation de services et les redémarrages internes. Utilisez le moteur de conteneurs. SIGTERM termine les requêtes HTTP, les tâches et les écritures de base de données. Prévoyez 90 secondes.

```bash
docker pull mvnc.pkg.one/oci/renop:latest
docker stop --time 90 renop
docker rm renop
docker run -d --name renop --restart unless-stopped --stop-timeout 90 \
  -p 3000:3000 -v renop-data:/data mvnc.pkg.one/oci/renop:latest
```

Utilisez une version précise ou un digest pour figer le déploiement. Une migration de configuration peut nécessiter la restauration de la sauvegarde correspondante pour revenir en arrière.

## Kubernetes

Avec SQLite ou le stockage local, utilisez une réplique, un volume persistant et `Recreate`. Ce fragment Deployment suppose un PVC `renop-data` existant ; ajoutez les métadonnées et sélecteurs habituels. Adaptez la sonde si le port change ou si TLS est activé dans l’application.

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

## Construction locale

Utilisez le Go personnalisé du dépôt, Node.js 24, pnpm et protoc. Préparez les sources une fois puis compilez chaque architecture. Le Dockerfile utilise `container-bin/<arch>/renop` ; il ne télécharge pas un autre exécutable et ne recompile pas avec une autre chaîne Go.

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
