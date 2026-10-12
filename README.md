# EasyStock

Backend Go pour la gestion commerciale : PostgreSQL, authentification Clerk, API GraphQL et commandes gRPC.

## Lancer

```bash
cp env.example .env
go build -o easystock ./cmd/easystock
```

Le binaire a quatre commandes :

- `easystock migrate` applique les migrations SQL (`DATABASE_URL`, rôle migrator).
- `easystock grpc` sert le commerce et les abonnements (`DATABASE_URL_APP`).
- `easystock api` sert `GET /health`, `POST /graphql` et `POST /webhooks/clerk`. Avec `GRAPHQL_PLAYGROUND=true`, `GET /playground` affiche GraphQL Playground.
- `easystock import [--dry-run] bundle.json` charge un export JSON dans PostgreSQL.

`docker compose up` démarre Postgres, les migrations, les rôles et les deux processus.

## Tests

```bash
go test ./internal/easystock/... -count=1
```

Les tests d’intégration démarrent un PostgreSQL 16 embarqué. Aucune base externe n’est requise.
