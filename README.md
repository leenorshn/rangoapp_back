# EasyStock

Backend Go pour la gestion commerciale : PostgreSQL, authentification par téléphone, API GraphQL et commandes gRPC.

## Lancer

```bash
cp env.example .env
go build -o easystock ./cmd/easystock
```

Le binaire a cinq commandes :

- `easystock migrate` applique les migrations SQL (`DATABASE_URL`, rôle migrator).
- `easystock grpc` sert le commerce et les abonnements (`DATABASE_URL_APP`).
- `easystock api` sert `GET /health` et `POST /graphql`. Avec `GRAPHQL_PLAYGROUND=true`, `GET /playground` affiche GraphQL Playground.
- `easystock worker` expire les abonnements et envoie les rappels (`DATABASE_URL_JOB`), toutes les 5 minutes.
- `easystock import [--dry-run] bundle.json` charge un export JSON dans PostgreSQL.

`docker compose up` démarre Postgres, Redis, les migrations, les rôles, l’API, gRPC et le worker.

L’authentification, les employés, les abonnements et la migration depuis Clerk sont décrits dans [docs/AUTH_ET_ABONNEMENTS.md](docs/AUTH_ET_ABONNEMENTS.md).

## Déploiement

Le déploiement DigitalOcean est décrit dans [DIGITALOCEAN_DEPLOYMENT.md](DIGITALOCEAN_DEPLOYMENT.md). Sur un droplet : `sudo sh scripts/setup-droplet.sh`, puis `sudo sh scripts/deploy.sh`.

## Tests

```bash
go test ./internal/easystock/... -count=1
```

Les tests d’intégration démarrent un PostgreSQL 16 embarqué. Aucune base externe n’est requise.
