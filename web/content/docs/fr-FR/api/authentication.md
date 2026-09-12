---
title: API d’authentification
order: 2
category: Référence API
description: Sessions, profils, méthodes de connexion, récupération et révocation
---

# API d’authentification

Le navigateur utilise le cookie HttpOnly `renop_session`. Son secret n’est jamais renvoyé par les API de profil ou de
sessions et il est refusé dans les en-têtes et URL. Les paramètres de sécurité privés exigent une session navigateur,
jamais un mot de passe ou un API Token.

La page de connexion du navigateur est `/account/login`. Après une connexion par mot de passe, Passkey ou fournisseur
tiers, le navigateur revient au chemin local indiqué par le paramètre facultatif `return_to`, sans conserver la requête
ni le fragment. Les adresses externes, les points de terminaison d’authentification et les valeurs de plus de 1 024
caractères sont remplacés par `/`. Une session expirée ouvre la page de connexion ; un refus d’accès pour un utilisateur
connecté renvoie à l’accueil sans fermer sa session.

La [vérification en deux étapes](../security/two-step-verification.md) décrit la configuration de l’authentificateur,
les Passkeys secondaires, les réponses de connexion en attente et la récupération. Une Passkey secondaire ne compte pas
comme méthode principale. La récupération hors ligne retire l’authentificateur et désactive les Passkeys secondaires ;
la récupération par email conserve ces réglages.

## Connexion par mot de passe ou e-mail

- **Chemin** : `POST /api/auth/login`
- **Authentification** : aucune.
- **Corps** : protobuf `LoginRequest`; les noms JSON figurent ci-dessous. `name` accepte le nom du compte ou son e-mail
  privé.

### Requête

```json
{
  "name": "admin",
  "secret": "your_password"
}
```

### Résultat de session

La réussite définit `renop_session` avec `HttpOnly`, `SameSite=Lax` et `Secure` sous HTTPS. Le protobuf `SessionDetails`
contient les droits et routes du compte, mais laisse `session_token` vide.

## Connexion Passkey et GitHub

- **Début Passkey** : `POST /api/auth/fido/login/begin`
- **Fin Passkey** : `POST /api/auth/fido/login/finish`
- **Début GitHub** : `GET /api/auth/oauth/github/start` (`GET /api/auth/github/start`)
- **Callback GitHub** : `GET /api/auth/github/callback`
- **Disponibilité GitHub** : `GET /api/auth/github/status`

GitHub n’apparaît qu’après configuration OAuth. RenoP demande la lecture du compte et des organisations, conserve les
identifiants immuables et l’instantané des principals, mais jamais le jeton d’accès OAuth.

Les identités GitHub non associées doivent terminer l'[inscription](../security/registration.md) et définir un mot de
passe. OAuth demande `read:user read:org user:email` ; le rappel exige le cookie du navigateur ayant lancé
l'autorisation.

Microsoft, Google, GitLab, Cloudflare, Stack Exchange et les clients OAuth personnalisés
utilisent [l’API de connexion externe](../security/oauth-login.md). Ils prennent en charge les associations de comptes,
la vérification d’e-mail requise à l’inscription, l’importation facultative du profil et la même politique de second
facteur.

## Compte courant et profils publics

- **Session courante** : `GET /api/auth/me`
- **Profil privé** : `GET /api/auth/profile`
- **Modifier nom ou pseudonyme** : `PUT /api/auth/profile`
- **Modifier le mot de passe** : `PUT /api/auth/profile/password`
- **Déconnexion** : `POST /api/auth/logout`
- **Profil public** : `GET /api/users/:username/profile`
- **Appartenances** : `GET /api/users/:username/memberships?format=cargo|docker|maven|npm`

Lorsqu’un second facteur est activé, changer le mot de passe exige un nouveau code TOTP ou une assertion de Passkey secondaire ; un code envoyé à l’adresse principale actuelle est aussi possible si l’envoi est activé. `PUT /api/auth/profile/password` reçoit le binaire `UpdatePasswordRequest` avec `factor` (`totp`, `passkey`, `email`) et la preuve (`totp_code`, `challenge_id` avec `passkey_credential`, ou `email_code`). Commencez la preuve Passkey via `POST /api/auth/profile/password/passkey/begin`. La preuve dépend de la session et des identifiants actuels, n’est utilisable qu’une fois, et les autres sessions sont révoquées après succès.

Les routes visibles utilisent le nom du compte ; l’identifiant immuable reste interne. Les dépôts `HIDDEN` sont omis,
et les appartenances privées ne sont visibles que par un lecteur autorisé.

## Sécurité du compte

Ces routes exigent la session navigateur courante et renvoient `Cache-Control: no-store`.

### E-mail et politique du mot de passe

- **Lire l’état** : `GET /api/auth/profile/security`
- **Définir l’e-mail** : `PUT /api/auth/profile/email` ;
  voir [Vérification de l’e-mail de sécurité](../security/email-verification.md) pour la confirmation par message en
  file et via GitHub.
- **Activer ou désactiver le mot de passe** : `PUT /api/auth/profile/password-login`
- Le mot de passe ne peut être désactivé que si une Passkey principale ou un compte tiers reste lié. Son activation
  exige un mot de passe
  défini.

### Réinitialisation par code e-mail

La page indépendante `/account/forgot-password` vérifie l’adresse privée enregistrée sur le compte. Si le service e-mail
est désactivé, cette page et les API de demande et de confirmation renvoient `404` ; les pages de connexion et de
récupération masquent leurs liens. La récupération hors ligne `/account/recovery` reste disponible.

- **Disponibilité** : `GET /api/auth/password-reset/status` renvoie `{"enabled":true}` ou `{"enabled":false}`.
- **Envoi du code** : `POST /api/auth/password-reset/request` reçoit `{"email":"admin@example.com"}` et renvoie `202`
  avec `{id,status,ticket}` après l’insertion durable dans la file, avant l’envoi.
- **Livraison** : `GET /api/auth/mail/:id` reçoit le ticket privé dans `X-Renop-Mail-Ticket`. La page consulte l’état
  toutes les 3 secondes, pendant 10 minutes au maximum, puis s’arrête en quittant la page. Elle affiche les échecs et
  les résultats incertains. L’acceptation du fournisseur ne prouve pas la livraison au destinataire. Ne placez pas les
  tickets dans les URL ou le stockage du navigateur.
- **Réinitialisation** : `POST /api/auth/password-reset/confirm` reçoit le JSON suivant. Les deux POST exigent
  `Content-Type: application/json` et un corps de 4 096 octets au maximum ; le mot de passe doit contenir 6–72 octets
  UTF-8.

```json
{"email":"admin@example.com","code":"01234567","new_password":"new_secure_password"}
```

Le code à huit chiffres expire après 10 minutes et autorise cinq erreurs. Chaque adresse a un délai de 60 secondes entre
émissions, en plus de la limite IP configurée. Un nouveau code invalide le précédent seulement après une insertion
réussie ; un échec conserve l’ancien. L’insertion, le code et le compteur sont validés dans la même transaction. Le
stockage est limité à 2 048 preuves ; les preuves expirées sont supprimées avant insertion et périodiquement.

Toute adresse valide autorisée reçoit le même message de vérification de propriété, y compris les adresses non
inscrites : l’état de livraison ne révèle pas l’existence d’un compte. La réinitialisation revérifie l’identité,
l’adresse, le mot de passe et la version de sécurité enregistrés à l’émission. Une inscription ultérieure ne peut pas
utiliser un ancien code ; la fermeture du compte invalide ses codes.

Le succès consomme le code, change le mot de passe, active sa connexion et révoque les sessions navigateur de façon
atomique. Il renvoie `{status:"success",username}`, efface le cookie et ramène à la connexion ; les autres identifiants
restent liés et les suspensions restent effectives. `ACCOUNT_EMAIL_CODE_INVALID` couvre les preuves invalides, expirées,
épuisées, consommées ou devenues obsolètes. Les limites renvoient `429` ; les erreurs de file ou de service renvoient
`503`. Toutes les réponses utilisent `Cache-Control: no-store`.

### Codes de récupération

La page dédiée à la récupération du compte, `/account/recovery`, est accessible depuis le lien de récupération de la
page de connexion et fonctionne sans envoi de courriels. Après la récupération, le navigateur efface sa session
précédente et revient à la connexion avec le nom d’utilisateur renseigné, en conservant une destination locale
`return_to` valide. Les codes de récupération et les mots de passe sont effacés en quittant la page et ne sont jamais
conservés dans l’historique du navigateur.

- **Générer** : `POST /api/auth/profile/recovery-codes`
- **Réinitialiser** : `POST /api/auth/recovery/password`
- La génération montre une fois douze codes. Seuls des vérificateurs Argon2id sont stockés. Quatre codes distincts et
  inutilisés sont consommés atomiquement ; les sessions sont révoquées et la connexion par mot de passe réactivée.

```json
{
  "identifier": "admin@example.com",
  "codes": ["CODE-ONE", "CODE-TWO", "CODE-THREE", "CODE-FOUR"],
  "new_password": "new_secure_password"
}
```

La récupération hors ligne exige l’adresse principale actuelle, ou une ancienne adresse principale dans les 14 jours suivant son remplacement, et quatre codes inutilisés. Une récupération par l’ancienne adresse la restaure, supprime celle remplacée et ferme la fenêtre de récupération. Les noms d’utilisateur et les alias ordinaires sont refusés. Un changement normal d’adresse principale bloque pendant 14 jours le renouvellement des codes, l’ajout/suppression de Passkeys et les modifications du second facteur (`ACCOUNT_SECURITY_HOLD`). Les facteurs existants restent utilisables ; le premier jeu de codes peut être créé. Les anciennes adresses principales ne peuvent pas être supprimées pendant ce délai. La réponse privée expose `security_hold_until` et `previous_primary_emails` (`email`, `expires_at`).

## Gestion des méthodes de connexion

- **Lister les Passkeys** : `GET /api/auth/profile/fido`
- **Enregistrer** : `POST /api/auth/profile/fido/register/begin` puis
  `POST /api/auth/profile/fido/register/finish`
- **Supprimer** : `DELETE /api/auth/profile/fido/:device_id`
- **Lire l’identité GitHub** : `GET /api/auth/profile/github`
- **Déconnecter GitHub** : `DELETE /api/auth/profile/oauth/github` (`DELETE /api/auth/profile/github`)

La dernière méthode de connexion fonctionnelle ne peut être supprimée ni désactivée.

## Sessions navigateur

- **Lister** : `GET /api/auth/profile/sessions`
- **Révoquer une session** : `DELETE /api/auth/profile/sessions/:session_id`
- **Révoquer les autres** : `POST /api/auth/profile/sessions/revoke-others`

La liste expose un ID public, la méthode, les dates, l’IP et l’agent utilisateur, jamais le secret du cookie.

## Fermeture définitive du compte

`GET /api/auth/profile/retirement` renvoie les conditions de fermeture du compte courant. Cette route et
`DELETE /api/auth/profile/retirement` exigent une session de navigateur. La suppression accepte du JSON :

```json
{"confirmation":"alice"}
```

La confirmation doit correspondre exactement au nom d'utilisateur. La fermeture réussie renvoie `204 No Content`.
Une confirmation incorrecte renvoie `400` et `ACCOUNT_RETIREMENT_CONFIRMATION`. Des conditions non remplies
renvoient `409`, `ACCOUNT_RETIREMENT_BLOCKED` et le plan actualisé, avec les champs `eligible`,
`protected_role`, `super_team_owner_count`, `maven_domain_owner_count`, `package_owner_count` et
`pending_review_count`.

Les administrateurs système et les modérateurs de dépôt ne peuvent pas fermer leur compte. Aucun rôle T4 d'équipe
globale, domaine Maven actif détenu, paquet non déprécié détenu au niveau L4 ou demande de révision en attente ne doit
subsister. Transférez la propriété, fermez les domaines ou dépréciez définitivement les paquets avant de réessayer.

La fermeture réserve définitivement le compte et son nom, retire les adhésions, libère toutes les associations de
connexion externes et supprime
les Passkeys, sessions, API tokens, photo, codes de récupération et messages. La connexion renvoie
`ACCOUNT_DELETED`. L'adresse privée reste réservée 14 jours et l'activité est conservée 30 jours. Le nettoyage
planifié traite les échéances par lots bornés. Les paquets publiés restent téléchargeables.

L’identité externe libérée peut être immédiatement liée à un autre compte actif. Cela ne libère pas le nom réservé,
ne raccourcit pas la rétention de l’adresse et ne restaure pas la propriété des ressources retirées.

Les administrateurs consultent les échéances avec `GET /api/tokens/:name/retention`, libèrent l'adresse par
`DELETE /api/tokens/:name/retention/email` et effacent l'activité par
`DELETE /api/tokens/:name/retention/audit`. Les champs `deleted_at`, `email_release_at`,
`email_released_at`, `audit_purge_at` et `audit_purged_at` utilisent des millisecondes Unix.
La route administrateur `DELETE /api/tokens/:name` applique les mêmes conditions de fermeture définitive.

[Documents juridiques et cookies](../configuration/legal.md)

[Vérification de sécurité](../security/captcha.md)

## Liens du profil public

PUT /api/auth/profile/links accepte un objet JSON avec le site, Discord, les liens personnalisés et `visibility.github` / `visibility.gitlab`. Ces options sont désactivées par défaut. Les URL GitHub et `providers` sont en lecture seule : elles proviennent des identités associées actuelles, et GitLab doit correspondre à l’autorité configurée. Les préférences ne sont visibles que par leur propriétaire. Les URL GitHub des équipes restent modifiables.

```json
{"website":"https://example.com","visibility":{"github":true,"gitlab":false}}
```

## Déconnexion et révocation du fournisseur

`POST /api/auth/logout` invalide la session locale avant de contacter le fournisseur utilisé à la connexion. Sans autorisation externe, la réponse est `204` ; sinon, le protobuf binaire `LogoutResponse` contient `local_revoked`, `provider` et `provider_status` (`revoked`, `failed`, `unavailable` ou `unsupported`). Un échec externe ne restaure pas la session locale. `POST /api/auth/oauth/:provider/revoke` effectue la même opération après vérification du fournisseur de la session authentifiée par cookie. Les jetons API ne peuvent pas l’appeler.

`POST /api/auth/oauth/:provider/backchannel-logout` accepte le champ de formulaire standard `logout_token`. Le JWT OIDC signé doit contenir l’émetteur et l’audience configurés, `iat`, `exp`, `jti` et l’événement de déconnexion back-channel. Il identifie `sub`, `sid` ou les deux et ne contient pas `nonce`. Les événements de plus de dix minutes sont rejetés.

`POST /api/auth/oauth/:provider/revoked` accepte le protobuf binaire `ProviderRevocationRequest`. Configurez le secret en écriture seule `revocation_secret`, d’au moins 32 octets, puis transmettez `X-Renop-Signature-256: sha256=<HMAC-SHA256 hexadécimal des octets exacts de la requête>`. Le message contient `event_id`, `subject`, `session_id`, `issued_at` en millisecondes Unix et l’`issuer` vérifié pour OIDC. Les clients OAuth seuls laissent l’émetteur vide. Les autres formats Webhook nécessitent un adaptateur de confiance ; leurs corps natifs ne sont pas acceptés directement.

Les événements valides et répétés renvoient `200`, les déclarations invalides ou anciennes `400`, un HMAC incorrect `401`, et une indisponibilité d’admission ou de stockage `429`/`503`. Seules les sessions correspondantes antérieures à l’événement dans l’autorité configurée sont révoquées. Les liaisons d’identité et jetons API sont conservés. Un rejeu ou une preuve MFA retardée ne peut recréer une session révoquée.

## Suspensions de comptes

Les administrateurs peuvent suspendre ou rétablir un compte depuis sa page publique. `GET /api/tokens/:name/ban` lit l’état et la protection ; `PUT /api/tokens/:name/ban` accepte le JSON `reason_code`, `reason` personnalisé, `expires_at` facultatif en millisecondes Unix et `ban_ip` ; `DELETE /api/tokens/:name/ban` rétablit l’accès. Retirez d’abord les rôles protégés des administrateurs et modérateurs. Les codes ci-dessous correspondent à des raisons traduites. Un `reason_code` vide utilise du texte libre sans traduction.

`harassment_abuse`, `spam_misleading`, `automation`, `alternate_accounts`, `security_rules`, `harmful_content`, `terms_violation`, `impersonation`, `copyright`.
