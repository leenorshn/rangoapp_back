# Changements d’API pour Android

Le contrat client est `POST /graphql` sur le binaire `cmd/easystock`, décrit dans `internal/easystock/api/schema.graphqls`.

## Authentification

Les jetons Clerk ne sont plus acceptés. Le parcours, les cookies, les codes d’erreur et l’écran d’expiration sont décrits dans `docs/AUTH_ET_ABONNEMENTS.md`.

Le mobile envoie `client: "mobile"` à `login`, `completeOwnerSignup`, `refreshSession` et `resetPassword`, puis conserve `refreshToken` dans le stockage sécurisé du système. Le jeton d’accès va dans `Authorization: Bearer`. Il dure 15 minutes et ne contient pas le rôle. `refreshSession` doit être sérialisé : un ancien refresh token révoque toute la famille. Un `scope` égal à `password_change` n’autorise que `changePassword`, `logout` et `logoutAllSessions`.

## Vente

`confirmSale` remplace `createSale` pour le nouveau serveur.

- Montants et quantités : chaînes décimales (`"10.00"`, `"2"`), plus des `Float`.
- `idempotencyKey` obligatoire, liée à l’entreprise, au type de commande et au contenu. La même clé et le même contenu renvoient la même vente. La même clé avec un autre contenu est refusée.
- `expectedUnitPrice` est contrôlé. Un écart de prix ou de quantité ne confirme pas en silence.
- Le crédit exige un client. Le paiement peut être partiel et crée une dette.
- `deleteSale` n’existe pas. Un retour est un remboursement, pas une suppression.
- Un brouillon ne réserve pas le stock. La confirmation revérifie session, permission, abonnement, stock et prix.

Champs exposés aujourd’hui : `saleId`, `currency`, `netAmount`, `paidAmount`, `dueAmount`, `debtId`. `benefice` n’est pas exposé.

## Catalogue

Le contrat à intégrer est `EASYSTOCK_CATALOG_ANDROID.md`.

`catalogChanges(after, afterId, limit, storeId)` pagine toujours par `(updatedAt, id)`. Les champs déjà lus restent. Avec `storeId`, le curseur inclut aussi les prix de cette boutique : un changement de prix ne réveille pas les autres boutiques. Les produits archivés restent dans le flux avec `archived: true`.

## Inscription

Le propriétaire passe par `requestOwnerSignupOtp`, `verifyOwnerSignupOtp`, puis `completeOwnerSignup`. Cela crée l'utilisateur, l'entreprise, le rôle `owner` et l'essai déjà configuré sur l'offre Essentiel. `createCompany` ouvre ensuite la première boutique et sa caisse. L'entreprise de la session vient du serveur. Un `organization_id` envoyé par le client n'est pas une autorisation.

Une boutique supplémentaire passe par `createStore`. Cette mutation n'ouvre pas d'entreprise. `stores` reliste les boutiques de la session : `id`, `name`, `timezone` et `cashRegister` (`id`, `name`). Le propriétaire et l'administrateur d'organisation voient toutes les boutiques ouvertes. Un vendeur ou un gérant ne voit que les siennes. `me.companies[].storeIds` et `employees[].storeIds` portent les affectations. Après réinstallation, l'application relit `stores` et `me` au lieu de rappeler `createCompany`.

Les employés ne reçoivent pas d'invitation. Le propriétaire appelle `createEmployee` avec le rôle `manager` ou `seller` et un mot de passe temporaire. La première connexion a le scope `password_change`.

`inviteMember` reste disponible pour les anciens clients. L'offre n'interdit aucun rôle. Seule l'expiration de l'abonnement bloque l'action.

## Lecture

`sales`, `customers`, `debts`, `stock` et `cash` sont lus dans le processus API. Les montants restent des chaînes décimales. Le stock ne contient pas le coût. Un caissier ne voit que ses propres ventes. Les montants d'une caisse ouverte ne sont visibles que par celui qui l'a ouverte, ou par un rôle qui lit les rapports. Une boutique d'une autre entreprise est introuvable.

`reports` couvre les ventes nettes, les encaissements par moyen, les dettes ouvertes, les dépenses, le stock faible, les produits les plus vendus et les comptages de caisse clôturée. `grossMargin` est la marge brute. Elle est absente pour un rôle sans droit de marge. L'offre ne la retire pas. La période est `[from, to)`. Les dettes ouvertes ne sont pas limitées à cette période. CDF et USD ne sont pas additionnés.

## Pas encore exposés en GraphQL

`payDebt`, les remboursements et les achats existent dans le service Go. La dette et le remboursement sont aussi en gRPC interne. Ils ne sont pas encore des mutations du schéma GraphQL. L'ouverture, la lecture et la clôture de caisse sont décrites dans `EASYSTOCK_CASH_ANDROID.md`. `confirmSale` accepte `cashSessionId` pour un paiement `cash`.
