# Guide d'Intégration Frontend - Système de Taux de Change

## 📋 Vue d'ensemble

Ce document fournit toutes les requêtes GraphQL nécessaires pour intégrer le système de gestion des taux de change dans votre application Next.js. Il couvre les queries, mutations, types TypeScript et exemples d'utilisation.

---

## 🔧 Types TypeScript / GraphQL

### Types de base

```typescript
// Types pour les taux de change
export type ExchangeRate = {
  fromCurrency: string;
  toCurrency: string;
  rate: number;
  isDefault: boolean;
  updatedAt: string;
  updatedBy: string;
};

// Input pour mettre à jour les taux
export type ExchangeRateInput = {
  fromCurrency: string;
  toCurrency: string;
  rate: number;
};

// Type pour la réponse de conversion
export type ConvertCurrencyResponse = {
  convertCurrency: number;
};

// Type pour la réponse de mise à jour
export type UpdateExchangeRatesResponse = {
  updateExchangeRates: {
    id: string;
    name: string;
    exchangeRates: ExchangeRate[];
    // ... autres champs de Company
  };
};
```

---

## 📥 QUERIES

### 1. Récupérer les taux de change de l'entreprise

**Query GraphQL :**

```graphql
query GetExchangeRates {
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

**Exemple d'utilisation avec Apollo Client / urql :**

```typescript
import { useQuery, gql } from '@apollo/client';

const GET_EXCHANGE_RATES = gql`
  query GetExchangeRates {
    exchangeRates {
      fromCurrency
      toCurrency
      rate
      isDefault
      updatedAt
      updatedBy
    }
  }
`;

// Hook personnalisé
export function useExchangeRates() {
  const { data, loading, error, refetch } = useQuery<{
    exchangeRates: ExchangeRate[];
  }>(GET_EXCHANGE_RATES);

  return {
    rates: data?.exchangeRates || [],
    loading,
    error,
    refetch,
  };
}
```

**Exemple avec fetch (sans bibliothèque GraphQL) :**

```typescript
async function fetchExchangeRates(token: string): Promise<ExchangeRate[]> {
  const response = await fetch('YOUR_GRAPHQL_ENDPOINT', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      query: `
        query GetExchangeRates {
          exchangeRates {
            fromCurrency
            toCurrency
            rate
            isDefault
            updatedAt
            updatedBy
          }
        }
      `,
    }),
  });

  const result = await response.json();
  if (result.errors) {
    throw new Error(result.errors[0].message);
  }
  return result.data.exchangeRates;
}
```

---

### 2. Convertir un montant entre devises

**Query GraphQL :**

```graphql
query ConvertCurrency(
  $amount: Float!
  $fromCurrency: String!
  $toCurrency: String!
) {
  convertCurrency(
    amount: $amount
    fromCurrency: $fromCurrency
    toCurrency: $toCurrency
  )
}
```

**Exemple d'utilisation :**

```typescript
import { useLazyQuery, gql } from '@apollo/client';

const CONVERT_CURRENCY = gql`
  query ConvertCurrency(
    $amount: Float!
    $fromCurrency: String!
    $toCurrency: String!
  ) {
    convertCurrency(
      amount: $amount
      fromCurrency: $fromCurrency
      toCurrency: $toCurrency
    )
  }
`;

// Hook personnalisé pour la conversion
export function useCurrencyConverter() {
  const [convert, { data, loading, error }] = useLazyQuery<{
    convertCurrency: number;
  }>(CONVERT_CURRENCY);

  const convertAmount = useCallback(
    async (amount: number, from: string, to: string) => {
      if (amount <= 0) {
        throw new Error('Amount must be positive');
      }
      if (from === to) {
        return amount; // Pas de conversion nécessaire
      }

      await convert({
        variables: {
          amount,
          fromCurrency: from,
          toCurrency: to,
        },
      });

      return data?.convertCurrency || 0;
    },
    [convert, data]
  );

  return {
    convertAmount,
    convertedAmount: data?.convertCurrency,
    loading,
    error,
  };
}
```

**Exemple avec fetch :**

```typescript
async function convertCurrency(
  token: string,
  amount: number,
  fromCurrency: string,
  toCurrency: string
): Promise<number> {
  // Cas spécial : même devise
  if (fromCurrency === toCurrency) {
    return amount;
  }

  const response = await fetch('YOUR_GRAPHQL_ENDPOINT', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      query: `
        query ConvertCurrency(
          $amount: Float!
          $fromCurrency: String!
          $toCurrency: String!
        ) {
          convertCurrency(
            amount: $amount
            fromCurrency: $fromCurrency
            toCurrency: $toCurrency
          )
        }
      `,
      variables: {
        amount,
        fromCurrency,
        toCurrency,
      },
    }),
  });

  const result = await response.json();
  if (result.errors) {
    throw new Error(result.errors[0].message);
  }
  return result.data.convertCurrency;
}
```

---

### 3. Récupérer les taux via la query `company`

**Query GraphQL :**

```graphql
query GetCompanyWithExchangeRates {
  company {
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

**Exemple d'utilisation :**

```typescript
import { useQuery, gql } from '@apollo/client';

const GET_COMPANY = gql`
  query GetCompanyWithExchangeRates {
    company {
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
`;

export function useCompany() {
  const { data, loading, error } = useQuery<{
    company: {
      id: string;
      name: string;
      exchangeRates: ExchangeRate[];
    };
  }>(GET_COMPANY);

  return {
    company: data?.company,
    exchangeRates: data?.company?.exchangeRates || [],
    loading,
    error,
  };
}
```

---

## 📤 MUTATIONS

### 1. Mettre à jour les taux de change

**Mutation GraphQL :**

```graphql
mutation UpdateExchangeRates($rates: [ExchangeRateInput!]!) {
  updateExchangeRates(rates: $rates) {
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

**Exemple d'utilisation :**

```typescript
import { useMutation, gql } from '@apollo/client';

const UPDATE_EXCHANGE_RATES = gql`
  mutation UpdateExchangeRates($rates: [ExchangeRateInput!]!) {
    updateExchangeRates(rates: $rates) {
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
`;

// Hook personnalisé
export function useUpdateExchangeRates() {
  const [updateRates, { data, loading, error }] = useMutation<
    UpdateExchangeRatesResponse
  >(UPDATE_EXCHANGE_RATES, {
    // Refetch les taux après la mise à jour
    refetchQueries: ['GetExchangeRates', 'GetCompanyWithExchangeRates'],
  });

  const update = useCallback(
    async (rates: ExchangeRateInput[]) => {
      // Validation côté client
      if (rates.length === 0) {
        throw new Error('At least one exchange rate is required');
      }

      for (const rate of rates) {
        if (rate.fromCurrency === rate.toCurrency) {
          throw new Error(
            `Cannot set exchange rate for same currency: ${rate.fromCurrency}`
          );
        }
        if (rate.rate <= 0) {
          throw new Error('Exchange rate must be positive');
        }
      }

      const result = await updateRates({
        variables: { rates },
      });

      return result.data?.updateExchangeRates;
    },
    [updateRates]
  );

  return {
    updateExchangeRates: update,
    updatedCompany: data?.updateExchangeRates,
    loading,
    error,
  };
}
```

**Exemple avec fetch :**

```typescript
async function updateExchangeRates(
  token: string,
  rates: ExchangeRateInput[]
): Promise<UpdateExchangeRatesResponse['updateExchangeRates']> {
  // Validation côté client
  if (rates.length === 0) {
    throw new Error('At least one exchange rate is required');
  }

  const response = await fetch('YOUR_GRAPHQL_ENDPOINT', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      query: `
        mutation UpdateExchangeRates($rates: [ExchangeRateInput!]!) {
          updateExchangeRates(rates: $rates) {
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
      `,
      variables: {
        rates,
      },
    }),
  });

  const result = await response.json();
  if (result.errors) {
    throw new Error(result.errors[0].message);
  }
  return result.data.updateExchangeRates;
}
```

---

## 🎯 Hooks Personnalisés Recommandés

### Hook complet pour la gestion des taux

```typescript
import { useState, useCallback } from 'react';
import { useExchangeRates } from './useExchangeRates';
import { useUpdateExchangeRates } from './useUpdateExchangeRates';
import { useCurrencyConverter } from './useCurrencyConverter';

export function useExchangeRateManagement() {
  const { rates, loading: loadingRates, error: ratesError, refetch } =
    useExchangeRates();
  const {
    updateExchangeRates,
    loading: updating,
    error: updateError,
  } = useUpdateExchangeRates();
  const { convertAmount, loading: converting } = useCurrencyConverter();

  // Trouver un taux spécifique
  const getRate = useCallback(
    (from: string, to: string): number | null => {
      if (from === to) return 1.0;

      // Chercher le taux direct
      const directRate = rates.find(
        (r) => r.fromCurrency === from && r.toCurrency === to
      );
      if (directRate) return directRate.rate;

      // Chercher le taux inverse
      const inverseRate = rates.find(
        (r) => r.fromCurrency === to && r.toCurrency === from
      );
      if (inverseRate) return 1.0 / inverseRate.rate;

      return null;
    },
    [rates]
  );

  // Mettre à jour un taux spécifique
  const updateRate = useCallback(
    async (from: string, to: string, newRate: number) => {
      const updatedRates = rates.map((r) =>
        r.fromCurrency === from && r.toCurrency === to
          ? { ...r, rate: newRate }
          : r
      );

      // Si le taux n'existe pas, l'ajouter
      const exists = rates.some(
        (r) => r.fromCurrency === from && r.toCurrency === to
      );
      if (!exists) {
        updatedRates.push({
          fromCurrency: from,
          toCurrency: to,
          rate: newRate,
          isDefault: false,
          updatedAt: new Date().toISOString(),
          updatedBy: '', // Sera rempli par le backend
        });
      }

      await updateExchangeRates(
        updatedRates.map((r) => ({
          fromCurrency: r.fromCurrency,
          toCurrency: r.toCurrency,
          rate: r.rate,
        }))
      );
      await refetch();
    },
    [rates, updateExchangeRates, refetch]
  );

  return {
    // Données
    rates,
    // Actions
    getRate,
    updateRate,
    convertAmount,
    refetch,
    // États
    loading: loadingRates || updating || converting,
    error: ratesError || updateError,
  };
}
```

---

## 🔄 Exemples d'Utilisation dans Next.js

### Exemple 1 : Page de gestion des taux (Admin)

```typescript
'use client';

import { useExchangeRateManagement } from '@/hooks/useExchangeRateManagement';
import { useState } from 'react';

export default function ExchangeRatesPage() {
  const { rates, updateRate, loading, error } = useExchangeRateManagement();
  const [editing, setEditing] = useState<{
    from: string;
    to: string;
    rate: number;
  } | null>(null);

  const handleSave = async () => {
    if (!editing) return;
    try {
      await updateRate(editing.from, editing.to, editing.rate);
      setEditing(null);
    } catch (err) {
      console.error('Failed to update rate:', err);
    }
  };

  if (loading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div>
      <h1>Exchange Rates Management</h1>
      <table>
        <thead>
          <tr>
            <th>From</th>
            <th>To</th>
            <th>Rate</th>
            <th>Default</th>
            <th>Updated At</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {rates.map((rate) => (
            <tr key={`${rate.fromCurrency}-${rate.toCurrency}`}>
              <td>{rate.fromCurrency}</td>
              <td>{rate.toCurrency}</td>
              <td>
                {editing?.from === rate.fromCurrency &&
                editing?.to === rate.toCurrency ? (
                  <input
                    type="number"
                    value={editing.rate}
                    onChange={(e) =>
                      setEditing({
                        ...editing,
                        rate: parseFloat(e.target.value),
                      })
                    }
                  />
                ) : (
                  rate.rate
                )}
              </td>
              <td>{rate.isDefault ? 'Yes' : 'No'}</td>
              <td>{new Date(rate.updatedAt).toLocaleDateString()}</td>
              <td>
                {editing?.from === rate.fromCurrency &&
                editing?.to === rate.toCurrency ? (
                  <>
                    <button onClick={handleSave}>Save</button>
                    <button onClick={() => setEditing(null)}>Cancel</button>
                  </>
                ) : (
                  <button
                    onClick={() =>
                      setEditing({
                        from: rate.fromCurrency,
                        to: rate.toCurrency,
                        rate: rate.rate,
                      })
                    }
                  >
                    Edit
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

### Exemple 2 : Composant de conversion de devise

```typescript
'use client';

import { useCurrencyConverter } from '@/hooks/useCurrencyConverter';
import { useState, useEffect } from 'react';

export function CurrencyConverter() {
  const { convertAmount, convertedAmount, loading, error } =
    useCurrencyConverter();
  const [amount, setAmount] = useState(0);
  const [from, setFrom] = useState('USD');
  const [to, setTo] = useState('CDF');

  useEffect(() => {
    if (amount > 0 && from !== to) {
      convertAmount(amount, from, to);
    }
  }, [amount, from, to, convertAmount]);

  return (
    <div>
      <div>
        <input
          type="number"
          value={amount}
          onChange={(e) => setAmount(parseFloat(e.target.value) || 0)}
          placeholder="Amount"
        />
        <select value={from} onChange={(e) => setFrom(e.target.value)}>
          <option value="USD">USD</option>
          <option value="EUR">EUR</option>
          <option value="CDF">CDF</option>
        </select>
        <span>→</span>
        <select value={to} onChange={(e) => setTo(e.target.value)}>
          <option value="USD">USD</option>
          <option value="EUR">EUR</option>
          <option value="CDF">CDF</option>
        </select>
      </div>
      {loading && <div>Converting...</div>}
      {error && <div>Error: {error.message}</div>}
      {convertedAmount !== undefined && (
        <div>
          <strong>{convertedAmount.toFixed(2)} {to}</strong>
        </div>
      )}
    </div>
  );
}
```

### Exemple 3 : Utilisation dans un formulaire de vente

```typescript
'use client';

import { useExchangeRateManagement } from '@/hooks/useExchangeRateManagement';
import { useState, useEffect } from 'react';

export function SaleForm() {
  const { getRate, convertAmount } = useExchangeRateManagement();
  const [total, setTotal] = useState(0);
  const [currency, setCurrency] = useState('USD');
  const [displayCurrency, setDisplayCurrency] = useState('CDF');
  const [convertedTotal, setConvertedTotal] = useState(0);

  useEffect(() => {
    if (currency === displayCurrency) {
      setConvertedTotal(total);
      return;
    }

    const rate = getRate(currency, displayCurrency);
    if (rate !== null) {
      setConvertedTotal(total * rate);
    } else {
      // Fallback : utiliser la query de conversion
      convertAmount(total, currency, displayCurrency).then((converted) => {
        setConvertedTotal(converted);
      });
    }
  }, [total, currency, displayCurrency, getRate, convertAmount]);

  return (
    <form>
      {/* ... autres champs du formulaire ... */}
      <div>
        <label>Total</label>
        <input
          type="number"
          value={total}
          onChange={(e) => setTotal(parseFloat(e.target.value) || 0)}
        />
        <select value={currency} onChange={(e) => setCurrency(e.target.value)}>
          <option value="USD">USD</option>
          <option value="EUR">EUR</option>
          <option value="CDF">CDF</option>
        </select>
      </div>
      <div>
        <label>Display in</label>
        <select
          value={displayCurrency}
          onChange={(e) => setDisplayCurrency(e.target.value)}
        >
          <option value="USD">USD</option>
          <option value="EUR">EUR</option>
          <option value="CDF">CDF</option>
        </select>
        <div>
          <strong>
            {convertedTotal.toFixed(2)} {displayCurrency}
          </strong>
        </div>
      </div>
    </form>
  );
}
```

---

## ⚠️ Gestion des Erreurs

### Types d'erreurs possibles

```typescript
// Erreurs communes à gérer
export enum ExchangeRateError {
  UNAUTHORIZED = 'Unauthorized',
  NO_COMPANY = 'User does not have a company yet',
  INVALID_CURRENCY = 'Invalid currency',
  INVALID_RATE = 'Exchange rate must be positive',
  SAME_CURRENCY = 'Cannot set exchange rate for same currency',
  ADMIN_ONLY = 'Only Admin can update exchange rates',
  NO_RATE_AVAILABLE = 'No exchange rate available',
}

// Fonction utilitaire pour gérer les erreurs
export function handleExchangeRateError(error: Error): string {
  const message = error.message.toLowerCase();

  if (message.includes('unauthorized')) {
    return 'Vous devez être connecté pour accéder aux taux de change';
  }
  if (message.includes('no company')) {
    return "Votre compte n'est pas associé à une entreprise";
  }
  if (message.includes('invalid currency')) {
    return 'Devise invalide. Utilisez USD, EUR ou CDF';
  }
  if (message.includes('must be positive')) {
    return 'Le taux de change doit être positif';
  }
  if (message.includes('same currency')) {
    return 'Impossible de définir un taux pour la même devise';
  }
  if (message.includes('only admin')) {
    return 'Seuls les administrateurs peuvent modifier les taux de change';
  }
  if (message.includes('no exchange rate available')) {
    return 'Aucun taux de change disponible pour cette paire de devises';
  }

  return 'Une erreur est survenue lors de la gestion des taux de change';
}
```

---

## 🔐 Permissions

### Vérification des permissions

```typescript
// Seuls les admins peuvent mettre à jour les taux
export function canUpdateExchangeRates(userRole: string): boolean {
  return userRole === 'Admin';
}

// Exemple d'utilisation
import { useMe } from '@/hooks/useMe'; // Hook pour récupérer l'utilisateur connecté

export function ExchangeRatesPage() {
  const { user } = useMe();
  const canUpdate = canUpdateExchangeRates(user?.role || '');

  return (
    <div>
      {canUpdate ? (
        <ExchangeRatesEditor />
      ) : (
        <div>Vous n'avez pas la permission de modifier les taux</div>
      )}
    </div>
  );
}
```

---

## 📊 Devises Supportées

```typescript
export const SUPPORTED_CURRENCIES = ['USD', 'EUR', 'CDF'] as const;

export type Currency = (typeof SUPPORTED_CURRENCIES)[number];

export function isValidCurrency(currency: string): currency is Currency {
  return SUPPORTED_CURRENCIES.includes(currency as Currency);
}
```

---

## 🔮 Historique des Taux (Future Feature)

**Note :** L'historique des taux de change est implémenté côté backend mais n'est pas encore exposé via GraphQL. Pour l'ajouter :

### Query à ajouter dans le schema GraphQL (backend)

```graphql
type ExchangeRateHistory {
  id: ID!
  companyId: ID!
  fromCurrency: String!
  toCurrency: String!
  rate: Float!
  previousRate: Float
  updatedBy: String!
  updatedAt: String!
  reason: String
}

type Query {
  exchangeRateHistory(
    fromCurrency: String
    toCurrency: String
    limit: Int
  ): [ExchangeRateHistory!]! @auth
}
```

### Utilisation côté frontend (une fois implémenté)

```typescript
const GET_EXCHANGE_RATE_HISTORY = gql`
  query GetExchangeRateHistory(
    $fromCurrency: String
    $toCurrency: String
    $limit: Int
  ) {
    exchangeRateHistory(
      fromCurrency: $fromCurrency
      toCurrency: $toCurrency
      limit: $limit
    ) {
      id
      fromCurrency
      toCurrency
      rate
      previousRate
      updatedBy
      updatedAt
      reason
    }
  }
`;
```

---

## 📝 Checklist d'Intégration

- [ ] Installer les dépendances GraphQL (Apollo Client, urql, ou fetch)
- [ ] Créer les types TypeScript pour ExchangeRate et ExchangeRateInput
- [ ] Implémenter les hooks personnalisés (useExchangeRates, useUpdateExchangeRates, useCurrencyConverter)
- [ ] Créer les composants UI pour afficher et éditer les taux
- [ ] Ajouter la gestion des erreurs
- [ ] Implémenter les validations côté client
- [ ] Tester les permissions (Admin uniquement pour les mises à jour)
- [ ] Ajouter le composant de conversion de devise dans les formulaires de vente
- [ ] Tester avec différentes devises (USD, EUR, CDF)
- [ ] Gérer les cas limites (même devise, taux manquants)

---

## 🚀 Exemple de Configuration Apollo Client

```typescript
// lib/apollo-client.ts
import { ApolloClient, InMemoryCache, createHttpLink } from '@apollo/client';
import { setContext } from '@apollo/client/link/context';

const httpLink = createHttpLink({
  uri: process.env.NEXT_PUBLIC_GRAPHQL_ENDPOINT || 'http://localhost:8080/query',
});

const authLink = setContext((_, { headers }) => {
  const token = localStorage.getItem('token'); // ou votre méthode de stockage
  return {
    headers: {
      ...headers,
      authorization: token ? `Bearer ${token}` : '',
    },
  };
});

export const apolloClient = new ApolloClient({
  link: authLink.concat(httpLink),
  cache: new InMemoryCache(),
});
```

---

**Date de création :** 2024-01-XX  
**Version :** 1.0  
**Compatible avec :** Next.js 13+, React 18+





