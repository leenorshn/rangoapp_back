# 📋 Requêtes GraphQL pour les Ventes (Frontend Next.js)

Ce document contient **toutes les requêtes GraphQL** nécessaires pour charger et gérer les ventes dans votre frontend Next.js.

---

## 🔐 Authentification

Toutes les requêtes nécessitent un token JWT dans les headers :
```
Authorization: Bearer <token>
```

---

## 1. Liste des Ventes Optimisée (`salesList`)

**Utilisation** : Pour la page `/ventes` - version optimisée sans détails complets.

### Requête de base avec pagination

```graphql
query SalesList(
  $storeId: String!
  $limit: Int!
  $offset: Int!
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: $offset
  ) {
    id
    date
    createdAt
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
    storeId
  }
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 50,
  "offset": 0
}
```

---

### Avec filtre de période

```graphql
query SalesListWithPeriod(
  $storeId: String!
  $limit: Int!
  $offset: Int!
  $period: String!
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: $offset
    period: $period
  ) {
    id
    date
    createdAt
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
    storeId
  }
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 50,
  "offset": 0,
  "period": "jour"
}
```

**Périodes disponibles** :
- `"jour"` : Aujourd'hui
- `"semaine"` : Cette semaine
- `"mois"` : Ce mois
- `"annee"` : Cette année

---

### Avec filtre de devise

```graphql
query SalesListWithCurrency(
  $storeId: String!
  $limit: Int!
  $offset: Int!
  $currency: String!
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: $offset
    currency: $currency
  ) {
    id
    date
    createdAt
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
    storeId
  }
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 50,
  "offset": 0,
  "currency": "USD"
}
```

**Devises disponibles** : `"USD"`, `"EUR"`, `"XAF"`, `"XOF"`, `"CDF"`

---

### Avec période personnalisée (dates)

```graphql
query SalesListCustomPeriod(
  $storeId: String!
  $limit: Int!
  $offset: Int!
  $startDate: String!
  $endDate: String!
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: $offset
    startDate: $startDate
    endDate: $endDate
  ) {
    id
    date
    createdAt
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
    storeId
  }
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 50,
  "offset": 0,
  "startDate": "2025-12-01",
  "endDate": "2025-12-31"
}
```

**Format de date** : `"YYYY-MM-DD"` ou `"YYYY-MM-DDTHH:mm:ssZ"`

---

### Requête complète avec tous les filtres

```graphql
query SalesListComplete(
  $storeId: String!
  $limit: Int!
  $offset: Int!
  $period: String
  $startDate: String
  $endDate: String
  $currency: String
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: $offset
    period: $period
    startDate: $startDate
    endDate: $endDate
    currency: $currency
  ) {
    id
    date
    createdAt
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
    storeId
  }
}
```

---

## 2. Comptage des Ventes (`salesCount`)

**Utilisation** : Pour la pagination - obtenir le nombre total de ventes.

### Comptage simple

```graphql
query SalesCount($storeId: String!) {
  salesCount(storeId: $storeId)
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
}
```

---

### Comptage avec filtre de période

```graphql
query SalesCountWithPeriod(
  $storeId: String!
  $period: String!
) {
  salesCount(storeId: $storeId, period: $period)
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "period": "jour"
}
```

---

### Comptage avec tous les filtres

```graphql
query SalesCountComplete(
  $storeId: String!
  $period: String
  $startDate: String
  $endDate: String
  $currency: String
) {
  salesCount(
    storeId: $storeId
    period: $period
    startDate: $startDate
    endDate: $endDate
    currency: $currency
  )
}
```

---

## 3. Liste des Ventes Complète (`sales`)

**Utilisation** : Pour les détails complets (si nécessaire). **⚠️ Plus lente que `salesList`**

### Requête de base

```graphql
query Sales(
  $storeId: String!
  $limit: Int!
  $offset: Int!
) {
  sales(
    storeId: $storeId
    limit: $limit
    offset: $offset
  ) {
    id
    basket {
      productId
      product {
        id
        name
        priceVente
        priceAchat
      }
      quantity
      price
    }
    priceToPay
    pricePayed
    change
    benefice
    currency
    client {
      id
      name
      phone
    }
    operator {
      id
      name
    }
    storeId
    store {
      id
      name
    }
    date
    createdAt
    updatedAt
  }
}
```

---

### Avec filtres

```graphql
query SalesWithFilters(
  $storeId: String!
  $limit: Int!
  $offset: Int!
  $period: String
  $currency: String
) {
  sales(
    storeId: $storeId
    limit: $limit
    offset: $offset
    period: $period
    currency: $currency
  ) {
    id
    basket {
      productId
      product {
        id
        name
      }
      quantity
      price
    }
    priceToPay
    pricePayed
    change
    benefice
    currency
    client {
      id
      name
    }
    date
    createdAt
  }
}
```

---

## 4. Détails d'une Vente (`sale`)

**Utilisation** : Pour afficher les détails complets d'une vente (modal, page détail).

```graphql
query SaleDetail($id: ID!) {
  sale(id: $id) {
    id
    basket {
      productId
      product {
        id
        name
        mark
        priceVente
        priceAchat
        stock
      }
      quantity
      price
    }
    priceToPay
    pricePayed
    change
    benefice
    currency
    client {
      id
      name
      phone
      storeId
    }
    operator {
      id
      name
      phone
    }
    storeId
    store {
      id
      name
      address
      phone
    }
    date
    createdAt
    updatedAt
  }
}
```

**Variables** :
```json
{
  "id": "65a1b2c3d4e5f6g7h8i9j0k1"
}
```

---

## 5. Statistiques des Ventes (`salesStats`)

**Utilisation** : Pour afficher les statistiques agrégées (dashboard, rapports).

### Statistiques de base

```graphql
query SalesStats($storeId: String!) {
  salesStats(storeId: $storeId) {
    totalSales
    totalRevenue
    totalItems
    averageSale
    totalBenefice
  }
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
}
```

---

### Statistiques avec période

```graphql
query SalesStatsWithPeriod(
  $storeId: String!
  $period: String!
) {
  salesStats(storeId: $storeId, period: $period) {
    totalSales
    totalRevenue
    totalItems
    averageSale
    totalBenefice
  }
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "period": "jour"
}
```

---

### Statistiques avec tous les filtres

```graphql
query SalesStatsComplete(
  $storeId: String!
  $period: String
  $startDate: String
  $endDate: String
  $currency: String
) {
  salesStats(
    storeId: $storeId
    period: $period
    startDate: $startDate
    endDate: $endDate
    currency: $currency
  ) {
    totalSales
    totalRevenue
    totalItems
    averageSale
    totalBenefice
  }
}
```

---

## 6. Requêtes Combinées (Recommandées)

### Liste + Comptage (pour pagination)

```graphql
query SalesListWithCount(
  $storeId: String!
  $limit: Int!
  $offset: Int!
  $period: String
  $currency: String
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: $offset
    period: $period
    currency: $currency
  ) {
    id
    date
    createdAt
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
    storeId
  }
  
  totalCount: salesCount(
    storeId: $storeId
    period: $period
    currency: $currency
  )
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 50,
  "offset": 0,
  "period": "jour",
  "currency": "USD"
}
```

**Calcul de pagination** :
```typescript
const totalPages = Math.ceil(totalCount / limit);
const currentPage = Math.floor(offset / limit) + 1;
```

---

### Liste + Statistiques (dashboard)

```graphql
query SalesDashboard(
  $storeId: String!
  $limit: Int!
  $period: String!
  $currency: String
) {
  salesList(
    storeId: $storeId
    limit: $limit
    offset: 0
    period: $period
    currency: $currency
  ) {
    id
    date
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
  }
  
  stats: salesStats(
    storeId: $storeId
    period: $period
    currency: $currency
  ) {
    totalSales
    totalRevenue
    totalItems
    averageSale
    totalBenefice
  }
  
  totalCount: salesCount(
    storeId: $storeId
    period: $period
    currency: $currency
  )
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 10,
  "period": "jour",
  "currency": "USD"
}
```

---

## 📝 Notes Importantes

### Quand utiliser `salesList` vs `sales` ?

- **`salesList`** : Pour la page liste `/ventes` (plus rapide, moins de données)
- **`sales`** : Pour les détails complets (modal, page détail, export)

### Paramètres optionnels

Tous les paramètres sont optionnels sauf `storeId` (si vous voulez filtrer par store) :
- `storeId` : Optionnel (si non fourni, retourne les ventes de tous les stores accessibles)
- `limit` : Optionnel (défaut: 50, max: 1000)
- `offset` : Optionnel (défaut: 0)
- `period` : Optionnel (`"jour"`, `"semaine"`, `"mois"`, `"annee"`)
- `startDate` / `endDate` : Optionnel (format `"YYYY-MM-DD"` ou RFC3339)
- `currency` : Optionnel (`"USD"`, `"EUR"`, `"XAF"`, `"XOF"`, `"CDF"`)

### Priorité des filtres de date

- Si `startDate` et `endDate` sont fournis, `period` est ignoré
- Si seulement `period` est fourni, les dates sont calculées automatiquement
- Si aucun n'est fourni, toutes les ventes sont retournées

### Performance

- **`salesList`** : Optimisé avec projection MongoDB (~30% plus rapide)
- **`salesStats`** : Utilise aggregation pipeline MongoDB (très rapide)
- **`salesCount`** : Utilise `CountDocuments` MongoDB (rapide)

---

## 🎯 Exemples d'Utilisation

### Page Liste des Ventes (avec pagination)

```graphql
query SalesListPage(
  $storeId: String!
  $page: Int!
  $pageSize: Int!
  $period: String
) {
  salesList(
    storeId: $storeId
    limit: $pageSize
    offset: $calcOffset
    period: $period
  ) {
    id
    date
    priceToPay
    pricePayed
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
  }
  
  totalCount: salesCount(
    storeId: $storeId
    period: $period
  )
}
```

**Variables** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "page": 1,
  "pageSize": 50,
  "period": "jour",
  "calcOffset": 0
}
```

**Note** : `calcOffset = (page - 1) * pageSize`

---

### Dashboard avec Statistiques

```graphql
query SalesDashboardToday($storeId: String!) {
  salesList(
    storeId: $storeId
    limit: 10
    period: "jour"
  ) {
    id
    date
    priceToPay
    pricePayed
    currency
    client {
      name
    }
  }
  
  stats: salesStats(
    storeId: $storeId
    period: "jour"
  ) {
    totalSales
    totalRevenue
    totalItems
    averageSale
    totalBenefice
  }
}
```

---

### Rapport Mensuel

```graphql
query SalesMonthlyReport(
  $storeId: String!
  $currency: String!
) {
  salesList(
    storeId: $storeId
    limit: 1000
    period: "mois"
    currency: $currency
  ) {
    id
    date
    priceToPay
    pricePayed
    change
    currency
    client {
      id
      name
    }
    basketCount
    totalItems
  }
  
  stats: salesStats(
    storeId: $storeId
    period: "mois"
    currency: $currency
  ) {
    totalSales
    totalRevenue
    totalItems
    averageSale
    totalBenefice
  }
  
  totalCount: salesCount(
    storeId: $storeId
    period: "mois"
    currency: $currency
  )
}
```

---

## 🔄 Mutations (Création/Suppression)

### Créer une vente

```graphql
mutation CreateSale($input: CreateSaleInput!) {
  createSale(input: $input) {
    id
    priceToPay
    pricePayed
    change
    currency
    date
    createdAt
  }
}
```

**Variables** :
```json
{
  "input": {
    "basket": [
      {
        "productId": "65a1b2c3d4e5f6g7h8i9j0k1",
        "quantity": 2,
        "price": 100.0
      }
    ],
    "priceToPay": 200.0,
    "pricePayed": 200.0,
    "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
    "currency": "USD",
    "clientId": "65a1b2c3d4e5f6g7h8i9j0k1"
  }
}
```

---

### Supprimer une vente

```graphql
mutation DeleteSale($id: ID!) {
  deleteSale(id: $id)
}
```

**Variables** :
```json
{
  "id": "65a1b2c3d4e5f6g7h8i9j0k1"
}
```

---

## 📚 Références

- Voir `OPTIMISATIONS_VENTES.md` pour les détails techniques backend
- Voir `graph/schema.graphqls` pour le schéma GraphQL complet


























