---
title: Mode démonstration
order: 5
category: Pour commencer
description: Explorer les données prédéfinies en lecture seule
---

# Mode démonstration

Le démarrage avec `--demo` crée un jeu de données isolé à la première exécution, puis le sert en lecture seule. Le
formulaire préremplit `admin` / `12345678`. Les exemples couvrent comptes, sessions, équipes, paquets
Maven/Cargo/npm/Docker, métadonnées de fichiers, revues, tickets, quotas, notifications, statistiques, journaux et
historique des e-mails.

```bash
./renop --demo
./renop --demo --demo-temp
```

Utilisez `--demo --demo-temp` pour enregistrer les paramètres système et les définitions des dépôts. Les modifications
persistent : redémarrez avec `--demo` seul pour les présenter en lecture seule. `--demo-temp` seul est refusé. Les
autres modifications et écritures de journaux restent interdites dans les deux modes.

Les fichiers sont `renop-demo.db` et `renop-demo-settings.db`, remplaçables par `RENOP_DEMO_DATABASE` et
`RENOP_DEMO_SETTINGS_DB`. Les données normales sont séparées. Un jeu existant est conservé, même si la liste des dépôts
est vide. Seules les bases sont créées : aucun fichier de paquet, index ou journal ; téléchargements et aperçus sont
indisponibles. Les sessions réelles restent en mémoire bornée, et les sessions prédéfinies ne permettent aucune
connexion.

`GET /api/demo` renvoie le protobuf binaire `DemoInfo` : `enabled`, `temporary` et identifiants publics de
démonstration. Le mode normal ne renvoie aucun identifiant. Les mutations refusées utilisent `403` avec
`demo_read_only`, les fichiers indisponibles `demo_file_unavailable`. Les tâches de courrier, miroir, maintenance et
mise à jour ne démarrent pas.
