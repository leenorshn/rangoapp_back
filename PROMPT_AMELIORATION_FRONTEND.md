# 🚀 Prompt d'Amélioration Frontend - RangoApp

## Contexte

Le backend de RangoApp a subi une refonte majeure de l'architecture des produits. L'application frontend doit être mise à jour pour s'adapter à ces changements. Ce prompt décrit toutes les modifications nécessaires pour que le frontend fonctionne correctement avec le nouveau backend.

---

## 🎯 Objectif Principal

Adapter le frontend à la nouvelle architecture backend où :
- **`Product`** = Template de produit (nom, marque uniquement)
- **`ProductInStock`** = Produit avec stock, prix, currency, fournisseur

---

## 🔴 PRIORITÉ 1 : Corrections Critiques (À faire immédiatement)

### 1.1 Corriger `CreateProduct` Mutation

**Problème** : Le frontend envoie probablement des champs qui n'existent plus.

**Action requise** :
```typescript
// ❌ SUPPRIMER ces champs du formulaire de création de produit
interface CreateProductInput {
  name: string;
  mark: string;
  storeId: string;
  // ❌ SUPPRIMER :
  // currency?: string;
  // providerId?: string;
  // priceVente?: number;
  // priceAchat?: number;
  // stock?: number;
}

// ✅ Nouveau formulaire simplifié
const createProduct = async (input: {
  name: string;
  mark: string;
  storeId: string;
}) => {
  const mutation = gql`
    mutation CreateProduct($input: CreateProductInput!) {
      createProduct(input: $input) {
        id
        name
        mark
        storeId
      }
    }
  `;
  // ... exécuter la mutation
};
```

**Workflow à implémenter** :
1. Créer le template `Product` avec seulement `name`, `mark`, `storeId`
2. Après création, rediriger vers une page "Ajouter du stock" ou afficher un formulaire `stockSupply`
3. Utiliser la mutation `stockSupply` pour ajouter le stock, prix, currency, fournisseur

---

### 1.2 Corriger `UpdateProduct` Mutation

**Problème** : Le frontend essaie probablement de modifier des champs qui n'existent plus.

**Action requise** :
```typescript
// ❌ SUPPRIMER ces champs du formulaire d'édition
interface UpdateProductInput {
  name?: string;
  mark?: string;
  // ❌ SUPPRIMER :
  // currency?: string;
  // providerId?: string;
}

// ✅ Nouveau formulaire simplifié
const updateProduct = async (id: string, input: {
  name?: string;
  mark?: string;
}) => {
  const mutation = gql`
    mutation UpdateProduct($id: ID!, $input: UpdateProductInput!) {
      updateProduct(id: $id, input: $input) {
        id
        name
        mark
      }
    }
  `;
  // ... exécuter la mutation
};
```

**Note** : Pour modifier le stock/prix d'un produit, il faut utiliser `stockSupply` (ajouter du stock) ou créer une nouvelle interface de gestion de `ProductInStock`.

---

### 1.3 Corriger `CreateSale` Mutation - CRITIQUE

**Problème** : Les ventes utilisent probablement `productId` au lieu de `productInStockId`.

**Action requise** :
```typescript
// ❌ ANCIEN CODE (ne fonctionne plus)
interface SaleProductInput {
  productId: string;  // ❌ SUPPRIMER
  quantity: number;
  price: number;
}

// ✅ NOUVEAU CODE
interface SaleProductInput {
  productInStockId: string;  // ✅ UTILISER ProductInStock ID
  quantity: number;
  price: number;
}

// ✅ Exemple de mutation corrigée
const createSale = async (input: {
  basket: Array<{
    productInStockId: string;  // ✅ Changé de productId
    quantity: number;
    price: number;
  }>;
  priceToPay: number;
  pricePayed: number;
  storeId: string;
  paymentType?: string;
  clientId?: string;
  currency?: string;
}) => {
  const mutation = gql`
    mutation CreateSale($input: CreateSaleInput!) {
      createSale(input: $input) {
        id
        priceToPay
        pricePayed
        currency
        paymentType
        amountDue
        debtStatus
      }
    }
  `;
  // ... exécuter la mutation
};
```

**Impact** : 
- ⚠️ **CRITIQUE** : Les ventes ne fonctionneront pas sans cette correction
- Tous les composants de panier/vente doivent être mis à jour
- Les listes de produits doivent afficher des `ProductInStock`, pas des `Product`

---

## 🟡 PRIORITÉ 2 : Nouvelles Fonctionnalités à Implémenter

### 2.1 Interface `StockSupply` (Ajouter du Stock)

**Nouvelle fonctionnalité requise** : Permettre d'ajouter du stock à un produit existant.

**À implémenter** :
```typescript
// ✅ Nouvelle mutation à utiliser
const stockSupply = async (input: {
  productId: string;        // ID du Product template
  quantity: number;
  priceAchat: number;
  priceVente: number;
  currency?: string;       // Optionnel, utilise defaultCurrency du store
  storeId: string;
  providerId: string;      // Obligatoire
  paymentType: "cash" | "debt";
  amountPaid?: number;      // Obligatoire si paymentType = "debt"
  date?: string;
}) => {
  const mutation = gql`
    mutation StockSupply($input: StockSupplyInput!) {
      stockSupply(input: $input) {
        id
        productInStock {
          id
          stock
          priceVente
          priceAchat
          currency
          provider {
            id
            name
          }
        }
      }
    }
  `;
  // ... exécuter la mutation
};
```

**Interface UI à créer** :
- Formulaire "Ajouter du stock" accessible depuis la page de détail d'un produit
- Champs : quantity, priceAchat, priceVente, currency (select), providerId (select), paymentType (radio), amountPaid (si debt)
- Validation : priceVente >= priceAchat, currency dans supportedCurrencies du store
- Après succès : afficher le nouveau `ProductInStock` créé

---

### 2.2 Utiliser `ProductInStock` dans les Listes de Produits

**Problème** : Les listes affichent probablement des `Product` (sans stock/prix).

**Action requise** :
```typescript
// ❌ ANCIENNE QUERY (ne montre pas le stock)
const GET_PRODUCTS = gql`
  query GetProducts($storeId: String!) {
    products(storeId: $storeId) {
      id
      name
      mark
      # ❌ Pas de stock, prix, currency ici
    }
  }
`;

// ✅ NOUVELLE QUERY (utiliser ProductInStock)
const GET_PRODUCTS_IN_STOCK = gql`
  query GetProductsInStock($storeId: String!) {
    productsInStock(storeId: $storeId) {
      id
      productId
      product {
        id
        name
        mark
      }
      priceVente
      priceAchat
      currency
      stock
      provider {
        id
        name
      }
      storeId
    }
  }
`;
```

**Composants à modifier** :
- Liste des produits : utiliser `productsInStock` au lieu de `products`
- Carte produit : afficher stock, prix, currency, fournisseur
- Filtres : ajouter filtres par currency, fournisseur, stock disponible
- Recherche : rechercher dans `product.name` et `product.mark`

---

### 2.3 Afficher les Informations de `ProductInStock`

**Nouveaux champs à afficher** :
- `stock` : Quantité disponible (afficher en rouge si < 10, en orange si < 50)
- `priceVente` : Prix de vente
- `priceAchat` : Prix d'achat (pour calculer la marge)
- `currency` : Devise (USD, EUR, CDF) avec badge/icône
- `provider` : Nom du fournisseur (lien vers détail fournisseur)

**Exemple de composant** :
```typescript
interface ProductInStockCardProps {
  productInStock: {
    id: string;
    product: { name: string; mark: string };
    priceVente: number;
    priceAchat: number;
    currency: string;
    stock: number;
    provider: { id: string; name: string };
  };
}

const ProductInStockCard = ({ productInStock }: ProductInStockCardProps) => {
  const margin = productInStock.priceVente - productInStock.priceAchat;
  const marginPercent = (margin / productInStock.priceAchat) * 100;
  const isLowStock = productInStock.stock < 10;
  const isOutOfStock = productInStock.stock <= 0;

  return (
    <Card>
      <CardHeader>
        <h3>{productInStock.product.name}</h3>
        <p>{productInStock.product.mark}</p>
      </CardHeader>
      <CardBody>
        <div>
          <span>Stock: </span>
          <Badge color={isOutOfStock ? 'red' : isLowStock ? 'orange' : 'green'}>
            {productInStock.stock}
          </Badge>
        </div>
        <div>
          <span>Prix de vente: </span>
          <strong>{productInStock.priceVente} {productInStock.currency}</strong>
        </div>
        <div>
          <span>Marge: </span>
          <strong>{marginPercent.toFixed(2)}%</strong>
        </div>
        <div>
          <span>Fournisseur: </span>
          <Link to={`/providers/${productInStock.provider.id}`}>
            {productInStock.provider.name}
          </Link>
        </div>
      </CardBody>
      <CardFooter>
        <Button 
          onClick={() => addToCart(productInStock.id)}
          disabled={isOutOfStock}
        >
          Ajouter au panier
        </Button>
      </CardFooter>
    </Card>
  );
};
```

---

## 🟢 PRIORITÉ 3 : Améliorations et Nouvelles Fonctionnalités

### 3.1 Gestion des Currencies du Store

**À implémenter** :
- Afficher `defaultCurrency` et `supportedCurrencies` dans les paramètres du store
- Valider que la currency sélectionnée est dans `supportedCurrencies`
- Afficher un sélecteur de currency lors de la création de `stockSupply`
- Convertir les prix si nécessaire (utiliser les `exchangeRates` de la company)

**Query à utiliser** :
```graphql
query GetStore($id: ID!) {
  store(id: $id) {
    id
    name
    defaultCurrency
    supportedCurrencies
    company {
      exchangeRates {
        fromCurrency
        toCurrency
        rate
      }
    }
  }
}
```

---

### 3.2 Workflow Complet de Création de Produit

**Nouveau workflow à implémenter** :

1. **Étape 1 : Créer le template**
   - Formulaire simple : nom, marque, store
   - Bouton "Créer et ajouter du stock" ou "Créer seulement"

2. **Étape 2 : Ajouter du stock** (si choisi)
   - Formulaire `stockSupply` pré-rempli avec le `productId` créé
   - Champs : quantity, priceAchat, priceVente, currency, provider, paymentType

3. **Étape 3 : Confirmation**
   - Afficher le `Product` créé
   - Afficher le `ProductInStock` créé (si applicable)
   - Bouton "Voir le produit" ou "Ajouter plus de stock"

**Exemple de composant** :
```typescript
const CreateProductWizard = () => {
  const [step, setStep] = useState(1);
  const [productId, setProductId] = useState<string | null>(null);

  if (step === 1) {
    return (
      <CreateProductForm
        onSuccess={(product) => {
          setProductId(product.id);
          setStep(2);
        }}
      />
    );
  }

  if (step === 2) {
    return (
      <StockSupplyForm
        productId={productId!}
        onSuccess={() => {
          setStep(3);
        }}
        onSkip={() => {
          setStep(3);
        }}
      />
    );
  }

  return <ProductCreatedConfirmation productId={productId!} />;
};
```

---

### 3.3 Gestion des Dettes Clients

**Fonctionnalités à implémenter** (voir `FRONTEND_UPDATE_PROMPT.md` pour détails) :
- Page liste des dettes (`debts` query)
- Page détail d'une dette (`debt` query)
- Dettes d'un client (`clientDebts` query)
- Paiement d'une dette (`payDebt` mutation)
- Affichage des dettes dans les détails du client
- Indicateurs visuels pour les ventes avec dettes

---

### 3.4 Système d'Inventaire

**Fonctionnalités à implémenter** (voir `FRONTEND_UPDATE_PROMPT.md` pour détails) :
- Créer un inventaire (`createInventory`)
- Ajouter des produits à l'inventaire (`addInventoryItem`)
- Afficher les écarts (quantité système vs physique)
- Finaliser l'inventaire (`completeInventory`)
- Annuler un inventaire (`cancelInventory`)
- Historique des inventaires

---

## 📋 Checklist de Migration

### Phase 1 : Corrections Critiques (Urgent)
- [ ] Supprimer `currency`, `providerId`, `priceVente`, `priceAchat`, `stock` de `CreateProductInput`
- [ ] Supprimer `currency`, `providerId` de `UpdateProductInput`
- [ ] Remplacer `productId` par `productInStockId` dans `SaleProductInput`
- [ ] Tester que les ventes fonctionnent avec `productInStockId`
- [ ] Mettre à jour tous les composants de panier/vente

### Phase 2 : Nouvelles Fonctionnalités (Important)
- [ ] Créer l'interface `stockSupply` (ajouter du stock)
- [ ] Modifier les listes de produits pour utiliser `ProductInStock`
- [ ] Afficher stock, prix, currency, fournisseur dans les cartes produits
- [ ] Implémenter le workflow de création de produit en 2 étapes
- [ ] Ajouter des indicateurs de stock faible/épuisé

### Phase 3 : Améliorations (Souhaitable)
- [ ] Gestion des currencies du store
- [ ] Conversion de devises
- [ ] Gestion complète des dettes clients
- [ ] Système d'inventaire complet
- [ ] Rapports et statistiques améliorés

---

## 🔍 Tests à Effectuer

### Tests Fonctionnels
1. ✅ Créer un produit (template seulement)
2. ✅ Ajouter du stock à un produit
3. ✅ Créer une vente avec `productInStockId`
4. ✅ Afficher la liste des produits avec stock
5. ✅ Filtrer les produits par currency, fournisseur, stock
6. ✅ Gérer les dettes clients
7. ✅ Effectuer un inventaire

### Tests de Régression
1. ✅ Vérifier que les anciennes ventes fonctionnent toujours
2. ✅ Vérifier que les produits existants s'affichent correctement
3. ✅ Vérifier que les mutations ne cassent pas avec les anciens champs

---

## 📝 Exemples de Code GraphQL

### Query : Obtenir les produits en stock
```graphql
query GetProductsInStock($storeId: String!) {
  productsInStock(storeId: $storeId) {
    id
    productId
    product {
      id
      name
      mark
    }
    priceVente
    priceAchat
    currency
    stock
    provider {
      id
      name
      phone
    }
    store {
      id
      name
      defaultCurrency
    }
    createdAt
    updatedAt
  }
}
```

### Mutation : Créer un produit et ajouter du stock
```graphql
# Étape 1 : Créer le template
mutation CreateProduct($input: CreateProductInput!) {
  createProduct(input: $input) {
    id
    name
    mark
    storeId
  }
}

# Étape 2 : Ajouter du stock
mutation StockSupply($input: StockSupplyInput!) {
  stockSupply(input: $input) {
    id
    productInStock {
      id
      stock
      priceVente
      priceAchat
      currency
      provider {
        id
        name
      }
    }
  }
}
```

### Mutation : Créer une vente
```graphql
mutation CreateSale($input: CreateSaleInput!) {
  createSale(input: $input) {
    id
    priceToPay
    pricePayed
    currency
    paymentType
    amountDue
    debtStatus
    basket {
      productInStock {
        id
        product {
          name
          mark
        }
        priceVente
      }
      quantity
      price
    }
  }
}
```

---

## 🎨 Recommandations UI/UX

1. **Indicateurs visuels** :
   - Stock épuisé : Badge rouge "Épuisé"
   - Stock faible (< 10) : Badge orange "Stock faible"
   - Stock normal : Badge vert avec quantité

2. **Workflow intuitif** :
   - Après création d'un produit, proposer immédiatement d'ajouter du stock
   - Afficher un message si un produit n'a pas de stock lors d'une tentative de vente
   - Permettre d'ajouter du stock directement depuis la page de détail du produit

3. **Validation** :
   - Valider que `priceVente >= priceAchat`
   - Valider que la currency est dans `supportedCurrencies`
   - Valider que le stock est suffisant avant d'ajouter au panier

4. **Feedback utilisateur** :
   - Messages de succès après création de produit
   - Messages d'erreur clairs si validation échoue
   - Confirmations avant actions importantes (finaliser inventaire, etc.)

---

## 📚 Ressources

- **Documentation complète** : `FRONTEND_UPDATE_ACTUEL.md`
- **Dettes clients** : `FRONTEND_UPDATE_PROMPT.md` section "Ventes - Gestion des Dettes"
- **Inventaire** : `FRONTEND_UPDATE_PROMPT.md` section "Inventaire - Nouveau Système"
- **Schéma GraphQL** : `graph/schema.graphqls`

---

## ⚠️ Notes Importantes

1. **Rétrocompatibilité** : Les anciens produits dans la base de données peuvent ne pas avoir de `ProductInStock` associé. Gérer ce cas dans le frontend.

2. **Migration des données** : Si nécessaire, créer des `ProductInStock` pour les anciens produits existants.

3. **Performance** : Les queries `productsInStock` peuvent être plus lourdes que `products`. Implémenter la pagination si nécessaire.

4. **Sécurité** : Valider côté frontend ET backend. Ne jamais faire confiance uniquement au frontend.

---

**Date de création** : 28 décembre 2025  
**Version Backend** : Architecture Product/ProductInStock  
**Priorité** : 🔴 Critique - À faire immédiatement

