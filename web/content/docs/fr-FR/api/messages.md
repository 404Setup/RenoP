---
title: API du centre de messages
order: 7
category: Référence API
description: Notifications, compteurs, actions de workflow et annonces administrateur
---

# API du centre de messages

Toutes les routes exigent une authentification. Les réponses utilisent protobuf par défaut et ne sont jamais mises en
cache. Un API Token requiert `messages:read`; la composition exige aussi `admin:notifications` et le rôle
administrateur.

## Lister ou effacer les messages

- **Lister** : `GET /api/messages?limit=30&cursor=...`
- **Effacer les messages résolus** : `DELETE /api/messages`
- `limit` vaut 1 à 100. `cursor` est le `next_cursor` opaque de la page précédente.
- L’effacement conserve tout message dont l’action de workflow est encore `pending`.

### Exemple de réponse décodée

```json
{
  "messages": [
    {
      "id": "00000000-0000-4000-8000-000000000001",
      "kind": "announcement",
      "severity": "info",
      "title": "Maintenance",
      "body": "Maintenance starts at 02:00 UTC.",
      "action_status": "",
      "created_at": 1787731200000,
      "read_at": 0
    }
  ],
  "unread_count": 1,
  "next_cursor": ""
}
```

## Lire le nombre de messages non lus

- **Chemin** : `GET /api/messages/unread-count`
- **Réponse décodée** : `{"unread_count":3}`

## Marquer ou supprimer

### Un message

- **Marquer comme lu** : `POST /api/messages/:id/read`
- **Supprimer** : `DELETE /api/messages/:id`
- Le message d’un autre compte renvoie `404`. Une action encore en attente renvoie `409`.

### Tous les messages

- **Tout marquer comme lu** : `POST /api/messages/read-all`
- La réponse indique le nombre de lignes modifiées.

## Envoyer une annonce administrateur

- **Chercher les destinataires** : `GET /api/messages/admin/users?q=alice` renvoie au plus huit noms.
- **Envoyer** : `POST /api/messages/admin`
- Utilisez `all: true` pour tous les comptes ou fournissez des `recipients` exacts. Le serveur borne le titre, le corps,
  la sévérité et le nombre de destinataires.

```json
{
  "recipients": ["alice", "bob"],
  "all": false,
  "severity": "warning",
  "title": "Scheduled maintenance",
  "body": "The service will restart at 02:00 UTC."
}
```

Les invitations et résultats système sont créés par leur service. Une exclusion d’équipe indique le dépôt et le paquet
ou domaine Maven, mais ne révèle volontairement pas le membre ayant effectué l’action.
Les tâches d’examen envoient des messages `review_pending` dédupliqués aux examinateurs autorisés. La première décision
supprime toutes les autres copies et envoie au demandeur un seul `review_result` localisé, sans identité de
l’examinateur.

## Notifications destinées à une session

Renseignez `session_id` avec l’identifiant public d’une session de navigateur, un seul destinataire et `all: false`. Un identifiant vide conserve l’accès pour toutes les identités de connexion du compte. Le serveur vérifie à nouveau l’activité et le propriétaire de la session ; une cible révoquée ou expirée renvoie `409`.

`GET /api/messages/admin/sessions?username=alice&cursor=...` renvoie au plus 100 sessions actives par page. Utilisez `next_cursor` pour les suivantes. La réponse contient des identifiants publics, des informations sur l’appareil et des dates, jamais de secrets de session. Les droits de gestionnaire sont requis ; un jeton API nécessite aussi `admin:notifications`.

Les listes, compteurs non lus et opérations individuelles ou groupées de lecture/suppression respectent la session authentifiée. Les autres sessions et jetons API ne peuvent ni lire ni modifier une notification ciblée, même avec son identifiant ou le cookie cible joint à une requête API. Ces notifications ne sont pas envoyées par courriel. Pour un destinataire unique, le formulaire affiche un sélecteur personnalisé animé, avec toutes les sessions par défaut.

```json
{"recipients":["alice"],"all":false,"session_id":"00000000-0000-4000-8000-000000000001","severity":"info","title":"Session notice","body":"Only this browser session can read this notification."}
```
