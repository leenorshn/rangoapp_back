## Requêtes importantes pour la `Caisse` (Front web Next.js)

Ce fichier liste les principales requêtes/mutations GraphQL liées à la **caisse**, prêtes à être utilisées dans un front Next.js (Apollo Client ou autre).  
Tu peux t’en servir comme *prompt* pour finaliser le menu de caisse (soldes, mouvements, rapports, filtres de période et de monnaie).

---

### 1. Vue globale de la caisse d’un store (solde + entrées/sorties + bénéfice)

**Objectif**: écran récapitulatif de caisse (par jour/semaine/mois/année) avec solde, total entrées, total sorties et bénéfice global.

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
    }
  }
}
```

**Variables possibles**:

```json
{
  "storeId": "STORE_ID",
  "currency": "USD",
  "period": "jour" 
}
```

- **`period`**: `"jour" | "semaine" | "mois" | "annee" | null` (null = tout l’historique)
- **`currency`**: `"USD" | "CDF" | "EUR" | "XAF" | "XOF"`

---

### 2. Liste des mouvements de caisse (entrées / sorties) avec filtres

**Objectif**: écran **historique des mouvements** (tableau) avec possibilité de filtrer par store, monnaie, période et limite.

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
    operation   # "Entree" ou "Sortie"
    description
    currency
    date
    storeId
    store {
      id
      name
    }
    createdAt
  }
}
```

**Variables exemple**:

```json
{
  "storeId": "STORE_ID",
  "currency": "USD",
  "period": "semaine",
  "limit": 50
}
```

---

### 3. Créer un mouvement de caisse manuel (entrée ou sortie)

**Objectif**: écran **“Nouvelle opération de caisse”** (ex: dépôt, retrait, correction de caisse).

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
    }
    createdAt
  }
}
```

**Variables exemple**:

```json
{
  "input": {
    "amount": 100.0,
    "operation": "Entree",
    "description": "Dépot initial caisse matin",
    "currency": "USD",
    "storeId": "STORE_ID",
    "date": "2025-12-01"  // ou "2025-12-01T09:00:00Z"
  }
}
```

> `operation`: `"Entree"` (argent qui entre) ou `"Sortie"` (argent qui sort)

---

### 4. Supprimer un mouvement de caisse

**Objectif**: permettre à un admin de supprimer une opération erronée.

```graphql
mutation DeleteCaisseTransaction($id: ID!) {
  deleteCaisseTransaction(id: $id)
}
```

**Variables**:

```json
{ "id": "CAISSE_TRANSACTION_ID" }
```

---

### 5. Rapport détaillé de caisse (période + monnaie + bénéfice)

**Objectif**: écran de **rapport de caisse** avec:
- totaux entrées/sorties
- **total bénéfice** (calculé à partir des ventes)
- solde initial / final
- liste détaillée des mouvements
- résumé par jour (entrées, sorties, bénéfice du jour, solde)

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

**Scénarios typiques**:

- **Rapport du jour en USD**:

```json
{
  "storeId": "STORE_ID",
  "currency": "USD",
  "period": "jour"
}
```

- **Rapport d’un mois spécifique avec dates custom (et bénéfice)**:

```json
{
  "storeId": "STORE_ID",
  "currency": "CDF",
  "startDate": "2025-12-01",
  "endDate": "2025-12-31"
}
```

---

### 6. Récupérer une transaction de caisse par ID

**Objectif**: écran détail d’un mouvement (ex: pour un modal de détail).

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
    }
    createdAt
    updatedAt
  }
}
```

**Variables**:

```json
{ "id": "CAISSE_TRANSACTION_ID" }
```

---

### 7. Idées d’intégration Next.js (pseudo-code)

**Hook générique pour la caisse (Apollo Client)**:

```ts
// useCaisseOverview.ts
import { useQuery, gql } from "@apollo/client";

const CAISSE_OVERVIEW = gql`
  query CaisseOverview($storeId: String, $currency: String, $period: String) {
    caisse(storeId: $storeId, currency: $currency, period: $period) {
      currentBalance
      in
      out
      totalBenefice
      currency
    }
  }
`;

export function useCaisseOverview(storeId?: string, currency?: string, period?: string) {
  return useQuery(CAISSE_OVERVIEW, {
    variables: { storeId, currency, period },
  });
}
```

Tu peux décliner la même approche pour:
- `caisseTransactions` (liste paginée / filtrée)
- `caisseRapport` (vue “analytics” de la caisse)
- `createCaisseTransaction` (formulaire d’ajout de mouvement)



























