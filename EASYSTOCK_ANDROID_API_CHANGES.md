# Changements d’API pour Android

Le contrat client est `POST /graphql` sur le binaire `cmd/easystock`, décrit dans `internal/easystock/api/schema.graphqls`.

## Authentification

`login` et `register` (téléphone + mot de passe) ne sont pas repris. L’application envoie un jeton de session Clerk (`Authorization: Bearer`), avec l’organisation active. Le serveur vérifie la signature JWKS, l’émetteur, l’expiration et la partie autorisée, puis charge le membre dans PostgreSQL. Un e-mail non vérifié ne fusionne pas deux comptes. Le jeton HMAC interne n’est pas accepté sur cette route.

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

`catalogChanges(after, afterId, limit)` pagine par `(updated_at, id)`. Les produits archivés restent dans le flux avec `archived: true` et `status: "archived"`. La page suivante reprend `updatedAt` et `id` du dernier élément.

## Inscription

Une session Clerk sans organisation peut appeler `registration` et `createCompany`. `createCompany` enregistre l'entreprise, le propriétaire, un essai de 14 jours et la première boutique avec sa caisse. Tant que l'entreprise n'est pas liée à Clerk, un second appel du même utilisateur renvoie la même entreprise. La devise et le nom peuvent encore changer tant qu'aucune boutique n'existe.

Tant que `registration.clerkLinked` est faux, les requêtes suivantes envoient `X-EasyStock-Company` avec l'identifiant renvoyé. Ce header est refusé pour une entreprise déjà liée, ou pour une entreprise dont l'utilisateur n'est pas membre. Dès que le jeton contient l'organisation Clerk, c'est elle qui sélectionne l'entreprise.

`registration.steps` vaut `company`, `store`, puis `catalog`.

`inviteMember` crée une invitation locale de 7 jours. Le jeton n'est renvoyé qu'à la création. L'offre Essentiel refuse les rôles opérationnels. `acceptInvitation` ouvre l'adhésion de l'utilisateur authentifié qui présente le jeton, puis l'affecte à la boutique. Le même utilisateur peut présenter le jeton à nouveau. Un autre utilisateur ne peut pas le réutiliser. L'API n'envoie pas l'e-mail.

## Lecture

`sales`, `customers`, `debts`, `stock` et `cash` sont lus dans le processus API. Les montants restent des chaînes décimales. Le stock ne contient pas le coût. Un caissier ne voit que ses propres ventes. Les montants d'une caisse ouverte ne sont visibles que par celui qui l'a ouverte, ou par un rôle qui lit les rapports. Une boutique d'une autre entreprise est introuvable.

`reports` couvre les ventes nettes, les encaissements par moyen, les dettes ouvertes, les dépenses, le stock faible, les produits les plus vendus et les comptages de caisse clôturée. `grossMargin` est la marge brute. Elle est absente pour l'offre Essentiel et pour un rôle sans droit de marge. La période est `[from, to)`. Les dettes ouvertes ne sont pas limitées à cette période. CDF et USD ne sont pas additionnés.

## Pas encore exposés en GraphQL

`payDebt`, les remboursements, l'ouverture et la clôture de caisse, et les achats existent dans le service Go. La dette et le remboursement sont aussi en gRPC interne. Ils ne sont pas encore des mutations du schéma GraphQL.
