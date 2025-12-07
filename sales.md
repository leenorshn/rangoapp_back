## Requêtes importantes pour `Sale` (Front mobile)

### 1. Créer une vente (écran caisse) – `createSale`

```graphql
mutation CreateSale($input: CreateSaleInput!) {
  createSale(input: $input) {
    id
    priceToPay
    pricePayed
    change
    currency
    date
    client { id name phone }
    operator { id name }
    store { id name }
    basket {
      productId
      quantity
      price
      product {
        id
        name
        priceVente
      }
    }
  }
}
```

Variables exemple :

```json
{
  "input": {
    "basket": [
      { "productId": "PRODUCT_ID_1", "quantity": 2, "price": 5.0 },
      { "productId": "PRODUCT_ID_2", "quantity": 1, "price": 10.0 }
    ],
    "priceToPay": 20.0,
    "pricePayed": 20.0,
    "clientId": "CLIENT_ID",
    "storeId": "STORE_ID",
    "currency": "USD",
    "date": "2025-11-29T10:00:00Z"
  }
}
```

---

### 2. Lister les ventes d’un store (historique) – `sales`

```graphql
query SalesByStore($storeId: String) {
  sales(storeId: $storeId) {
    id
    date
    priceToPay
    pricePayed
    change
    currency
    client { id name }
    operator { id name }
  }
}
```

Variables :

```json
{ "storeId": "STORE_ID" }
```

> Si `storeId` est omis, le backend renvoie les ventes des stores accessibles.

---

### 3. Détail d’une vente (écran ticket) – `sale`

```graphql
query SaleDetail($id: ID!) {
  sale(id: $id) {
    id
    date
    priceToPay
    pricePayed
    change
    currency
    client { id name phone }
    operator { id name }
    store { id name }
    basket {
      productId
      quantity
      price
      product {
        id
        name
        mark
        priceVente
      }
    }
  }
}
```

Variables :

```json
{ "id": "SALE_ID" }
```

---

### 4. Générer une facture imprimable après une vente – `createFactureFromSale`

```graphql
mutation CreateFactureFromSale($saleId: ID!) {
  createFactureFromSale(saleId: $saleId) {
    id
    factureNumber
    date
    price
    currency
    client { id name phone }
    store { id name address }
    products {
      productId
      quantity
      price
      product { id name }
    }
  }
}
```

Variables :

```json
{ "saleId": "SALE_ID" }
```

---

### 5. Supprimer une vente (correction d’erreur caisse) – `deleteSale`

```graphql
mutation DeleteSale($id: ID!) {
  deleteSale(id: $id)
}
```

Variables :

```json
{ "id": "SALE_ID" }
```
















