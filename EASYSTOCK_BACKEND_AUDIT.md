# Audit backend Rango → EasyStock

Date : 12 octobre 2026. Le serveur MongoDB et l’authentification JWT décrits ci-dessous ont été retirés du dépôt. L’entrée actuelle est `cmd/easystock`, sur PostgreSQL.

## Ce qui existait avant le retrait

Monolithe Go 1.24, module `rangoapp`, entrée `server.go`.

- API GraphQL gqlgen (`graph/schema.graphqls`, resolvers dans `graph/schema.resolvers.go`).
- MongoDB (`go.mongodb.org/mongo-driver`) comme seule base métier. Collections décrites dans `DATABASE_COLLECTIONS.md` : `users`, `companies`, `stores`, `products`, `products_in_stock`, `clients`, `providers`, `factures`, `sales`, `debts`, `debtPayments`, `provider_debts`, `stock_supplies`, `stock_movements`, `inventories`, `trans`, `subscriptions`, `subscription_plans`, taux de change.
- Authentification locale : téléphone + mot de passe bcrypt, JWT HS256 signé par `JWT_SECRET` (`utils/jwt.go`, `middlewares/auth.go`). Claims : user id, company id, rôle, boutiques.
- Rôles réels : `Admin` et `User`. L’admin voit toutes les boutiques de l’entreprise ; l’utilisateur est limité à `assignedStoreId`.
- Vente Mongo transactionnelle (`database/sale_db.go`) : contrôle client, stock, type `cash` / `debt` / `advance`, écriture vente, dette, mouvement de stock et transaction de caisse.
- Dettes clients et fournisseurs, inventaires, approvisionnements, caisse (entrées/sorties), rapports de ventes et de stock, abonnement local avec essai et cron d’expiration (`services/cron.go`).
- Santé HTTP, arrêt gracieux, CORS, Docker pour l’image HTTP actuelle.
- Le client Android (`rangoapp`) consomme ce schéma : `login`, `register`, `createSale`, `payDebt`, `supplyStock`, listes par `storeId`.

## Ce qui manque par rapport au PRD EasyStock

- PostgreSQL, migrations versionnées, sqlc, contraintes d’intégrité interentreprises.
- Clerk (identité, sessions, organisations, adhésions) et webhooks signés.
- Rôles `owner`, `organization_admin`, `store_manager`, `cashier`, `stockkeeper`, `accountant` et permissions fines.
- Isolation RLS et rôle SQL applicatif sans `BYPASSRLS`.
- Montants exacts (`NUMERIC`), idempotence SQL, sessions de caisse immuables.
- Conditionnements (facteur d’unité), prix par boutique, brouillons sans réservation.
- Retours en contre-opération (une vente confirmée est supprimable aujourd’hui via `deleteSale`).
- Abonnements Essentiel / Pro / Business, grâce, quotas, adaptateur de paiement local distinct de Clerk Billing.
- Rapports de marge brute séparés du bénéfice net, exports filtrés, aucune somme CDF+USD implicite.
- Synchronisation paginée et conflits explicites pour Android.
- Frontière gRPC interne. Le PRD V1 n’exige pas de microservices ; la mission demande une frontière gRPC autour du service transactionnel Commerce, pas un service par domaine.
- Stockage S3 des justificatifs, sauvegarde/restauration, import Mongo reproductible.

## Ce qui doit être corrigé

- Course sur le stock : `CreateSale` vérifie la quantité avant la transaction, puis fait `$inc` sans prédicat `stock >= quantité`. Deux ventes concurrentes de la dernière unité peuvent toutes les deux réussir.
- Les prix, quantités et marges sont des `float64`. `benefice` est exposé dans le type GraphQL `Sale`.
- Le JWT local fait confiance au `companyId` embarqué dans le jeton émis par ce même serveur. Il n’y a pas d’organisation Clerk ni de projection réconciliée.
- `deleteSale` et `deleteFacture` contredisent l’historique immuable.
- La caisse est un journal d’entrées/sorties, pas une session avec fonds initial, écart et clôture immuable. Les espèces ne sont pas séparées des autres moyens au niveau d’une session.
- L’abonnement est un document local. Aucune éligibilité Clerk Billing / Stripe n’est vérifiable dans ce dépôt, et rien ne couvre Airtel Money ou M-Pesa.
- Les index et les contrôles d’accès sont applicatifs uniquement. Deviner un identifiant d’une autre entreprise n’est pas arrêté par le moteur de données.
- Deux collections historiques (`stock`, `mouvements_stock`) coexistent avec `stock_movements`.

## À conserver

Le vocabulaire métier déjà compris par Android : entreprise, boutique, produit, client, fournisseur, vente, dette, approvisionnement, inventaire, caisse. Les règles utiles déjà codées : client obligatoire pour le crédit, paiement partiel, stock rattaché à une boutique, essai puis expiration, pas de commission sur les ventes.

## Décision d’architecture

Le binaire `cmd/easystock` parle PostgreSQL. Les commandes d’écriture passent par gRPC vers le processus Commerce ; les lectures autorisées utilisent le même rôle SQL et les mêmes règles. Le client MongoDB et le JWT local ont été retirés du dépôt.
