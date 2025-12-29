# Guide de Migration - Système de Devises et Taux de Change

## 📋 Vue d'ensemble

Ce guide vous explique comment migrer votre base de données existante vers le nouveau système de gestion des devises et taux de change.

## 🎯 Objectif de la Migration

La migration effectue les actions suivantes :

### Pour les Companies
- ✅ Ajoute les taux de change par défaut (1 USD = 2200 CDF)
- ✅ Préserve les taux personnalisés existants
- ✅ Marque les taux ajoutés comme "système" pour traçabilité

### Pour les Stores
- ✅ Définit une devise par défaut (USD si non spécifié)
- ✅ Définit les devises supportées ([USD, CDF] si non spécifié)
- ✅ Valide que la devise par défaut est dans les devises supportées
- ✅ Corrige automatiquement les incohérences

## 🚀 Étapes de Migration

### 1. Backup de la Base de Données

**IMPORTANT** : Toujours faire un backup avant une migration !

```bash
# Backup MongoDB
mongodump --uri="YOUR_MONGO_URI" --out=backup-$(date +%Y%m%d-%H%M%S)
```

### 2. Test en Environnement de Développement

```bash
# Sur votre environnement de dev
export MONGO_URI="mongodb://localhost:27017/rangoapp"
go run scripts/migrate_currency_exchange_rates.go
```

### 3. Vérification des Résultats

Après la migration, vérifiez via MongoDB ou GraphQL :

#### Via MongoDB Shell
```javascript
// Vérifier une company
db.companies.findOne({}, {name: 1, exchangeRates: 1})

// Vérifier un store
db.stores.findOne({}, {name: 1, defaultCurrency: 1, supportedCurrencies: 1})
```

#### Via GraphQL
```graphql
query VerifyMigration {
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
  
  stores {
    id
    name
    defaultCurrency
    supportedCurrencies
  }
}
```

### 4. Migration en Production

```bash
# Sur votre serveur de production
export MONGO_URI="your_production_mongodb_uri"
go run scripts/migrate_currency_exchange_rates.go
```

## 📊 Comprendre la Sortie du Script

### Exemple de Sortie Normale

```
🚀 Script de migration: Système de gestion des devises et taux de change
============================================================================

✅ Connected to MongoDB

📊 ÉTAPE 1/2: Mise à jour des companies avec les taux de change
─────────────────────────────────────────────────────────────────────────────

📌 Found 3 companies

[1/3] Processing company: Mon Entreprise (ID: 507f...)
   ✅ Success! Added default exchange rates:
      • 1 USD = 2200 CDF
      • Updated by: system
      • Date: 2024-12-17 10:30:00

[2/3] Processing company: Tech Corp (ID: 508f...)
   ⏭️  Already has 1 exchange rate(s) configured, skipping

[3/3] Processing company: Retail Store (ID: 509f...)
   ✅ Success! Added default exchange rates:
      • 1 USD = 2200 CDF
      • Updated by: system
      • Date: 2024-12-17 10:30:15


📊 ÉTAPE 2/2: Vérification et mise à jour des stores
─────────────────────────────────────────────────────────────────────────────

📌 Found 5 stores

[1/5] Processing store: Boutique Centre (ID: 607f...)
   ⚠️  No default currency, setting to USD
   ⚠️  No supported currencies, setting to [USD, CDF]
   ✅ Store updated successfully

[2/5] Processing store: Boutique Nord (ID: 608f...)
   ✓ Default currency: USD
   ✓ Supported currencies: [USD CDF]
   ✓ Store already correctly configured

============================================================================
📈 RÉSUMÉ FINAL
============================================================================

🏢 COMPANIES:
   • Total: 3
   • ✅ Updated: 2
   • ⏭️  Skipped: 1
   • ❌ Errors: 0

🏪 STORES:
   • Total: 5
   • ✅ Updated: 1
   • ⏭️  Skipped: 4
   • ❌ Errors: 0

============================================================================

✅ Migration completed successfully!
```

### Interprétation des Symboles

- ✅ **Success** : Action effectuée avec succès
- ⏭️ **Skipped** : Déjà configuré, aucune action nécessaire
- ⚠️ **Warning** : Valeur manquante ou incohérence détectée et corrigée
- ✓ **Check** : Configuration validée comme correcte
- ❌ **Error** : Erreur rencontrée (la migration continue pour les autres entités)

## 🔧 Résolution des Problèmes

### Erreur : "MONGO_URI environment variable is required"

**Solution :** Définissez la variable d'environnement

```bash
export MONGO_URI="mongodb://localhost:27017/rangoapp"
# Ou créez un fichier .env à la racine du projet
echo "MONGO_URI=mongodb://localhost:27017/rangoapp" > .env
```

### Erreur : "Failed to connect to MongoDB"

**Solutions possibles :**
1. Vérifiez que MongoDB est en cours d'exécution
2. Vérifiez l'URI de connexion
3. Vérifiez les permissions réseau/firewall
4. Vérifiez les credentials si authentification requise

### Le script dit "Already configured" mais je veux réinitialiser

Si vous voulez réinitialiser les taux d'une company :

```javascript
// Via MongoDB Shell
db.companies.updateOne(
  {_id: ObjectId("your_company_id")},
  {$set: {exchangeRates: []}}
)
```

Puis relancez le script.

### Des stores ont toujours des valeurs vides

Vérifiez les logs du script. Si le script indique une mise à jour mais que les valeurs sont toujours vides :

1. Vérifiez les permissions MongoDB
2. Vérifiez que vous êtes connecté à la bonne base de données
3. Relancez le script (il est idempotent)

## ✅ Vérifications Post-Migration

### Checklist

- [ ] Toutes les companies ont au moins un taux de change
- [ ] Tous les stores ont une `defaultCurrency`
- [ ] Tous les stores ont des `supportedCurrencies`
- [ ] La `defaultCurrency` de chaque store est dans ses `supportedCurrencies`
- [ ] Les taux personnalisés des companies ont été préservés
- [ ] Le système fonctionne correctement (tests fonctionnels)

### Tests Fonctionnels

```graphql
# Test 1: Récupérer les taux
query {
  exchangeRates {
    fromCurrency
    toCurrency
    rate
  }
}

# Test 2: Convertir une devise
query {
  convertCurrency(amount: 100, fromCurrency: "USD", toCurrency: "CDF")
}

# Test 3: Créer une vente (devrait utiliser la devise du store)
mutation {
  createSale(input: {
    basket: [{productId: "...", quantity: 1, price: 50}]
    priceToPay: 50
    pricePayed: 50
    storeId: "..."
  }) {
    id
    currency
  }
}
```

## 📝 Notes Importantes

### Idempotence

Le script est **idempotent** : vous pouvez l'exécuter plusieurs fois sans risque. Il ne modifiera que ce qui doit l'être.

### Temps d'Exécution

- Pour 100 companies + 500 stores : ~5-10 secondes
- Pour 1000 companies + 5000 stores : ~30-60 secondes

### Impact sur le Système

- ✅ Aucun downtime nécessaire
- ✅ Les opérations en cours ne sont pas affectées
- ✅ La migration peut être faite en production sans interruption

### Rollback

Si nécessaire, vous pouvez rollback en restaurant le backup :

```bash
mongorestore --uri="YOUR_MONGO_URI" --drop backup-directory/
```

## 🎓 Après la Migration

Une fois la migration terminée :

1. ✅ Les nouvelles companies auront automatiquement les taux par défaut
2. ✅ Les nouveaux stores auront automatiquement USD comme devise par défaut
3. ✅ Les utilisateurs peuvent modifier les taux via GraphQL (admins uniquement)
4. ✅ La conversion de devises est disponible pour tous

### Prochaines Étapes

1. Informer les utilisateurs de la nouvelle fonctionnalité
2. Former les administrateurs sur la modification des taux
3. Documenter les taux de change dans votre documentation utilisateur
4. Configurer des rappels pour mettre à jour les taux mensuellement si nécessaire

## 📞 Support

En cas de problème :
1. Consultez les logs du script
2. Vérifiez la documentation dans `EXCHANGE_RATES.md`
3. Consultez le README dans `scripts/README.md`
4. Contactez l'équipe technique

## 🔄 Mises à Jour Futures

Le script de migration est conçu pour être évolutif. Si de nouvelles devises sont ajoutées :

1. Mettez à jour `isValidCurrency()` dans `database/store_db.go`
2. Ajoutez les nouveaux taux dans `GetDefaultExchangeRates()` dans `database/exchange_rate_db.go`
3. Le script de migration peut être adapté si nécessaire

---

**Date de création :** Décembre 2024  
**Version du script :** 1.0  
**Compatibilité :** RangoApp Backend v1.0+






