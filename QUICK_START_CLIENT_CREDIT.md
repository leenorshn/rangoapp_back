# 🚀 Quick Start - Système de Crédit Client

Guide rapide pour utiliser le nouveau système de crédit client.

## 📋 Ce qui a été ajouté

✅ Chaque client peut avoir une **limite de crédit**  
✅ Le système calcule automatiquement la **dette actuelle**  
✅ Le système calcule le **crédit disponible**  
✅ **Vérification automatique** avant vente à crédit  
✅ **Blocage automatique** si crédit insuffisant  

## 🎯 Utilisation en 5 Minutes

### 1. Créer un Client avec Crédit

```graphql
mutation {
  createClient(input: {
    name: "Jean Dupont"
    phone: "+243123456789"
    storeId: "your_store_id"
    creditLimit: 10000  # 10000 USD de crédit
  }) {
    id
    name
    creditLimit       # 10000
    currentDebt       # 0 (aucune dette)
    availableCredit   # 10000 (tout disponible)
  }
}
```

### 2. Vendre à Crédit

```graphql
mutation {
  createSale(input: {
    basket: [
      {productId: "prod1", quantity: 2, price: 1500}
      {productId: "prod2", quantity: 1, price: 2000}
    ]
    priceToPay: 5000
    pricePayed: 0           # Aucun paiement immédiat
    clientId: "client_id"
    storeId: "store_id"
    currency: "USD"
    paymentType: "debt"     # ← VENTE À CRÉDIT
  }) {
    id
    amountDue      # 5000 (à crédit)
    debtStatus     # "unpaid"
    debtId         # ID de la dette créée
  }
}
```

✅ **Si crédit suffisant** : Vente créée + Dette créée  
❌ **Si crédit insuffisant** : Erreur avec message clair

### 3. Consulter le Crédit d'un Client

```graphql
query {
  client(id: "client_id") {
    name
    creditLimit       # Limite autorisée
    currentDebt       # Dette actuelle (calculée auto)
    availableCredit   # Crédit disponible (calculé auto)
  }
}
```

**Exemple de réponse :**
```json
{
  "data": {
    "client": {
      "name": "Jean Dupont",
      "creditLimit": 10000,
      "currentDebt": 5000,
      "availableCredit": 5000
    }
  }
}
```

### 4. Client Paie sa Dette

```graphql
mutation {
  payDebt(
    debtId: "debt_id"
    amount: 2000
    description: "Paiement partiel"
  ) {
    id
    totalAmount    # 5000
    amountPaid     # 2000
    amountDue      # 3000 (reste)
    status         # "partial"
  }
}
```

Après paiement, consultez à nouveau le client :
```graphql
query {
  client(id: "client_id") {
    currentDebt       # 3000 (réduit!)
    availableCredit   # 7000 (augmenté!)
  }
}
```

### 5. Modifier la Limite de Crédit (Admin uniquement)

```graphql
mutation {
  updateClientCreditLimit(
    clientId: "client_id"
    creditLimit: 15000  # Nouvelle limite
  ) {
    id
    name
    creditLimit       # 15000 (augmenté!)
    currentDebt       # 3000 (inchangé)
    availableCredit   # 12000 (augmenté!)
  }
}
```

## 💡 Exemples de Scénarios

### Scénario 1 : Client Sans Crédit (Défaut)

```graphql
# Créer sans spécifier creditLimit
mutation {
  createClient(input: {
    name: "Nouveau Client"
    phone: "+243999999999"
    storeId: "store_id"
    # creditLimit non spécifié = 0
  })
}

# Essayer de vendre à crédit
mutation {
  createSale(input: {
    # ...
    paymentType: "debt"
  })
}
# ❌ Erreur: "Crédit insuffisant. Crédit disponible: 0.00"
```

### Scénario 2 : Vente Excédant le Crédit

```graphql
# Client a:
# - creditLimit: 10000
# - currentDebt: 8000
# - availableCredit: 2000

# Tenter de vendre 3000 USD à crédit
mutation {
  createSale(input: {
    priceToPay: 3000
    pricePayed: 0
    paymentType: "debt"
    # ...
  })
}

# ❌ Erreur: "Crédit insuffisant. Crédit disponible: 2000.00, Montant requis: 3000.00"
```

### Scénario 3 : Vente avec Paiement Partiel

```graphql
# Vendre 5000 USD, client paie 2000 cash, reste à crédit
mutation {
  createSale(input: {
    basket: [{productId: "prod1", quantity: 1, price: 5000}]
    priceToPay: 5000
    pricePayed: 2000      # Paie 2000 cash
    paymentType: "debt"   # Reste (3000) à crédit
    clientId: "client_id"
    # ...
  }) {
    priceToPay      # 5000
    pricePayed      # 2000
    amountDue       # 3000 (sur le crédit)
    debtStatus      # "partial"
  }
}

# Le crédit du client est réduit de 3000 seulement
```

### Scénario 4 : Client Paie Complètement

```graphql
# Client a 3 dettes impayées:
# - Dette 1: 2000 USD
# - Dette 2: 3000 USD
# - Dette 3: 1000 USD
# Total: 6000 USD

# Client paie 6000 USD
# (Paiements appliqués aux dettes dans l'ordre)

mutation {
  payDebt(debtId: "debt1_id", amount: 2000, description: "Paiement dette 1")
}
mutation {
  payDebt(debtId: "debt2_id", amount: 3000, description: "Paiement dette 2")
}
mutation {
  payDebt(debtId: "debt3_id", amount: 1000, description: "Paiement dette 3")
}

# Résultat:
query {
  client(id: "client_id") {
    currentDebt       # 0 (toutes payées!)
    availableCredit   # 10000 (limite complète disponible)
  }
}
```

## 🎓 Concepts Clés

### Formule du Crédit Disponible

```
availableCredit = creditLimit - currentDebt
```

### Calcul Automatique

Le système calcule **automatiquement** :
- `currentDebt` = Somme des dettes avec status "unpaid" ou "partial"
- `availableCredit` = creditLimit - currentDebt

Vous n'avez **rien à calculer manuellement** !

### Flux d'une Vente à Crédit

```
1. Client demande achat à crédit
   ↓
2. Système vérifie: availableCredit >= montant ?
   ├─ OUI → Vente créée + Dette créée
   └─ NON → Vente refusée avec erreur
```

## ✅ Checklist de Test

Testez ces scénarios pour valider le système :

- [ ] Créer client avec creditLimit = 5000
- [ ] Vérifier availableCredit = 5000
- [ ] Vente à crédit de 2000 USD (devrait réussir)
- [ ] Vérifier currentDebt = 2000, availableCredit = 3000
- [ ] Tenter vente de 4000 USD (devrait échouer)
- [ ] Payer 1000 USD
- [ ] Vérifier currentDebt = 1000, availableCredit = 4000
- [ ] Augmenter limite à 10000 (Admin)
- [ ] Vérifier availableCredit = 9000
- [ ] Vente de 5000 USD (devrait réussir maintenant)

## 🔒 Permissions

| Action | Admin | User |
|--------|-------|------|
| Créer client avec crédit | ✅ | ✅ |
| **Modifier limite** | **✅** | **❌** |
| Vendre à crédit | ✅ | ✅ |
| Voir crédit | ✅ | ✅ |
| Recevoir paiement | ✅ | ✅ |

## ⚠️ Points d'Attention

### 1. Client Requis

```graphql
# ❌ ERREUR
mutation {
  createSale(input: {
    # ...
    paymentType: "debt"
    # clientId NON spécifié
  })
}
# Erreur: "Un client doit être spécifié pour les ventes à crédit"

# ✅ CORRECT
mutation {
  createSale(input: {
    # ...
    paymentType: "debt"
    clientId: "client_id"  # ← Client obligatoire!
  })
}
```

### 2. Limite Négative Interdite

```graphql
# ❌ ERREUR
mutation {
  updateClientCreditLimit(
    clientId: "client_id"
    creditLimit: -1000  # Négatif!
  })
}
# Erreur: "Credit limit cannot be negative"
```

### 3. Dette > Limite

Si vous réduisez la limite sous la dette actuelle :

```graphql
# Client a: creditLimit = 10000, currentDebt = 8000

mutation {
  updateClientCreditLimit(
    clientId: "client_id"
    creditLimit: 5000  # Réduit sous la dette!
  }) {
    creditLimit       # 5000
    currentDebt       # 8000 (inchangé)
    availableCredit   # 0 (car dette > limite)
  }
}

# Le client ne peut plus acheter à crédit jusqu'à paiement!
```

## 📚 Documentation Complète

- **Guide détaillé** : `CLIENT_CREDIT_SYSTEM.md`
- **Résumé technique** : `CREDIT_SYSTEM_IMPLEMENTATION.md`

## 💬 Questions Fréquentes

**Q: Comment donner du crédit à un client existant ?**
```graphql
mutation {
  updateClientCreditLimit(clientId: "...", creditLimit: 5000)
}
```

**Q: Comment voir tous les clients avec dette ?**
```graphql
query {
  clients(storeId: "...") {
    name
    currentDebt
    availableCredit
  }
}
# Filtrer côté client: currentDebt > 0
```

**Q: Que se passe-t-il si je vends en "cash" ?**  
R: Le système de crédit n'est pas vérifié. Seules les ventes avec `paymentType: "debt"` ou `"advance"` utilisent le crédit.

**Q: Puis-je avoir des limites différentes par store ?**  
R: Oui, chaque client est lié à un store avec sa propre limite.

---

**Prêt à utiliser !** 🎉  
Pour plus de détails, consultez `CLIENT_CREDIT_SYSTEM.md`










