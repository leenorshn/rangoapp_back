# Catalogue EasyStock pour Android

Contrat `POST /graphql`. Les montants, quantités et facteurs sont des chaînes décimales (`"500"`, `"1.5"`, `"24"`), jamais des flottants. Les dates sont des `DateTime` RFC3339. L'entreprise vient de la session : aucun `companyId` envoyé par l'application n'est lu.

Une ligne vendable se construit avec l'identifiant produit, le **nom** du conditionnement, la quantité de ce conditionnement et le `unitPrice` renvoyé pour la boutique et la devise. Le parcours d'encaissement, y compris la dette et la date d'échéance, est décrit dans `EASYSTOCK_FAST_SALE_ANDROID.md`.

## Droits

Le propriétaire gère le catalogue. Le gérant aussi, sauf si `catalog.manage` lui est refusé. Le vendeur lit les produits et les prix de ses boutiques, et ne peut pas les modifier. `cost` est `null` sans la permission de marge. L'offre ne le retire pas. Ce n'est pas une erreur, et le coût n'apparaît dans aucun autre champ.

Une devise sans prix n'est pas vendable. USD et CDF sont indépendants : omettre l'autre prix ne fait pas échouer la création.

## Créer le premier produit

Le SKU est facultatif. Un prix dont `unitPrice` est absent est ignoré.

```graphql
mutation ($input: ProductInput!) {
  createProduct(input: $input) {
    id
    sku
    name
    baseUnit
    allowFractional
    alertThreshold
    units {
      id
      name
      factor
      isBase
      active
      prices { currency unitPrice }
    }
  }
}
```

```json
{
  "input": {
    "name": "Eau minérale",
    "baseUnit": "bouteille",
    "storeId": "<storeId>",
    "prices": [
      { "unitName": "bouteille", "currency": "USD", "unitPrice": "500" },
      { "currency": "CDF" }
    ]
  }
}
```

L'unité de base est créée avec le facteur `1`. Après cet appel, l'étape d'inscription `catalog` passe à `done: true`.

## Conditionnements et prix ensuite

```graphql
mutation ($productId: ID!, $input: ProductUnitInput!) {
  createProductUnit(productId: $productId, input: $input) {
    id
    name
    factor
  }
}
```

```json
{ "productId": "<productId>", "input": { "name": "carton", "factor": "24" } }
```

```graphql
mutation ($input: ProductPriceInput!) {
  upsertProductPrice(input: $input) {
    id
    currency
    unitPrice
  }
}
```

```json
{
  "input": {
    "storeId": "<storeId>",
    "productId": "<productId>",
    "unitName": "carton",
    "currency": "USD",
    "unitPrice": "10000"
  }
}
```

Le facteur doit être supérieur à zéro. Celui de l'unité de base reste `1`. Un facteur déjà utilisé par une vente ou un achat ne peut plus changer (`factor_locked`) : les anciennes lignes gardent leur quantité de base. Il faut alors créer un autre conditionnement.

## Lire et synchroniser

```graphql
query ($storeId: ID!, $after: DateTime, $afterId: ID) {
  catalogChanges(storeId: $storeId, after: $after, afterId: $afterId, limit: 100) {
    id
    name
    sku
    barcode
    categoryName
    category { id name }
    baseUnit
    allowFractional
    alertThreshold
    status
    archived
    updatedAt
    units {
      id
      name
      factor
      barcode
      active
      prices { currency unitPrice }
    }
  }
}
```

La page suivante reprend `updatedAt` et `id` du dernier élément, avec le même `storeId`. Un produit archivé reste dans la page pour être retiré de la liste vendable. Un conditionnement `active: false` ne se vend plus. Les prix absents ont été retirés.

`products(storeId, search, categoryId, forSale, after, afterId, limit)` liste les produits actifs. Avec `forSale: true`, seuls les produits qui ont encore un conditionnement actif autorisé à la vente sont renvoyés, et les autres conditionnements sont omis. Si `storeId` est fourni, `availableQuantity` (unité de base) et `availability` (`available` ou `out_of_stock`) sont présents. `imageMediaId` est présent quand une image est enregistrée. `product(id, storeId)` et `productByBarcode(storeId, barcode)` renvoient le même objet, avec `matchedUnitId` pour le code-barres reconnu.

```graphql
query ($id: ID!, $storeId: ID) {
  product(id: $id, storeId: $storeId) {
    id
    name
    units { id name factor prices { currency unitPrice } }
    cost { currency avgCost costComplete }
  }
}
```

## Vendre avec confirmSale

Pour deux cartons dont le facteur est `24`, envoyer la quantité `"2"` et le `unitPrice` du carton. Le stock, exprimé en bouteilles, baisse de 48. Une quantité entière de cartons reste acceptée quand `allowFractional` est faux. `"1.5"` bouteille est refusé dans ce cas (`quantity`).

```graphql
mutation ($input: ConfirmSaleInput!) {
  confirmSale(input: $input) {
    saleId
    currency
    netAmount
    paidAmount
    dueAmount
  }
}
```

```json
{
  "input": {
    "idempotencyKey": "<uuid côté appareil>",
    "storeId": "<storeId>",
    "currency": "USD",
    "lines": [
      {
        "productId": "<productId>",
        "unitName": "carton",
        "quantity": "2",
        "expectedUnitPrice": "10000"
      }
    ],
    "payments": [{ "method": "external", "amount": "20000" }]
  }
}
```

`expectedUnitPrice` est comparé au prix serveur. S'il a changé, la vente est refusée avec le code `price_changed` et le message « Le prix a changé depuis la préparation de la vente. Vérifiez le nouveau prix avant de confirmer. » Relire `product` ou `catalogChanges`, afficher le nouveau prix, puis renvoyer `confirmSale` avec une nouvelle confirmation. Les autres codes utiles : `product_archived`, `unit_archived`, `stock_conflict`, `not_found` (pas de prix dans cette devise).

Le stock disponible se lit avec la query `stock` existante. Il est dans l'unité de base.
