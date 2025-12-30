# Scripts d'administration

## Créer des souscriptions d'essai pour toutes les companies

Ce script crée automatiquement une souscription d'essai de 14 jours pour toutes les companies qui n'en ont pas encore.

### Utilisation

1. **Assurez-vous d'avoir les variables d'environnement configurées** (fichier `.env` ou variables d'environnement système) :
   - `MONGO_URI` : URI de connexion MongoDB
   - `MONGO_DB_NAME` : Nom de la base de données (optionnel, défaut: `rangodb`)

2. **Exécutez le script** :

```bash
# Option 1: Compiler et exécuter
go run scripts/create_trial_subscriptions.go

# Option 2: Compiler puis exécuter
go build -o scripts/create_trial_subscriptions scripts/create_trial_subscriptions.go
./scripts/create_trial_subscriptions
```

### Comportement

- Le script récupère toutes les companies de la base de données
- Pour chaque company :
  - Si une souscription existe déjà, elle est ignorée
  - Si aucune souscription n'existe, une souscription d'essai de 14 jours est créée avec :
    - Plan: `trial`
    - Statut: `active`
    - Max Stores: 1
    - Max Users: 1
    - Date de fin d'essai: aujourd'hui + 14 jours

### Résultat

Le script affiche :
- Le nombre total de companies trouvées
- Pour chaque company traitée : succès, ignorée ou erreur
- Un résumé final avec :
  - Nombre de souscriptions créées
  - Nombre de souscriptions ignorées (déjà existantes)
  - Nombre d'erreurs

### Exemple de sortie

```
🔍 Récupération de toutes les companies...
📊 Nombre total de companies trouvées: 5

[1/5] Traitement de la company: Acme Corp (ID: 507f1f77bcf86cd799439011)
  ✅ Souscription d'essai créée avec succès!
     - Plan: trial
     - Statut: active
     - Date de fin d'essai: 2024-01-15 10:30:00
     - Max Stores: 1
     - Max Users: 1

[2/5] Traitement de la company: Tech Solutions (ID: 507f1f77bcf86cd799439012)
  ⏭️  Souscription déjà existante, ignorée

...

============================================================
📈 RÉSUMÉ
============================================================
✅ Souscriptions créées avec succès: 3
⏭️  Souscriptions ignorées (déjà existantes): 2
❌ Erreurs: 0
📊 Total traité: 5
============================================================
```

## Étendre les dates de souscription de 15 jours

Ce script met à jour toutes les souscriptions existantes en ajoutant 15 jours à partir de la date de création de la company.

### Utilisation

1. **Assurez-vous d'avoir les variables d'environnement configurées** (fichier `.env` ou variables d'environnement système) :
   - `MONGO_URI` : URI de connexion MongoDB
   - `MONGO_DB_NAME` : Nom de la base de données (optionnel, défaut: `rangodb`)

2. **Exécutez le script** :

```bash
# Option 1: Compiler et exécuter (recommandé)
go run scripts/extend_subscriptions.go

# Option 2: Compiler puis exécuter
go build -o scripts/extend_subscriptions scripts/extend_subscriptions.go
./scripts/extend_subscriptions
```

### Comportement

- Le script récupère toutes les companies de la base de données
- Pour chaque company :
  - Si aucune souscription n'existe, elle est ignorée
  - Si une souscription existe :
    - Pour les souscriptions d'essai (`trial`) : met à jour `TrialEndDate` = date de création de la company + 15 jours
    - Pour les souscriptions payantes : met à jour `SubscriptionEndDate` = date de création de la company + 15 jours

### Résultat

Le script affiche :
- Le nombre total de companies trouvées
- Pour chaque company traitée :
  - La date de création de la company
  - Les informations de la souscription actuelle
  - La nouvelle date de fin calculée
  - Succès, ignorée ou erreur
- Un résumé final avec :
  - Nombre de souscriptions mises à jour
  - Nombre de souscriptions ignorées (sans souscription ou sans date)
  - Nombre d'erreurs

### Exemple de sortie

```
🔍 Récupération de toutes les companies...
📊 Nombre total de companies trouvées: 5

[1/5] Traitement de la company: Acme Corp (ID: 507f1f77bcf86cd799439011)
  📅 Date de création de la company: 2024-01-01 10:00:00
  📋 Souscription actuelle:
     - Plan: trial
     - Statut: active
     - Date de fin d'essai actuelle: 2024-01-15 10:00:00
  ✅ Souscription mise à jour avec succès!
     - Nouvelle date de fin (TrialEndDate): 2024-01-16 10:00:00
     - Jours ajoutés: 15

[2/5] Traitement de la company: Tech Solutions (ID: 507f1f77bcf86cd799439012)
  📅 Date de création de la company: 2024-01-05 14:30:00
  ⚠️  Aucune souscription trouvée pour cette company, ignorée

...

============================================================
📈 RÉSUMÉ
============================================================
✅ Souscriptions mises à jour avec succès: 3
⏭️  Souscriptions ignorées (sans souscription ou sans date): 2
❌ Erreurs: 0
📊 Total traité: 5
📅 Jours ajoutés par souscription: 15
============================================================
```

## Ajouter 15 jours d'essai à toutes les companies

Ce script ajoute ou étend une période d'essai de 15 jours pour toutes les companies existantes, qu'elles aient déjà une souscription ou non.

### Utilisation

1. **Assurez-vous d'avoir les variables d'environnement configurées** (fichier `.env` ou variables d'environnement système) :
   - `MONGO_URI` : URI de connexion MongoDB
   - `MONGO_DB_NAME` : Nom de la base de données (optionnel, défaut: `rangodb`)

2. **Exécutez le script** :

```bash
# Option 1: Compiler et exécuter (recommandé)
go run scripts/add_trial_to_all_companies.go

# Option 2: Compiler puis exécuter
go build -o scripts/add_trial_to_all_companies scripts/add_trial_to_all_companies.go
./scripts/add_trial_to_all_companies
```

### Comportement

Le script récupère toutes les companies de la base de données et pour chaque company :

**Si aucune souscription n'existe :**
- Crée une nouvelle souscription d'essai de 15 jours avec :
  - Plan: `trial`
  - Statut: `active`
  - Date de début: maintenant
  - Date de fin d'essai: maintenant + 15 jours
  - Max Stores: 1
  - Max Users: 1

**Si une souscription existe déjà :**
- Pour les souscriptions d'essai (`trial`) : ajoute 15 jours à `TrialEndDate` (date actuelle + 15 jours)
- Pour les souscriptions payantes avec date de fin : ajoute 15 jours à `SubscriptionEndDate`
- Pour les souscriptions sans date de fin : ajoute un `TrialEndDate` de 15 jours
- Réactive automatiquement les souscriptions expirées (statut passe à `active`)

### Résultat

Le script affiche :
- Le nombre total de companies trouvées
- Pour chaque company traitée :
  - Les informations de la souscription (existante ou nouvelle)
  - Les dates avant et après l'extension
  - Succès ou erreur
- Un résumé final avec :
  - Nombre total de souscriptions traitées avec succès
  - Nombre de nouvelles souscriptions créées
  - Nombre de souscriptions étendues
  - Nombre d'erreurs

### Exemple de sortie

```
🔍 Récupération de toutes les companies...
📊 Nombre total de companies trouvées: 5

[1/5] Traitement de la company: Acme Corp (ID: 507f1f77bcf86cd799439011)
  📝 Aucune souscription existante, création d'une nouvelle période d'essai de 15 jours...
  ✅ Souscription d'essai créée avec succès!
     - Plan: trial
     - Statut: active
     - Date de début: 2024-12-17 10:30:00
     - Date de fin d'essai: 2025-01-01 10:30:00

[2/5] Traitement de la company: Tech Solutions (ID: 507f1f77bcf86cd799439012)
  🔄 Souscription existante trouvée (Plan: trial, Statut: active)
     - Date de fin actuelle: 2024-12-20 14:30:00
     - Nouvelle date de fin: 2025-01-04 14:30:00
  ✅ Période d'essai étendue de 15 jours!

[3/5] Traitement de la company: Business Inc (ID: 507f1f77bcf86cd799439013)
  🔄 Souscription existante trouvée (Plan: business, Statut: expired)
     - Date de fin actuelle: 2024-11-30 10:00:00
     - Nouvelle date de fin: 2024-12-15 10:00:00
  ✅ Abonnement étendu de 15 jours!

...

======================================================================
📈 RÉSUMÉ
======================================================================
✅ Total traité avec succès: 5
   - Nouvelles souscriptions créées: 2
   - Souscriptions étendues: 3
⏭️  Souscriptions ignorées: 0
❌ Erreurs: 0
📊 Total de companies: 5
======================================================================
```

### Cas d'usage

Ce script est utile pour :
- Offrir une période d'essai promotionnelle à tous les clients existants
- Compenser une interruption de service
- Tester une nouvelle fonctionnalité avec tous les utilisateurs
- Migration ou mise à jour du système d'abonnement

## Ajouter les taux de change par défaut aux companies existantes

Ce script ajoute les taux de change par défaut (1 USD = 2200 CDF) à toutes les companies qui n'en ont pas encore.

### Utilisation

1. **Assurez-vous d'avoir les variables d'environnement configurées** :
   - `MONGO_URI` : URI de connexion MongoDB

2. **Exécutez le script** :

```bash
# Option 1: Compiler et exécuter (recommandé)
go run scripts/add_exchange_rates_to_companies.go

# Option 2: Compiler puis exécuter
go build -o scripts/add_exchange_rates_to_companies scripts/add_exchange_rates_to_companies.go
./scripts/add_exchange_rates_to_companies
```

### Comportement

Le script récupère toutes les companies de la base de données et pour chaque company :

**Si aucun taux de change n'existe :**
- Ajoute les taux de change par défaut :
  - 1 USD = 2200 CDF
  - Marqué comme taux par défaut (`isDefault: true`)
  - Créé par l'utilisateur système (`updatedBy: "system"`)

**Si des taux de change existent déjà :**
- La company est ignorée pour préserver ses taux personnalisés

### Résultat

Le script affiche :
- Le nombre total de companies trouvées
- Pour chaque company traitée :
  - Mise à jour réussie avec les taux ajoutés
  - Ou ignorée si elle a déjà des taux configurés
- Un résumé final avec :
  - Nombre total de companies
  - Nombre de companies mises à jour
  - Nombre de companies ignorées (déjà configurées)

### Exemple de sortie

```
✅ Connected to MongoDB
📊 Found 5 companies

✅ Updated: Acme Corp (ID: 507f1f77bcf86cd799439011) - Added default exchange rates (1 USD = 2200 CDF)
⏭️  Skipped: Tech Solutions (ID: 507f1f77bcf86cd799439012) - Already has exchange rates
✅ Updated: Business Inc (ID: 507f1f77bcf86cd799439013) - Added default exchange rates (1 USD = 2200 CDF)
⏭️  Skipped: Retail Store (ID: 507f1f77bcf86cd799439014) - Already has exchange rates
✅ Updated: Services LLC (ID: 507f1f77bcf86cd799439015) - Added default exchange rates (1 USD = 2200 CDF)

📈 Summary:
   - Total companies: 5
   - Updated: 3
   - Skipped (already configured): 2

✅ Migration completed successfully!
```

### Cas d'usage

Ce script est utile pour :
- Migrer vers le nouveau système de taux de change
- Ajouter les taux par défaut aux companies créées avant l'implémentation de cette fonctionnalité
- Réinitialiser les taux d'une company (en supprimant d'abord ses taux existants)

### Notes

- Ce script est **idempotent** : vous pouvez l'exécuter plusieurs fois sans problème
- Les companies avec des taux personnalisés ne seront jamais écrasées
- Pour les nouvelles companies, les taux sont automatiquement ajoutés à la création

## Migration complète du système de devises et taux de change

Ce script complet met à jour **toutes les companies ET tous les stores** avec le nouveau système de gestion des devises et taux de change. C'est le script recommandé pour migrer l'ensemble du système.

### Utilisation

1. **Assurez-vous d'avoir les variables d'environnement configurées** :
   - `MONGO_URI` : URI de connexion MongoDB

2. **Exécutez le script** :

```bash
# Option 1: Compiler et exécuter (recommandé)
go run scripts/migrate_currency_exchange_rates.go

# Option 2: Compiler puis exécuter
go build -o scripts/migrate_currency_exchange_rates scripts/migrate_currency_exchange_rates.go
./scripts/migrate_currency_exchange_rates
```

### Comportement

Le script effectue une migration en **2 étapes** :

#### ÉTAPE 1 : Mise à jour des Companies

Pour chaque company :

**Si aucun taux de change n'existe :**
- ✅ Ajoute les taux de change par défaut :
  - 1 USD = 2200 CDF
  - Marqué comme taux par défaut (`isDefault: true`)
  - Créé par l'utilisateur système (`updatedBy: "system"`)

**Si des taux de change existent déjà :**
- ⏭️  La company est ignorée pour préserver ses taux personnalisés

#### ÉTAPE 2 : Vérification et mise à jour des Stores

Pour chaque store :

**Si `defaultCurrency` n'est pas défini :**
- ✅ Définit `defaultCurrency` à "USD"

**Si `supportedCurrencies` n'est pas défini :**
- ✅ Définit `supportedCurrencies` à ["USD", "CDF"]

**Si `defaultCurrency` n'est pas dans `supportedCurrencies` :**
- ✅ Ajoute `defaultCurrency` à la liste des devises supportées

**Si tout est correctement configuré :**
- ✓ Le store est ignoré

### Résultat

Le script affiche :

1. **Pour chaque company** :
   - Nom et ID
   - Action effectuée (mise à jour ou ignorée)
   - Détails des taux ajoutés si applicable

2. **Pour chaque store** :
   - Nom et ID
   - Configuration actuelle des devises
   - Action effectuée (mise à jour ou ignorée)
   - Détails des modifications si applicable

3. **Résumé final** :
   - Statistiques complètes pour companies et stores
   - Nombre total, mis à jour, ignorés, erreurs

### Exemple de sortie

```
🚀 Script de migration: Système de gestion des devises et taux de change
============================================================================

⚠️  No .env file found, using environment variables
✅ Connected to MongoDB

📊 ÉTAPE 1/2: Mise à jour des companies avec les taux de change
─────────────────────────────────────────────────────────────────────────────

📌 Found 5 companies

[1/5] Processing company: Acme Corp (ID: 507f1f77bcf86cd799439011)
   ✅ Success! Added default exchange rates:
      • 1 USD = 2200 CDF
      • Updated by: system
      • Date: 2024-12-17 10:30:00

[2/5] Processing company: Tech Solutions (ID: 507f1f77bcf86cd799439012)
   ⏭️  Already has 1 exchange rate(s) configured, skipping

[3/5] Processing company: Business Inc (ID: 507f1f77bcf86cd799439013)
   ✅ Success! Added default exchange rates:
      • 1 USD = 2200 CDF
      • Updated by: system
      • Date: 2024-12-17 10:30:15

[4/5] Processing company: Retail Store (ID: 507f1f77bcf86cd799439014)
   ⏭️  Already has 2 exchange rate(s) configured, skipping

[5/5] Processing company: Services LLC (ID: 507f1f77bcf86cd799439015)
   ✅ Success! Added default exchange rates:
      • 1 USD = 2200 CDF
      • Updated by: system
      • Date: 2024-12-17 10:30:30


📊 ÉTAPE 2/2: Vérification et mise à jour des stores
─────────────────────────────────────────────────────────────────────────────

📌 Found 8 stores

[1/8] Processing store: Store A (ID: 607f1f77bcf86cd799439021)
   ⚠️  No default currency, setting to USD
   ⚠️  No supported currencies, setting to [USD, CDF]
   ✅ Store updated successfully

[2/8] Processing store: Store B (ID: 607f1f77bcf86cd799439022)
   ✓ Default currency: USD
   ✓ Supported currencies: [USD CDF]
   ✓ Store already correctly configured

[3/8] Processing store: Store C (ID: 607f1f77bcf86cd799439023)
   ✓ Default currency: CDF
   ✓ Supported currencies: [USD]
   ⚠️  Default currency not in supported list, adding it
   ✅ Store updated successfully

[4/8] Processing store: Store D (ID: 607f1f77bcf86cd799439024)
   ✓ Default currency: USD
   ✓ Supported currencies: [USD CDF EUR]
   ✓ Store already correctly configured

[5/8] Processing store: Store E (ID: 607f1f77bcf86cd799439025)
   ⚠️  No default currency, setting to USD
   ✓ Supported currencies: [CDF]
   ⚠️  Default currency not in supported list, adding it
   ✅ Store updated successfully

[6/8] Processing store: Store F (ID: 607f1f77bcf86cd799439026)
   ✓ Default currency: EUR
   ✓ Supported currencies: [EUR USD]
   ✓ Store already correctly configured

[7/8] Processing store: Store G (ID: 607f1f77bcf86cd799439027)
   ✓ Default currency: USD
   ✓ Supported currencies: [USD CDF]
   ✓ Store already correctly configured

[8/8] Processing store: Store H (ID: 607f1f77bcf86cd799439028)
   ⚠️  No default currency, setting to USD
   ⚠️  No supported currencies, setting to [USD, CDF]
   ✅ Store updated successfully


============================================================================
📈 RÉSUMÉ FINAL
============================================================================

🏢 COMPANIES:
   • Total: 5
   • ✅ Updated: 3
   • ⏭️  Skipped: 2
   • ❌ Errors: 0

🏪 STORES:
   • Total: 8
   • ✅ Updated: 4
   • ⏭️  Skipped: 4
   • ❌ Errors: 0

============================================================================

✅ Migration completed successfully!
```

### Cas d'usage

Ce script est parfait pour :
- **Migration initiale** vers le nouveau système de devises et taux de change
- **Mise à jour complète** après déploiement de la nouvelle fonctionnalité
- **Vérification** que toutes les companies et stores sont correctement configurés
- **Réparation** des configurations incomplètes ou manquantes

### Avantages

- ✅ **Complet** : Met à jour companies ET stores en une seule exécution
- ✅ **Idempotent** : Peut être exécuté plusieurs fois sans problème
- ✅ **Sécurisé** : Préserve les configurations personnalisées existantes
- ✅ **Détaillé** : Affiche chaque action effectuée avec des messages clairs
- ✅ **Robuste** : Gère les erreurs et continue la migration
- ✅ **Intelligent** : Détecte et corrige les incohérences automatiquement

### Garanties

- 🔒 **Aucune perte de données** : Les taux personnalisés sont toujours préservés
- 🔒 **Validation automatique** : Vérifie que `defaultCurrency` est dans `supportedCurrencies`
- 🔒 **Configuration par défaut** : Ajoute USD et CDF si rien n'est configuré
- 🔒 **Traçabilité** : Tous les taux ajoutés sont marqués avec date et auteur

### Notes importantes

1. **Backup recommandé** : Bien que le script soit sécurisé, il est recommandé de faire un backup avant la migration
2. **Temps d'exécution** : Peut prendre quelques secondes à quelques minutes selon le nombre d'entités
3. **Environnement** : Testez d'abord sur un environnement de développement
4. **Rollback** : En cas de problème, les taux peuvent être supprimés manuellement via MongoDB

### Après la migration

Une fois la migration terminée :

1. ✅ Toutes les companies ont des taux de change configurés
2. ✅ Tous les stores ont une devise par défaut
3. ✅ Tous les stores ont une liste de devises supportées
4. ✅ Le système est prêt pour utiliser les nouvelles fonctionnalités de conversion

Les utilisateurs peuvent maintenant :
- Consulter les taux de change via GraphQL
- Convertir des montants entre devises
- Mettre à jour les taux (administrateurs uniquement)























