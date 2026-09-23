---
title: Jetons d'API et utilisateurs
order: 3
category: Référence de l'API
description: Cycle de vie précis des jetons d'API, limites d'authentification et points de terminaison utilisateur
---

# Jetons d'API et utilisateurs

Les jetons d'API sont des identifiants durables pour les machines. RenoP enregistre uniquement le hachage SHA-256 de
chaque secret. La valeur en clair n'est retournée qu'une seule fois lors de la création ou de la rotation.

Chaque requête doit satisfaire deux contrôles indépendants :

- Le jeton doit inclure la portée exigée par le point de terminaison.
- Le compte propriétaire doit toujours avoir le droit d'effectuer cette opération sur la ressource ciblée.

La mise à jour du rôle ou des autorisations d'équipe prend effet immédiatement sans devoir recréer les jetons.

## Gérer vos jetons d'API

Les points de terminaison de gestion exigent le cookie HttpOnly `renop_session`. Les jetons d'API, mots de passe et
paramètres de requête ne peuvent pas administrer les secrets.

### Lister les portées attribuables

`GET /api/auth/profile/api-tokens/scopes`

La réponse est filtrée selon le compte actuel. Les portées d'administration ne sont jamais proposées aux utilisateurs
ordinaires.

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

### Créer un jeton

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

`expires_at` est un horodatage Unix facultatif en millisecondes entre cinq minutes et cinq ans. Une valeur nulle ou
omise crée un jeton sans expiration. Chaque compte peut posséder jusqu'à 50 jetons d'API.

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

### Consulter les métadonnées des jetons

`GET /api/auth/profile/api-tokens`

La réponse contient les métadonnées non sensibles et la limite du compte, sans jamais révéler les secrets.

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

### Modifier un jeton

`PUT /api/auth/profile/api-tokens/{token_id}`

Modifiez le nom, les portées ou les cibles restreintes d'un jeton sans altérer son secret existant.

```json
{
  "name": "CI publishing updated",
  "scopes": ["repository:read", "repository:publish", "package:metadata"],
  "targets": {
    "repository:publish": ["releases"]
  }
}
```

Le point de terminaison renvoie les métadonnées mises à jour :

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

### Faire tourner un jeton

`POST /api/auth/profile/api-tokens/{token_id}/rotate`

Régénérez le secret d'un jeton tout en conservant son nom et ses portées. L'ancien secret cesse de fonctionner
immédiatement.

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

### Mettre à jour l'état d'un jeton

`PUT /api/auth/profile/api-tokens/{token_id}/state`

Désactivez ou réactivez temporairement un jeton d'API sans le révoquer.

```json
{
  "disabled": true
}
```

Le point de terminaison confirme l'état mis à jour :

```json
{
  "disabled": true
}
```

### Révoquer un jeton

`DELETE /api/auth/profile/api-tokens/{token_id}`

Une révocation réussie renvoie HTTP 204 No Content et invalide immédiatement le cache d'authentification.

## Gérer les sessions actives et les blocages IP

Inspectez les sessions actives du navigateur, les requêtes Basic Auth et les accès par jeton d'API. Suivez les adresses
IP récentes et bloquez directement les clients suspects.

### Lister les sessions actives

`GET /api/auth/profile/sessions`

Renvoie les sessions actives regroupées par appareil avec jusqu'à dix adresses IP récentes pour chaque entrée.

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

### Révoquer les sessions actives

`DELETE /api/auth/profile/sessions/{session_id}`

Révoquez une session individuelle à partir de son identifiant. Pour fermer toutes les autres sessions en conservant
l'appareil actuel :

`POST /api/auth/profile/sessions/revoke-others`

Les deux points de terminaison renvoient HTTP 200 avec StatusOk en cas de succès.

### Gérer les adresses IP bloquées

Le blocage IP au niveau du compte empêche des adresses spécifiques de se connecter à ce compte.

Consulter la liste des adresses IP bloquées :

`GET /api/auth/profile/ip-bans`

```json
{
  "ips": ["203.0.113.195"]
}
```

Bloquer une nouvelle adresse IP :

`POST /api/auth/profile/ip-bans`

```json
{
  "ip": "203.0.113.195"
}
```

Débloquer une adresse IP :

`DELETE /api/auth/profile/ip-bans/{ip}`

## Référence des portées

| Portée                | Capacité                                                                   |
|:----------------------|:---------------------------------------------------------------------------|
| `repository:read`     | Lire les catalogues, métadonnées, fichiers, images et versions du dépôt    |
| `repository:publish`  | Publier via Maven, npm, Cargo, Docker, fichiers ou téléversement morcelé   |
| `repository:delete`   | Supprimer des fichiers, versions de paquet, étiquettes ou images           |
| `package:create`      | Réserver de nouveaux paquets npm/Cargo ou images Docker après autorisation |
| `package:metadata`    | Mettre à jour les descriptions et métadonnées de paquets                   |
| `package:lifecycle`   | Archiver, restaurer ou modifier la visibilité des paquets et versions      |
| `team:manage`         | Consulter et administrer les équipes npm, Cargo, Docker et Maven           |
| `domain:read`         | Lire la configuration privée des domaines de publication Maven             |
| `domain:create`       | Créer des domaines de publication Maven                                    |
| `domain:verify`       | Demander ou forcer la vérification de propriété de domaine Maven           |
| `domain:lifecycle`    | Fermer ou récupérer des domaines de publication Maven                      |
| `messages:read`       | Lire, marquer et supprimer les messages du compte                          |
| `account:read`        | Lire les données privées du compte et le journal personnel d'activité      |
| `account:write`       | Mettre à jour le profil public du compte via l'API                         |
| `statistics:read`     | Consulter les statistiques de téléchargement accessibles au compte         |
| `admin:users`         | Administrer les comptes utilisateurs et leurs appareils de connexion       |
| `admin:repositories`  | Administrer les dépôts et reconstruire les index                           |
| `admin:settings`      | Administrer les paramètres système et les diagnostics                      |
| `admin:audit`         | Lire ou purger les journaux d'activité visibles par l'administrateur       |
| `admin:notifications` | Rédiger des notifications d'administration                                 |
| `admin:updates`       | Vérifier, téléverser, installer les mises à jour et redémarrer le système  |
| `admin:statistics`    | Consulter les statistiques de téléchargement de l'ensemble du système      |

Les portées `admin:*` ne peuvent être créées que par un administrateur et cessent d'autoriser les actions dès la perte
de ce rôle.

## Utiliser un jeton

Utilisez un jeton comme identifiant Bearer pour l'automatisation d'API :

```http
Authorization: Bearer rnp_pat_REDACTED
```

Les clients de paquets doivent utiliser Basic Auth avec un jeton d'API comme mot de passe. Le mot de passe de connexion
au compte est interdit. Les droits découlent strictement des portées du jeton.

```http
Authorization: Basic YWxpY2U6cm5wX3BhdF9SRURBQ1RFRF9UT0tFTg==
```

Un client npm transmet le jeton via `_authToken` ou l'authentification Basic. Cargo l'envoie dans l'en-tête
Authorization. Docker échange les identifiants sur `GET /v2/token`.

## Points de terminaison de compatibilité

Les opérations d'administration utilisateur sont situées sur `GET /api/tokens`. L'ancien point de terminaison
`POST /api/auth/profile/token` reste disponible pour la rétrocompatibilité. Les nouvelles intégrations doivent utiliser
les points de terminaison de profil.
