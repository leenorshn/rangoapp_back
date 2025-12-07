# Prompt de Mise à Jour Frontend - Modifications Backend

Ce document liste toutes les modifications GraphQL nécessitant une mise à jour du frontend.

## 📋 Table des Matières
1. [Produits - Currency et Provider](#produits---currency-et-provider)
2. [Ventes - Gestion des Dettes](#ventes---gestion-des-dettes)
3. [Inventaire - Nouveau Système](#inventaire---nouveau-système)

---

## 🛍️ Produits - Currency et Provider

### Modifications des Types

#### `Product` - Nouveaux champs
```graphql
type Product {
  # ... champs existants ...
  currency: String! # Nouveau: Currency du produit (ex: "USD", "EUR", "CDF")
  providerId: String # Nouveau: ID du fournisseur (optionnel)
  provider: Provider # Nouveau: Fournisseur associé (optionnel)
}
```

#### `Store` - Nouveaux champs
```graphql
type Store {
  # ... champs existants ...
  defaultCurrency: String! # Nouveau: Currency par défaut de la boutique
  supportedCurrencies: [String!]! # Nouveau: Liste des currencies supportées
}
```

### Modifications des Inputs

#### `CreateProductInput` - Nouveaux champs
```graphql
input CreateProductInput {
  # ... champs existants ...
  currency: String # Nouveau: Optionnel, utilise defaultCurrency de la boutique si non fourni
  providerId: String # Nouveau: ID du fournisseur (optionnel)
}
```

#### `UpdateProductInput` - Nouveaux champs
```graphql
input UpdateProductInput {
  # ... champs existants ...
  currency: String # Nouveau: Currency du produit
  providerId: String # Nouveau: Optionnel, peut être null pour retirer le fournisseur
}
```

#### `CreateStoreInput` - Nouveaux champs
```graphql
input CreateStoreInput {
  # ... champs existants ...
  defaultCurrency: String # Nouveau: Optionnel, défaut: "USD"
  supportedCurrencies: [String!] # Nouveau: Optionnel, si non fourni, utilise defaultCurrency
}
```

#### `UpdateStoreInput` - Nouveaux champs
```graphql
input UpdateStoreInput {
  # ... champs existants ...
  defaultCurrency: String # Nouveau: Currency par défaut
  supportedCurrencies: [String!] # Nouveau: Liste des currencies supportées (doit inclure defaultCurrency)
}
```

### Currencies Supportées
- **USD** (Dollar américain)
- **EUR** (Euro)
- **CDF** (Franc congolais)

### Actions Frontend Requises
1. Ajouter un sélecteur de currency lors de la création/édition d'un produit
2. Afficher la currency dans la liste et les détails des produits
3. Ajouter un sélecteur de fournisseur lors de la création/édition d'un produit
4. Afficher le fournisseur associé dans les détails du produit
5. Ajouter la gestion de `defaultCurrency` et `supportedCurrencies` lors de la création/édition d'un store
6. Valider que la currency sélectionnée est dans la liste des currencies supportées du store

---

## 💰 Ventes - Gestion des Dettes

### Nouveaux Types

#### `Debt`
```graphql
type Debt {
  id: ID!
  saleId: String!
  sale: Sale!
  clientId: String!
  client: Client!
  storeId: String!
  store: Store!
  totalAmount: Float! # Montant total de la dette
  amountPaid: Float! # Montant déjà payé
  amountDue: Float! # Montant restant dû
  currency: String!
  status: String! # "paid", "partial", "unpaid"
  payments: [DebtPayment!]! # Historique des paiements
  createdAt: String!
  updatedAt: String!
}
```

#### `DebtPayment`
```graphql
type DebtPayment {
  id: ID!
  debtId: String!
  debt: Debt!
  amount: Float!
  currency: String!
  operatorId: String!
  operator: User!
  storeId: String!
  store: Store!
  description: String!
  createdAt: String!
}
```

### Modifications du Type `Sale`

#### `Sale` - Nouveaux champs
```graphql
type Sale {
  # ... champs existants ...
  paymentType: String! # Nouveau: "cash", "debt", "advance"
  amountDue: Float! # Nouveau: Montant dû (dette restante)
  debtStatus: String! # Nouveau: "paid", "partial", "unpaid", "none"
  debtId: String # Nouveau: ID de la dette si applicable
  debt: Debt # Nouveau: Dette associée si applicable
}
```

#### `SaleList` - Nouveaux champs
```graphql
type SaleList {
  # ... champs existants ...
  paymentType: String! # Nouveau: "cash", "debt", "advance"
  amountDue: Float! # Nouveau: Montant dû (dette restante)
  debtStatus: String! # Nouveau: "paid", "partial", "unpaid", "none"
}
```

### Modifications des Inputs

#### `CreateSaleInput` - Nouveau champ
```graphql
input CreateSaleInput {
  # ... champs existants ...
  paymentType: String # Nouveau: Optionnel, "cash", "debt", "advance" (défaut: "cash")
}
```

**Note importante** : 
- Si `paymentType` est "debt" ou "advance", un `clientId` est **requis**
- Si `paymentType` est "debt" et `pricePayed < priceToPay`, une dette sera automatiquement créée
- Si `paymentType` est "advance" et `pricePayed < priceToPay`, une dette sera également créée

### Nouvelles Queries

#### `debts`
```graphql
query Debts($storeId: String, $status: String) {
  debts(storeId: $storeId, status: $status) {
    id
    saleId
    sale { id }
    clientId
    client { id name phone }
    storeId
    store { id name }
    totalAmount
    amountPaid
    amountDue
    currency
    status
    payments {
      id
      amount
      description
      createdAt
      operator { id name }
    }
    createdAt
    updatedAt
  }
}
```

#### `debt`
```graphql
query Debt($id: ID!) {
  debt(id: $id) {
    id
    saleId
    sale { id priceToPay pricePayed }
    clientId
    client { id name phone }
    storeId
    store { id name }
    totalAmount
    amountPaid
    amountDue
    currency
    status
    payments {
      id
      amount
      description
      createdAt
      operator { id name }
    }
    createdAt
    updatedAt
  }
}
```

#### `clientDebts`
```graphql
query ClientDebts($clientId: String!, $storeId: String) {
  clientDebts(clientId: $clientId, storeId: $storeId) {
    id
    saleId
    sale { id }
    totalAmount
    amountPaid
    amountDue
    currency
    status
    createdAt
    updatedAt
  }
}
```

### Nouvelles Mutations

#### `payDebt`
```graphql
mutation PayDebt($debtId: ID!, $amount: Float!, $description: String!) {
  payDebt(debtId: $debtId, amount: $amount, description: $description) {
    id
    totalAmount
    amountPaid
    amountDue
    status
    payments {
      id
      amount
      description
      createdAt
    }
  }
}
```

### Actions Frontend Requises
1. Ajouter un sélecteur de `paymentType` lors de la création d'une vente
2. Si `paymentType` est "debt" ou "advance", rendre le champ `clientId` obligatoire
3. Afficher `paymentType`, `amountDue`, et `debtStatus` dans la liste des ventes
4. Afficher les informations de dette dans les détails d'une vente
5. Créer une page/interface pour :
   - Lister toutes les dettes (`debts` query)
   - Voir les détails d'une dette (`debt` query)
   - Voir les dettes d'un client (`clientDebts` query)
   - Payer une dette (`payDebt` mutation)
6. Afficher l'historique des paiements pour chaque dette
7. Filtrer les dettes par statut ("paid", "partial", "unpaid")
8. Afficher un indicateur visuel pour les ventes avec dettes en attente

---

## 📦 Inventaire - Nouveau Système

### Nouveaux Types

#### `Inventory`
```graphql
type Inventory {
  id: ID!
  storeId: String!
  store: Store!
  operatorId: String!
  operator: User!
  status: String! # "draft", "in_progress", "completed", "cancelled"
  startDate: String!
  endDate: String # Date de fin (si status = "completed")
  description: String!
  items: [InventoryItem!]!
  totalItems: Int!
  totalValue: Float!
  createdAt: String!
  updatedAt: String!
}
```

#### `InventoryItem`
```graphql
type InventoryItem {
  productId: String!
  product: Product!
  productName: String!
  systemQuantity: Float! # Quantité dans le système
  physicalQuantity: Float! # Quantité physique comptée
  difference: Float! # Différence (physicalQuantity - systemQuantity)
  unitPrice: Float!
  totalValue: Float!
  reason: String # Raison de l'écart (vol, casse, erreur, etc.)
  countedBy: String!
  countedByUser: User!
  countedAt: String!
}
```

### Nouveaux Inputs

#### `CreateInventoryInput`
```graphql
input CreateInventoryInput {
  storeId: String!
  description: String!
}
```

#### `AddInventoryItemInput`
```graphql
input AddInventoryItemInput {
  inventoryId: String!
  productId: String!
  physicalQuantity: Float!
  reason: String # Optionnel: Raison de l'écart
}
```

### Nouvelles Queries

#### `inventories`
```graphql
query Inventories($storeId: String, $status: String) {
  inventories(storeId: $storeId, status: $status) {
    id
    storeId
    store { id name }
    operatorId
    operator { id name }
    status
    startDate
    endDate
    description
    totalItems
    totalValue
    createdAt
    updatedAt
  }
}
```

#### `inventory`
```graphql
query Inventory($id: ID!) {
  inventory(id: $id) {
    id
    storeId
    store { id name }
    operatorId
    operator { id name }
    status
    startDate
    endDate
    description
    items {
      productId
      product { id name mark }
      productName
      systemQuantity
      physicalQuantity
      difference
      unitPrice
      totalValue
      reason
      countedBy
      countedByUser { id name }
      countedAt
    }
    totalItems
    totalValue
    createdAt
    updatedAt
  }
}
```

#### `activeInventory`
```graphql
query ActiveInventory($storeId: String!) {
  activeInventory(storeId: $storeId) {
    id
    status
    description
    startDate
    totalItems
    totalValue
    items {
      productId
      product { id name }
      systemQuantity
      physicalQuantity
      difference
    }
  }
}
```

### Nouvelles Mutations

#### `createInventory`
```graphql
mutation CreateInventory($input: CreateInventoryInput!) {
  createInventory(input: $input) {
    id
    storeId
    status
    description
    startDate
    totalItems
    totalValue
  }
}
```

#### `addInventoryItem`
```graphql
mutation AddInventoryItem($input: AddInventoryItemInput!) {
  addInventoryItem(input: $input) {
    id
    status
    items {
      productId
      product { id name }
      systemQuantity
      physicalQuantity
      difference
      reason
    }
    totalItems
    totalValue
  }
}
```

#### `completeInventory`
```graphql
mutation CompleteInventory($inventoryId: ID!, $adjustStock: Boolean!) {
  completeInventory(inventoryId: $inventoryId, adjustStock: $adjustStock) {
    id
    status
    endDate
    totalItems
    totalValue
  }
}
```

#### `cancelInventory`
```graphql
mutation CancelInventory($inventoryId: ID!) {
  cancelInventory(inventoryId: $inventoryId) {
    id
    status
  }
}
```

### Statuts d'Inventaire
- **draft** : En cours de préparation
- **in_progress** : En cours de comptage
- **completed** : Terminé
- **cancelled** : Annulé

### Actions Frontend Requises
1. Créer une page/interface pour gérer les inventaires :
   - Liste des inventaires avec filtres par store et statut
   - Détails d'un inventaire avec tous les items
   - Vue de l'inventaire actif pour un store

2. Interface de création d'inventaire :
   - Formulaire avec `storeId` et `description`
   - Vérifier qu'il n'y a pas déjà un inventaire actif pour le store

3. Interface de comptage :
   - Permettre d'ajouter des produits à l'inventaire
   - Afficher la quantité système vs quantité physique
   - Calculer et afficher la différence automatiquement
   - Permettre d'ajouter une raison pour les écarts
   - Mettre à jour un produit déjà compté

4. Interface de finalisation :
   - Afficher un résumé de l'inventaire (total items, valeur totale)
   - Afficher les écarts (produits avec différence)
   - Option pour ajuster automatiquement le stock
   - Confirmer la finalisation

5. Indicateurs visuels :
   - Différence positive (vert) : plus de stock que prévu
   - Différence négative (rouge) : moins de stock que prévu
   - Différence nulle (gris) : stock conforme

6. Rapports :
   - Afficher la valeur totale de l'inventaire
   - Afficher le nombre total de produits inventoriés
   - Afficher les produits avec écarts significatifs

---

## 🔄 Résumé des Modifications

### Types Modifiés
- `Product` : + `currency`, `providerId`, `provider`
- `Store` : + `defaultCurrency`, `supportedCurrencies`
- `Sale` : + `paymentType`, `amountDue`, `debtStatus`, `debtId`, `debt`
- `SaleList` : + `paymentType`, `amountDue`, `debtStatus`

### Nouveaux Types
- `Debt`
- `DebtPayment`
- `Inventory`
- `InventoryItem`

### Inputs Modifiés
- `CreateProductInput` : + `currency`, `providerId`
- `UpdateProductInput` : + `currency`, `providerId`
- `CreateStoreInput` : + `defaultCurrency`, `supportedCurrencies`
- `UpdateStoreInput` : + `defaultCurrency`, `supportedCurrencies`
- `CreateSaleInput` : + `paymentType`

### Nouveaux Inputs
- `CreateInventoryInput`
- `AddInventoryItemInput`

### Nouvelles Queries
- `debts(storeId, status)`
- `debt(id)`
- `clientDebts(clientId, storeId)`
- `inventories(storeId, status)`
- `inventory(id)`
- `activeInventory(storeId)`

### Nouvelles Mutations
- `payDebt(debtId, amount, description)`
- `createInventory(input)`
- `addInventoryItem(input)`
- `completeInventory(inventoryId, adjustStock)`
- `cancelInventory(inventoryId)`

---

## ⚠️ Notes Importantes

1. **Currencies** : Seules "USD", "EUR", et "CDF" sont supportées
2. **Dettes** : Un `clientId` est requis si `paymentType` est "debt" ou "advance"
3. **Inventaire** : Un seul inventaire actif (draft ou in_progress) peut exister par store à la fois
4. **Ajustement de stock** : Lors de la finalisation d'un inventaire, l'ajustement automatique du stock est optionnel via le paramètre `adjustStock`

---

## 📝 Exemple de Workflow Complet

### Workflow de Vente avec Dette
1. Créer une vente avec `paymentType: "debt"` et `clientId` requis
2. Si `pricePayed < priceToPay`, une dette est automatiquement créée
3. Utiliser `clientDebts` pour voir toutes les dettes d'un client
4. Utiliser `payDebt` pour enregistrer un paiement partiel ou total
5. La dette se met à jour automatiquement avec le statut approprié

### Workflow d'Inventaire
1. Créer un inventaire avec `createInventory`
2. Ajouter des produits avec `addInventoryItem` (peut être fait plusieurs fois)
3. Vérifier l'inventaire actif avec `activeInventory`
4. Finaliser avec `completeInventory` (optionnel : ajuster le stock automatiquement)
5. Consulter l'historique avec `inventories`

---

**Date de mise à jour** : 2 décembre 2025
**Version Backend** : Dernière version avec gestion des currencies, dettes et inventaire

