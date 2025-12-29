# 🔄 Mise à Jour Frontend - État Actuel du Backend

**Date de mise à jour** : 28 décembre 2025  
**Version Backend** : Architecture avec Product/ProductInStock séparés

---

## ⚠️ IMPORTANT : Changement d'Architecture

L'architecture des produits a changé. Il y a maintenant **deux entités distinctes** :

1. **`Product`** : Template de produit (nom, marque) - **SANS** prix, stock, currency
2. **`ProductInStock`** : Produit en stock avec prix, stock, currency, fournisseur

### Workflow de Création de Produit

1. **Créer le template** : `createProduct(input: CreateProductInput!)` 
   - Input: `name`, `mark`, `storeId`
   - Retourne: `Product` (template)

2. **Ajouter du stock** : `stockSupply(input: StockSupplyInput!)`
   - Input: `productId`, `quantity`, `priceAchat`, `priceVente`, `currency`, `storeId`, `providerId`, `paymentType`, `amountPaid`, `date`
   - Crée un `ProductInStock` avec les informations de stock

---

## 📋 Modifications Requises pour le Frontend

### 1. ⚠️ **CreateProductInput - CHANGEMENT MAJEUR**

**AVANT** (obsolète - ne plus utiliser) :
```graphql
input CreateProductInput {
  name: String!
  mark: String!
  storeId: String!
  currency: String      # ❌ N'EXISTE PLUS
  providerId: String    # ❌ N'EXISTE PLUS
  priceVente: Float     # ❌ N'EXISTE PLUS
  priceAchat: Float     # ❌ N'EXISTE PLUS
  stock: Float          # ❌ N'EXISTE PLUS
}
```

**MAINTENANT** (actuel) :
```graphql
input CreateProductInput {
  name: String!
  mark: String!
  storeId: String!
  # C'est tout ! Pas de prix, stock, currency ici
}
```

**Action Frontend** : 
- ✅ Supprimer les champs `currency`, `providerId`, `priceVente`, `priceAchat`, `stock` du formulaire de création de produit
- ✅ Créer d'abord le template avec `createProduct`
- ✅ Ensuite, utiliser `stockSupply` pour ajouter du stock

---

### 2. ⚠️ **UpdateProductInput - CHANGEMENT MAJEUR**

**AVANT** (obsolète) :
```graphql
input UpdateProductInput {
  name: String
  mark: String
  currency: String      # ❌ N'EXISTE PLUS
  providerId: String   # ❌ N'EXISTE PLUS
}
```

**MAINTENANT** (actuel) :
```graphql
input UpdateProductInput {
  name: String
  mark: String
  # Seulement le nom et la marque peuvent être modifiés
}
```

**Action Frontend** :
- ✅ Supprimer les champs `currency` et `providerId` du formulaire d'édition de produit
- ✅ Pour modifier le stock/prix, utiliser les mutations de `ProductInStock` (si disponibles) ou créer un nouveau `stockSupply`

---

### 3. ✅ **Store - Déjà Prêt**

```graphql
type Store {
  id: ID!
  name: String!
  address: String!
  phone: String!
  companyId: String!
  company: Company!
  defaultCurrency: String!        # ✅ Existe
  supportedCurrencies: [String!]! # ✅ Existe
  createdAt: String!
  updatedAt: String!
}
```

**Action Frontend** :
- ✅ Afficher `defaultCurrency` et `supportedCurrencies` dans les détails du store
- ✅ Utiliser ces informations pour valider les currencies lors de la création de `stockSupply`

---

### 4. ✅ **ProductInStock - Nouveau Type à Utiliser**

```graphql
type ProductInStock {
  id: ID!
  productId: String!
  product: Product!        # Référence au template
  priceVente: Float!
  priceAchat: Float!
  currency: String!        # USD, EUR, CDF
  stock: Float!
  storeId: String!
  store: Store!
  providerId: String!
  provider: Provider!
  createdAt: String!
  updatedAt: String!
}
```

**Action Frontend** :
- ✅ Utiliser `ProductInStock` pour afficher les produits avec stock
- ✅ Afficher `priceVente`, `priceAchat`, `currency`, `stock`, `provider` dans les listes de produits
- ✅ Utiliser `productInStockId` (pas `productId`) dans les ventes

---

### 5. ✅ **StockSupplyInput - Pour Ajouter du Stock**

```graphql
input StockSupplyInput {
  productId: String!      # ID du template Product
  quantity: Float!
  priceAchat: Float!
  priceVente: Float!
  currency: String        # Optionnel: utilise defaultCurrency du store
  storeId: String!
  providerId: String!     # Obligatoire
  paymentType: String!    # "cash" ou "debt"
  amountPaid: Float      # Obligatoire si paymentType = "debt"
  date: String           # Optionnel, défaut: maintenant
}
```

**Action Frontend** :
- ✅ Créer une interface pour ajouter du stock à un produit existant
- ✅ Utiliser cette mutation après avoir créé un `Product` template
- ✅ Gérer les paiements aux fournisseurs (cash ou debt)

---

### 6. ✅ **SaleProductInput - CHANGEMENT IMPORTANT**

**AVANT** (obsolète) :
```graphql
input SaleProductInput {
  productId: String!      # ❌ N'EXISTE PLUS
  quantity: Float!
  price: Float!
}
```

**MAINTENANT** (actuel) :
```graphql
input SaleProductInput {
  productInStockId: String!  # ✅ Utiliser ProductInStock ID
  quantity: Float!
  price: Float!
}
```

**Action Frontend** :
- ✅ **CRITIQUE** : Changer `productId` en `productInStockId` dans les ventes
- ✅ Utiliser les `ProductInStock` dans le panier, pas les `Product`
- ✅ Vérifier que le produit a du stock avant de l'ajouter au panier

---

### 7. ✅ **Ventes avec Dettes - Déjà Prêt**

```graphql
type Sale {
  id: ID!
  basket: [SaleProduct!]!
  priceToPay: Float!
  pricePayed: Float!
  currency: String!
  clientId: String
  client: Client
  storeId: String!
  store: Store!
  paymentType: String!    # "cash", "debt", "advance"
  amountDue: Float!      # Montant dû
  debtStatus: String!     # "paid", "partial", "unpaid", "none"
  debtId: String
  debt: Debt
  date: String!
  createdAt: String!
  updatedAt: String!
}
```

**Action Frontend** :
- ✅ Afficher `paymentType`, `amountDue`, `debtStatus` dans les listes de ventes
- ✅ Gérer les dettes clients (voir section Dettes ci-dessous)

---

### 8. ✅ **Dettes Clients - Déjà Prêt**

Voir le document `FRONTEND_UPDATE_PROMPT.md` pour les détails complets sur :
- Type `Debt`
- Type `DebtPayment`
- Queries : `debts`, `debt`, `clientDebts`
- Mutation : `payDebt`

**Action Frontend** :
- ✅ Implémenter la gestion des dettes clients
- ✅ Afficher les dettes dans les détails du client
- ✅ Permettre le paiement partiel ou total des dettes

---

### 9. ✅ **Inventaire - Déjà Prêt**

Voir le document `FRONTEND_UPDATE_PROMPT.md` pour les détails complets sur :
- Type `Inventory`
- Type `InventoryItem`
- Queries : `inventories`, `inventory`, `activeInventory`
- Mutations : `createInventory`, `addInventoryItem`, `completeInventory`, `cancelInventory`

**Action Frontend** :
- ✅ Implémenter le système d'inventaire complet
- ✅ Gérer les écarts de stock
- ✅ Ajuster automatiquement le stock après inventaire

---

## 🔴 Points Critiques à Corriger Immédiatement

### 1. **CreateProduct - Supprimer les champs obsolètes**
```typescript
// ❌ NE PLUS FAIRE
const createProduct = {
  name: "Produit",
  mark: "Marque",
  storeId: "123",
  currency: "USD",      // ❌ N'existe plus
  providerId: "456",   // ❌ N'existe plus
  priceVente: 100,     // ❌ N'existe plus
  priceAchat: 50,      // ❌ N'existe plus
  stock: 10            // ❌ N'existe plus
}

// ✅ FAIRE MAINTENANT
const createProduct = {
  name: "Produit",
  mark: "Marque",
  storeId: "123"
}

// Puis créer le stock séparément
const stockSupply = {
  productId: product.id,
  quantity: 10,
  priceAchat: 50,
  priceVente: 100,
  currency: "USD",
  storeId: "123",
  providerId: "456",
  paymentType: "cash"
}
```

### 2. **UpdateProduct - Supprimer les champs obsolètes**
```typescript
// ❌ NE PLUS FAIRE
const updateProduct = {
  name: "Nouveau nom",
  currency: "EUR"      // ❌ N'existe plus
}

// ✅ FAIRE MAINTENANT
const updateProduct = {
  name: "Nouveau nom"
  // Seulement name et mark
}
```

### 3. **Ventes - Utiliser productInStockId**
```typescript
// ❌ NE PLUS FAIRE
const saleProduct = {
  productId: "123",     // ❌ N'existe plus
  quantity: 2,
  price: 100
}

// ✅ FAIRE MAINTENANT
const saleProduct = {
  productInStockId: "789",  // ✅ ID du ProductInStock
  quantity: 2,
  price: 100
}
```

---

## 📝 Workflow Complet Recommandé

### Créer un Produit avec Stock

1. **Créer le template**
```graphql
mutation {
  createProduct(input: {
    name: "Produit Test"
    mark: "Marque Test"
    storeId: "store123"
  }) {
    id
    name
    mark
  }
}
```

2. **Ajouter du stock**
```graphql
mutation {
  stockSupply(input: {
    productId: "product123"
    quantity: 100
    priceAchat: 50
    priceVente: 100
    currency: "USD"
    storeId: "store123"
    providerId: "provider456"
    paymentType: "cash"
  }) {
    id
    productInStock {
      id
      stock
      priceVente
      currency
    }
  }
}
```

3. **Vendre le produit**
```graphql
mutation {
  createSale(input: {
    basket: [{
      productInStockId: "productInStock789"  # ✅ ID du ProductInStock
      quantity: 2
      price: 100
    }]
    priceToPay: 200
    pricePayed: 200
    storeId: "store123"
    paymentType: "cash"
  }) {
    id
  }
}
```

---

## ✅ Résumé des Actions Frontend

### À Supprimer/Corriger
- ❌ Champs `currency`, `providerId`, `priceVente`, `priceAchat`, `stock` dans `CreateProductInput`
- ❌ Champs `currency`, `providerId` dans `UpdateProductInput`
- ❌ Utilisation de `productId` dans `SaleProductInput` (remplacer par `productInStockId`)

### À Ajouter/Implémenter
- ✅ Interface pour `stockSupply` (ajouter du stock à un produit)
- ✅ Utilisation de `ProductInStock` dans les listes de produits
- ✅ Utilisation de `productInStockId` dans les ventes
- ✅ Affichage des informations de `ProductInStock` (prix, stock, currency, provider)
- ✅ Gestion des dettes clients (voir `FRONTEND_UPDATE_PROMPT.md`)
- ✅ Système d'inventaire (voir `FRONTEND_UPDATE_PROMPT.md`)

---

## 📚 Documentation Complémentaire

- **Dettes Clients** : Voir `FRONTEND_UPDATE_PROMPT.md` section "Ventes - Gestion des Dettes"
- **Inventaire** : Voir `FRONTEND_UPDATE_PROMPT.md` section "Inventaire - Nouveau Système"
- **Store Currencies** : Déjà implémenté et fonctionnel

---

**Note** : Le document `FRONTEND_UPDATE_PROMPT.md` contient des informations obsolètes concernant les champs `currency` et `providerId` sur `Product`. Ces informations ne sont plus valides avec l'architecture actuelle.

