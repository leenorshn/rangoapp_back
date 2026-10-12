# Schéma PostgreSQL EasyStock

Source exécutable : `internal/easystock/migrate/sql/000001_schema.up.sql` et `000002_rls.up.sql`. Les montants sont `numeric(20,4)`, les quantités `numeric(20,6)`. Les clés sont des UUID. Les horodatages sont `timestamptz` en UTC. Aucune donnée métier n’utilise `float`.

## Rôles

| Rôle | Usage |
| --- | --- |
| propriétaire des tables (superuser / migrator) | migrations, import Mongo, contournement RLS |
| `easystock_app` | lectures et écritures métier, `NOBYPASSRLS` |
| `easystock_identity` | projection Clerk, webhooks, recherche de membre |
| `easystock_job` | transitions d’abonnement |

Les mots de passe ne sont pas dans les migrations. Ils sont posés après coup (`ALTER ROLE ... PASSWORD`) depuis l’environnement. L’application n’est pas propriétaire des tables et n’a aucun `DELETE`.

## Contexte locataire

Dans chaque transaction métier :

```sql
SELECT set_config('app.company_id', $1, true);
```

Le troisième argument rend la valeur locale à la transaction. Les politiques `FORCE ROW LEVEL SECURITY` comparent `company_id` à `app_setting_uuid('app.company_id')`. `SET row_security = off` ne donne pas de lignes au rôle applicatif.

## Tables

Identité et accès : `companies`, `stores`, `users`, `organization_memberships`, `store_assignments`, `permission_overrides`, `invitations`.

Catalogue et stock : `products`, `product_units`, `product_prices`, `suppliers`, `purchases`, `purchase_items`, `stock_balances`, `stock_movements`, `inventory_sessions`, `inventory_counts`.

Ventes et dettes : `sale_drafts`, `sales`, `sale_items`, `payments`, `refunds`, `refund_items`, `debts`, `debt_payments`.

Caisse : `cash_registers`, `cash_sessions`, `cash_transactions`, `cash_session_counts`, `expenses`.

Abonnements : `subscription_plans`, `subscription_snapshots`, `local_subscription_payments`, `entitlement_grants`.

Technique : `webhook_events`, `projection_versions`, `audit_logs`, `idempotency_keys`, `legacy_id_map`, `media_objects`, `company_exchange_rates`.

`company_exchange_rates` garde un sens explicite (`from_currency` → `to_currency`, `numeric(20,8)`). L'inverse est calculé à la lecture et n'est pas stocké. `media_objects.status` vaut `pending` jusqu'à confirmation du dépôt, puis `ready`. `products.image_media_id` pointe vers une image de la même entreprise. `expenses.media_id` pointe vers un justificatif.

Les clés étrangères composées `(company_id, id)` empêchent une vente d’une entreprise de référencer le stock d’une autre. `legacy_id_map` n’a pas de droit pour `easystock_app`.

## Règles portées par la base

- Une session de caisse ouverte par caisse (`cash_sessions_one_open`).
- Une session close et ses comptages ne se mettent plus à jour.
- Un mouvement de caisse ne s’attache qu’à une session ouverte.
- `UPDATE` et `DELETE` d’une vente confirmée sont refusés.
- Le dernier propriétaire actif ne peut pas être retiré.
- `stock_balances.quantity >= 0`.
- `paid_amount + due_amount = net_amount` sur une vente.
- Offres semées : `essentiel` (5 USD, 1 boutique, 2 membres), `pro` (10 USD, 1 boutique, 5 membres, export), `business` (20 USD, 3 boutiques, 15 membres, multi-boutiques).

## sqlc

`sqlc.yaml` génère `internal/easystock/db/dbsql` depuis le SQL de migration. La requête utilisée est le curseur catalogue `ListCatalogChanges`. Le reste du commerce est du SQL écrit à la main dans les mêmes transactions.
