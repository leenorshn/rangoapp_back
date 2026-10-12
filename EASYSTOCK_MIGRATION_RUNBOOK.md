# Runbook d’import vers PostgreSQL

Le dépôt ne contient plus de serveur MongoDB ni de JWT local. L’import lit un fichier JSON. Aucune bascule de production n’a été exécutée. Docker Compose local a été démarré le 12 octobre 2026 ; ce n’est pas la bascule.

## Avant la fenêtre

1. Déployer PostgreSQL 16 avec le volume persistant de `docker-compose.yml`.
2. Lancer `easystock migrate` avec `DATABASE_URL` du rôle migrator.
3. Poser les mots de passe de `easystock_app`, `easystock_identity` et `easystock_job` sans les journaliser. Le service `roles` de Compose le fait en développement.
4. Configurer Clerk (émetteur, JWKS, secret webhook `whsec_`, parties autorisées) et `INTERNAL_CONTEXT_SECRET` (32 caractères minimum).
5. En production, présenter un certificat au processus gRPC et `GRPC_TLS_CA` au processus API. Ne pas publier le port gRPC. `GRPC_INSECURE_DEV` est réservé au réseau privé de développement.
6. Répéter un import à blanc.

## Import

Le binaire ne se connecte à aucune base source. Un export JSON au format `legacyimport.Bundle` est produit hors ligne, sans mots de passe.

```sh
easystock import --dry-run bundle.json
easystock import bundle.json
```

`--dry-run` valide dans une transaction puis l’annule. Les UUID sont des UUID v5 sur l’espace `6f0b9c3e-1a4d-4e7a-9c2b-8d5e6f7a8b9c`, donc reproductibles. `legacy_id_map` évite un second effet. Les totaux de ventes sont comparés par entreprise et par devise. Un écart termine le processus avec le code 2. Un stock négatif ou un paiement supérieur au net est une anomalie (identifiant legacy seulement) et n’est pas chargé. Les comptes ne sont pas fusionnés par e-mail.

## Fenêtre de bascule

1. Geler les écritures de la base source.
2. Export final, puis `easystock import` sans `--dry-run`.
3. Vérifier les totaux du rapport et un échantillon de stock par boutique et par devise.
4. Basculer le client vers `POST /graphql` pour les opérations déjà portées.
5. Conserver une copie en lecture de la base source le temps de la vérification.

## Retour arrière

Le retour vers l’ancienne base n’emporte pas les écritures faites dans PostgreSQL après la bascule. Il faut les lister (ventes, dettes, caisse, stock) et les réconcilier avant de rouvrir l’ancienne base. Ne pas supprimer le volume PostgreSQL pour revenir en arrière.

## Sauvegarde

`scripts/backup.sh` produit un dump custom `pg_dump`. `scripts/restore.sh` le recharge avec `pg_restore`. Les deux exigent `DATABASE_URL` du rôle migrator. Le 12 octobre 2026, `scripts/backup.sh` a écrit `/tmp/easystock-20261012.dump` depuis le Postgres local (16.15, format custom, 433 entrées). `scripts/restore.sh` n’a pas été lancé.
