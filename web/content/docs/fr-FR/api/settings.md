---
title: API des paramètres
order: 8
category: Référence API
description: Paramètres de service par domaine, dépôts et reconstruction d’index
---

# API des paramètres

Les routes exigent un compte administrateur ou un API Token avec `admin:settings` ou `admin:repositories`, selon
l’opération. Les réponses utilisent protobuf lorsque `proto/api/v1/api.proto` le prévoit.

## Découvrir les domaines de paramètres

- **Chemin** : `GET /api/settings/domains`
- **Réponse** : noms stables pris en charge, notamment `server`, `proxy`, `storage`, `updater` et `frontend`.

## Pages de paramètres dans le navigateur

Chaque domaine parmi les 13 annoncés dispose de sa propre page. Sur ordinateur, les catégories figurent à côté
du formulaire ; sur petit écran, un sélecteur les remplace. Précédent et suivant suivent le même ordre. Ouvrir une
page charge uniquement sa configuration, sans charger tous les paramètres du service.

Chaque page conserve son brouillon lors du changement de catégorie ou de langue. L’enregistrement met à jour
uniquement la page active et bloque temporairement la saisie et la navigation. Un échec conserve le brouillon ;
l’abandon recharge cette page après confirmation. La navigation signale les changements non enregistrés. Le
rechargement ou la fermeture du navigateur avertit des modifications, mais les brouillons restent seulement en
mémoire et sont effacés à la déconnexion ou au changement de compte. Les secrets déjà stockés restent masqués et
les secrets saisis sont retirés du brouillon après un enregistrement réussi.

GPG reste dans la configuration du service. Limites des équipes globales, quotas, inscription, cache, courriel, OAuth et
sécurité des domaines disposent de pages distinctes. Les paramètres juridiques et OAuth unifiés utilisent le protobuf
binaire ; les autres conservent leurs formats documentés. Libellés et indications restent associés aux contrôles, et la
navigation place le focus sur le titre.

La catégorie active, les marqueurs de brouillon et le texte secondaire utilisent les couleurs communes des thèmes clair
et sombre.

## Lire et modifier un domaine

- **Lire** : `GET /api/settings/domain/:name`
- **Modifier** : `PUT /api/settings/domain/:name`
- **Comportement** : le schéma dépend de `:name`. Les champs inconnus et valeurs invalides sont refusés. Les changements
  d’hôte, port, TLS, base de données ou certains paramètres d’exécution peuvent imposer un redémarrage.

**Connexion externe** : `GET /api/settings/oauth-providers` renvoie le protobuf binaire `OAuthSettings` avec GitHub
intégré, les clients sans secrets et les préréglages. `PUT /api/settings/oauth-providers` exige
`replace_providers: true` et la liste `providers`, et enregistre les deux groupes atomiquement dans 128 KiB. Jusqu’à 12
autres clients plus GitHub sont pris en charge. Omettre GitHub le conserve ; désactivez son entrée et utilisez
`clear_client_secret` pour effacer ses identifiants. Une liste vide supprime seulement les autres clients.
`revocation_secret` est en écriture seule, conservé si vide pour le même client et effacé par `clear_revocation_secret`.
Les endpoints compatibles `GET /api/settings/github-oauth` et `PUT /api/settings/github-oauth` conservent JSON et gèrent
la même configuration.

[OAuth](../security/oauth-login.md)

## Paramètres des dépôts

Préférez `/api/settings/repositories`. Les alias préfixés par Maven restent disponibles pour compatibilité.

Les changements sont validés en base avant de remplacer la configuration active. La suppression du dernier dépôt
reste effective après redémarrage ; le YAML hérité sert uniquement à la migration initiale.

### Lister les dépôts

- **Chemin** : `GET /api/settings/repositories`
- **Alias** : `GET /api/settings/maven/repositories`

### Créer, modifier, supprimer ou migrer

- **Créer ou modifier** : `PUT /api/settings/repositories/:name`
- **Supprimer** : `DELETE /api/settings/repositories/:name`
- **Migrer Maven/files** : `POST /api/settings/repositories/:name/migrate/:target`, avec `maven` ou `files`. Les objets
  ne sont pas déplacés ; le catalogue Maven est reconstruit lors du retour vers Maven.

## Reconstruire l’index de recherche

- **Chemin** : `POST /api/settings/index/rebuild`
- **Comportement** : soumet une reconstruction fusionnée en arrière-plan, sans lancer deux tâches concurrentes.

## Réservation des domaines de publication

`GET /api/settings/maven-domains` et `PUT /api/settings/maven-domains` utilisent JSON.
La découverte contient `maven_domains`. Valeur par défaut :

```json
{"release_value":2,"release_unit":"year"}
```

`release_value` est un entier de 1 à 100 ; `release_unit` accepte `month` ou `year`, selon le calendrier UTC.
La base de paramètres conserve ces champs sous `maven_domains`.
Une modification enregistrée concerne les nouveaux verrouillages de sécurité, sans modifier les dates existantes
ni le délai distinct de 31 jours d’une fermeture volontaire. Voir l’[état des domaines Maven](maven.md).

[Documents juridiques et cookies](../configuration/legal.md)

Les pages de paramètres, éditeurs de fournisseurs et champs associés utilisent des transitions annulables et respectent
la réduction des animations. Les listes vides partagent le style de notification. Les listes déroulantes fermées ne
conservent ni menu ni écouteurs du document ; ceux-ci sont créés à l’ouverture.

Les ressources intégrées sont transmises en flux depuis l’exécutable. RenoP conserve le type, la taille et l’ETag sans
garder une autre copie de chaque bundle et variante compressée dans le tas Go. La négociation et les requêtes
conditionnelles restent disponibles.

[Vérification de sécurité](../security/captcha.md)

`capacity_limit_bytes` fixe la limite d’octets installés par dépôt ; `0` signifie illimité. L’interface utilise les MiB.
Sur disque comme sur S3, le total comprend artefacts, sommes de contrôle, objets en attente de revue et cache des
miroirs, hors copies temporaires. Une réservation avant validation partage la limite entre envois concurrents. Un
dépassement retourne `507` avec `repository_capacity_exceeded` ; une réponse de miroir valide peut encore être transmise
sans cache. Abaisser la limite sous l’usage existant ne bloque pas les lectures. Après une modification externe du
stockage, reconstruisez l’index ou redémarrez pour recalculer. Un ancien client omettant le champ facultatif conserve la
limite.

Les documents juridiques sont édités dans Frontend et enregistrés atomiquement avec la marque ; omettre `legal` conserve
les documents. Les contrôles d’index sont dans Stockage. Les anciens endpoints restent disponibles. Serveur expose
IP/port, TLS, base et performance ; écoute et base nécessitent un redémarrage.
