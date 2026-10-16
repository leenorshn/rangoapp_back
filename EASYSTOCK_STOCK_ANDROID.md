# EasyStock — module Stock pour Android

Contrat GraphQL de `POST /graphql`. Les montants et les quantités sont des chaînes décimales. Les dates sont des `DateTime` RFC3339. Le `companyId` n’est jamais envoyé : il vient du jeton.

Les erreurs métier ont un code stable dans `errors[].extensions.code`.

Le stock d’un produit est **un seul solde par boutique, dans l’unité de base**. Un carton de 24 bouteilles augmente ce solde de 24. Il n’existe pas de solde séparé par conditionnement.

## Permissions

| Code | Qui l’a par défaut | Usage |
|---|---|---|
| `stock.read` | propriétaire, administrateur, gérant, responsable, vendeur, caissier, magasinier, comptable | quantités, inventaire en lecture |
| `stock.receive` | propriétaire, administrateur, gérant, responsable, magasinier | réception |
| `stock.opening.create` | idem | ouverture du stock déjà présent |
| `stock.loss.create` | idem | perte, casse, vol, péremption |
| `stock.return.create` | idem | retour fournisseur |
| `stock.transfer.create` | idem | créer un transfert en brouillon |
| `stock.transfer.dispatch` | idem | expédier depuis la boutique d'origine |
| `stock.transfer.receive` | idem | réceptionner dans la boutique de destination |
| `stock.transfer.cancel` | idem | annuler depuis la boutique d'origine |
| `stock.inventory.start` | propriétaire, administrateur, gérant, responsable, magasinier | démarrer un inventaire |
| `stock.inventory.count` | idem | compter |
| `stock.inventory.validate` | idem | valider les écarts |
| `stock.adjust` | idem | ajustement historique en une étape, non exposé à Android |
| `stock.movements.read` | tous les rôles ci-dessus | journal |
| `stock.valuation.read` | propriétaire, administrateur, comptable, et seulement si l’offre autorise les marges (Pro ou Business) | coûts d’achat |

Le vendeur voit les quantités. Il ne valide pas un inventaire. Un droit peut être retiré par une exception `permission_overrides`.

L’affectation à la boutique est exigée, sauf pour le propriétaire et l’administrateur d’organisation. Une entreprise expirée ne peut plus écrire.

## 1. Créer un produit

Mutation existante `createProduct`. La création **n’écrit aucun mouvement de stock**. Le stock déjà présent dans la boutique passe par `openInitialStock`. Une livraison fournisseur passe par `receiveStock`.

Champs ajoutés sur chaque conditionnement : `allowPurchase`, `allowSale`, `allowFractional`. S’ils sont absents, l’achat et la vente sont autorisés, et le fractionnaire est celui du produit.

L’unité de base est créée automatiquement avec le facteur `1`. Ne la renvoyez pas dans `units`.

```graphql
mutation ($input: ProductInput!) {
  createProduct(input: $input) {
    id
    sku
    name
    baseUnit
    allowFractional
    alertThreshold
    units { id name factor allowPurchase allowSale allowFractional prices { currency unitPrice } }
  }
}
```

```json
{
  "input": {
    "name": "Eau minérale",
    "categoryName": "Boissons",
    "baseUnit": "bouteille",
    "barcode": "6001234567890",
    "alertThreshold": "24",
    "storeId": "STORE_ID",
    "units": [
      { "name": "demi-carton", "factor": "12", "allowPurchase": false, "allowSale": true },
      { "name": "carton", "factor": "24", "allowPurchase": true, "allowSale": true }
    ],
    "prices": [
      { "storeId": "STORE_ID", "unitName": "bouteille", "currency": "USD", "unitPrice": "0.25" },
      { "storeId": "STORE_ID", "unitName": "bouteille", "currency": "CDF", "unitPrice": "650" },
      { "storeId": "STORE_ID", "unitName": "carton", "currency": "USD", "unitPrice": "5.40" },
      { "storeId": "STORE_ID", "unitName": "carton", "currency": "CDF", "unitPrice": "14000" }
    ]
  }
}
```

Les noms d’unités sont libres : pièce, carton, demi-carton, kilogramme, gramme, litre, millilitre, mètre, centimètre, sac, paquet, bouteille, ou un nom propre à la boutique. Le facteur appartient au produit. Rien ne suppose qu’un carton contient 24 pièces.

`upsertProductPrice`, `createProductUnit` et `updateProductUnit` restent les opérations de mise à jour. Un facteur déjà utilisé dans une vente ou une réception répond `factor_locked` : créez un autre conditionnement. Les ventes déjà enregistrées gardent leur quantité.

Quand `allowFractional` change sur le produit, l’unité de base suit. Les autres conditionnements gardent leur propre drapeau.

Les prix modifiés ressortent dans `catalogChanges`.

Codes : `product`, `factor`, `barcode`, `already_exists`, `product_archived`, `unit_archived`, `catalog.manage`, `store`.

Permission : `catalog.manage`.

Scénario : l’écran article envoie `createProduct`, puis affiche les conditionnements renvoyés. Le stock affiché est 0 jusqu’à une ouverture initiale ou une réception.

## 2. Recevoir du stock

```graphql
mutation ($input: ReceiveStockInput!) {
  receiveStock(input: $input) {
    id
    storeId
    supplierId
    currency
    totalCost
    reference
    note
    lines { productId productUnitId unitName quantity baseQuantity unitCost lineCost }
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "receipt-eau-001",
    "storeId": "STORE_ID",
    "supplierName": "Dépôt central",
    "currency": "USD",
    "reference": "BL-104",
    "note": "Livraison du matin",
    "lines": [
      {
        "productId": "PRODUCT_ID",
        "productUnitId": "CARTON_UNIT_ID",
        "quantity": "5",
        "unitCost": "4"
      }
    ]
  }
}
```

`supplierId` et `supplierName` sont facultatifs. `productUnitId` ou `unitName` suffit. Cinq cartons de facteur 24 donnent `baseQuantity` `120` et `totalCost` `20`. Le solde est augmenté, jamais remplacé. Le coût moyen pondéré est recalculé dans la devise de la réception. Une réception dans une autre devise laisse le coût précédent et marque ce produit `cost_complete = false`.

La même clé et le même contenu renvoient la même réception. La même clé avec un autre contenu répond `idempotency_mismatch`.

Codes : `receive`, `unit_not_for_purchase`, `unit_archived`, `product_archived`, `quantity`, `not_found`, `idempotency_key`, `idempotency_mismatch`, `idempotency_in_progress`, `stock.receive`, `store`.

Permission : `stock.receive`.

Scénario : l’écran réception envoie la mutation. En cas de réseau douteux, il renvoie **la même** `idempotencyKey` et le même corps. Il affiche ensuite `baseQuantity` comme quantité entrée en unité de base.

## 2a. Ouvrir le stock initial

L’ouverture n’est pas une réception. Elle n’écrit ni achat, ni fournisseur, ni mouvement de caisse. Elle n’est acceptée que si le produit n’a encore aucun mouvement dans la boutique et que son solde est nul. Un stock revenu à zéro après une vente ne peut pas être rouvert.

```graphql
mutation ($input: OpenInitialStockInput!) {
  openInitialStock(input: $input) {
    id
    storeId
    note
    createdBy
    createdAt
    lines { productId productUnitId unitName quantity factor baseQuantity unitCost currency lineCost }
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "ouverture-001",
    "storeId": "STORE_ID",
    "note": "Stock déjà en rayon",
    "lines": [
      {
        "productId": "EAU_ID",
        "productUnitId": "CARTON_UNIT_ID",
        "quantity": "10",
        "unitCost": "4",
        "currency": "USD"
      },
      {
        "productId": "SUCRE_ID",
        "productUnitId": "SAC_UNIT_ID",
        "quantity": "2",
        "unitCost": "20",
        "currency": "USD"
      }
    ]
  }
}
```

Dix cartons de facteur 24 donnent `baseQuantity` `240` et un mouvement `opening`. Deux sacs de 25 kg donnent `50`. `unitCost` et `currency` vont ensemble. S’ils sont absents, la quantité est enregistrée et le coût de référence n’est pas inventé. Le coût, lorsqu’il est envoyé, met à jour le coût moyen dans cette devise. USD et CDF ne sont pas additionnés.

La même clé et le même contenu renvoient la même ouverture. Un autre contenu répond `idempotency_mismatch`. Une deuxième ouverture, ou une ouverture après une réception ou une vente, répond `opening_closed`.

Codes : `opening`, `opening_closed`, `quantity`, `product_archived`, `unit_archived`, `not_found`, `idempotency_key`, `idempotency_mismatch`, `idempotency_in_progress`, `stock.opening.create`, `store`.

Permission : `stock.opening.create`.

## 2b. Enregistrer une perte

```graphql
mutation ($input: StockLossInput!) {
  recordStockLoss(input: $input) {
    id
    productId
    storeId
    type
    quantity
    originUnit
    originQuantity
    reason
    actorId
    createdAt
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "perte-eau-001",
    "storeId": "STORE_ID",
    "productId": "EAU_ID",
    "productUnitId": "BOUTEILLE_UNIT_ID",
    "quantity": "5",
    "reason": "damaged",
    "note": "Casse à la livraison interne"
  }
}
```

Motifs acceptés : `damaged`, `expired`, `stolen`, `missing`, `other`. Cinq bouteilles sur un solde de 120 écrivent un mouvement `loss` de `-5` et laissent 115. La quantité est strictement positive. Une perte supérieure au stock répond `stock_conflict` et ne laisse pas de solde négatif. Aucun mouvement de caisse n’est créé. La valeur sortie est le coût moyen du moment ; elle n’est pas renvoyée par cette mutation.

Permission : `stock.loss.create`.

Codes : `loss`, `reason`, `quantity`, `stock_conflict`, `product_archived`, `unit_archived`, `not_found`, `idempotency_key`, `idempotency_mismatch`, `idempotency_in_progress`, `stock.loss.create`, `store`.

## 2c. Retourner du stock au fournisseur

`returnStockToSupplier` sort du stock. Aucun paiement fournisseur et aucune écriture de caisse ne sont créés.

```graphql
mutation ($input: SupplierReturnInput!) {
  returnStockToSupplier(input: $input) {
    id
    storeId
    supplierId
    reference
    reason
    createdAt
    lines { productId productUnitId unitName quantity factor baseQuantity }
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "retour-fournisseur-001",
    "storeId": "STORE_ID",
    "supplierName": "Brasserie",
    "reference": "AV-14",
    "reason": "Produits abîmés à la livraison",
    "lines": [
      { "productId": "EAU_ID", "productUnitId": "CARTON_UNIT_ID", "quantity": "2" }
    ]
  }
}
```

`supplierId` ou `supplierName` est facultatif. Si `supplierId` est envoyé, le fournisseur doit appartenir à la boutique. Le motif est obligatoire, de 1 à 500 caractères. La quantité est strictement positive et convertie en unité de base. Deux cartons de facteur 24 retirent 48 unités de base et écrivent un mouvement `purchase_return` négatif, avec l'unité d'origine. Toutes les lignes passent ensemble : si une ligne dépasse le stock, rien n'est écrit. La valeur sortie est le coût moyen du moment ; elle n'est pas renvoyée.

Permission : `stock.return.create`.

Codes : `return`, `reason`, `reference`, `quantity`, `stock_conflict`, `product_archived`, `unit_archived`, `not_found`, `idempotency_key`, `idempotency_mismatch`, `idempotency_in_progress`, `stock.return.create`, `store`.

## 2d. Transférer entre deux boutiques

Un transfert est un seul document. Il ne crée jamais une sortie et une entrée indépendantes. La quantité en transit est celle du document, pas un troisième solde de stock.

```graphql
mutation ($input: StockTransferInput!) {
  createStockTransfer(input: $input) {
    id
    status
    version
    inTransit
    lines { productId unitName quantity factor baseQuantity }
    events { fromStatus toStatus actorId createdAt }
  }
}

mutation ($input: StockTransferActionInput!) {
  dispatchStockTransfer(input: $input) { id status version inTransit }
}

mutation ($input: StockTransferActionInput!) {
  receiveStockTransfer(input: $input) { id status inTransit }
}

mutation ($input: StockTransferActionInput!) {
  cancelStockTransfer(input: $input) { id status inTransit }
}

query ($id: ID!) {
  stockTransfer(id: $id) { id status version inTransit sourceStoreId destinationStoreId }
}
```

Statuts : `draft`, `in_transit`, `received`, `cancelled`.

- `createStockTransfer` crée un brouillon. Aucun mouvement, aucune réservation. Le stock n'est pas vérifié.
- `dispatchStockTransfer` écrit `transfer_out` sur la boutique d'origine et passe le document à `in_transit`. `inTransit` devient la somme des quantités de base. Une deuxième expédition répond `transfer_dispatched`.
- `receiveStockTransfer` écrit `transfer_in` sur la boutique de destination. `inTransit` revient à `0`. Une deuxième réception répond `transfer_received`. Réceptionner un brouillon répond `transfer_status`.
- `cancelStockTransfer` sur un brouillon n'écrit aucun mouvement. Sur un envoi déjà parti, le `transfer_out` reste dans le journal et un `transfer_in` compensatoire revient sur la boutique d'origine, avec le motif `returned_to_source`. Un transfert déjà reçu répond `transfer_received`. Un transfert déjà annulé répond `transfer_cancelled`.

La boutique d'origine et la destination appartiennent à la même entreprise et doivent être différentes (`transfer_store`). L'expédition et l'annulation s'autorisent sur l'origine. La réception s'autorise sur la destination : un magasinier affecté seulement à l'origine reçoit `store`. Ce n'est pas une vente : aucun chiffre d'affaires et aucun mouvement de caisse.

Chaque changement de statut incrémente `version` et ajoute un événement. La même clé d'idempotence rejoue le résultat. Une nouvelle clé sur une action déjà faite est refusée.

Permissions : `stock.transfer.create`, `stock.transfer.dispatch`, `stock.transfer.receive`, `stock.transfer.cancel`.

Codes : `transfer`, `transfer_store`, `transfer_status`, `transfer_dispatched`, `transfer_received`, `transfer_cancelled`, `quantity`, `stock_conflict`, `not_found`, `idempotency_key`, `idempotency_mismatch`, `idempotency_in_progress`, `stock.transfer.create`, `stock.transfer.dispatch`, `stock.transfer.receive`, `stock.transfer.cancel`, `store`.

## 3. Vendre

`confirmSale` ne change pas. `expectedUnitPrice` reste le prix serveur. Une unité avec `allowSale: false` répond `unit_not_for_sale`. Une quantité fractionnaire n’est acceptée que si le conditionnement l’autorise (`quantity` sinon).

Deux cartons vendus retirent `2 × facteur` unités de base. Trois bouteilles en retirent 3. Le stock ne devient pas négatif : `stock_conflict`.

## 4. Inventaire physique

Compter ne change pas le stock. La validation seule écrit les ajustements.

### Démarrer

```graphql
mutation ($input: StartStockInventoryInput!) {
  startStockInventory(input: $input) {
    id
    companyId
    storeId
    createdBy
    status
    note
    startedAt
    completedAt
    lines { id }
  }
}
```

```json
{ "input": { "storeId": "STORE_ID", "note": "Inventaire du matin" } }
```

Statut renvoyé : `counting`. Une seule session `draft` ou `counting` est ouverte par boutique (`inventory_open`).

Permission : `stock.inventory.start`.

### Compter

Plusieurs conditionnements du même produit sont autorisés. Trois cartons de 24 et cinq bouteilles donnent un total de base de 77.

```graphql
mutation ($input: RecordInventoryCountInput!) {
  recordInventoryCount(input: $input) {
    id
    productId
    productName
    productUnitId
    unitName
    factor
    countedQuantity
    baseQuantity
    theoreticalQuantity
    countedBaseTotal
    variance
    varianceValue
    varianceCurrency
    costIncomplete
    reason
  }
}
```

```json
{
  "input": {
    "inventoryId": "INVENTORY_ID",
    "productId": "PRODUCT_ID",
    "productUnitId": "CARTON_UNIT_ID",
    "countedQuantity": "3",
    "reason": "casse"
  }
}
```

`theoreticalQuantity`, `countedBaseTotal` et `variance` sont le regroupement du produit en unité de base, répété sur chaque ligne. `variance = quantité physique − quantité théorique`. `varianceValue` est cet écart multiplié par le coût moyen. Il est absent et `costIncomplete` vaut `true` si le coût n’est pas fiable. Recompter le même conditionnement remplace la ligne.

Permission : `stock.inventory.count`.

### Valider

```graphql
mutation ($input: CompleteStockInventoryInput!) {
  completeStockInventory(input: $input) {
    id
    status
    completedAt
    lines { productId variance varianceValue costIncomplete }
  }
}
```

```json
{ "input": { "inventoryId": "INVENTORY_ID" } }
```

Au moment de la validation, les soldes sont verrouillés et les quantités théoriques sont relues. Si une vente ou une réception a eu lieu depuis le comptage, la réponse est `inventory_stale` : il faut rappeler `recordInventoryCount` pour les produits concernés, puis valider. Aucun stock n’est modifié quand la validation est refusée.

Un écart non nul écrit un mouvement `inventory_adjustment` et met le solde à la quantité comptée. Un écart nul n’écrit pas de mouvement. Une seconde validation répond `inventory_completed`.

Permission : `stock.inventory.validate`.

Relecture : `stockInventory(id: ID!)`.

Codes : `inventory`, `inventory_open`, `inventory_stale`, `inventory_completed`, `inventory_closed`, `quantity`, `product_archived`, `unit_archived`, `stock.inventory.start`, `stock.inventory.count`, `stock.inventory.validate`.

Scénario Android : un écran par boutique. Conserver `inventoryId`. Afficher l’écart avant validation. Si `inventory_stale`, rouvrir les lignes du produit et renvoyer le comptage.

## 5. Journal

```graphql
query ($store: ID!, $product: ID, $type: String, $after: DateTime, $afterId: ID) {
  stockMovements(storeId: $store, productId: $product, type: $type, after: $after, afterId: $afterId, limit: 50) {
    id
    productId
    sku
    productName
    storeId
    type
    quantity
    originUnit
    originQuantity
    refType
    refId
    actorId
    reason
    createdAt
  }
}
```

`quantity` est signée, en unité de base. `originQuantity` est la quantité saisie dans `originUnit`. Les mouvements d’avant cette version n’ont pas toujours l’unité d’origine.

| Valeur renvoyée | Écriture réelle | Filtre accepté |
|---|---|---|
| `receipt` | `receive` | `receipt`, `receive`, `purchase` |
| `sale` | `sale` | `sale` |
| `sale_return` | `refund` | `sale_return`, `refund` |
| `inventory_adjustment` | `inventory_adjustment` ou l’ancien `adjustment` | `inventory_adjustment`, `adjustment` |
| `opening` | `opening` | `opening` |
| `loss` | `loss` | `loss` |
| `purchase_return` | `purchase_return` | `purchase_return` |
| `transfer_out` | `transfer_out` | `transfer_out` |
| `transfer_in` | `transfer_in` | `transfer_in` |

`opening` est produit par `openInitialStock`. `loss` est produit par `recordStockLoss`. `purchase_return` est produit par `returnStockToSupplier`. `transfer_out` est produit à l'expédition. `transfer_in` est produit à la réception, ou sur la boutique d'origine quand un envoi en transit est annulé (`reason` = `returned_to_source`). Un filtre inconnu répond `movement_type`.

Le curseur est la paire `createdAt` + `id` du dernier élément, comme pour `stock`.

Permission : `stock.movements.read`.

## 6. Statistiques

```graphql
query ($store: ID) {
  stockSummary(storeId: $store) {
    totalProducts
    productsInStock
    outOfStockProducts
    lowStockProducts
    stockCostValues { currency amount }
    costMethod
    stockRetailValues { currency amount }
    retailMethod
    incompleteCostProducts
    incompleteRetailProducts
    updatedAt
  }
}
```

Sans `storeId`, le propriétaire et l’administrateur voient toutes les boutiques. Les autres rôles voient la somme de leurs affectations. Avec `storeId`, une seule boutique.

- `totalProducts` : produits actifs du catalogue.
- `productsInStock` : quantité strictement positive.
- `outOfStockProducts` : quantité nulle.
- `lowStockProducts` : quantité positive et inférieure ou égale au seuil. Ces produits sont aussi comptés dans `productsInStock`.
- `costMethod` : `weighted_average`. Valeur = quantité × coût moyen, par devise. USD et CDF ne sont jamais additionnés.
- `retailMethod` : `base_unit_price`. Valeur = quantité × prix de l’unité de base, par devise. Un article qui a les deux prix apparaît dans les deux totaux de vente potentielle, mais une seule fois dans les compteurs.
- `incompleteCostProducts` : stock positif dont le coût moyen n’est pas fiable. Ces quantités sont exclues des totaux de coût.
- `incompleteRetailProducts` : stock positif sans prix actif de l’unité de base.

`stockCostValues` et `incompleteCostProducts` sont **nuls** si l’appelant n’a pas `stock.valuation.read` ou si l’offre n’inclut pas les marges. Ce n’est pas un coût de zéro. Les prix de vente restent visibles.

Permission de lecture : `stock.read`.

## 7. Consultation du stock

`stock(storeId, after, afterId, limit)` est inchangé : quantité en unité de base, seuil, `belowAlert`.

## Codes d’erreur utiles

| Code | Signification |
|---|---|
| `unauthenticated` | jeton absent ou invalide |
| `stock.receive`, `stock.opening.create`, `stock.loss.create`, `stock.return.create`, `stock.transfer.create`, `stock.transfer.dispatch`, `stock.transfer.receive`, `stock.transfer.cancel` | action refusée |
| `opening_closed` | le produit a déjà un mouvement ou une quantité dans la boutique |
| `reason` | motif de perte absent ou hors de la liste |
| `store` | pas affecté à la boutique |
| `not_found` | produit, conditionnement, boutique ou inventaire invisible pour cette entreprise |
| `unit_not_for_purchase` | conditionnement interdit à l’achat |
| `unit_not_for_sale` | conditionnement interdit à la vente |
| `quantity` | quantité nulle, négative, ou fractionnaire non autorisée |
| `factor_locked` | facteur déjà utilisé |
| `idempotency_mismatch` | clé déjà utilisée pour un autre contenu |
| `inventory_open` | un inventaire est déjà ouvert dans la boutique |
| `inventory_stale` | le stock a bougé pendant le comptage |
| `inventory_completed` | déjà validé |
| `stock_conflict` | vente, perte, retour ou expédition au-delà du stock |
| `transfer_store` | origine et destination identiques |
| `transfer_status` | action impossible dans l'état actuel, par exemple réceptionner un brouillon |
| `transfer_dispatched` | transfert déjà expédié |
| `transfer_received` | transfert déjà réceptionné, ou annulation d'un transfert reçu |
| `transfer_cancelled` | transfert déjà annulé |
| `return` | retour fournisseur illisible ou sans ligne |
| `price_changed` | `expectedUnitPrice` ne correspond plus au prix serveur |
| `SUBSCRIPTION_EXPIRED` | écriture refusée, abonnement expiré |
