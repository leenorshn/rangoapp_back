# 📋 Toutes les Requêtes GraphQL pour la Caisse

Ce document liste **toutes les requêtes et mutations GraphQL** disponibles pour gérer la caisse dans l'application RangoApp.

---

## 🔐 Authentification

Toutes les requêtes nécessitent un token JWT dans les headers :
```
Authorization: Bearer <token>
```

---

## 📊 QUERIES (Lecture)

### 1. Vue globale de la caisse (`caisse`)

**Description** : Récupère le solde actuel, les entrées, sorties et le bénéfice total pour un store ou tous les stores accessibles.

**Requête** :
```graphql
query CaisseOverview(
  $storeId: String
  $currency: String
  $period: String
) {
  caisse(storeId: $storeId, currency: $currency, period: $period) {
    currentBalance
    in
    out
    totalBenefice
    currency
    storeId
    store {
      id
      name
      address
      phone
    }
  }
}
```

**Variables possibles** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "jour"
}
```

**Paramètres** :
- `storeId` (optionnel) : ID du store. Si non fourni, agrège tous les stores accessibles
- `currency` (optionnel) : `"USD"`, `"EUR"`, `"XAF"`, `"XOF"`, `"CDF"`. Si non fourni, toutes les devises
- `period` (optionnel) : 
  - `"jour"` : Aujourd'hui
  - `"semaine"` : Cette semaine (lundi à dimanche)
  - `"mois"` : Ce mois
  - `"annee"` : Cette année
  - `null` : Tout l'historique

**Exemples de variables** :

```json
// Caisse du jour en USD pour un store spécifique
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "jour"
}

// Caisse de la semaine en CDF pour tous les stores
{
  "currency": "CDF",
  "period": "semaine"
}

// Caisse du mois sans filtre de devise
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "period": "mois"
}

// Caisse complète (tout l'historique)
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
}
```

---

### 2. Liste des transactions de caisse (`caisseTransactions`)

**Description** : Récupère la liste des transactions (entrées/sorties) avec filtres optionnels.

**Requête** :
```graphql
query CaisseTransactions(
  $storeId: String
  $currency: String
  $period: String
  $limit: Int
) {
  caisseTransactions(
    storeId: $storeId
    currency: $currency
    period: $period
    limit: $limit
  ) {
    id
    amount
    operation
    description
    currency
    date
    storeId
    store {
      id
      name
      address
    }
    createdAt
    updatedAt
  }
}
```

**Variables possibles** :
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "semaine",
  "limit": 50
}
```

**Paramètres** :
- `storeId` (optionnel) : ID du store
- `currency` (optionnel) : Filtre par devise
- `period` (optionnel) : Filtre par période (`"jour"`, `"semaine"`, `"mois"`, `"annee"`)
- `limit` (optionnel) : Nombre maximum de transactions à retourner

**Exemples de variables** :

```json
// 50 dernières transactions du jour en USD
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "jour",
  "limit": 50
}

// Toutes les transactions de la semaine
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "period": "semaine"
}

// 10 dernières transactions toutes devises confondues
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "limit": 10
}
```

---

### 3. Transaction de caisse par ID (`caisseTransaction`)

**Description** : Récupère les détails d'une transaction spécifique.

**Requête** :
```graphql
query CaisseTransactionById($id: ID!) {
  caisseTransaction(id: $id) {
    id
    amount
    operation
    description
    currency
    date
    storeId
    store {
      id
      name
      address
      phone
    }
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

### 4. Rapport détaillé de caisse (`caisseRapport`)

**Description** : Génère un rapport complet avec totaux, bénéfice, solde initial/final, liste des transactions et résumé par jour.

**Requête** :
```graphql
query CaisseRapport(
  $storeId: String
  $currency: String
  $period: String
  $startDate: String
  $endDate: String
) {
  caisseRapport(
    storeId: $storeId
    currency: $currency
    period: $period
    startDate: $startDate
    endDate: $endDate
  ) {
    storeId
    store {
      id
      name
      address
      phone
    }
    currency
    period
    startDate
    endDate
    totalEntrees
    totalSorties
    totalBenefice
    soldeInitial
    soldeFinal
    nombreTransactions
    transactions {
      id
      amount
      operation
      description
      currency
      date
      createdAt
    }
    resumeParJour {
      date
      entrees
      sorties
      benefice
      solde
      nombreTransactions
    }
  }
}
```

**Variables possibles** :

**Option 1 : Utiliser une période prédéfinie**
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "mois"
}
```

**Option 2 : Utiliser des dates personnalisées**
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "CDF",
  "startDate": "2025-12-01",
  "endDate": "2025-12-31"
}
```

**Option 3 : Format RFC3339 pour les dates**
```json
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "startDate": "2025-12-01T00:00:00Z",
  "endDate": "2025-12-31T23:59:59Z"
}
```

**Paramètres** :
- `storeId` (optionnel) : ID du store
- `currency` (optionnel) : Filtre par devise
- `period` (optionnel) : `"jour"`, `"semaine"`, `"mois"`, `"annee"` (ignoré si `startDate`/`endDate` fournis)
- `startDate` (optionnel) : Date de début (format `"YYYY-MM-DD"` ou RFC3339)
- `endDate` (optionnel) : Date de fin (format `"YYYY-MM-DD"` ou RFC3339)

**Note** : Si `startDate` et `endDate` sont fournis, `period` est ignoré.

**Exemples de variables** :

```json
// Rapport du jour en USD
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "jour"
}

// Rapport de la semaine en CDF
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "CDF",
  "period": "semaine"
}

// Rapport du mois de décembre 2025
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "startDate": "2025-12-01",
  "endDate": "2025-12-31"
}

// Rapport de l'année 2025
{
  "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
  "currency": "USD",
  "period": "annee"
}
```

---

## ✏️ MUTATIONS (Écriture)

### 1. Créer une transaction de caisse (`createCaisseTransaction`)

**Description** : Crée une nouvelle transaction manuelle (entrée ou sortie) dans la caisse.

**Requête** :
```graphql
mutation CreateCaisseTransaction($input: CreateCaisseTransactionInput!) {
  createCaisseTransaction(input: $input) {
    id
    amount
    operation
    description
    currency
    date
    storeId
    store {
      id
      name
      address
    }
    createdAt
    updatedAt
  }
}
```

**Variables** :
```json
{
  "input": {
    "amount": 100.0,
    "operation": "Entree",
    "description": "Dépôt initial caisse matin",
    "currency": "USD",
    "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
    "date": "2025-12-01T09:00:00Z"
  }
}
```

**Paramètres du input** :
- `amount` (requis) : Montant de la transaction (doit être > 0)
- `operation` (requis) : `"Entree"` ou `"Sortie"`
- `description` (requis) : Description de la transaction
- `currency` (requis) : `"USD"`, `"EUR"`, `"XAF"`, `"XOF"`, `"CDF"`
- `storeId` (requis) : ID du store
- `date` (optionnel) : Date de la transaction (format RFC3339 ou `"YYYY-MM-DD"`). Si non fourni, utilise la date actuelle

**Exemples de variables** :

```json
// Entrée : Dépôt initial
{
  "input": {
    "amount": 500.0,
    "operation": "Entree",
    "description": "Dépôt initial caisse matin",
    "currency": "USD",
    "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
  }
}

// Sortie : Retrait pour achat
{
  "input": {
    "amount": 200.0,
    "operation": "Sortie",
    "description": "Retrait pour achat fournitures",
    "currency": "USD",
    "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
  }
}

// Entrée : Correction de caisse avec date personnalisée
{
  "input": {
    "amount": 50.0,
    "operation": "Entree",
    "description": "Correction erreur caisse",
    "currency": "CDF",
    "storeId": "65a1b2c3d4e5f6g7h8i9j0k1",
    "date": "2025-12-01"
  }
}

// Sortie : Paiement facture
{
  "input": {
    "amount": 150.0,
    "operation": "Sortie",
    "description": "Paiement facture électricité",
    "currency": "USD",
    "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
  }
}
```

---

### 2. Supprimer une transaction de caisse (`deleteCaisseTransaction`)

**Description** : Supprime une transaction de caisse (utile pour corriger les erreurs).

**Requête** :
```graphql
mutation DeleteCaisseTransaction($id: ID!) {
  deleteCaisseTransaction(id: $id)
}
```

**Variables** :
```json
{
  "id": "65a1b2c3d4e5f6g7h8i9j0k1"
}
```

**Retour** : `true` si la suppression réussit, erreur sinon.

---

## 📝 Notes importantes

### Transactions automatiques

Les transactions de caisse sont créées automatiquement dans les cas suivants :

1. **Lors d'une vente** (`createSale`) :
   - Une transaction `"Entree"` est automatiquement créée avec le montant `pricePayed`
   - Description : `"Vente - Montant reçu: X.XX CURRENCY"`

2. **Lors de la création d'une facture** (`createFacture`) :
   - Une transaction `"Entree"` est automatiquement créée avec le montant de la facture
   - Description : `"Vente facture FACTURE_NUMBER"`

### Calcul du bénéfice

Le bénéfice (`totalBenefice`) est calculé automatiquement à partir des ventes :
- **Formule** : `(Prix de vente - Prix d'achat) × Quantité` pour chaque produit vendu
- Le bénéfice est inclus dans :
  - `caisse.totalBenefice`
  - `caisseRapport.totalBenefice`
  - `caisseRapport.resumeParJour[].benefice`

### Filtres de période

Les périodes sont calculées comme suit :
- **jour** : De 00:00:00 à 23:59:59 du jour actuel
- **semaine** : Du lundi 00:00:00 au dimanche 23:59:59 de la semaine actuelle
- **mois** : Du 1er jour du mois à 00:00:00 au dernier jour à 23:59:59
- **annee** : Du 1er janvier 00:00:00 au 31 décembre 23:59:59

### Formats de date acceptés

- Format RFC3339 : `"2025-12-01T09:00:00Z"`
- Format date simple : `"2025-12-01"` (sera interprété comme 00:00:00 dans le fuseau local)

### Devises supportées

- `USD` : Dollar américain
- `EUR` : Euro
- `XAF` : Franc CFA (BEAC)
- `XOF` : Franc CFA (BCEAO)
- `CDF` : Franc congolais

---

## 🎯 Cas d'usage courants

### 1. Dashboard de caisse (vue du jour)

```graphql
query DashboardCaisse($storeId: String!) {
  caisse(storeId: $storeId, period: "jour", currency: "USD") {
    currentBalance
    in
    out
    totalBenefice
    currency
  }
  
  caisseTransactions(storeId: $storeId, period: "jour", limit: 10) {
    id
    amount
    operation
    description
    date
  }
}
```

### 2. Rapport mensuel complet

```graphql
query RapportMensuel($storeId: String!, $currency: String!) {
  caisseRapport(storeId: $storeId, currency: $currency, period: "mois") {
    totalEntrees
    totalSorties
    totalBenefice
    soldeInitial
    soldeFinal
    resumeParJour {
      date
      entrees
      sorties
      benefice
      solde
    }
  }
}
```

### 3. Historique des transactions avec pagination

```graphql
query HistoriqueTransactions($storeId: String!, $limit: Int!) {
  caisseTransactions(storeId: $storeId, limit: $limit) {
    id
    amount
    operation
    description
    currency
    date
    createdAt
  }
}
```

### 4. Rapport personnalisé (période spécifique)

```graphql
query RapportPersonnalise(
  $storeId: String!
  $startDate: String!
  $endDate: String!
  $currency: String!
) {
  caisseRapport(
    storeId: $storeId
    startDate: $startDate
    endDate: $endDate
    currency: $currency
  ) {
    totalEntrees
    totalSorties
    totalBenefice
    soldeInitial
    soldeFinal
    transactions {
      id
      amount
      operation
      description
      date
    }
  }
}
```

---

## 🔄 Exemples complets avec curl

### Récupérer la caisse du jour

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "query": "query { caisse(storeId: \"65a1b2c3d4e5f6g7h8i9j0k1\", period: \"jour\", currency: \"USD\") { currentBalance in out totalBenefice currency } }"
  }'
```

### Créer une transaction

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "query": "mutation($input: CreateCaisseTransactionInput!) { createCaisseTransaction(input: $input) { id amount operation description } }",
    "variables": {
      "input": {
        "amount": 100.0,
        "operation": "Entree",
        "description": "Dépôt initial",
        "currency": "USD",
        "storeId": "65a1b2c3d4e5f6g7h8i9j0k1"
      }
    }
  }'
```

---

## 📚 Ressources supplémentaires

- Voir `caisse.md` pour plus de détails sur l'utilisation frontend
- Voir `database/caisse_db.go` pour l'implémentation backend
- Voir `graph/schema.graphqls` pour le schéma GraphQL complet

























