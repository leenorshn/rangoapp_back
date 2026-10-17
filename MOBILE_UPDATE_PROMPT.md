Tu es un ingénieur Android senior (Kotlin, Jetpack Compose, Apollo). Tu travailles dans le dépôt mobile EasyStock :

https://github.com/leenorshn/rangoapp.git

Dernier état connu : commit `965cefe` (16 octobre 2026, message `ok`).

MISSION
Aligner l’application sur le backend EasyStock actuel. Le contrat client est uniquement `POST /graphql`. La source de vérité est le schéma du backend, pas les anciens fichiers du mobile :

- `internal/easystock/api/schema.graphqls`
- `docs/AUTH_ET_ABONNEMENTS.md`
- `EASYSTOCK_ANDROID_API_CHANGES.md`
- `EASYSTOCK_CATALOG_ANDROID.md`
- `EASYSTOCK_FAST_SALE_ANDROID.md`
- `EASYSTOCK_CASH_ANDROID.md`
- `EASYSTOCK_STOCK_ANDROID.md`

Lis le code avant de modifier. Réutilise l’authentification, les sessions, les employés, l’abonnement et la coquille `ui/easystock` déjà branchés. Ne réécris pas ces parcours s’ils appellent déjà le bon contrat.

Le dossier `app/src/main/graphql/` (createSale, deleteSale, factures, providers, caisse transactions, register) est l’ancien API. Ne le reconnecte pas. Le client vivant est `app/src/main/graphql-identity/` et `CommerceRepository`.

Ne te limite pas à des recommandations : remplace le schéma Apollo, régénère les opérations, branche les écrans et retire les appels que le serveur ne connaît plus.

Règles transverses

- Langue de l’interface : français. Libellés : Vente, Argent reçu, Dette, Stock restant.
- Montants, quantités et facteurs : chaînes décimales (`"21500"`, `"1.5"`, `"24"`). Jamais de Float envoyé au serveur.
- Instants : `DateTime` RFC3339 UTC. `debtDueDate` et `dueOn` : date civile `AAAA-MM-JJ`, sans heure.
- CDF et USD ne sont jamais additionnés. Chaque montant garde sa devise.
- L’entreprise vient de la session. N’envoie pas `companyId` comme autorisation.
- Chaque appel métier envoie `Authorization: Bearer <accessToken>`.
- `client: "mobile"` sur `login`, `completeOwnerSignup`, `refreshSession` et `resetPassword`. `refreshToken` reste dans le stockage sécurisé. Le jeton d’accès dure 15 minutes et ne contient pas le rôle faisant autorité.
- `refreshSession` est sérialisé. Réutiliser un ancien refresh token révoque toute la famille. Pas de fenêtre de grâce. Un échec réseau ne supprime pas la session. `unauthenticated` la supprime.
- Fermer l’application ne déconnecte pas.
- Le code stable est `errors[].extensions.code`. Affiche le message français du serveur et l’action de correction. Ne confirme jamais une erreur en silence.
- Une clé d’idempotence UUID est créée avant l’appel et réutilisée telle quelle après un timeout, avec le même corps. Une nouvelle clé sert seulement à une autre action. `idempotency_mismatch` signifie que la clé a déjà servi pour un autre contenu.
- Le serveur recalcule prix, totaux et stock. Le total affiché n’est pas enregistré. L’appareil affiche les quantités et soldes renvoyés, il ne les recalcule pas comme source de vérité.
- Un brouillon local ne réserve rien. La confirmation revérifie session, permission, abonnement, stock et prix.
- `SUBSCRIPTION_EXPIRED` et `SUBSCRIPTION_REQUIRED` bloquent les écritures métier. Profil, sessions, mot de passe et, pour le propriétaire, le renouvellement restent ouverts.
- Masquer un module n’est pas une autorisation. Le serveur reste la source de vérité.

1. Remplacer le schéma client

Copie le schéma exécutable du backend dans `app/src/main/graphql-identity/schema.graphqls`, puis réécris les opérations pour qu’elles ne demandent que des champs présents.

Retire du client, et n’affiche pas un succès simulé, tout ce que le schéma actuel n’expose pas :

- `requestMediaUpload`, `confirmMediaUpload`, `productMedia`, `media`, `setProductImage`
- `createExpense`
- `exchangeRates`, `convertAmount`, `setExchangeRate`
- `payDebt`, remboursement, `deleteSale`, `createSale`
- logo d’entreprise (`company_logo`, `setCompanyLogo`)
- factures, fournisseurs et transactions de caisse de l’ancien schéma

`product.imageMediaId` existe, mais aucune query ne fournit d’URL. N’invente pas l’upload. Le logo préparé à l’inscription reste local : si l’envoi échoue avec `media_kind`, montre l’erreur, ne prétends pas que le logo est enregistré.

`reports.expenses` reste lisible. Il n’y a pas de mutation pour créer une dépense. Retire le formulaire qui appelle `createExpense`.

L’écran de taux ne doit plus appeler `setExchangeRate`. Retire-le ou indique qu’il n’est pas disponible. N’affiche pas une équivalence calculée sur l’appareil comme si le serveur l’avait fournie.

2. Ce qui est déjà aligné — ne pas le casser

Conserver tels quels, en vérifiant seulement qu’ils compilent contre le nouveau schéma :

- OTP propriétaire : `requestOwnerSignupOtp`, `verifyOwnerSignupOtp`, `completeOwnerSignup`
- `login`, `refreshSession`, `logout`, `logoutAllSessions`, `revokeSession`, `mySessions`, `me`
- `changePassword`, `requestPasswordResetOtp`, `verifyPasswordResetOtp`, `resetPassword`
- `createEmployee`, `updateEmployeeRole`, `deactivateEmployee`, `reactivateEmployee`, `employees`
- `subscription`, `myOrganizationSubscription`, `renewalInstructions`, `submitSubscriptionPayment`, `mySubscriptionPaymentRequests`, `updateReminderPreferences`
- `registerPushDevice`, `unregisterPushDevice`, `myNotifications`, `markNotificationRead`
- `registration`, `createCompany` pour la première boutique

Ajoute seulement les champs nouveaux de ces types :

- `me.companies[].storeIds`
- `employees[].storeIds`

Le propriétaire crée un employé `manager` ou `seller`, avec un mot de passe temporaire. La première connexion a le scope `password_change` : seuls le changement de mot de passe, la déconnexion et `me` sont proposés. L’offre n’interdit pas ces rôles. `PHONE_IN_USE` ne modifie pas le compte existant. `inviteMember` n’est pas le parcours normal.

Après réinstallation, relis `me` et `stores`. Ne rappelle pas `createCompany` si l’entreprise et la boutique existent.

3. Boutiques

```graphql
query Stores {
  stores {
    id
    name
    timezone
    cashRegister { id name }
  }
}

mutation CreateStore($input: CreateStoreInput!) {
  createStore(input: $input) { id name timezone cashRegister { id name } }
}
```

`createStore` ouvre une boutique supplémentaire et sa caisse. Elle n’ouvre pas une entreprise. Le propriétaire et l’administrateur voient toutes les boutiques ouvertes. Un vendeur ou un gérant ne voit que les siennes. Une boutique d’une autre entreprise est introuvable (`not_found` ou `store`).

Conserve `storeId` et `cashRegister.id` pour la vente et la caisse. Le fuseau affiché des échéances est celui de la boutique (`Africa/Lubumbashi` pour Beni et Butembo si c’est la valeur renvoyée).

4. Catalogue

Le propriétaire gère le catalogue. Le gérant aussi, sauf refus de `catalog.manage`. Le vendeur lit les produits et les prix de ses boutiques et ne peut pas les modifier. `cost` est `null` sans le droit de marge. L’offre ne le retire pas. Ce n’est pas une erreur et ce n’est pas zéro.

`createProduct` n’écrit aucun stock. L’unité de base est créée avec le facteur `"1"`. Ne la renvoie pas dans `units`. Un prix sans `unitPrice` est ignoré. USD et CDF sont indépendants.

Champs d’un conditionnement à lire et à envoyer : `name`, `factor`, `dimension` (`count`, `mass` ou `volume`, défaut `count`), `barcode`, `allowPurchase`, `allowSale`, `allowFractional`. S’ils sont absents à la création, l’achat et la vente sont autorisés, et le fractionnaire suit le produit.

Opérations : `createProduct`, `updateProduct`, `archiveProduct`, `createProductUnit`, `updateProductUnit`, `archiveProductUnit`, `upsertProductPrice`, `archiveProductPrice`, `setProductDensity`.

Lecture pour la vente :

```graphql
query Products($storeId: ID!, $search: String, $after: DateTime, $afterId: ID) {
  products(storeId: $storeId, search: $search, forSale: true, after: $after, afterId: $afterId, limit: 50) {
    id
    name
    sku
    barcode
    imageMediaId
    baseUnit
    availableQuantity
    availability
    updatedAt
    units {
      id
      name
      factor
      dimension
      barcode
      allowSale
      allowFractional
      prices { currency unitPrice }
    }
  }
}
```

`search` porte sur le nom, le SKU et le code-barres. `forSale: true` retire les archivés et les produits sans conditionnement actif vendable. `availableQuantity` est le stock de la boutique dans l’unité de base. `availability` vaut `available` ou `out_of_stock` et sert à l’affichage : la confirmation relit le stock. Un scan exact passe par `productByBarcode(storeId, barcode)` et utilise `matchedUnitId`.

Synchronisation : `catalogChanges(storeId, after, afterId, limit)`. Le curseur est (`updatedAt`, `id`) du dernier élément, avec le même `storeId`. Demande maintenant les unités, les prix, le code-barres, la catégorie, le seuil, `archived`, la densité et `crossDimensionConversionEnabled`. Un produit `archived: true` sort de la liste vendable. Un conditionnement `active: false` ne se vend plus. Les prix absents ont été retirés. Avec `storeId`, un changement de prix ne réveille pas les autres boutiques.

`setProductDensity` est réservé à `catalog.manage`. L’appareil envoie la quantité du conditionnement. Il n’applique pas la formule kg/litre et n’affiche `baseQuantity` qu’après la réponse. Une densité changée ne réécrit pas le stock déjà en rayon.

`factor_locked` : le facteur a déjà servi. Crée un autre conditionnement, ne modifie pas l’ancien. `dimension_locked` et `density` s’affichent tels quels.

L’étape d’inscription `catalog` passe à `done` après un `createProduct` réussi. Ne la coche pas localement avant la réponse.

5. Vente

`confirmSale` est la seule écriture de vente. Ajoute au document actuel `cashSessionId`, `debtDueDate`, et dans la réponse `reference`, `customerId`, `customerName`, `debtDueDate`, `createdAt`, `itemCount`.

Une ligne envoie `productId`, le **nom** du conditionnement (`unitName`, pas son id), `quantity` de ce conditionnement, et `expectedUnitPrice` égal au `unitPrice` affiché pour cette devise. `"1.5"` n’est accepté que si `allowFractional` est vrai.

Paiements :

- `method: "cash"` exige `cashSessionId` d’une session ouverte dans cette boutique. Seule la part cash entre dans le tiroir.
- `method: "external"` est un encaissement déclaré (mobile money). Il ne demande pas de session et n’entre pas dans le tiroir.
- Somme égale au total : vente soldée, pas de dette.
- Somme inférieure : `customerId` et `debtDueDate` obligatoires. Le stock est déduit même si l’avance est nulle. Pour une avance nulle, envoie `payments: []` et omets `cashSessionId`.
- Somme supérieure : refus `payment`. Pas de monnaie rendue.
- Montant négatif ou nul sur une ligne : refus `payment`.

`debtDueDate` est le jour prévu dans le fuseau de la boutique. Le jour de la vente est accepté. Un jour précédent est refusé.

Succès : « Vente effectuée avec succès ». Si `dueAmount` est supérieur à zéro, ajoute « Dette enregistrée : … » avec la devise. `reference` est égal à `saleId`.

| Code | Action |
|---|---|
| `price_changed` | Relire le produit, montrer le nouveau prix, renvoyer avec une nouvelle clé |
| `stock_conflict` | Réduire ou retirer la ligne |
| `product_archived`, `unit_archived`, `unit_not_for_sale` | Retirer ou changer le conditionnement |
| `quantity` | Corriger la quantité |
| `not_found` | Recharger produit, prix, client ou caisse |
| `payment` | Ajuster les montants |
| `customer` | Choisir ou créer le client |
| `debt_due_date` | Choisir aujourd’hui ou plus tard |
| `cash_session` | Ouvrir la caisse, ou encaisser sans espèces |
| `credit_ceiling` | Réduire le crédit ou encaisser le reste |
| `credit.authorize` | Encaisser, la dette est refusée pour ce rôle |
| `sales.create` | Ce rôle ne vend pas |
| `SUBSCRIPTION_EXPIRED` | Bloquer la vente |

Le vendeur et le caissier peuvent vendre, ouvrir leur caisse, créer un client et enregistrer une dette, sauf refus explicite de `credit.authorize` ou plafond dépassé. Le magasinier ne vend pas. Un caissier ne voit que ses propres ventes. `openDebts` est `null` pour le vendeur : ne l’affiche pas comme zéro.

`sales(storeId, after, afterId, limit)` reste la liste. Le curseur est la date de création et l’id, du plus récent au plus ancien.

6. Clients

La vente peut créer un client sans quitter l’écran.

```graphql
mutation CreateCustomer($input: CreateCustomerInput!) {
  createCustomer(input: $input) {
    id
    name
    phone
    whatsappPhone
    address
  }
}
```

Nom, adresse et au moins un numéro sont obligatoires. Un numéro local `097…` est envoyé tel quel : le serveur le normalise en `+243`. Le WhatsApp est facultatif quand le téléphone principal sert déjà. `already_exists` : sélectionner le client existant de l’entreprise. `phone` : corriger le numéro.

Étends la query `customers` avec `whatsappPhone`, `address` et `openDebts { currency amount }`. La recherche actuelle par curseur reste. Ajoute `search` (nom, téléphone ou WhatsApp). `openDebts` null signifie que le rôle ne lit pas les dettes.

Il n’existe pas de mutation de modification ou de suppression de client. N’en invente pas.

7. Dettes, en lecture

Étends `debts` avec `advanceAmount` et `overdue`.

- `originalAmount` est le solde initial, pas le total du panier
- `advanceAmount` est le montant payé à la vente
- `collectedAmount` augmente seulement lors d’un remboursement ultérieur, qui n’est pas encore une mutation GraphQL
- `dueAmount` est le reste
- `dueOn` est l’échéance
- `status` vaut `open`, `partial` ou `settled`
- `overdue` est vrai si l’échéance est dépassée et qu’il reste un solde

`debts(storeId, status: "overdue")` filtre ces créances. Le statut stocké ne devient pas `overdue`.

Aucun bouton Payer. `payDebt` n’est pas dans le schéma. Aucun remboursement ni suppression de vente.

8. Caisse

Remplace la lecture limitée à `openSession { id openedAt balances }` par le contrat actuel.

```graphql
mutation OpenCash($input: OpenCashSessionInput!) {
  openCashSession(input: $input) {
    id
    storeId
    cashRegisterId
    openedBy
    openedAt
    status
    openingBalances { currency amount }
    theoreticalBalances { currency amount }
  }
}
```

`cashRegisterId` vient de `stores.cashRegister`. Un fonds initial nul est valide. Un montant négatif répond `opening`. La devise est `USD` ou `CDF`, en majuscules, une seule fois.

`cash(storeId)` liste les caisses et la session ouverte : `openingBalances`, `totalIn`, `totalOut`, `theoreticalBalances`, `balances`. `balances` et `theoreticalBalances` sont le même solde : fonds initial + entrées − sorties. `totalIn` ne contient pas le fonds initial.

`cashSession(id)`, `cashSessions(storeId, status, after, afterId, limit)` et `cashTransactions(sessionId, after, afterId, limit)` couvrent le détail et l’historique. Sessions de la plus récente à la plus ancienne, curseur (`openedAt`, `id`). Journal du plus ancien au plus récent, curseur (`createdAt`, `id`).

Si l’appelant n’est ni celui qui a ouvert, ni un rôle qui lit les rapports, les listes de montants et le journal sont `null`. Affiche l’identité de la session, pas des zéros.

Clôture :

```graphql
mutation CloseCash($input: CloseCashSessionInput!) {
  closeCashSession(input: $input) {
    id
    status
    closedBy
    closedAt
    theoreticalBalances { currency amount }
    countedBalances { currency amount }
    variances { currency amount }
    note
  }
}
```

L’écart est montant compté − solde théorique, par devise, sans conversion. Aucun mouvement correctif n’est créé. Chaque devise qui a un mouvement, y compris un fonds à zéro, doit être comptée.

Le vendeur et le caissier ouvrent et clôturent leur propre session. Ils ne clôturent pas celle d’un collègue (`cash_session`). Un propriétaire ou un gérant le peut.

| Code | Action |
|---|---|
| `cash_session_open` | Reprendre la session renvoyée par `cash` |
| `cash_session` | Ouvrir la caisse, ou utiliser la session de cet utilisateur |
| `cash_session_closed` | Montrer la session déjà clôturée |
| `opening` | Corriger les fonds initiaux |
| `count` | Compter chaque devise mouvementée, une seule fois |
| `not_found` | Caisse ou session invisible pour cette entreprise |

`confirmSale` reçoit `cashSessionId` dès qu’un paiement a `method: "cash"`. Le champ reste absent sans espèces.

9. Stock

Le stock est un seul solde par boutique, dans l’unité de base. Un carton de facteur 24 augmente ce solde de 24. Il n’y a pas de solde par conditionnement.

Le vendeur voit les quantités (`stock`, `stockSummary` sans coûts, `stockMovements`). Il ne réceptionne pas, n’ouvre pas le stock, n’enregistre pas une perte, ne transfère pas et ne valide pas un inventaire. Ces écritures demandent les permissions `stock.receive`, `stock.opening.create`, `stock.loss.create`, `stock.return.create`, `stock.transfer.*` et `stock.inventory.*`. Le propriétaire, l’administrateur, le gérant, le responsable et le magasinier les ont par défaut. Une entreprise expirée ne peut plus écrire. L’affectation à la boutique est exigée, sauf propriétaire et administrateur d’organisation.

`stock(storeId, after, afterId, limit)` reste valable. Le curseur suivant est `updatedAt` et `productId` ensemble. Une page déjà demandée ne remplace pas ses lignes par un solde plus récent : après une écriture, relis depuis le début pour afficher le solde courant.

Ajoute les écritures, chacune avec sa clé d’idempotence :

- `receiveStock` : augmente le solde, ne le remplace pas. `productUnitId` ou `unitName`. `supplierId` et `supplierName` facultatifs. Le coût moyen est recalculé dans la devise de la réception. Une autre devise laisse le coût précédent et marque le coût incomplet. Affiche `baseQuantity`.
- `openInitialStock` : une seule fois par produit et par boutique, seulement si aucun mouvement et solde nul. Pas un achat, pas une caisse. Après `opening_closed`, ne retente pas avec une nouvelle clé. `unitCost` et `currency` vont ensemble ; absents, la quantité est enregistrée sans coût inventé.
- `recordStockLoss` : motifs `damaged`, `expired`, `stolen`, `missing`, `other`. Quantité strictement positive. Au-delà du stock : `stock_conflict`, solde inchangé. La valeur sortie n’est pas dans la réponse. Ne l’invente pas.
- `returnStockToSupplier` : sort du stock. Pas de paiement fournisseur, pas de caisse. Motif obligatoire. Tout le document passe ou rien.
- Transfert : `createStockTransfer` (brouillon, aucun mouvement), `dispatchStockTransfer`, `receiveStockTransfer`, `cancelStockTransfer`, `stockTransfer(id)`. L’input d’action du schéma est `{ idempotencyKey, id }`, pas `transferId`. Conserve `id` et `version`. `inTransit` s’affiche à part du solde. La destination ne compte la quantité qu’après réception. Annuler un envoi déjà parti ramène la quantité sur l’origine (`returned_to_source`). Origine et destination sont deux boutiques de la même entreprise.
- Inventaire : `startStockInventory`, `recordInventoryCount`, `completeStockInventory`, `stockInventory(id)`. Compter ne change pas le stock. Une seule session ouverte par boutique (`inventory_open`). `variance = physique − théorique`. `varianceValue` absent n’est pas zéro : il manque le droit de valorisation ou le coût n’est pas fiable (`costIncomplete`). `inventory_stale` : recommencer le comptage des produits concernés, puis valider. Rien n’est écrit si la validation est refusée.

Journal `stockMovements`. `quantity` est signée, en unité de base. Filtres utiles : `receipt`, `sale`, `sale_return`, `inventory_adjustment`, `opening`, `loss`, `purchase_return`, `transfer_out`, `transfer_in`. Curseur (`createdAt`, `id`) vers les mouvements plus anciens. `from` et `to` sont inclusifs. Une période inversée répond `period`. Le journal ne contient pas le coût de sortie.

`stockSummary(storeId)` : compteurs, valeur au coût moyen et valeur au prix de l’unité de base, par devise. `stockCostValues` et `incompleteCostProducts` sont nuls sans `stock.valuation.read` et `margins.read`. L’offre ne les retire pas. Ne les affiche pas comme zéro. La valeur encore en transit reste dans le résumé de la boutique d’origine jusqu’à la réception.

Codes à traiter en plus de ceux de la vente : `opening_closed`, `reason`, `unit_not_for_purchase`, `factor_locked`, `density`, `dimension`, `dimension_locked`, `inventory_open`, `inventory_stale`, `inventory_completed`, `transfer_store`, `transfer_status`, `transfer_dispatched`, `transfer_received`, `transfer_cancelled`, `movement_type`, `period`, `idempotency_in_progress`.

10. Rapports

`reports(storeId, currency, from, to)` existe déjà. La période est `[from, to)`. Les dettes ouvertes ne sont pas limitées à cette période. `grossMargin` est la marge brute, pas le bénéfice net. Elle est absente pour un rôle sans droit de marge : masque le bloc, n’affiche pas zéro. L’offre ne la retire pas. `marginIncomplete` signale un coût incomplet. `cashCounts` reprend les écarts des sessions déjà clôturées. Pas d’export CSV. Une consolidation sans `storeId` reste permise tant que l’abonnement n’est pas expiré.

11. Navigation à brancher sur ces contrats

Accueil : bannière d’abonnement, raccourci Vendre, stock faible si `stock.read`, notifications.
Vendre : produits `forSale`, scan, panier, client, espèces avec session ou externe, dette avec échéance, reçu.
Activité : `sales` selon le rôle.
Stock : liste, réception, ouverture initiale, perte, retour fournisseur, transfert, inventaire, journal, résumé. Masque les écritures que le rôle n’a pas.
Clients : recherche, création, dettes en lecture.
Caisse : ouverture, solde théorique par devise, journal, clôture.
Plus : profil, sessions, boutiques, employés si propriétaire, abonnement si propriétaire, déconnexion.

Les écrans dessinés dans `ui/easystock` qui montrent encore des données fictives (accueil, caisse, stock, vente) doivent lire ces queries. Ne laisse pas un écran d’exemple à côté du même écran branché.

12. Vérification

- Le projet compile après régénération Apollo.
- Aucune opération générée ne référence `createSale`, `deleteSale`, `createExpense`, `requestMediaUpload`, `setExchangeRate`, `productMedia` ou `payDebt`.
- Une vente cash sans session ouverte est refusée avant l’appel, ou affiche `cash_session`.
- Une vente à crédit sans client ou sans `debtDueDate` n’est pas envoyée.
- Rejouer le même `confirmSale` après un timeout ne crée pas une seconde vente dans l’interface : la même clé et le même corps sont renvoyés.
- `price_changed` ne remplace pas le prix silencieusement.
- Un vendeur ne voit pas un coût nul à la place d’un coût masqué.
- `stock` après réception affiche le solde relu, pas un solde local additionné au panier.
