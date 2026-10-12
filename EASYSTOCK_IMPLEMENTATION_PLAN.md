# Plan d’implémentation EasyStock

## Ordre

1. Audit et frontières (ce document, le schéma, l’architecture gRPC).
2. PostgreSQL : tables, contraintes, rôle applicatif, RLS, tests d’isolation.
3. Projection Clerk, rôles et permissions locales.
4. Noyau Commerce dans une seule transaction : catalogue, stock, vente, dette, retour, caisse.
5. gRPC `CommerceService` et `SubscriptionService`, appelés par l’API pour les commandes.
6. Abonnements, quotas, paiement local, rapports et exports.
7. Brouillons, synchronisation catalogue, idempotence de confirmation.
8. Import JSON, Docker Compose, sauvegarde, runbook. Pas de bascule de production dans cette mission.

## Décisions

- Un seul processus transactionnel pour ventes, stock, dettes, paiements et caisse. Pas de transaction distribuée.
- L’API GraphQL et le serveur gRPC sont le même binaire, lancés en sous-commandes `api` et `grpc`, sur un réseau Docker interne. Le port gRPC n’est pas publié.
- L’API GraphQL est servie par `cmd/easystock`. Les écarts pour Android sont listés dans `EASYSTOCK_ANDROID_API_CHANGES.md`.
- Clerk est la source des identités et des rôles d’organisation. PostgreSQL en garde la projection, les affectations de boutique et les plafonds.
- Clerk Billing n’est pas supposé encaisser Airtel Money ou M-Pesa. Le droit local est une ligne `entitlement_grants.source = local`, jamais une facture Clerk marquée payée.
- Redis n’est pas introduit : l’idempotence financière est une contrainte SQL.

## Hors de ce dépôt

Application Android, portail Next.js, création du compte Clerk, éligibilité Stripe, choix de l’opérateur de paiement local, gel des écritures de production.
