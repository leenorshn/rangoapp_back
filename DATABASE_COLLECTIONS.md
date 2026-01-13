# 📊 Collections MongoDB - Base de Données "rangodb"

**Base de données** : `rangodb` (configurable via `MONGO_DB_NAME`, défaut: "rangodb")  
**URI de connexion** : Configurée via `MONGO_URI` (variable d'environnement)

---

## 🔌 Configuration de Connexion

### Paramètres de Connexion
- **MaxPoolSize** : 50 connexions maximum
- **MinPoolSize** : 5 connexions minimum
- **MaxConnIdleTime** : 30 secondes
- **ConnectTimeout** : 10 secondes (configurable via `DB_CONNECT_TIMEOUT_SECONDS`)
- **ServerSelectionTimeout** : 5 secondes
- **SocketTimeout** : 30 secondes
- **HeartbeatInterval** : 10 secondes

### Retry Logic
- **MaxRetries** : 3 tentatives (configurable via `DB_MAX_RETRIES`)
- **InitialDelay** : 2 secondes
- **Backoff** : Exponentiel (2x à chaque tentative)

---

## 📋 Liste Complète des Collections

### 1. **users** - Utilisateurs
**Fichier** : `database/user_db.go`  
**Indexes** :
- `uid` (unique)
- `companyId`
- `storeIds`

**Champs principaux** :
- `_id`, `uid`, `name`, `phone`, `password`, `role`, `companyId`, `storeIds`, `assignedStoreId`, `isBlocked`, `createdAt`, `updatedAt`

---

### 2. **companies** - Entreprises
**Fichier** : `database/company_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `name`, `address`, `phone`, `email`, `description`, `type`, `logo`, `rccm`, `idNat`, `idCommerce`, `createdAt`, `updatedAt`

---

### 3. **stores** - Boutiques
**Fichier** : `database/store_db.go`  
**Indexes** :
- `companyId`

**Champs principaux** :
- `_id`, `name`, `address`, `phone`, `companyId`, `defaultCurrency`, `supportedCurrencies`, `createdAt`, `updatedAt`

---

### 4. **products** - Produits (Templates)
**Fichier** : `database/product_db.go`  
**Indexes** :
- `storeId`
- `storeId + name` (compound)

**Champs principaux** :
- `_id`, `name`, `mark`, `storeId`, `createdAt`, `updatedAt`

**Note** : Ce sont des templates de produits, sans stock ni prix.

---

### 5. **products_in_stock** - Produits en Stock
**Fichier** : `database/product_in_stock_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `productId`, `priceVente`, `priceAchat`, `currency`, `stock`, `storeId`, `providerId`, `createdAt`, `updatedAt`

**Note** : Contient les produits avec stock, prix, currency et fournisseur.

---

### 6. **clients** - Clients
**Fichier** : `database/client_db.go`  
**Indexes** :
- `storeId`

**Champs principaux** :
- `_id`, `name`, `phone`, `storeId`, `creditLimit`, `createdAt`, `updatedAt`

---

### 7. **providers** - Fournisseurs
**Fichier** : `database/provider_db.go`  
**Indexes** :
- `storeId`

**Champs principaux** :
- `_id`, `name`, `phone`, `address`, `storeId`, `createdAt`, `updatedAt`

---

### 8. **factures** - Factures
**Fichier** : `database/facture_db.go`  
**Indexes** :
- `storeId + factureNumber` (compound, unique)
- `storeId`
- `storeId + createdAt` (compound)

**Champs principaux** :
- `_id`, `factureNumber`, `products`, `quantity`, `date`, `price`, `currency`, `clientId`, `storeId`, `createdAt`, `updatedAt`

---

### 9. **rapportStore** - Rapports de Stock
**Fichier** : `database/rapport_store_db.go`  
**Indexes** :
- `storeId`
- `productId`

**Champs principaux** :
- `_id`, `type` (entree/sortie), `productId`, `quantity`, `date`, `storeId`, `createdAt`, `updatedAt`

---

### 10. **trans** - Transactions de Caisse
**Fichier** : `database/caisse_db.go`  
**Indexes** :
- `storeId`
- `currency`
- `date` (descending)
- `operation`
- `storeId + currency + date` (compound)
- `storeId + createdAt` (compound)

**Champs principaux** :
- `_id`, `amount`, `operation` (Entree/Sortie), `description`, `currency`, `storeId`, `operatorId`, `date`, `createdAt`, `updatedAt`

---

### 11. **sales** - Ventes
**Fichier** : `database/sale_db.go`  
**Indexes** :
- `storeId`
- `date` (descending)
- `createdAt` (descending)
- `currency`
- `clientId`
- `storeId + createdAt` (compound)
- `storeId + currency + createdAt` (compound)
- `storeId + date` (compound)

**Champs principaux** :
- `_id`, `basket` (ProductInBasket[]), `priceToPay`, `pricePayed`, `currency`, `clientId`, `operatorId`, `storeId`, `paymentType`, `amountDue`, `debtStatus`, `debtId`, `date`, `createdAt`, `updatedAt`

---

### 12. **subscriptions** - Abonnements
**Fichier** : `database/subscription_db.go`  
**Indexes** :
- `companyId` (unique)
- `status`
- `trialEndDate`
- `subscriptionEndDate`

**Champs principaux** :
- `_id`, `companyId`, `planId`, `status`, `trialEndDate`, `subscriptionEndDate`, `createdAt`, `updatedAt`

---

### 13. **subscription_plans** - Plans d'Abonnement
**Fichier** : `database/subscription_plan_db.go`  
**Indexes** :
- `planId` (unique)
- `isActive`
- `price`

**Champs principaux** :
- `_id`, `planId`, `name`, `description`, `price`, `features`, `isActive`, `createdAt`, `updatedAt`

---

### 14. **debts** - Dettes Clients
**Fichier** : `database/debt_db.go`  
**Indexes** :
- `storeId`
- `status`
- `createdAt` (descending)
- `storeId + status + createdAt` (compound)
- `clientId + storeId` (compound)

**Champs principaux** :
- `_id`, `saleId`, `clientId`, `storeId`, `totalAmount`, `amountPaid`, `amountDue`, `currency`, `status`, `payments`, `createdAt`, `updatedAt`

---

### 15. **provider_debts** - Dettes Fournisseurs
**Fichier** : `database/provider_debt_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `supplyId`, `providerId`, `storeId`, `totalAmount`, `amountPaid`, `amountDue`, `currency`, `status`, `createdAt`, `updatedAt`, `paidAt`

---

### 16. **provider_debt_payments** - Paiements Dettes Fournisseurs
**Fichier** : `database/provider_debt_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `providerDebtId`, `amount`, `currency`, `operatorId`, `storeId`, `description`, `createdAt`

---

### 17. **inventories** - Inventaires
**Fichier** : `database/inventory_db.go`  
**Indexes** :
- `storeId`
- `status`
- `createdAt` (descending)
- `storeId + status + createdAt` (compound)

**Champs principaux** :
- `_id`, `storeId`, `operatorId`, `status` (draft/in_progress/completed/cancelled), `startDate`, `endDate`, `description`, `items` (InventoryItem[]), `totalItems`, `totalValue`, `createdAt`, `updatedAt`

---

### 18. **stock_movements** - Mouvements de Stock
**Fichier** : `database/mouvement_stock_db.go`  
**Indexes** :
- `productId`
- `storeId`
- `type`
- `createdAt` (descending)
- `currency`
- `operatorId`
- `productId + storeId` (compound)
- `storeId + createdAt` (compound)
- `storeId + type + createdAt` (compound)
- `referenceType + referenceId` (compound)

**Champs principaux** :
- `_id`, `productId`, `storeId`, `type` (ENTREE/SORTIE/AJUSTEMENT), `quantity`, `unitPrice`, `totalValue`, `currency`, `operatorId`, `reason`, `referenceType`, `referenceId`, `createdAt`, `updatedAt`

**Note** : Il y a aussi une collection `mouvements_stock` (ancienne) qui peut être utilisée pour la compatibilité.

---

### 19. **stock_supplies** - Approvisionnements
**Fichier** : `database/stock_supply_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `productId`, `productInStockId`, `quantity`, `priceAchat`, `priceVente`, `currency`, `providerId`, `storeId`, `operatorId`, `paymentType`, `providerDebtId`, `date`, `createdAt`, `updatedAt`

---

### 20. **exchange_rate_history** - Historique des Taux de Change
**Fichier** : `database/exchange_rate_history_db.go`  
**Indexes** :
- `companyId + fromCurrency + toCurrency + createdAt` (compound, TTL)

**Champs principaux** :
- `_id`, `companyId`, `fromCurrency`, `toCurrency`, `rate`, `createdAt`

**Note** : Index TTL pour supprimer automatiquement les anciens enregistrements.

---

### 21. **debtPayments** - Paiements Dettes Clients
**Fichier** : `database/debt_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `debtId`, `amount`, `currency`, `operatorId`, `storeId`, `description`, `createdAt`

**Note** : Historique des paiements pour les dettes clients.

---

### 22. **stock** - Stock (Ancienne Collection)
**Fichier** : `database/stock_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `productId`, `storeId`, `quantity`, `createdAt`, `updatedAt`

**Note** : Ancienne collection de stock. Peut être utilisée pour compatibilité ou migration.

---

### 23. **mouvements_stock** - Mouvements de Stock (Ancienne Collection)
**Fichier** : `database/mouvement_stock_db.go`  
**Indexes** : (à vérifier)

**Champs principaux** :
- `_id`, `productId`, `storeId`, `type`, `quantity`, `date`, `createdAt`, `updatedAt`

**Note** : Ancienne collection de mouvements de stock. La nouvelle collection `stock_movements` est recommandée.

---

## 🔗 Relations entre Collections

### Hiérarchie Principale
```
Company
  ├── Stores
  │   ├── Products (templates)
  │   │   └── ProductsInStock
  │   ├── Clients
  │   ├── Providers
  │   ├── Sales
  │   │   └── Debts (client debts)
  │   ├── Factures
  │   ├── StockSupplies
  │   │   └── ProviderDebts
  │   ├── Inventories
  │   ├── StockMovements
  │   └── Transactions (trans)
  └── Subscription
```

### Relations Clés
- **Company → Stores** : `companyId`
- **Store → Products** : `storeId`
- **Product → ProductInStock** : `productId`
- **Store → Clients** : `storeId`
- **Store → Providers** : `storeId`
- **Sale → Debt** : `debtId` (optionnel)
- **StockSupply → ProviderDebt** : `providerDebtId` (optionnel)
- **StockSupply → ProductInStock** : `productInStockId`

---

## 📊 Statistiques des Collections

Pour obtenir des statistiques sur chaque collection, vous pouvez utiliser :

```javascript
// Dans MongoDB shell ou Compass
db.collection_name.stats()
db.collection_name.countDocuments()
```

---

## 🔍 Requêtes Utiles

### Lister toutes les collections
```javascript
db.getCollectionNames()
```

### Compter les documents par collection
```javascript
db.users.countDocuments()
db.stores.countDocuments()
db.products.countDocuments()
db.products_in_stock.countDocuments()
// ... etc
```

### Vérifier les indexes
```javascript
db.collection_name.getIndexes()
```

---

## ⚙️ Configuration

### Variables d'Environnement
- `MONGO_URI` : URI de connexion MongoDB (requis)
- `MONGO_DB_NAME` : Nom de la base de données (défaut: "rangodb")
- `DB_TIMEOUT_SECONDS` : Timeout pour les opérations DB (défaut: 5s)
- `DB_CONNECT_TIMEOUT_SECONDS` : Timeout pour la connexion (défaut: 10s)
- `DB_MAX_RETRIES` : Nombre de tentatives de reconnexion (défaut: 3)

---

## 📝 Notes Importantes

1. **Architecture Product/ProductInStock** :
   - `products` : Templates de produits (nom, marque)
   - `products_in_stock` : Produits avec stock, prix, currency, fournisseur

2. **Dettes** :
   - `debts` : Dettes clients (créées lors de ventes avec `paymentType: "debt"`)
   - `provider_debts` : Dettes fournisseurs (créées lors de `stockSupply` avec `paymentType: "debt"`)

3. **Mouvements de Stock** :
   - `stock_movements` : Nouvelle collection (recommandée)
   - `mouvements_stock` : Ancienne collection (peut être utilisée pour compatibilité)

4. **Stock** :
   - `stock` : Ancienne collection de stock (peut être utilisée pour compatibilité)
   - `products_in_stock` : Nouvelle collection recommandée

5. **Paiements** :
   - `debtPayments` : Paiements des dettes clients
   - `provider_debt_payments` : Paiements des dettes fournisseurs

4. **Indexes** : Tous les indexes sont créés automatiquement au démarrage via `createIndexes()`

---

---

## 📊 Tableau Récapitulatif des Collections

| # | Collection | Fichier Source | Statut | Description |
|---|------------|----------------|--------|-------------|
| 1 | `users` | `user_db.go` | ✅ Actif | Utilisateurs de l'application |
| 2 | `companies` | `company_db.go` | ✅ Actif | Entreprises |
| 3 | `stores` | `store_db.go` | ✅ Actif | Boutiques |
| 4 | `products` | `product_db.go` | ✅ Actif | Templates de produits |
| 5 | `products_in_stock` | `product_in_stock_db.go` | ✅ Actif | Produits avec stock |
| 6 | `clients` | `client_db.go` | ✅ Actif | Clients |
| 7 | `providers` | `provider_db.go` | ✅ Actif | Fournisseurs |
| 8 | `factures` | `facture_db.go` | ✅ Actif | Factures |
| 9 | `rapportStore` | `rapport_store_db.go` | ✅ Actif | Rapports de stock |
| 10 | `trans` | `caisse_db.go` | ✅ Actif | Transactions de caisse |
| 11 | `sales` | `sale_db.go` | ✅ Actif | Ventes |
| 12 | `subscriptions` | `subscription_db.go` | ✅ Actif | Abonnements |
| 13 | `subscription_plans` | `subscription_plan_db.go` | ✅ Actif | Plans d'abonnement |
| 14 | `debts` | `debt_db.go` | ✅ Actif | Dettes clients |
| 15 | `debtPayments` | `debt_db.go` | ✅ Actif | Paiements dettes clients |
| 16 | `provider_debts` | `provider_debt_db.go` | ✅ Actif | Dettes fournisseurs |
| 17 | `provider_debt_payments` | `provider_debt_db.go` | ✅ Actif | Paiements dettes fournisseurs |
| 18 | `inventories` | `inventory_db.go` | ✅ Actif | Inventaires |
| 19 | `stock_movements` | `mouvement_stock_db.go` | ✅ Actif | Mouvements de stock (nouveau) |
| 20 | `stock_supplies` | `stock_supply_db.go` | ✅ Actif | Approvisionnements |
| 21 | `exchange_rate_history` | `exchange_rate_history_db.go` | ✅ Actif | Historique taux de change |
| 22 | `stock` | `stock_db.go` | ⚠️ Ancien | Stock (ancienne collection) |
| 23 | `mouvements_stock` | `mouvement_stock_db.go` | ⚠️ Ancien | Mouvements stock (ancienne) |

**Total** : **23 collections** (21 actives + 2 anciennes pour compatibilité)

---

## 🔍 Requête pour Lister Toutes les Collections

```javascript
// Dans MongoDB shell
use rangodb
db.getCollectionNames().sort()

// Résultat attendu :
[
  "companies",
  "debtPayments",
  "debts",
  "exchange_rate_history",
  "factures",
  "inventories",
  "mouvements_stock",
  "products",
  "products_in_stock",
  "provider_debt_payments",
  "provider_debts",
  "providers",
  "rapportStore",
  "sales",
  "stock",
  "stock_movements",
  "stock_supplies",
  "stores",
  "subscription_plans",
  "subscriptions",
  "trans",
  "users"
]
```

---

**Date de mise à jour** : 28 décembre 2025  
**Version Backend** : Architecture complète avec Product/ProductInStock
