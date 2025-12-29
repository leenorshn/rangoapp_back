# Modifications Backend - Rapports de Stock (Entrées/Sorties)

## 📋 Objectif

Créer un système complet de rapports de stock permettant de suivre et analyser tous les mouvements de stock (entrées et sorties) avec des statistiques détaillées, des filtres avancés et des résumés par période.

## 🔍 État Actuel

### Frontend
- ✅ Système d'inventaire (`/stock/inventory`) - Comparaison stock système vs physique
- ✅ Liste des produits avec stock actuel
- ✅ Système de rapports pour la caisse (entrées/sorties de caisse)
- ❌ **Manquant** : Page de rapports de stock (`/stock/rapports`)
- ❌ **Manquant** : Historique des mouvements de stock
- ❌ **Manquant** : Statistiques d'entrées/sorties de stock

### Backend
- ✅ Système de produits avec gestion du stock
- ✅ Système d'inventaire
- ❌ **Manquant** : Query pour les rapports de stock
- ❌ **Manquant** : Suivi des mouvements de stock (entrées/sorties)
- ❌ **Manquant** : Historique des transactions de stock

## ✅ Modifications Requises

### 1. Ajouter un Type GraphQL pour les Mouvements de Stock

**Type à créer dans le schéma GraphQL :**

```graphql
type StockMovement {
  id: ID!
  productId: ID!
  product: Product!
  storeId: ID!
  store: Store!
  type: StockMovementType!  # "ENTREE" | "SORTIE" | "AJUSTEMENT"
  quantity: Float!
  unitPrice: Float!
  totalValue: Float!
  currency: String!
  reason: String
  reference: String  # Référence à une vente, achat, inventaire, etc.
  referenceType: String  # "SALE", "PURCHASE", "INVENTORY", "ADJUSTMENT", "TRANSFER"
  referenceId: ID
  operatorId: ID!
  operator: User!
  createdAt: String!
  updatedAt: String!
}

enum StockMovementType {
  ENTREE
  SORTIE
  AJUSTEMENT
}
```

### 2. Ajouter un Type GraphQL pour les Rapports de Stock

**Type à créer :**

```graphql
type StockReport {
  storeId: ID!
  store: Store!
  currency: String!
  period: String!  # "day", "week", "month", "year", "custom"
  startDate: String!
  endDate: String!
  
  # Totaux généraux
  totalEntrees: Float!
  totalSorties: Float!
  totalAjustements: Float!
  soldeInitial: Float!
  soldeFinal: Float!
  nombreMouvements: Int!
  
  # Détails par produit
  mouvementsParProduit: [StockMovementByProduct!]!
  
  # Résumé par jour
  resumeParJour: [StockReportResumeJour!]!
  
  # Liste complète des mouvements
  mouvements: [StockMovement!]!
}

type StockMovementByProduct {
  productId: ID!
  product: Product!
  totalEntrees: Float!
  totalSorties: Float!
  totalAjustements: Float!
  soldeInitial: Float!
  soldeFinal: Float!
  nombreMouvements: Int!
  valeurTotaleEntrees: Float!
  valeurTotaleSorties: Float!
}

type StockReportResumeJour {
  date: String!
  entrees: Float!
  sorties: Float!
  ajustements: Float!
  solde: Float!
  nombreMouvements: Int!
  valeurTotaleEntrees: Float!
  valeurTotaleSorties: Float!
}
```

### 3. Ajouter les Queries GraphQL

**Queries à ajouter :**

```graphql
type Query {
  # ... autres queries existantes
  
  # Récupérer le rapport de stock
  stockReport(
    storeId: String
    productId: String
    currency: String
    period: String  # "day", "week", "month", "year", "custom"
    startDate: String
    endDate: String
    type: StockMovementType  # Filtrer par type de mouvement
  ): StockReport!
  
  # Récupérer l'historique des mouvements de stock
  stockMovements(
    storeId: String
    productId: String
    type: StockMovementType
    startDate: String
    endDate: String
    limit: Int
    offset: Int
  ): [StockMovement!]!
  
  # Statistiques de stock
  stockStats(
    storeId: String
    productId: String
    period: String
    startDate: String
    endDate: String
  ): StockStats!
}

type StockStats {
  totalProducts: Int!
  totalValue: Float!
  productsLowStock: Int!  # Produits en stock faible (< seuil)
  productsOutOfStock: Int!  # Produits en rupture
  totalEntrees: Float!
  totalSorties: Float!
  topProductsByMovements: [ProductMovementStats!]!
}

type ProductMovementStats {
  product: Product!
  totalEntrees: Float!
  totalSorties: Float!
  nombreMouvements: Int!
}
```

### 4. Structure de Données Attendue

**Exemple de réponse pour `stockReport` :**

```json
{
  "data": {
    "stockReport": {
      "storeId": "store-123",
      "store": {
        "id": "store-123",
        "name": "Boutique Principale"
      },
      "currency": "USD",
      "period": "month",
      "startDate": "2024-12-01",
      "endDate": "2024-12-31",
      "totalEntrees": 1500.50,
      "totalSorties": 850.25,
      "totalAjustements": 50.00,
      "soldeInitial": 5000.00,
      "soldeFinal": 5700.25,
      "nombreMouvements": 45,
      "mouvementsParProduit": [
        {
          "productId": "prod-1",
          "product": {
            "id": "prod-1",
            "name": "Produit A",
            "mark": "Marque X"
          },
          "totalEntrees": 100.0,
          "totalSorties": 50.0,
          "totalAjustements": 5.0,
          "soldeInitial": 20.0,
          "soldeFinal": 75.0,
          "nombreMouvements": 8,
          "valeurTotaleEntrees": 1000.00,
          "valeurTotaleSorties": 500.00
        }
      ],
      "resumeParJour": [
        {
          "date": "2024-12-01",
          "entrees": 150.0,
          "sorties": 80.0,
          "ajustements": 10.0,
          "solde": 5080.0,
          "nombreMouvements": 5,
          "valeurTotaleEntrees": 1500.00,
          "valeurTotaleSorties": 800.00
        }
      ],
      "mouvements": [
        {
          "id": "movement-1",
          "productId": "prod-1",
          "product": { "id": "prod-1", "name": "Produit A" },
          "type": "ENTREE",
          "quantity": 10.0,
          "unitPrice": 10.00,
          "totalValue": 100.00,
          "currency": "USD",
          "reason": "Achat fournisseur",
          "reference": "purchase-123",
          "referenceType": "PURCHASE",
          "referenceId": "purchase-123",
          "operatorId": "user-1",
          "operator": { "id": "user-1", "name": "John Doe" },
          "createdAt": "2024-12-01T10:00:00Z"
        }
      ]
    }
  }
}
```

### 5. Implémentation Backend

#### 5.1 Modèle de Données (Base de données)

**Table `stock_movements` :**

```sql
CREATE TABLE stock_movements (
  id VARCHAR(50) PRIMARY KEY,
  product_id VARCHAR(50) NOT NULL,
  store_id VARCHAR(50) NOT NULL,
  type ENUM('ENTREE', 'SORTIE', 'AJUSTEMENT') NOT NULL,
  quantity DECIMAL(10, 2) NOT NULL,
  unit_price DECIMAL(10, 2) NOT NULL,
  total_value DECIMAL(10, 2) NOT NULL,
  currency VARCHAR(3) NOT NULL DEFAULT 'USD',
  reason TEXT,
  reference VARCHAR(100),  -- Référence externe (ID de vente, achat, etc.)
  reference_type VARCHAR(50),  -- "SALE", "PURCHASE", "INVENTORY", "ADJUSTMENT", "TRANSFER"
  reference_id VARCHAR(50),
  operator_id VARCHAR(50) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  
  FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
  FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
  FOREIGN KEY (operator_id) REFERENCES users(id) ON DELETE RESTRICT,
  
  INDEX idx_product_store (product_id, store_id),
  INDEX idx_store_date (store_id, created_at),
  INDEX idx_type (type),
  INDEX idx_reference (reference_type, reference_id),
  INDEX idx_operator (operator_id),
  INDEX idx_created_at (created_at)
);
```

#### 5.2 Triggers pour Enregistrement Automatique

**Créer des triggers pour enregistrer automatiquement les mouvements :**

```sql
-- Trigger après création d'une vente (SORTIE)
DELIMITER $$
CREATE TRIGGER after_sale_created
AFTER INSERT ON sale_items
FOR EACH ROW
BEGIN
  INSERT INTO stock_movements (
    id, product_id, store_id, type, quantity, unit_price, 
    total_value, currency, reason, reference, reference_type, reference_id, operator_id
  ) VALUES (
    CONCAT('mov-', UUID()),
    NEW.product_id,
    NEW.store_id,
    'SORTIE',
    NEW.quantity,
    NEW.unit_price,
    NEW.quantity * NEW.unit_price,
    NEW.currency,
    CONCAT('Vente #', NEW.sale_id),
    CONCAT('sale-', NEW.sale_id),
    'SALE',
    NEW.sale_id,
    (SELECT operator_id FROM sales WHERE id = NEW.sale_id)
  );
END$$
DELIMITER ;

-- Trigger après création d'un produit (ENTREE initiale)
-- Trigger après achat fournisseur (ENTREE)
-- Trigger après ajustement d'inventaire (AJUSTEMENT)
-- etc.
```

#### 5.3 Resolver GraphQL

**Exemple d'implémentation (Go/Gin) :**

```go
// Resolver pour le rapport de stock
func (r *queryResolver) StockReport(ctx context.Context, args struct {
    StoreID   *string
    ProductID *string
    Currency  *string
    Period    *string
    StartDate *string
    EndDate   *string
    Type      *model.StockMovementType
}) (*model.StockReport, error) {
    // Récupérer les mouvements selon les filtres
    movements, err := r.db.GetStockMovements(ctx, args)
    if err != nil {
        return nil, err
    }
    
    // Calculer les totaux
    report := &model.StockReport{
        TotalEntrees:      calculateTotalEntrees(movements),
        TotalSorties:      calculateTotalSorties(movements),
        TotalAjustements:  calculateTotalAjustements(movements),
        NombreMouvements:  len(movements),
        Mouvements:        movements,
    }
    
    // Calculer le résumé par jour
    report.ResumeParJour = calculateResumeParJour(movements)
    
    // Calculer par produit
    report.MouvementsParProduit = calculateParProduit(movements)
    
    return report, nil
}

// Resolver pour l'historique des mouvements
func (r *queryResolver) StockMovements(ctx context.Context, args struct {
    StoreID   *string
    ProductID *string
    Type      *model.StockMovementType
    StartDate *string
    EndDate   *string
    Limit     *int
    Offset    *int
}) ([]*model.StockMovement, error) {
    return r.db.GetStockMovements(ctx, args)
}
```

**Exemple d'implémentation (Node.js/Express) :**

```typescript
const resolvers = {
  Query: {
    stockReport: async (_, args) => {
      const movements = await db.stockMovements.findMany({
        where: {
          storeId: args.storeId || undefined,
          productId: args.productId || undefined,
          type: args.type || undefined,
          createdAt: {
            gte: args.startDate ? new Date(args.startDate) : undefined,
            lte: args.endDate ? new Date(args.endDate) : undefined,
          },
        },
        include: {
          product: true,
          store: true,
          operator: true,
        },
        orderBy: { createdAt: 'desc' },
      });
      
      return calculateStockReport(movements, args);
    },
    
    stockMovements: async (_, args) => {
      return await db.stockMovements.findMany({
        where: {
          storeId: args.storeId || undefined,
          productId: args.productId || undefined,
          type: args.type || undefined,
          createdAt: {
            gte: args.startDate ? new Date(args.startDate) : undefined,
            lte: args.endDate ? new Date(args.endDate) : undefined,
          },
        },
        include: {
          product: true,
          store: true,
          operator: true,
        },
        orderBy: { createdAt: 'desc' },
        take: args.limit || 100,
        skip: args.offset || 0,
      });
    },
    
    stockStats: async (_, args) => {
      // Calculer les statistiques globales
      const stats = await calculateStockStats(args);
      return stats;
    },
  },
};
```

### 6. Types de Mouvements de Stock

**Types de mouvements à enregistrer :**

1. **ENTREE** :
   - Achat fournisseur
   - Retour client
   - Ajustement positif (inventaire)
   - Transfert entrant (entre boutiques)
   - Production/Assemblage

2. **SORTIE** :
   - Vente client
   - Retour fournisseur
   - Ajustement négatif (inventaire)
   - Transfert sortant (entre boutiques)
   - Perte/Casse
   - Utilisation interne

3. **AJUSTEMENT** :
   - Correction d'inventaire
   - Correction d'erreur
   - Expiration/Perte de qualité

### 7. Intégration avec les Modules Existants

**Points d'intégration :**

1. **Module Ventes** :
   - Enregistrer automatiquement une SORTIE lors de la création d'une vente
   - Référencer la vente dans `reference_id` et `reference_type = "SALE"`

2. **Module Produits** :
   - Enregistrer une ENTREE lors de la création d'un produit avec stock initial
   - Enregistrer les ajustements lors de la modification du stock

3. **Module Inventaire** :
   - Enregistrer des AJUSTEMENTS lors de la finalisation d'un inventaire
   - Référencer l'inventaire dans `reference_id` et `reference_type = "INVENTORY"`

4. **Module Fournisseurs** (si achat implémenté) :
   - Enregistrer une ENTREE lors d'un achat fournisseur
   - Référencer l'achat dans `reference_id` et `reference_type = "PURCHASE"`

## 📝 Checklist d'Implémentation

### Backend
- [ ] Créer le type GraphQL `StockMovement`
- [ ] Créer le type GraphQL `StockReport`
- [ ] Créer le type GraphQL `StockStats`
- [ ] Créer la table `stock_movements` en base de données
- [ ] Créer les index pour optimiser les requêtes
- [ ] Implémenter le resolver `stockReport`
- [ ] Implémenter le resolver `stockMovements`
- [ ] Implémenter le resolver `stockStats`
- [ ] Créer les triggers pour enregistrement automatique des mouvements
- [ ] Intégrer avec le module Ventes (enregistrer sorties)
- [ ] Intégrer avec le module Inventaire (enregistrer ajustements)
- [ ] Intégrer avec le module Produits (enregistrer entrées initiales)
- [ ] Ajouter la gestion des erreurs
- [ ] Ajouter la validation des données
- [ ] Tester les queries GraphQL
- [ ] Optimiser les performances (cache, pagination)

### Frontend (à faire après le backend)
- [ ] Créer la query GraphQL `STOCK_REPORT_QUERY`
- [ ] Créer la query GraphQL `STOCK_MOVEMENTS_QUERY`
- [ ] Créer la query GraphQL `STOCK_STATS_QUERY`
- [ ] Créer les types TypeScript correspondants
- [ ] Créer la page `/stock/rapports`
- [ ] Implémenter les filtres (période, produit, type, devise)
- [ ] Afficher le résumé général (totaux, statistiques)
- [ ] Afficher le résumé par jour
- [ ] Afficher les mouvements par produit
- [ ] Afficher l'historique détaillé des mouvements
- [ ] Ajouter l'export PDF/Excel
- [ ] Gérer les états de chargement et d'erreur

## 🎯 Fonctionnalités Recommandées pour la Page Frontend

### Vue d'Ensemble
- **Statistiques globales** :
  - Total entrées (quantité + valeur)
  - Total sorties (quantité + valeur)
  - Total ajustements
  - Solde initial et final
  - Nombre de mouvements

### Filtres
- **Période** : Jour, Semaine, Mois, Année, Personnalisé
- **Produit** : Sélection d'un produit spécifique
- **Type de mouvement** : Entrée, Sortie, Ajustement, Tous
- **Devise** : USD, EUR, CDF
- **Boutique** : Si multi-boutiques

### Tableaux et Graphiques
- **Résumé par jour** : Graphique linéaire ou barres
- **Mouvements par produit** : Tableau avec totaux
- **Historique détaillé** : Liste complète avec pagination
- **Top produits** : Produits avec le plus de mouvements

### Export
- Export PDF du rapport
- Export Excel des données
- Impression du rapport

## 🔄 Migration des Données Existantes

Si vous avez déjà des données historiques (ventes, inventaires), vous devrez :

1. **Créer les mouvements rétroactifs** :
   - Parcourir toutes les ventes historiques et créer des mouvements SORTIE
   - Parcourir tous les inventaires et créer des mouvements AJUSTEMENT
   - Parcourir tous les produits et créer des mouvements ENTREE pour le stock initial

2. **Script de migration** :
```sql
-- Exemple : Créer des mouvements à partir des ventes existantes
INSERT INTO stock_movements (
  id, product_id, store_id, type, quantity, unit_price, 
  total_value, currency, reason, reference, reference_type, reference_id, operator_id, created_at
)
SELECT 
  CONCAT('mov-', UUID()),
  si.product_id,
  s.store_id,
  'SORTIE',
  si.quantity,
  si.unit_price,
  si.quantity * si.unit_price,
  si.currency,
  CONCAT('Vente #', s.id),
  CONCAT('sale-', s.id),
  'SALE',
  s.id,
  s.operator_id,
  s.created_at
FROM sale_items si
JOIN sales s ON si.sale_id = s.id
WHERE s.created_at < NOW();  -- Seulement les ventes passées
```

## 🧪 Tests à Effectuer

1. **Query `stockReport`**
   - Tester avec différents filtres (période, produit, type)
   - Vérifier les calculs de totaux
   - Vérifier le résumé par jour
   - Vérifier les mouvements par produit

2. **Query `stockMovements`**
   - Tester la pagination
   - Tester les filtres
   - Vérifier l'ordre chronologique

3. **Triggers automatiques**
   - Vérifier qu'une vente crée bien un mouvement SORTIE
   - Vérifier qu'un inventaire crée bien des mouvements AJUSTEMENT
   - Vérifier qu'un produit créé crée bien un mouvement ENTREE

4. **Performance**
   - Vérifier les temps de réponse avec beaucoup de données
   - Vérifier l'utilisation des index
   - Tester avec des périodes longues (1 an+)

## 📚 Exemple de Query GraphQL Complète

```graphql
query StockReport {
  stockReport(
    storeId: "store-123"
    period: "month"
    startDate: "2024-12-01"
    endDate: "2024-12-31"
    currency: "USD"
  ) {
    store {
      id
      name
    }
    totalEntrees
    totalSorties
    totalAjustements
    soldeInitial
    soldeFinal
    nombreMouvements
    resumeParJour {
      date
      entrees
      sorties
      ajustements
      solde
      nombreMouvements
    }
    mouvementsParProduit {
      product {
        id
        name
        mark
      }
      totalEntrees
      totalSorties
      soldeFinal
      valeurTotaleEntrees
      valeurTotaleSorties
    }
    mouvements(limit: 10) {
      id
      type
      quantity
      unitPrice
      totalValue
      reason
      product {
        name
      }
      operator {
        name
      }
      createdAt
    }
  }
}
```

## 🎯 Avantages de cette Approche

1. **Traçabilité complète** : Tous les mouvements de stock sont enregistrés
2. **Audit** : Possibilité de retracer l'origine de chaque mouvement
3. **Analyse** : Statistiques détaillées pour la prise de décision
4. **Conformité** : Respect des normes de gestion de stock
5. **Automatisation** : Enregistrement automatique via triggers
6. **Flexibilité** : Filtres avancés pour analyses spécifiques

## ⚠️ Notes Importantes

- Les mouvements doivent être **immutables** (non modifiables après création)
- En cas d'erreur, créer un mouvement de correction plutôt que de modifier
- Considérer l'ajout d'un champ `cancelled` pour annuler un mouvement si nécessaire
- Penser à la gestion des devises multiples si applicable
- Considérer l'ajout d'un cache pour les rapports fréquemment consultés
- Penser à la purge des anciennes données (archivage après X années)

## 🔗 Intégration avec le Frontend

Une fois le backend implémenté, le frontend pourra :

1. Afficher un tableau de bord avec les statistiques de stock
2. Filtrer les mouvements par période, produit, type
3. Visualiser les tendances avec des graphiques
4. Exporter les rapports en PDF/Excel
5. Recevoir des alertes pour les produits en rupture ou stock faible
