# 💳 Système de Crédit Client

## 📋 Vue d'ensemble

Le système de crédit client permet aux magasins d'accorder des lignes de crédit à leurs clients pour effectuer des achats à crédit. Chaque client a une limite de crédit autorisée et peut effectuer des ventes à crédit tant qu'il n'a pas dépassé sa limite.

## 🎯 Fonctionnalités

### Pour les Clients
✅ **Limite de crédit** : Montant maximum autorisé à acheter à crédit  
✅ **Dette actuelle** : Somme des achats à crédit non payés  
✅ **Crédit disponible** : Montant encore disponible pour acheter à crédit  
✅ **Historique** : Toutes les dettes et paiements sont enregistrés  

### Pour les Magasins
✅ **Vérification automatique** : Le système vérifie le crédit disponible avant la vente  
✅ **Gestion flexible** : Les administrateurs peuvent ajuster les limites  
✅ **Paiements partiels** : Les clients peuvent payer progressivement  
✅ **Traçabilité complète** : Historique complet des dettes et paiements  

## 🏗️ Structure des Données

### Client
```graphql
type Client {
  id: ID!
  name: String!
  phone: String!
  storeId: String!
  store: Store!
  creditLimit: Float!        # Limite de crédit autorisée
  currentDebt: Float!        # Dette actuelle (calculée)
  availableCredit: Float!    # Crédit disponible (calculée)
  createdAt: String!
  updatedAt: String!
}
```

**Calculs :**
- `currentDebt` = Somme des dettes avec status "unpaid" ou "partial"
- `availableCredit` = `creditLimit` - `currentDebt`

### Sale (Vente)
```graphql
type Sale {
  # ... autres champs
  paymentType: String!  # "cash", "debt", "advance"
  amountDue: Float!     # Montant dû
  debtStatus: String!   # "paid", "partial", "unpaid", "none"
  debtId: String        # ID de la dette créée
}
```

### Debt (Dette)
```graphql
type Debt {
  id: ID!
  saleId: String!
  clientId: String!
  totalAmount: Float!   # Montant total de la vente
  amountPaid: Float!    # Montant déjà payé
  amountDue: Float!     # Montant restant
  status: String!       # "paid", "partial", "unpaid"
  payments: [DebtPayment!]!
}
```

## 🚀 API GraphQL

### Queries

#### 1. Récupérer un client avec son crédit

```graphql
query {
  client(id: "client123") {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Réponse :**
```json
{
  "data": {
    "client": {
      "id": "client123",
      "name": "Jean Dupont",
      "creditLimit": 10000,
      "currentDebt": 3500,
      "availableCredit": 6500
    }
  }
}
```

#### 2. Liste des clients avec crédit

```graphql
query {
  clients(storeId: "store123") {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

### Mutations

#### 1. Créer un client avec limite de crédit

```graphql
mutation {
  createClient(input: {
    name: "Marie Martin"
    phone: "+243123456789"
    storeId: "store123"
    creditLimit: 5000  # Optionnel, défaut: 0
  }) {
    id
    name
    creditLimit
    availableCredit
  }
}
```

#### 2. Modifier la limite de crédit d'un client

```graphql
mutation {
  updateClientCreditLimit(
    clientId: "client123"
    creditLimit: 15000
  ) {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Permissions :** Seuls les **administrateurs** peuvent modifier les limites de crédit.

#### 3. Créer une vente à crédit

```graphql
mutation {
  createSale(input: {
    basket: [
      {productId: "prod1", quantity: 2, price: 1500}
      {productId: "prod2", quantity: 1, price: 2000}
    ]
    priceToPay: 5000
    pricePayed: 0        # Aucun paiement immédiat
    clientId: "client123"
    storeId: "store123"
    currency: "USD"
    paymentType: "debt"  # Vente à crédit
  }) {
    id
    priceToPay
    pricePayed
    amountDue
    debtStatus
    debtId
  }
}
```

**Validations automatiques :**
- ✅ Vérifie que le client existe
- ✅ Vérifie que le client a assez de crédit disponible
- ✅ Crée automatiquement une dette
- ❌ Refuse la vente si crédit insuffisant

**Erreur si crédit insuffisant :**
```json
{
  "errors": [{
    "message": "Crédit insuffisant. Crédit disponible: 2000.00, Montant requis: 5000.00"
  }]
}
```

#### 4. Payer une dette

```graphql
mutation {
  payDebt(
    debtId: "debt123"
    amount: 2000
    description: "Paiement partiel"
  ) {
    id
    totalAmount
    amountPaid
    amountDue
    status
    payments {
      amount
      createdAt
    }
  }
}
```

## 📊 Workflow Typique

### 1. Création d'un Client avec Crédit

```
1. Admin crée un client avec creditLimit = 10000 USD
2. Le client a maintenant:
   - creditLimit: 10000
   - currentDebt: 0
   - availableCredit: 10000
```

### 2. Première Vente à Crédit

```
Client achète pour 3000 USD à crédit:

1. Vérification: availableCredit (10000) >= montant (3000) ✓
2. Création de la vente avec paymentType = "debt"
3. Création automatique d'une dette de 3000 USD
4. Nouveau solde client:
   - creditLimit: 10000
   - currentDebt: 3000
   - availableCredit: 7000
```

### 3. Deuxième Vente à Crédit

```
Client achète pour 5000 USD à crédit:

1. Vérification: availableCredit (7000) >= montant (5000) ✓
2. Création de la vente et de la dette
3. Nouveau solde:
   - creditLimit: 10000
   - currentDebt: 8000
   - availableCredit: 2000
```

### 4. Tentative de Vente Excédant le Crédit

```
Client tente d'acheter pour 3000 USD:

1. Vérification: availableCredit (2000) < montant (3000) ✗
2. Erreur: "Crédit insuffisant"
3. Vente refusée
```

### 5. Paiement Partiel

```
Client paie 4000 USD:

1. Le paiement est appliqué à la dette la plus ancienne
2. Dette 1: 3000 USD → 0 USD (payée complètement)
3. Dette 2: 5000 USD → 4000 USD (reste 1000 USD)
4. Nouveau solde:
   - creditLimit: 10000
   - currentDebt: 4000
   - availableCredit: 6000
```

### 6. Paiement Complet

```
Client paie les 4000 USD restants:

1. Toutes les dettes sont payées
2. Nouveau solde:
   - creditLimit: 10000
   - currentDebt: 0
   - availableCredit: 10000
```

## 💡 Cas d'Utilisation

### Cas 1 : Nouveau Client Sans Crédit

```graphql
# Créer le client sans crédit
mutation {
  createClient(input: {
    name: "Client Sans Crédit"
    phone: "+243999999999"
    storeId: "store123"
    # creditLimit non spécifié = 0
  }) {
    id
    creditLimit  # = 0
  }
}

# Tentative de vente à crédit
mutation {
  createSale(input: {
    # ...
    paymentType: "debt"
  })
}
# Erreur: "Crédit insuffisant. Crédit disponible: 0.00"
```

### Cas 2 : Client Fidèle avec Grande Limite

```graphql
# Créer un client VIP
mutation {
  createClient(input: {
    name: "Client VIP"
    phone: "+243888888888"
    storeId: "store123"
    creditLimit: 50000  # Grande limite
  }) {
    id
    creditLimit  # = 50000
  }
}

# Peut acheter jusqu'à 50000 USD à crédit
```

### Cas 3 : Augmenter la Limite d'un Client

```graphql
# Client fiable, on augmente sa limite
mutation {
  updateClientCreditLimit(
    clientId: "client123"
    creditLimit: 20000  # Augmentation
  ) {
    id
    name
    creditLimit      # = 20000
    currentDebt      # = 8000 (inchangé)
    availableCredit  # = 12000 (augmenté!)
  }
}
```

### Cas 4 : Réduire la Limite d'un Client

```graphql
# Attention: ne pas réduire sous la dette actuelle!
mutation {
  updateClientCreditLimit(
    clientId: "client123"
    creditLimit: 5000  # Réduction
  ) {
    id
    creditLimit      # = 5000
    currentDebt      # = 8000 (plus que la limite!)
    availableCredit  # = 0 (car dette > limite)
  }
}
# Le client ne peut plus acheter à crédit jusqu'à ce qu'il paie
```

### Cas 5 : Vente avec Paiement Partiel

```graphql
mutation {
  createSale(input: {
    basket: [
      {productId: "prod1", quantity: 1, price: 5000}
    ]
    priceToPay: 5000
    pricePayed: 2000     # Paiement partiel
    clientId: "client123"
    storeId: "store123"
    paymentType: "debt"   # Le reste à crédit
  }) {
    id
    priceToPay       # = 5000
    pricePayed       # = 2000
    amountDue        # = 3000 (à crédit)
    debtStatus       # = "partial"
  }
}
```

## 🔒 Sécurité et Permissions

### Permissions par Rôle

| Action | Admin | User |
|--------|-------|------|
| Créer client avec crédit | ✅ | ✅ |
| Modifier limite de crédit | ✅ | ❌ |
| Vendre à crédit | ✅ | ✅ |
| Consulter dettes | ✅ | ✅ |
| Recevoir paiements | ✅ | ✅ |

### Validations Automatiques

1. **Limite positive** : `creditLimit` ≥ 0
2. **Client requis** : Vente à crédit impossible sans client
3. **Crédit suffisant** : `availableCredit` ≥ `montant de la vente`
4. **Appartenance au store** : Le client doit appartenir au store

## 📈 Rapports et Analyses

### Total des Crédits Accordés

```graphql
query {
  clients(storeId: "store123") {
    id
    name
    creditLimit
    currentDebt
  }
}

# Calculer côté client:
# - Total des limites accordées: sum(creditLimit)
# - Total des dettes actuelles: sum(currentDebt)
# - Taux d'utilisation: sum(currentDebt) / sum(creditLimit)
```

### Clients à Risque

```graphql
query {
  clients(storeId: "store123") {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}

# Identifier côté client:
# - Clients à 90%+: currentDebt / creditLimit >= 0.9
# - Clients au maximum: availableCredit = 0
```

### Clients avec Dette

```graphql
query {
  debts(storeId: "store123", status: "unpaid") {
    id
    client {
      name
      phone
    }
    amountDue
    createdAt
  }
}
```

## 🛠️ Configuration Recommandée

### Limites de Crédit Suggérées

| Type de Client | Limite Suggérée | Usage |
|----------------|-----------------|-------|
| Nouveau | 0 - 1000 USD | Clients non vérifiés |
| Régulier | 5000 - 10000 USD | Clients avec historique |
| VIP | 20000 - 50000 USD | Clients très fidèles |
| Entreprise | 50000+ USD | Partenaires commerciaux |

### Bonnes Pratiques

1. **Commencer prudemment** : Limites basses pour nouveaux clients
2. **Augmenter progressivement** : Basé sur l'historique de paiement
3. **Réviser régulièrement** : Vérifier les limites mensuellement
4. **Politique claire** : Communiquer les conditions de crédit
5. **Suivi rigoureux** : Relancer les clients avec dettes anciennes

## ⚠️ Points d'Attention

### Dette Supérieure à la Limite

Si vous réduisez la limite d'un client sous sa dette actuelle :
- `availableCredit` = 0
- Le client ne peut plus acheter à crédit
- Il doit d'abord réduire sa dette

### Suppression de Client

Vous ne pouvez pas supprimer un client avec des dettes impayées. Options :
1. Attendre que toutes les dettes soient payées
2. Annuler/liquider les dettes manuellement
3. Archiver le client (fonctionnalité future)

### Conversion de Devises

Les limites de crédit sont dans la devise du store. Si le store supporte plusieurs devises, utilisez la devise par défaut pour les limites.

## 🔄 Migration

### Ajouter des Limites aux Clients Existants

Les clients existants auront automatiquement `creditLimit = 0`. Pour leur donner du crédit :

```graphql
mutation {
  updateClientCreditLimit(
    clientId: "existing_client_id"
    creditLimit: 5000
  ) {
    id
    creditLimit
  }
}
```

Ou utilisez un script pour mettre à jour en masse :

```javascript
// Pseudo-code
clients.forEach(client => {
  updateClientCreditLimit(client.id, 5000)
})
```

## 📞 Support

### Questions Fréquentes

**Q: Peut-on avoir des limites de crédit différentes par store ?**  
R: Oui, chaque client est lié à un store spécifique avec sa propre limite.

**Q: Comment gérer les impayés anciens ?**  
R: Utilisez la query `debts` avec un filtre de date et relancez les clients.

**Q: Peut-on avoir une limite de crédit négative ?**  
R: Non, la validation empêche les limites négatives.

**Q: Comment fonctionne le paiement partiel ?**  
R: Le paiement réduit l'`amountDue` de la dette et libère le crédit correspondant.

---

**Version :** 1.0.0  
**Date :** Décembre 2024  
**Statut :** ✅ Production Ready








