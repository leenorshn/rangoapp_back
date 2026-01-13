# Quick Start - Taux de Change

Guide rapide pour utiliser le système de taux de change.

## 🚀 Pour commencer

### 1. Lancer la migration (une seule fois)

```bash
# En développement
export MONGO_URI="mongodb://localhost:27017/rangoapp"
go run scripts/migrate_currency_exchange_rates.go
```

### 2. Tester avec GraphQL

#### Récupérer les taux de votre entreprise

```graphql
query {
  exchangeRates {
    fromCurrency
    toCurrency
    rate
    isDefault
    updatedAt
  }
}
```

**Résultat attendu :**
```json
{
  "data": {
    "exchangeRates": [
      {
        "fromCurrency": "USD",
        "toCurrency": "CDF",
        "rate": 2200,
        "isDefault": true,
        "updatedAt": "2024-12-17T10:30:00Z"
      }
    ]
  }
}
```

#### Convertir 100 USD en CDF

```graphql
query {
  convertCurrency(
    amount: 100
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

**Résultat :**
```json
{
  "data": {
    "convertCurrency": 220000
  }
}
```

#### Mettre à jour le taux (Admin uniquement)

```graphql
mutation {
  updateExchangeRates(rates: [
    {
      fromCurrency: "USD"
      toCurrency: "CDF"
      rate: 2250
    }
  ]) {
    id
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      updatedAt
      updatedBy
    }
  }
}
```

## 💡 Exemples d'Utilisation

### Afficher un prix en deux devises

```graphql
query ProductPrice {
  product(id: "123") {
    name
    priceVente
    currency
  }
  
  priceInCDF: convertCurrency(
    amount: 50  # prix en USD
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

**Frontend (React) :**
```tsx
const { data } = useQuery(PRODUCT_PRICE_QUERY);

return (
  <div>
    <h3>{data.product.name}</h3>
    <p>
      Prix: {data.product.priceVente} {data.product.currency}
      ({data.priceInCDF} CDF)
    </p>
  </div>
);
```

### Rapport de ventes en devise unique

```graphql
query SalesReport($storeId: String!) {
  sales(storeId: $storeId) {
    id
    priceToPay
    currency
  }
}

# Puis convertir chaque montant côté client ou backend
```

### Dashboard multi-stores avec conversion

```graphql
query Dashboard {
  stores {
    id
    name
    defaultCurrency
  }
  
  exchangeRates {
    fromCurrency
    toCurrency
    rate
  }
}

# Utiliser les taux pour afficher tous les montants dans la même devise
```

## 🔧 Code Backend (Go)

### Récupérer un taux

```go
rate, err := db.GetExchangeRate(companyID, "USD", "CDF")
if err != nil {
    return err
}
fmt.Printf("1 USD = %.2f CDF\n", rate)
```

### Convertir un montant

```go
amount := 100.0
converted, err := db.ConvertCurrency(companyID, amount, "USD", "CDF")
if err != nil {
    return err
}
fmt.Printf("%.2f USD = %.2f CDF\n", amount, converted)
```

### Mettre à jour les taux

```go
rates := []database.ExchangeRate{
    {
        FromCurrency: "USD",
        ToCurrency:   "CDF",
        Rate:         2250.0,
    },
}

company, err := db.UpdateExchangeRates(companyID, userID, rates)
if err != nil {
    return err
}
```

## 📱 Frontend (TypeScript/React)

### Hook personnalisé pour la conversion

```typescript
// hooks/useCurrencyConversion.ts
import { useQuery } from '@apollo/client';
import { gql } from '@apollo/client';

const CONVERT_CURRENCY = gql`
  query ConvertCurrency($amount: Float!, $from: String!, $to: String!) {
    convertCurrency(
      amount: $amount
      fromCurrency: $from
      toCurrency: $to
    )
  }
`;

export function useCurrencyConversion(
  amount: number,
  from: string,
  to: string
) {
  const { data, loading, error } = useQuery(CONVERT_CURRENCY, {
    variables: { amount, from, to },
    skip: !amount || from === to,
  });

  return {
    convertedAmount: data?.convertCurrency,
    loading,
    error,
  };
}

// Utilisation
function ProductCard({ product }) {
  const { convertedAmount } = useCurrencyConversion(
    product.price,
    product.currency,
    'CDF'
  );

  return (
    <div>
      <p>Prix: {product.price} {product.currency}</p>
      {convertedAmount && (
        <p className="text-gray-600">
          (~{convertedAmount.toFixed(0)} CDF)
        </p>
      )}
    </div>
  );
}
```

### Composant de gestion des taux (Admin)

```typescript
// components/ExchangeRateManager.tsx
import { useState } from 'react';
import { useMutation, useQuery } from '@apollo/client';

const GET_RATES = gql`
  query {
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      updatedAt
    }
  }
`;

const UPDATE_RATES = gql`
  mutation UpdateRates($rates: [ExchangeRateInput!]!) {
    updateExchangeRates(rates: $rates) {
      id
      exchangeRates {
        rate
        updatedAt
      }
    }
  }
`;

function ExchangeRateManager() {
  const { data } = useQuery(GET_RATES);
  const [updateRates] = useMutation(UPDATE_RATES);
  const [usdToCdf, setUsdToCdf] = useState('');

  const handleUpdate = async () => {
    await updateRates({
      variables: {
        rates: [{
          fromCurrency: 'USD',
          toCurrency: 'CDF',
          rate: parseFloat(usdToCdf),
        }],
      },
    });
  };

  return (
    <div className="p-4">
      <h2>Taux de Change</h2>
      
      {data?.exchangeRates.map(rate => (
        <div key={`${rate.fromCurrency}-${rate.toCurrency}`}>
          <p>
            1 {rate.fromCurrency} = {rate.rate} {rate.toCurrency}
          </p>
          <p className="text-sm text-gray-600">
            Dernière mise à jour: {new Date(rate.updatedAt).toLocaleDateString()}
          </p>
        </div>
      ))}

      <div className="mt-4">
        <label>
          Nouveau taux USD → CDF:
          <input
            type="number"
            value={usdToCdf}
            onChange={e => setUsdToCdf(e.target.value)}
            className="ml-2 border p-2"
          />
        </label>
        <button onClick={handleUpdate} className="ml-2 bg-blue-500 text-white px-4 py-2">
          Mettre à jour
        </button>
      </div>
    </div>
  );
}
```

## 🎯 Cas d'Usage Rapides

### 1. Afficher le total d'une caisse en plusieurs devises

```graphql
query CaisseMultiCurrency($storeId: String!) {
  caisse(storeId: $storeId, currency: "USD") {
    currentBalance
    currency
  }
  
  balanceInCDF: convertCurrency(
    amount: 1500  # balance en USD
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

### 2. Créer une vente avec conversion automatique

```graphql
mutation CreateSaleWithConversion($input: CreateSaleInput!) {
  createSale(input: $input) {
    id
    priceToPay
    currency
  }
  
  # La devise est automatiquement celle du store
  # Vous pouvez ensuite convertir pour l'affichage
}
```

### 3. Rapport consolidé multi-stores

```graphql
query ConsolidatedReport {
  stores {
    id
    name
    defaultCurrency
  }
  
  sales {
    priceToPay
    currency
    storeId
  }
  
  exchangeRates {
    fromCurrency
    toCurrency
    rate
  }
}

# Côté client: convertir tous les montants en une devise de référence
```

## 🔄 Workflow Typique

### Setup Initial (Une fois)
1. ✅ Déployer le code avec le nouveau système
2. ✅ Exécuter le script de migration
3. ✅ Vérifier que toutes les companies ont des taux

### Utilisation Quotidienne
1. Les utilisateurs créent des ventes normalement
2. Le système utilise la devise du store automatiquement
3. Les conversions sont faites à la demande pour l'affichage

### Maintenance Mensuelle (Admin)
1. Vérifier le taux du marché
2. Mettre à jour via GraphQL si nécessaire
3. Les nouveaux taux s'appliquent immédiatement

## 💡 Tips & Tricks

### Arrondir les Montants Convertis

```typescript
// Arrondir à 2 décimales
const rounded = Math.round(convertedAmount * 100) / 100;

// Arrondir au franc près
const roundedCDF = Math.round(convertedAmount);

// Formatter pour l'affichage
const formatted = new Intl.NumberFormat('fr-FR').format(roundedCDF);
```

### Cache des Conversions

```typescript
// Apollo Client cache config
const cache = new InMemoryCache({
  typePolicies: {
    Query: {
      fields: {
        convertCurrency: {
          keyArgs: ['fromCurrency', 'toCurrency'],
          // Cache pendant 5 minutes
        },
      },
    },
  },
});
```

### Validation des Montants

```typescript
function isValidAmount(amount: number): boolean {
  return amount > 0 && Number.isFinite(amount);
}

function isValidCurrency(currency: string): boolean {
  return ['USD', 'CDF', 'EUR'].includes(currency);
}
```

## 📚 Ressources

- **Documentation complète :** `EXCHANGE_RATES.md`
- **Guide de migration :** `MIGRATION_GUIDE.md`
- **Résumé d'implémentation :** `IMPLEMENTATION_SUMMARY.md`
- **Code source :** `database/exchange_rate_db.go`
- **Schema GraphQL :** `graph/schema.graphqls`

## ❓ FAQ

**Q: Puis-je avoir des taux différents par store ?**  
R: Non, les taux sont au niveau de la company. Tous les stores d'une même entreprise utilisent les mêmes taux.

**Q: Comment ajouter une nouvelle devise ?**  
R: Modifiez `isValidCurrency()` dans `database/store_db.go` et ajoutez les taux dans `GetDefaultExchangeRates()`.

**Q: Les transactions passées sont-elles converties avec les nouveaux taux ?**  
R: Non, les transactions gardent leur montant et devise d'origine. Seule l'affichage utilise les taux actuels.

**Q: Que se passe-t-il si je supprime un taux ?**  
R: Le système utilisera le taux par défaut hardcodé.

**Q: Puis-je avoir un historique des taux ?**  
R: Pas dans la version actuelle, mais c'est une évolution possible future.

---

**Besoin d'aide ?** Consultez la documentation complète ou contactez l'équipe technique.










