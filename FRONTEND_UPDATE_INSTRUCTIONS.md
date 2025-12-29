# 🎨 Instructions de Mise à Jour Frontend - Next.js

**Date :** 17 Décembre 2024  
**Nouvelles fonctionnalités Backend :**
1. Système de Gestion des Taux de Change
2. Système de Crédit Client

---

## 📋 Vue d'ensemble des Changements

### Nouveautés Disponibles

✅ **Taux de Change**
- Chaque entreprise peut configurer ses taux de change
- Conversion automatique entre devises (USD, CDF, EUR)
- API pour consulter et modifier les taux

✅ **Crédit Client**
- Chaque client peut avoir une limite de crédit
- Calcul automatique de la dette actuelle et du crédit disponible
- Vérification automatique avant vente à crédit
- Blocage si crédit insuffisant

---

## 🔄 PARTIE 1 : SYSTÈME DE TAUX DE CHANGE

### 📊 Nouveaux Types GraphQL

#### Type ExchangeRate

```graphql
type ExchangeRate {
  fromCurrency: String!      # Ex: "USD"
  toCurrency: String!        # Ex: "CDF"
  rate: Float!               # Ex: 2200
  isDefault: Boolean!        # true = taux système par défaut
  updatedAt: String!         # Date dernière modification
  updatedBy: String!         # ID utilisateur qui a modifié
}
```

#### Champs Ajoutés au Type Company

```graphql
type Company {
  # ... champs existants
  exchangeRates: [ExchangeRate!]!  # NOUVEAU
}
```

### 📡 Nouvelles Queries Disponibles

#### 1. Récupérer les Taux de Change

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

**Utilisation :** Page de configuration des taux, affichage dans les rapports

#### 2. Convertir un Montant

```graphql
query ConvertCurrency($amount: Float!, $from: String!, $to: String!) {
  convertCurrency(
    amount: $amount
    fromCurrency: $from
    toCurrency: $to
  )
}
```

**Exemple :**
```graphql
query {
  convertCurrency(amount: 100, fromCurrency: "USD", toCurrency: "CDF")
}
# Retourne: 220000
```

**Utilisation :** Afficher les prix en plusieurs devises, rapports consolidés

#### 3. Récupérer Taux avec Info Company

```graphql
query GetCompanyWithRates {
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

### 🔧 Nouvelles Mutations Disponibles

#### Mettre à Jour les Taux (Admin uniquement)

```graphql
mutation UpdateExchangeRates($rates: [ExchangeRateInput!]!) {
  updateExchangeRates(rates: $rates) {
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

**Variables :**
```json
{
  "rates": [
    {
      "fromCurrency": "USD",
      "toCurrency": "CDF",
      "rate": 2300
    },
    {
      "fromCurrency": "EUR",
      "toCurrency": "CDF",
      "rate": 2500
    }
  ]
}
```

### 🎨 Éléments UI à Ajouter

#### 1. Page de Gestion des Taux de Change (Admin)

**Emplacement suggéré :** `Settings > Taux de Change` ou `Configuration > Devises`

**Fonctionnalités :**
- Afficher les taux actuels dans un tableau
- Afficher la date de dernière mise à jour
- Formulaire pour modifier les taux
- Indication visuelle pour les taux par défaut
- Historique des modifications (si disponible)

**Layout suggéré :**
```
╔════════════════════════════════════════╗
║  Taux de Change                        ║
╠════════════════════════════════════════╣
║  De    │  Vers  │  Taux    │  Modifié ║
╠════════════════════════════════════════╣
║  USD   │  CDF   │  2200.00 │  il y a 2j║
║  EUR   │  CDF   │  2400.00 │  il y a 2j║
╠════════════════════════════════════════╣
║  [Modifier les Taux]                   ║
╚════════════════════════════════════════╝
```

**Validation :**
- Taux doit être > 0
- Devises doivent être différentes
- Seuls les admins peuvent modifier

#### 2. Affichage Multi-Devises sur les Produits

**Emplacement :** Partout où un prix est affiché

**Exemple :**
```
Prix: 50 USD (110,000 CDF)
      ↑        ↑
   Principal  Converti
```

**Requête pour obtenir la conversion :**
```graphql
query GetProductWithConversion($productId: ID!, $targetCurrency: String!) {
  product(id: $productId) {
    id
    name
    priceVente
    currency
  }
  
  # Si le produit est en USD et vous voulez afficher en CDF
  convertCurrency(
    amount: 50  # priceVente du produit
    fromCurrency: "USD"
    toCurrency: $targetCurrency
  )
}
```

#### 3. Widget de Conversion Rapide

**Emplacement :** Dans la sidebar ou en haut de page

**Fonctionnalités :**
- Input montant
- Sélecteur devise source
- Sélecteur devise cible
- Affichage résultat en temps réel

**Requête :**
```graphql
query QuickConvert($amount: Float!, $from: String!, $to: String!) {
  convertCurrency(amount: $amount, fromCurrency: $from, toCurrency: $to)
}
```

#### 4. Rapports avec Conversion

**Emplacement :** Page des ventes, caisse, rapports

**Fonctionnalités :**
- Toggle pour afficher en devise originale ou convertie
- Total consolidé en une seule devise

**Exemple de requête pour rapport multi-devises :**
```graphql
query SalesReportWithConversion($storeId: String!) {
  sales(storeId: $storeId) {
    id
    priceToPay
    currency
  }
  
  exchangeRates {
    fromCurrency
    toCurrency
    rate
  }
}
```

**Note :** Faire la conversion côté client avec les taux récupérés

---

## 💳 PARTIE 2 : SYSTÈME DE CRÉDIT CLIENT

### 📊 Nouveaux Champs GraphQL

#### Champs Ajoutés au Type Client

```graphql
type Client {
  id: ID!
  name: String!
  phone: String!
  storeId: String!
  store: Store!
  
  # NOUVEAUX CHAMPS
  creditLimit: Float!        # Limite de crédit autorisée
  currentDebt: Float!        # Dette actuelle (calculé automatiquement)
  availableCredit: Float!    # Crédit disponible (calculé automatiquement)
  
  createdAt: String!
  updatedAt: String!
}
```

**Calculs automatiques :**
- `currentDebt` = Somme des dettes avec status "unpaid" ou "partial"
- `availableCredit` = `creditLimit` - `currentDebt`

### 📡 Queries Modifiées

#### Récupérer un Client (avec info crédit)

```graphql
query GetClient($id: ID!) {
  client(id: $id) {
    id
    name
    phone
    creditLimit
    currentDebt
    availableCredit
    createdAt
  }
}
```

#### Liste des Clients (avec info crédit)

```graphql
query GetClients($storeId: String) {
  clients(storeId: $storeId) {
    id
    name
    phone
    creditLimit
    currentDebt
    availableCredit
  }
}
```

### 🔧 Mutations Modifiées et Nouvelles

#### 1. Créer un Client (avec crédit)

```graphql
mutation CreateClient($input: CreateClientInput!) {
  createClient(input: $input) {
    id
    name
    phone
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Variables :**
```json
{
  "input": {
    "name": "Jean Dupont",
    "phone": "+243123456789",
    "storeId": "store123",
    "creditLimit": 10000
  }
}
```

**Note :** `creditLimit` est optionnel, défaut = 0

#### 2. Modifier un Client (incluant crédit)

```graphql
mutation UpdateClient($id: ID!, $input: UpdateClientInput!) {
  updateClient(id: $id, input: $input) {
    id
    name
    phone
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Variables pour modifier le crédit :**
```json
{
  "id": "client123",
  "input": {
    "creditLimit": 15000
  }
}
```

#### 3. NOUVELLE : Mettre à Jour Limite de Crédit (Admin)

```graphql
mutation UpdateClientCreditLimit($clientId: ID!, $creditLimit: Float!) {
  updateClientCreditLimit(
    clientId: $clientId
    creditLimit: $creditLimit
  ) {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Variables :**
```json
{
  "clientId": "client123",
  "creditLimit": 20000
}
```

**Permission :** Admin uniquement

#### 4. Vente à Crédit (mutation existante, comportement modifié)

```graphql
mutation CreateSaleOnCredit($input: CreateSaleInput!) {
  createSale(input: $input) {
    id
    priceToPay
    pricePayed
    amountDue
    debtStatus
    debtId
  }
}
```

**Variables pour vente à crédit :**
```json
{
  "input": {
    "basket": [
      {"productId": "prod1", "quantity": 2, "price": 1500}
    ],
    "priceToPay": 3000,
    "pricePayed": 0,
    "clientId": "client123",
    "storeId": "store123",
    "currency": "USD",
    "paymentType": "debt"
  }
}
```

**Comportement :**
- ✅ Vérification automatique du crédit disponible
- ✅ Vente créée si crédit suffisant
- ❌ Erreur si crédit insuffisant : `"Crédit insuffisant. Crédit disponible: X, Montant requis: Y"`

### 🎨 Éléments UI à Ajouter

#### 1. Fiche Client Enrichie

**Emplacement :** Page détail client

**Nouveaux éléments à afficher :**

```
╔════════════════════════════════════════╗
║  Client: Jean Dupont                   ║
║  Tel: +243 123 456 789                 ║
╠════════════════════════════════════════╣
║  💳 CRÉDIT                             ║
║  ├─ Limite autorisée:    10,000 USD   ║
║  ├─ Dette actuelle:       3,500 USD   ║
║  └─ Crédit disponible:    6,500 USD   ║
╠════════════════════════════════════════╣
║  📊 Utilisation: ████████░░ 35%        ║
╠════════════════════════════════════════╣
║  [Voir Dettes] [Modifier Limite]       ║
╚════════════════════════════════════════╝
```

**Indicateurs visuels :**
- 🟢 Crédit disponible > 70% de la limite
- 🟡 Crédit disponible entre 30% et 70%
- 🔴 Crédit disponible < 30%
- 🚫 Crédit épuisé (disponible = 0)

**Requête :**
```graphql
query GetClientDetails($id: ID!) {
  client(id: $id) {
    id
    name
    phone
    creditLimit
    currentDebt
    availableCredit
  }
  
  # Optionnel : liste des dettes
  clientDebts(clientId: $id, storeId: $storeId) {
    id
    totalAmount
    amountDue
    status
    createdAt
  }
}
```

#### 2. Liste Clients avec Indicateurs de Crédit

**Emplacement :** Page liste des clients

**Colonnes à ajouter :**
- Limite de crédit
- Dette actuelle
- Crédit disponible
- Badge de statut (🟢🟡🔴)

**Requête :**
```graphql
query GetClientsWithCredit($storeId: String) {
  clients(storeId: $storeId) {
    id
    name
    phone
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Filtres suggérés :**
- Clients avec crédit disponible
- Clients à crédit épuisé
- Clients avec dettes
- Clients sans crédit autorisé

#### 3. Formulaire de Vente - Vérification Crédit

**Emplacement :** Page de création de vente

**Nouveaux éléments :**

1. **Sélection du type de paiement :**
   - ○ Cash (Comptant)
   - ○ Crédit (À crédit)
   - ○ Mixte (Partiel)

2. **Si "Crédit" sélectionné :**
   ```
   ╔════════════════════════════════════════╗
   ║  💳 Vente à Crédit                     ║
   ╠════════════════════════════════════════╣
   ║  Client: [Sélecteur]                   ║
   ║  Crédit disponible: 6,500 USD          ║
   ║  Montant de la vente: 3,000 USD        ║
   ║  ✓ Crédit suffisant                    ║
   ╚════════════════════════════════════════╝
   ```

3. **Validation en temps réel :**
   - Dès que le client est sélectionné, afficher son crédit disponible
   - Comparer avec le montant de la vente
   - Afficher ✓ ou ✗ selon disponibilité

**Requête de vérification :**
```graphql
query CheckClientCredit($clientId: ID!) {
  client(id: $clientId) {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Logique frontend :**
```
SI availableCredit >= montantVente ALORS
  ✓ Afficher en vert "Crédit suffisant"
  Activer bouton "Valider la vente"
SINON
  ✗ Afficher en rouge "Crédit insuffisant (Disponible: X, Requis: Y)"
  Désactiver bouton "Valider la vente"
  Suggérer : "Augmenter la limite" ou "Paiement partiel"
FIN SI
```

#### 4. Page de Gestion du Crédit (Admin)

**Emplacement :** `Settings > Gestion du Crédit` ou sous profil client

**Fonctionnalités :**

1. **Modifier la limite de crédit :**
   ```
   ╔════════════════════════════════════════╗
   ║  Modifier Limite de Crédit             ║
   ╠════════════════════════════════════════╣
   ║  Client: Jean Dupont                   ║
   ║  Limite actuelle: 10,000 USD           ║
   ║  Dette actuelle: 3,500 USD             ║
   ║                                        ║
   ║  Nouvelle limite: [________] USD       ║
   ║                                        ║
   ║  ⚠️  La dette actuelle est de 3,500    ║
   ║      Ne descendez pas sous ce montant  ║
   ║                                        ║
   ║  [Annuler]  [Enregistrer]              ║
   ╚════════════════════════════════════════╝
   ```

**Mutation :**
```graphql
mutation UpdateLimit($clientId: ID!, $newLimit: Float!) {
  updateClientCreditLimit(clientId: $clientId, creditLimit: $newLimit) {
    id
    creditLimit
    availableCredit
  }
}
```

#### 5. Dashboard - Vue d'ensemble Crédit

**Emplacement :** Page d'accueil ou dashboard

**Widgets suggérés :**

1. **Total Crédit Accordé**
   ```
   ╔════════════════════════════╗
   ║  💳 Crédit Total           ║
   ║  250,000 USD               ║
   ║  Sur 50 clients            ║
   ╚════════════════════════════╝
   ```

2. **Crédit Utilisé**
   ```
   ╔════════════════════════════╗
   ║  📊 Utilisation            ║
   ║  85,000 USD (34%)          ║
   ║  ████████░░░░░░░░░░        ║
   ╚════════════════════════════╝
   ```

3. **Clients à Risque**
   ```
   ╔════════════════════════════╗
   ║  ⚠️  Crédit Épuisé         ║
   ║  5 clients                 ║
   ║  [Voir la liste]           ║
   ╚════════════════════════════╝
   ```

**Requête pour le dashboard :**
```graphql
query GetCreditDashboard($storeId: String) {
  clients(storeId: $storeId) {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Calculs côté client :**
- Total crédit accordé : `sum(creditLimit)`
- Total utilisé : `sum(currentDebt)`
- Taux d'utilisation : `sum(currentDebt) / sum(creditLimit) * 100`
- Clients à risque : `count(availableCredit < creditLimit * 0.1)`

#### 6. Historique et Suivi des Dettes

**Emplacement :** Sous la fiche client

**Affichage :**
```
╔════════════════════════════════════════════════════════╗
║  📋 Historique des Dettes                              ║
╠════════════════════════════════════════════════════════╣
║  Date       │ Montant │ Payé    │ Restant │ Statut    ║
╠════════════════════════════════════════════════════════╣
║  15/12/2024 │ 5,000   │ 5,000   │ 0       │ ✓ Payée   ║
║  10/12/2024 │ 3,500   │ 2,000   │ 1,500   │ ⏳ Partiel║
║  05/12/2024 │ 2,000   │ 0       │ 2,000   │ ⚠️ Impayée║
╚════════════════════════════════════════════════════════╝
```

**Requête :**
```graphql
query GetClientDebts($clientId: String!, $storeId: String) {
  clientDebts(clientId: $clientId, storeId: $storeId) {
    id
    totalAmount
    amountPaid
    amountDue
    status
    createdAt
    payments {
      id
      amount
      createdAt
      description
    }
  }
}
```

---

## 🔔 Messages d'Erreur à Gérer

### Taux de Change

1. **Erreur de conversion** (devise invalide)
   ```json
   {
     "errors": [{
       "message": "Invalid currency: ABC or XYZ"
     }]
   }
   ```
   **UI :** "Devises non supportées"

2. **Modification non autorisée** (non-admin)
   ```json
   {
     "errors": [{
       "message": "Only admins can update exchange rates"
     }]
   }
   ```
   **UI :** "Vous n'avez pas les permissions nécessaires"

### Crédit Client

1. **Crédit insuffisant**
   ```json
   {
     "errors": [{
       "message": "Crédit insuffisant. Crédit disponible: 2000.00, Montant requis: 5000.00"
     }]
   }
   ```
   **UI :** Afficher l'erreur + suggérer alternatives :
   - Paiement partiel
   - Augmenter la limite (si admin)
   - Vente en plusieurs fois

2. **Client requis pour crédit**
   ```json
   {
     "errors": [{
       "message": "Un client doit être spécifié pour les ventes à crédit"
     }]
   }
   ```
   **UI :** "Veuillez sélectionner un client pour une vente à crédit"

3. **Limite négative**
   ```json
   {
     "errors": [{
       "message": "Credit limit cannot be negative"
     }]
   }
   ```
   **UI :** "La limite de crédit doit être positive"

---

## 💡 Recommandations UX

### Taux de Change

1. **Affichage Contextuel**
   - Montrer la conversion partout où c'est pertinent
   - Ne pas surcharger l'interface
   - Permettre de basculer entre devises

2. **Mise à Jour**
   - Demander confirmation avant modification
   - Montrer l'ancien et le nouveau taux
   - Indiquer qui a fait la modification

3. **Historique**
   - Garder trace des modifications (si disponible)
   - Afficher la date de dernière mise à jour

### Crédit Client

1. **Indicateurs Visuels**
   - Codes couleur clairs (vert/jaune/rouge)
   - Badges de statut
   - Barres de progression

2. **Prévention**
   - Vérification en temps réel
   - Affichage anticipé du crédit disponible
   - Suggestions alternatives

3. **Transparence**
   - Montrer clairement les limites
   - Afficher l'historique des dettes
   - Indiquer les dates de paiement

4. **Workflows Simplifiés**
   - Création client avec crédit en un clic
   - Modification rapide des limites
   - Paiement de dette facilité

---

## 📱 Responsive et Accessibilité

### Mobile

- Les tableaux de taux doivent être scrollables horizontalement
- Les indicateurs de crédit doivent être visibles sans scroll
- Les formulaires doivent être tactiles (gros boutons)

### Accessibilité

- Utiliser des labels ARIA pour les indicateurs visuels
- Fournir des alternatives textuelles aux codes couleur
- Assurer la navigation au clavier

---

## 🔄 Ordre d'Implémentation Suggéré

### Phase 1 : Fondations (1-2 jours)
1. ✅ Mettre à jour les types GraphQL (TypeScript)
2. ✅ Créer les hooks/services pour les nouvelles queries
3. ✅ Tester les requêtes dans l'API

### Phase 2 : Crédit Client (2-3 jours)
4. ✅ Afficher creditLimit, currentDebt, availableCredit sur fiche client
5. ✅ Ajouter indicateurs visuels (badges, couleurs)
6. ✅ Modifier formulaire de création client
7. ✅ Ajouter vérification crédit dans formulaire de vente
8. ✅ Page de gestion des limites (Admin)

### Phase 3 : Taux de Change (2-3 jours)
9. ✅ Page de configuration des taux (Admin)
10. ✅ Widget de conversion rapide
11. ✅ Affichage multi-devises sur produits
12. ✅ Conversion dans les rapports

### Phase 4 : Améliorations (1-2 jours)
13. ✅ Dashboard avec statistiques crédit
14. ✅ Filtres avancés clients
15. ✅ Tests et corrections

---

## 📚 Ressources Backend

- **Documentation complète :** `EXCHANGE_RATES.md`
- **Guide crédit client :** `CLIENT_CREDIT_SYSTEM.md`
- **Quick start :** `QUICK_START_EXCHANGE_RATES.md` + `QUICK_START_CLIENT_CREDIT.md`
- **Exemples de tests :** `DEPLOY_AND_TEST.md`

---

## ✅ Checklist Frontend

### Taux de Change
- [ ] Page gestion taux (Admin)
- [ ] Affichage multi-devises sur produits
- [ ] Widget conversion
- [ ] Rapports avec conversion
- [ ] Gestion des erreurs

### Crédit Client
- [ ] Champs crédit sur fiche client
- [ ] Indicateurs visuels (couleurs, badges)
- [ ] Vérification en temps réel dans vente
- [ ] Blocage si crédit insuffisant
- [ ] Page gestion limite (Admin)
- [ ] Dashboard statistiques
- [ ] Historique des dettes
- [ ] Messages d'erreur clairs

---

**Prêt à implémenter ! 🚀**

Toutes les requêtes GraphQL sont prêtes et testées. Le backend est opérationnel et attend le frontend !






