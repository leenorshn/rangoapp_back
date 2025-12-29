# Système de Gestion des Taux de Change

## 📋 Vue d'ensemble

Le système de gestion des taux de change permet à chaque entreprise de configurer et gérer ses propres taux de conversion entre les devises supportées (USD, CDF, EUR).

## 🎯 Fonctionnalités

### Taux par Défaut

Lors de la création d'une entreprise, les taux suivants sont automatiquement configurés :
- **1 USD = 2200 CDF** (taux par défaut en RDC)

Ces taux peuvent être modifiés à tout moment par un administrateur.

## 🔧 API GraphQL

### Types

```graphql
type ExchangeRate {
  fromCurrency: String!      # Devise source (USD, CDF, EUR)
  toCurrency: String!        # Devise cible (USD, CDF, EUR)
  rate: Float!              # Taux de conversion
  isDefault: Boolean!       # Indique si c'est un taux système par défaut
  updatedAt: String!        # Date de dernière mise à jour
  updatedBy: String!        # ID de l'utilisateur qui a modifié
}
```

### Queries

#### 1. Récupérer les taux de change de l'entreprise

```graphql
query {
  exchangeRates {
    fromCurrency
    toCurrency
    rate
    isDefault
    updatedAt
    updatedBy
  }
}
```

**Réponse exemple :**
```json
{
  "data": {
    "exchangeRates": [
      {
        "fromCurrency": "USD",
        "toCurrency": "CDF",
        "rate": 2200,
        "isDefault": true,
        "updatedAt": "2024-01-15T10:30:00Z",
        "updatedBy": "system"
      }
    ]
  }
}
```

#### 2. Convertir un montant entre deux devises

```graphql
query {
  convertCurrency(
    amount: 100
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

**Réponse :**
```json
{
  "data": {
    "convertCurrency": 220000
  }
}
```

**Cas particuliers :**
- Si `fromCurrency` = `toCurrency`, retourne le montant sans conversion (rate = 1)
- Si le taux inverse existe (ex: CDF->USD quand USD->CDF est configuré), calcule automatiquement : rate = 1/2200 = 0.00045454

#### 3. Récupérer les informations de l'entreprise avec les taux

```graphql
query {
  company {
    id
    name
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      isDefault
      updatedAt
    }
  }
}
```

### Mutations

#### Mettre à jour les taux de change

```graphql
mutation {
  updateExchangeRates(rates: [
    {
      fromCurrency: "USD"
      toCurrency: "CDF"
      rate: 2250
    },
    {
      fromCurrency: "EUR"
      toCurrency: "CDF"
      rate: 2450
    }
  ]) {
    id
    name
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      isDefault
      updatedAt
      updatedBy
    }
  }
}
```

**Permissions :** Seuls les administrateurs peuvent mettre à jour les taux de change.

**Comportement :**
- Les nouveaux taux remplacent les anciens pour la même paire de devises
- Les taux existants pour d'autres paires sont conservés
- Le champ `isDefault` est automatiquement mis à `false` pour les taux personnalisés
- Le champ `updatedBy` contient l'ID de l'utilisateur qui a effectué la modification
- Le champ `updatedAt` est automatiquement mis à jour

## 📊 Cas d'utilisation

### 1. Afficher les prix dans différentes devises

```graphql
query GetProductWithPriceInCDF {
  product(id: "123") {
    id
    name
    priceVente
    currency
  }
  convertCurrency(
    amount: 50  # prix en USD
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

### 2. Générer des rapports multi-devises

Lorsque vous générez un rapport de caisse ou de ventes, vous pouvez convertir tous les montants dans une devise commune pour le calcul des totaux.

```graphql
query {
  sales(storeId: "store123", currency: "USD") {
    id
    priceToPay
    currency
  }
  
  # Convertir le total en CDF
  convertCurrency(
    amount: 1500  # total des ventes en USD
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

### 3. Mettre à jour le taux de change mensuellement

```graphql
mutation UpdateMonthlyRate {
  updateExchangeRates(rates: [
    {
      fromCurrency: "USD"
      toCurrency: "CDF"
      rate: 2300  # nouveau taux du mois
    }
  ]) {
    id
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      updatedAt
    }
  }
}
```

## 🔒 Sécurité et Permissions

- **Lecture des taux** : Tous les utilisateurs authentifiés de l'entreprise
- **Conversion de devise** : Tous les utilisateurs authentifiés de l'entreprise
- **Modification des taux** : Seuls les administrateurs

## 💾 Structure de Données

### Base de données (MongoDB)

Les taux de change sont stockés directement dans le document de l'entreprise :

```json
{
  "_id": "company_id",
  "name": "Mon Entreprise",
  "exchangeRates": [
    {
      "fromCurrency": "USD",
      "toCurrency": "CDF",
      "rate": 2200,
      "isDefault": true,
      "updatedAt": "2024-01-15T10:30:00Z",
      "updatedBy": "system"
    }
  ]
}
```

## 🧪 Tests et Validation

### Validations automatiques

Le système effectue les validations suivantes :
- ✅ Les devises doivent être valides (USD, CDF, EUR)
- ✅ Le taux doit être positif (> 0)
- ✅ Impossible de définir un taux pour la même devise (USD -> USD)
- ✅ Le montant à convertir doit être positif

### Taux système par défaut

Si aucun taux n'est configuré pour une paire de devises, le système utilise les taux par défaut :

```
USD -> CDF : 2200
USD -> EUR : 0.92
EUR -> USD : 1.09
EUR -> CDF : 2400
CDF -> USD : 1/2200 = 0.00045454
CDF -> EUR : 1/2400 = 0.00041666
```

## 🚀 Exemples d'intégration Frontend

### React/TypeScript exemple

```typescript
// Récupérer les taux de change
const { data } = useQuery(gql`
  query {
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      updatedAt
    }
  }
`);

// Convertir un montant
const convertPrice = async (amount: number, from: string, to: string) => {
  const { data } = await client.query({
    query: gql`
      query ConvertPrice($amount: Float!, $from: String!, $to: String!) {
        convertCurrency(amount: $amount, fromCurrency: $from, toCurrency: $to)
      }
    `,
    variables: { amount, from, to }
  });
  return data.convertCurrency;
};

// Mettre à jour les taux (Admin uniquement)
const updateRates = async (rates: ExchangeRateInput[]) => {
  const { data } = await client.mutate({
    mutation: gql`
      mutation UpdateRates($rates: [ExchangeRateInput!]!) {
        updateExchangeRates(rates: $rates) {
          id
          exchangeRates {
            fromCurrency
            toCurrency
            rate
          }
        }
      }
    `,
    variables: { rates }
  });
  return data.updateExchangeRates;
};
```

## 📝 Notes importantes

1. **Historique** : Actuellement, le système ne garde pas d'historique des taux. Seul le taux actuel est stocké.

2. **Conversion inverse** : Le système calcule automatiquement les conversions inverses. Si USD->CDF = 2200, alors CDF->USD = 1/2200.

3. **Transactions existantes** : Les transactions déjà enregistrées conservent leur montant dans la devise d'origine. La conversion n'est appliquée qu'au moment de l'affichage ou des rapports.

4. **Devises supportées** : Actuellement limité à USD, CDF, et EUR. Pour ajouter d'autres devises, modifier la fonction `isValidCurrency` dans `database/store_db.go`.

## 🔄 Migrations

Pour ajouter les taux de change par défaut aux entreprises existantes, un script de migration peut être créé dans `scripts/` si nécessaire.







