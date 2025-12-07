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




