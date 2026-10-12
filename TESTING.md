# Tests EasyStock

Les tests vivent dans `internal/easystock`. Ils utilisent un PostgreSQL 16 embarqué et le rôle `easystock_app`, sans superutilisateur.

```bash
go test ./internal/easystock/... -count=1
```

Couverture actuelle : vente atomique et idempotence, concurrence sur la dernière unité, remboursement, clôture de caisse, isolation entre entreprises, abonnement expiré, quotas, paiement local, jeton Clerk, GraphQL `confirmSale`, import JSON et curseur catalogue.
