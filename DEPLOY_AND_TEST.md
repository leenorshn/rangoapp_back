# 🚀 Guide de Déploiement et Test

## ⚡ Déploiement Rapide

### 1. Prérequis

```bash
# Vérifier Go
go version  # Doit être 1.16+

# Vérifier MongoDB
mongosh --version  # Ou mongo --version
```

### 2. Configuration de l'Environnement

```bash
# Copier l'exemple d'environnement
cp env.example .env

# Éditer .env avec vos valeurs
nano .env  # ou vim .env ou code .env
```

**Variables essentielles dans `.env` :**
```bash
# MongoDB
MONGO_URI=mongodb://localhost:27017/rangoapp
MONGO_DB_NAME=rangoapp

# JWT
JWT_SECRET=votre_secret_tres_long_et_securise_ici_min_32_caracteres
JWT_REFRESH_SECRET=votre_refresh_secret_different_et_long

# Serveur
PORT=8080
ENV=development

# CORS (optionnel en dev)
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
```

### 3. Compilation

```bash
# Compiler le projet
go build -o rangoapp .

# Vérifier la compilation
ls -lh rangoapp
```

### 4. Migration des Données (IMPORTANT)

```bash
# AVANT de démarrer le serveur, migrer les données
export MONGO_URI="mongodb://localhost:27017/rangoapp"

# Migration complète (taux de change + devises stores)
go run scripts/migrate_currency_exchange_rates.go
```

**Résultat attendu :**
```
✅ Connected to MongoDB
📌 Found X companies
✅ Updated: Y companies
📌 Found Z stores
✅ Updated: W stores
✅ Migration completed successfully!
```

### 5. Démarrer le Serveur

```bash
# Option 1: Avec le binaire compilé
./rangoapp

# Option 2: Directement avec go run
go run server.go
```

**Vous devriez voir :**
```
🚀 Server starting...
📊 Environment: development
🔌 MongoDB Connected
🌐 Server running on http://localhost:8080
✅ GraphQL Playground: http://localhost:8080/graphql
```

---

## 🧪 Tests Manuels

### Test 1 : Vérifier le Serveur

```bash
# Dans un nouveau terminal
curl http://localhost:8080/health

# Réponse attendue:
{"status":"ok"}
```

### Test 2 : GraphQL Playground

Ouvrir dans le navigateur :
```
http://localhost:8080/graphql
```

### Test 3 : Tester les Taux de Change

#### 3.1 Login (pour obtenir un token)

```graphql
mutation {
  login(phone: "votre_phone", password: "votre_password") {
    accessToken
    user {
      id
      name
      role
    }
  }
}
```

**Copier le `accessToken`** et l'ajouter dans les headers HTTP :
```json
{
  "Authorization": "Bearer VOTRE_TOKEN_ICI"
}
```

#### 3.2 Récupérer les Taux de Change

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
        "updatedAt": "2024-12-17T..."
      }
    ]
  }
}
```

#### 3.3 Tester la Conversion

```graphql
query {
  convertCurrency(
    amount: 100
    fromCurrency: "USD"
    toCurrency: "CDF"
  )
}
```

**Résultat attendu :**
```json
{
  "data": {
    "convertCurrency": 220000
  }
}
```

#### 3.4 Mettre à Jour les Taux (Admin uniquement)

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

### Test 4 : Tester le Crédit Client

#### 4.1 Créer un Client avec Crédit

```graphql
mutation {
  createClient(input: {
    name: "Test Client"
    phone: "+243999888777"
    storeId: "VOTRE_STORE_ID"
    creditLimit: 5000
  }) {
    id
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Résultat attendu :**
```json
{
  "data": {
    "createClient": {
      "id": "...",
      "name": "Test Client",
      "creditLimit": 5000,
      "currentDebt": 0,
      "availableCredit": 5000
    }
  }
}
```

#### 4.2 Vente à Crédit (Succès)

```graphql
mutation {
  createSale(input: {
    basket: [
      {productId: "VOTRE_PRODUCT_ID", quantity: 1, price: 2000}
    ]
    priceToPay: 2000
    pricePayed: 0
    clientId: "CLIENT_ID_DU_TEST_4.1"
    storeId: "VOTRE_STORE_ID"
    currency: "USD"
    paymentType: "debt"
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

**Résultat attendu :**
```json
{
  "data": {
    "createSale": {
      "id": "...",
      "priceToPay": 2000,
      "pricePayed": 0,
      "amountDue": 2000,
      "debtStatus": "unpaid",
      "debtId": "..."
    }
  }
}
```

#### 4.3 Vérifier le Crédit Réduit

```graphql
query {
  client(id: "CLIENT_ID_DU_TEST_4.1") {
    name
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Résultat attendu :**
```json
{
  "data": {
    "client": {
      "name": "Test Client",
      "creditLimit": 5000,
      "currentDebt": 2000,
      "availableCredit": 3000
    }
  }
}
```

#### 4.4 Vente à Crédit (Échec - Crédit Insuffisant)

```graphql
mutation {
  createSale(input: {
    basket: [
      {productId: "VOTRE_PRODUCT_ID", quantity: 1, price: 4000}
    ]
    priceToPay: 4000
    pricePayed: 0
    clientId: "CLIENT_ID_DU_TEST_4.1"
    storeId: "VOTRE_STORE_ID"
    currency: "USD"
    paymentType: "debt"
  }) {
    id
  }
}
```

**Résultat attendu (ERREUR) :**
```json
{
  "errors": [
    {
      "message": "Crédit insuffisant. Crédit disponible: 3000.00, Montant requis: 4000.00"
    }
  ]
}
```

✅ **Si vous obtenez cette erreur, c'est parfait ! Le système fonctionne.**

#### 4.5 Payer la Dette

```graphql
mutation {
  payDebt(
    debtId: "DEBT_ID_DU_TEST_4.2"
    amount: 1000
    description: "Paiement test"
  ) {
    id
    totalAmount
    amountPaid
    amountDue
    status
  }
}
```

#### 4.6 Vérifier le Crédit Libéré

```graphql
query {
  client(id: "CLIENT_ID_DU_TEST_4.1") {
    creditLimit
    currentDebt
    availableCredit
  }
}
```

**Résultat attendu :**
```json
{
  "data": {
    "client": {
      "creditLimit": 5000,
      "currentDebt": 1000,
      "availableCredit": 4000
    }
  }
}
```

---

## 🐛 Dépannage

### Problème : MongoDB ne se connecte pas

**Erreur :**
```
Failed to connect to MongoDB
```

**Solution :**
```bash
# Vérifier que MongoDB est lancé
sudo systemctl status mongod  # Linux
brew services list  # macOS

# Démarrer MongoDB si nécessaire
sudo systemctl start mongod  # Linux
brew services start mongodb-community  # macOS
```

### Problème : "Unauthorized"

**Erreur :**
```json
{
  "errors": [{"message": "Unauthorized"}]
}
```

**Solution :**
1. Vous devez d'abord faire un `login`
2. Copier le `accessToken`
3. L'ajouter dans les HTTP Headers du Playground :
   ```json
   {
     "Authorization": "Bearer VOTRE_TOKEN"
   }
   ```

### Problème : "Store not found"

**Solution :**
```graphql
# Lister vos stores
query {
  stores {
    id
    name
  }
}

# Utiliser un ID valide dans vos mutations
```

### Problème : "Product not found"

**Solution :**
```graphql
# Lister vos produits
query {
  products(storeId: "VOTRE_STORE_ID") {
    id
    name
    stock
  }
}

# Utiliser un ID valide et vérifier le stock
```

### Problème : Port déjà utilisé

**Erreur :**
```
bind: address already in use
```

**Solution :**
```bash
# Trouver le processus utilisant le port 8080
lsof -i :8080

# Tuer le processus
kill -9 PID_DU_PROCESSUS

# Ou utiliser un autre port
export PORT=8081
./rangoapp
```

---

## 📊 Checklist de Test

### Taux de Change
- [ ] Migration exécutée
- [ ] Récupération des taux fonctionne
- [ ] Conversion USD → CDF fonctionne
- [ ] Conversion CDF → USD fonctionne (inverse)
- [ ] Mise à jour des taux (Admin) fonctionne
- [ ] Erreur si utilisateur non-admin tente mise à jour

### Crédit Client
- [ ] Création client avec creditLimit fonctionne
- [ ] Champs calculés (currentDebt, availableCredit) corrects
- [ ] Vente à crédit avec crédit suffisant fonctionne
- [ ] Vente à crédit avec crédit insuffisant est bloquée
- [ ] Message d'erreur clair et informatif
- [ ] Paiement de dette fonctionne
- [ ] Crédit se libère après paiement
- [ ] Mise à jour limite crédit (Admin) fonctionne

---

## 🎯 Tests d'Intégration Suggérés

### Scénario Complet : Parcours Client

```
1. Créer client avec creditLimit = 10000
   ✓ availableCredit = 10000

2. Vente 1 : 3000 USD à crédit
   ✓ Vente créée
   ✓ Dette créée
   ✓ availableCredit = 7000

3. Vente 2 : 5000 USD à crédit
   ✓ Vente créée
   ✓ availableCredit = 2000

4. Vente 3 : 3000 USD à crédit
   ✗ Erreur "Crédit insuffisant"

5. Paiement : 4000 USD
   ✓ Dette réduite
   ✓ availableCredit = 6000

6. Vente 4 : 5000 USD à crédit
   ✓ Maintenant possible!
```

---

## 📝 Notes Importantes

### Avant le Déploiement en Production

1. **Backup de la BDD**
   ```bash
   mongodump --uri="mongodb://localhost:27017/rangoapp" --out=backup-$(date +%Y%m%d)
   ```

2. **Tester en Dev/Staging d'abord**
   - Toutes les queries
   - Toutes les mutations
   - Tous les cas d'erreur

3. **Vérifier les Permissions**
   - Admin peut tout faire
   - User peut créer ventes mais pas modifier limites

4. **Monitorer**
   - Logs du serveur
   - Performance MongoDB
   - Temps de réponse API

### En Cas de Problème

1. **Consulter les logs**
   ```bash
   # Si lancé en background
   tail -f rangoapp.log
   ```

2. **Vérifier MongoDB**
   ```bash
   mongosh
   use rangoapp
   db.companies.findOne()
   db.clients.findOne()
   ```

3. **Rollback si nécessaire**
   ```bash
   mongorestore --uri="mongodb://localhost:27017/rangoapp" --drop backup-directory/
   ```

---

## ✅ Si Tout Fonctionne

**Félicitations ! 🎉**

Votre système est opérationnel avec :
- ✅ Gestion des taux de change
- ✅ Système de crédit client
- ✅ Vérifications automatiques
- ✅ Sécurité implémentée

**Prochaines étapes :**
1. Tester avec de vraies données
2. Former les utilisateurs
3. Monitorer les performances
4. Collecter les feedbacks

---

**Besoin d'aide ?** Consultez :
- `EXCHANGE_RATES.md` pour les taux de change
- `CLIENT_CREDIT_SYSTEM.md` pour le crédit client
- `RECENT_CHANGES_REVIEW.md` pour la revue complète








