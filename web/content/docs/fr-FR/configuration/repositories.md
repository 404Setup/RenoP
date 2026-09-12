---
title: Dépôts et miroirs
order: 2
category: Configuration
description: Moteurs, visibilité, miroirs, migration et stockage S3
---

# Dépôts et miroirs

Les définitions des dépôts sont stockées dans la base de données et modifiées dans la gestion des dépôts. Lors de
la première mise à niveau, RenoP importe `repositories.yaml` (ou `RENOP_REPOSITORIES`) uniquement en l’absence de
configuration en base, puis archive le fichier sous `repositories.yaml.migrated.<id>`. Une entrée invalide arrête
le démarrage sans remplacer le fichier. La base reste prioritaire, même si aucun dépôt n’y figure. Une nouvelle
installation laisse la liste des dépôts vide et ne produit pas de YAML. Sauvegardez la base et protégez l’archive,
qui peut contenir des identifiants S3 et de miroir. Le nom est un slug minuscule immuable et le premier segment URL.

Une nouvelle installation commence avec une liste de dépôts vide. Créez les dépôts explicitement dans l’administration. Les instantanés existants et la migration unique des anciennes configurations conservent leurs dépôts.

## Exemple de migration héritée

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
    mirrors: []
  crates:
    name: crates
    format: cargo
    visibility: PUBLIC
    mirrors: []
  containers:
    name: containers
    format: docker
    visibility: PRIVATE
    allow_redeployment: false
    mirrors: []
```

## Champs du dépôt

| Champ                   | Défaut          | Description                                                                 |
|:------------------------|:----------------|:----------------------------------------------------------------------------|
| `name`                  | Requis          | Slug immuable et préfixe URL                                                |
| `format`                | `maven`         | `maven`, `maven-classic`, `files`, `npm`, `cargo`, `docker`, `conan`, `conda`, `conda-native`, `apk`, `apt`, `rpm`, `yum` |
| `visibility`            | `PUBLIC`        | `PUBLIC`, `HIDDEN` ou `PRIVATE`                                             |
| `allow_redeployment`    | `false`         | Redéploiement Maven ou remplacement files/Docker si pris en charge          |
| `require_gpg_signature` | `false`         | Validation OpenPGP détachée obligatoire pour Maven                          |
| `publication_review`    | `off`           | Examen Maven/npm/Cargo/Docker : `off`, `new_packages` ou `every_version`    |
| `download_statistics`   | Selon le moteur | Activé pour Maven/npm/Cargo/Docker ; `files` doit être activé explicitement |
| `mirrors`               | `[]`            | Miroirs ordonnés                                                            |
| `s3`                    | absent          | Stockage S3 propre au dépôt                                                 |

Pour npm et Docker, `new_packages` examine la demande de création explicite avant de réserver le nom ;
`every_version` examine aussi chaque version ou manifeste ultérieur. Maven et Cargo ne créent pas de paquet vide : leur
politique `new_packages` examine donc la première publication. Les imports de miroir contournent toujours l’examen.

`maven-classic` ne change que l’interface et conserve les règles Maven. `files` est non structuré, sans sommes, POM ni
signature. Maven peut migrer vers `files` puis revenir sans déplacer les objets ; le retour reconstruit le catalogue et
restaure la politique Maven. La migration conserve l’état effectif des statistiques de téléchargement.

Dans les dépôts `files`, les envois et téléchargements miroir préservent les fichiers voisins même si les chemins
contiennent `SNAPSHOT` ou se terminent par `.md5`, `.asc` ou `-javadoc.jar`. Le nettoyage des versions et l’extraction
Javadoc s’appliquent uniquement à Maven.

L’examen des publications prend en charge Maven, npm, Cargo, Docker et les ressources natives gérées. Pour Maven, il force `allow_redeployment` à
`false` ; npm
conserve sa transaction de version et de dist-tags immuables. Les fichiers locaux restent masqués jusqu’à l’approbation
d’un modérateur du dépôt ou d’un administrateur système, et les miroirs ne sont jamais examinés. Une tâche en attente
interdit la modification, la suppression ou la migration du dépôt.

Un dépôt `npm` exige la réservation avant publication, conserve versions SemVer immuables et dist-tags, gère les
paquets privés scoped, les équipes L0-L4 et les miroirs par nom exact ou règle `@scope/*`.

### Visibilité

- **PUBLIC** : lecture et découverte anonymes.
- **HIDDEN** : absent des catalogues anonymes ou sans droit et des profils. Les gestionnaires et les utilisateurs ayant
  un droit explicite de navigation le découvrent ; un chemin de fichier exact reste lisible.
- **PRIVATE** : lecture, listes et écriture exigent un droit explicite. Une image Docker privée ajoute son équipe L0-L4.

## Miroirs amont

Un objet absent peut être diffusé depuis un miroir activé, puis persisté sans mettre tout le corps en mémoire. Cargo et
Docker interdisent une création locale si un nom applicable existe en amont.

```yaml
mirrors:
  - name: "central"
    url: "https://repo1.maven.org/maven2"
    persist: true
    cache_ttl_secs: 86400
    negative_cache: true
    timeout_secs: 30
    proxy: ""
    allow_artifacts: []
    deny_artifacts: []
```

| Champ             | Défaut  | Description                                   |
|:------------------|:--------|:----------------------------------------------|
| `name`            | Requis  | Nom unique dans le dépôt                      |
| `url`             | Requis  | URL de base amont                             |
| `persist`         | `true`  | Stocker les réponses réussies                 |
| `cache_ttl_secs`  | `86400` | Durée du cache positif                        |
| `negative_cache`  | `true`  | Mettre en cache les absences prises en charge |
| `timeout_secs`    | `30`    | Délai d’une requête amont                     |
| `proxy`           | `""`    | Route globale, `direct` ou proxy nommé        |
| `allow_artifacts` | `[]`    | Règles d’autorisation selon le format         |
| `deny_artifacts`  | `[]`    | Règles de refus prioritaires                  |

Les identifiants utilisent les champs structurés d’autorisation et ne doivent jamais être inclus dans `url`.

## Stockage compatible S3

Chaque dépôt choisit Disk ou son propre S3. Le verrou du dépôt sérialise un changement avec uploads, suppressions,
commits GPG et écritures de miroir.

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

MinIO demande souvent `force_path_style`. Avec `redirect_downloads`, RenoP autorise puis renvoie une redirection signée
de courte durée ; sinon il diffuse l’objet lui-même.

`capacity_limit_bytes` fixe la limite d’octets installés par dépôt ; `0` signifie illimité. L’interface utilise les MiB. Sur disque comme sur S3, le total comprend artefacts, sommes de contrôle, objets en attente de revue et cache des miroirs, hors copies temporaires. Une réservation avant validation partage la limite entre envois concurrents. Un dépassement retourne `507` avec `repository_capacity_exceeded` ; une réponse de miroir valide peut encore être transmise sans cache. Abaisser la limite sous l’usage existant ne bloque pas les lectures. Après une modification externe du stockage, reconstruisez l’index ou redémarrez pour recalculer. Un ancien client omettant le champ facultatif conserve la limite.

## Dépôts de paquets natifs

Conan prend en charge les révisions des recettes et des binaires avec son client natif. Conda et Conda native acceptent les paquets `.conda` et `.tar.bz2` dans des répertoires de plateforme comme `noarch/` et génèrent `repodata.json`. APK génère `APKINDEX.tar.gz` à côté des paquets dans des répertoires d’architecture comme `x86_64/`. apt accepte les fichiers `.deb`, génère `Packages`, `Packages.gz` et `dists/<suite>/Release`, et associe `pool/<component>/` au composant correspondant. rpm/yum génère `repodata/repomd.xml` et les métadonnées référencées.

Réservez chaque ressource native hébergée dans le navigateur du dépôt avant publication. Son identité vient des métadonnées du paquet ou de la recette Conan, sans préfixe d’équipe globale obligatoire. L0 lit, L1 publie, L2 remplace ou supprime les versions, L3 gère les paramètres et membres, L4 possède la ressource. Au moins un propriétaire L4 doit rester. Le droit d’écriture du dépôt ne donne pas accès aux ressources d’autrui. Les miroirs natifs restent en lecture seule.

`new_packages` valide la première publication après réservation ; `every_version` valide chaque version native ou révision Conan. Les signatures incomplètes et publications en attente restent invisibles dans les téléchargements et index, même après redémarrage. APK exige une clé publique RSA PEM et une signature native couvrant contrôle et contenu. RPM exige une signature OpenPGP fiable couvrant le contenu directement ou via son empreinte signée. Configurez la clé dans les paramètres de la ressource. APT signe les index du dépôt, sans signature détachée par `.deb`. Conda utilise ses empreintes natives, sans fichier `.asc` supplémentaire.

Les nouveaux envois ne peuvent pas remplacer les index générés. Les anciennes métadonnées sont conservées jusqu’à suppression par un administrateur. Un index automatique est limité à 10 000 paquets ; répartissez les grands ensembles entre dépôts ou sous-répertoires natifs. Un paquet est limité à 8 GiB, avec 256 fichiers non publiés par ressource et 16 384 par instance. Les anciens fichiers non enregistrés restent lisibles ; un administrateur doit les traiter avant leur remplacement par une publication gérée en conflit.

Les métadonnées APK, apt et rpm générées sont signées avec des clés indépendantes conservées dans la base privée des paramètres. Les clés publiques se trouvent respectivement dans `renop.rsa.pub`, `renop.asc` et `repodata/repomd.xml.key`. apt fournit aussi `InRelease` et `Release.gpg`, et rpm fournit `repomd.xml.asc`. Importez la clé avant d’utiliser les commandes présentées par le navigateur de dépôt. Les signatures des paquets rpm restent la responsabilité des éditeurs et nécessitent leurs propres clés.

Conan utilise le manifeste officiel de son extension de signature et une signature OpenPGP. Installez `scripts/conan/sign.py` dans `<CONAN_HOME>/extensions/plugins/sign/sign.py`, enregistrez la clé publique, puis définissez `RENOP_CONAN_GPG_KEY` avec l’empreinte de la clé privée et `RENOP_CONAN_GPG_KEYRING` avec un trousseau public binaire fiable pour `gpgv`. Les fichiers restent masqués jusqu’à réception de tout le manifeste et de sa signature. Les nouvelles tentatives identiques pour des fichiers Conan publiés sont idempotentes.

```sh
conan cache sign "PACKAGE/*"
conan upload "PACKAGE/*" --remote "REPOSITORY" --confirm
```


## Contenus de fichiers partagés

Les fichiers Disk identiques partagent leur contenu immuable par des liens physiques tout en conservant leurs chemins logiques. Les écritures remplacent atomiquement un fichier ; supprimer un chemin préserve les autres références. Une seule tâche de fond analyse les fichiers existants par lots, ignore les dépôts occupés et marque une pause entre les lots. Les index modifiables, les fichiers temporaires et les caches de miroirs Disk avec expiration restent indépendants.

S3 utilise des références logiques vers un contenu partagé à partir de 64 KiB ; les objets plus petits restent directs. Les empreintes et références sont conservées dans l’index privé des fichiers. Les enregistrements connus évitent les vérifications répétées des métadonnées. Un ancien objet sans empreinte fiable reste intact jusqu’à ce qu’un téléversement normal ou un téléchargement complet via RenoP fournisse cette empreinte ; la déduplication ne télécharge pas ces objets uniquement pour les comparer. La reconstruction de l’index peut utiliser des pages LIST et vérifier une fois les métadonnées de référence manquantes. La collecte régulière traite la file de récupération indexée au lieu de parcourir constamment le compartiment.

Une sauvegarde S3 complète comprend l’espace privé `.renop-content-v1`, les objets des dépôts et l’index privé. Les déploiements RenoP indépendants doivent utiliser des préfixes de clés distincts. Les contenus sans référence sont récupérés après un délai de grâce. La capacité des dépôts et les statistiques de téléchargement utilisent toujours les tailles logiques, indépendamment du partage physique.
