# Guide : Configurer les Variables d'Environnement dans Cloud Run

Ce guide vous montre comment ajouter toutes les variables d'environnement nécessaires à votre service Cloud Run.

## 📋 Variables d'Environnement Requises

### ✅ Obligatoires
- `MONGO_URI` - URI de connexion MongoDB
- `MONGO_DB_NAME` - Nom de la base de données
- `JWT_SECRET` - Secret JWT (minimum 32 caractères)

### ⚙️ Optionnelles (avec valeurs par défaut)
- `PORT` - Port du serveur (défaut: 8080, Cloud Run le définit automatiquement)
- `DB_TIMEOUT_SECONDS` - Timeout des opérations DB (défaut: 5)
- `DB_CONNECT_TIMEOUT_SECONDS` - Timeout de connexion (défaut: 10)
- `DB_MAX_RETRIES` - Nombre de tentatives de reconnexion (défaut: 3)
- `HEALTH_CHECK_INTERVAL_SECONDS` - Intervalle de vérification de santé (défaut: 30)
- `LOG_LEVEL` - Niveau de log (défaut: INFO)
- `ALLOWED_ORIGINS` - Origines CORS autorisées (séparées par des virgules)
- `CORS_DEBUG` - Activer le debug CORS (défaut: false)

---

## 🎯 Méthode 1 : Via la Console Google Cloud (Interface Graphique)

### Étapes :

1. **Accédez à Cloud Run Console**
   - Allez sur : https://console.cloud.google.com/run
   - Sélectionnez votre projet

2. **Sélectionnez votre service**
   - Cliquez sur le service `rangoapp-backend`

3. **Éditez la révision**
   - Cliquez sur **"EDIT & DEPLOY NEW REVISION"** (en haut)

4. **Ajoutez les variables d'environnement**
   - Cliquez sur l'onglet **"Variables and Secrets"**
   - Cliquez sur **"ADD VARIABLE"** pour chaque variable
   - Ajoutez les variables suivantes :

   | Nom | Valeur | Exemple |
   |-----|--------|---------|
   | `MONGO_URI` | Votre URI MongoDB | `mongodb+srv://user:pass@cluster.mongodb.net/rangodb?retryWrites=true&w=majority` |
   | `MONGO_DB_NAME` | Nom de la DB | `rangodb` |
   | `JWT_SECRET` | Secret JWT (32+ caractères) | `your-very-long-and-secure-secret-key-at-least-32-characters-long` |
   | `PORT` | Port du serveur | `8080` |
   | `DB_TIMEOUT_SECONDS` | Timeout DB | `5` |
   | `DB_CONNECT_TIMEOUT_SECONDS` | Timeout connexion | `10` |
   | `DB_MAX_RETRIES` | Tentatives max | `3` |
   | `HEALTH_CHECK_INTERVAL_SECONDS` | Intervalle health check | `30` |
   | `LOG_LEVEL` | Niveau de log | `INFO` |
   | `ALLOWED_ORIGINS` | Origines CORS | `https://rangoweb-ioelziq27-leenorshns-projects.vercel.app` |
   | `CORS_DEBUG` | (Optionnel) Debug CORS | `true` |

5. **Déployez**
   - Cliquez sur **"DEPLOY"** en bas de la page
   - Attendez que le déploiement se termine

---

## 🖥️ Méthode 2 : Via gcloud CLI (Ligne de Commande)

### Option A : Utiliser le fichier YAML (Recommandé)

1. **Éditez le fichier `cloudrun-env.yaml`**
   ```bash
   # Ouvrez le fichier et remplacez les valeurs par vos vraies valeurs
   nano cloudrun-env.yaml
   # ou
   code cloudrun-env.yaml
   ```

2. **Mettez à jour le service avec toutes les variables**
   ```bash
   gcloud run services update rangoapp-backend \
     --region europe-west1 \
     --update-env-vars-file cloudrun-env.yaml
   ```

### Option B : Ajouter toutes les variables en une seule commande

```bash
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars \
    MONGO_URI="mongodb+srv://user:password@cluster.mongodb.net/rangodb?retryWrites=true&w=majority",\
    MONGO_DB_NAME="rangodb",\
    JWT_SECRET="your-very-long-and-secure-secret-key-at-least-32-characters-long",\
    PORT="8080",\
    DB_TIMEOUT_SECONDS="5",\
    DB_CONNECT_TIMEOUT_SECONDS="10",\
    DB_MAX_RETRIES="3",\
    HEALTH_CHECK_INTERVAL_SECONDS="30",\
    LOG_LEVEL="INFO",\
    ALLOWED_ORIGINS="https://rangoweb-ioelziq27-leenorshns-projects.vercel.app"
```

### Option C : Ajouter les variables une par une

```bash
# Variables obligatoires
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars MONGO_URI="mongodb+srv://user:password@cluster.mongodb.net/rangodb?retryWrites=true&w=majority"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars MONGO_DB_NAME="rangodb"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars JWT_SECRET="your-very-long-and-secure-secret-key-at-least-32-characters-long"

# Variables optionnelles
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars PORT="8080"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars DB_TIMEOUT_SECONDS="5"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars DB_CONNECT_TIMEOUT_SECONDS="10"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars DB_MAX_RETRIES="3"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars HEALTH_CHECK_INTERVAL_SECONDS="30"

gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars LOG_LEVEL="INFO"

# IMPORTANT : CORS pour votre frontend
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars ALLOWED_ORIGINS="https://rangoweb-ioelziq27-leenorshns-projects.vercel.app"
```

---

## ✅ Vérifier les Variables Configurées

### Via la Console
1. Allez sur Cloud Run Console
2. Sélectionnez votre service
3. Cliquez sur l'onglet **"VARIABLES AND SECRETS"**
4. Vous verrez toutes les variables configurées

### Via CLI
```bash
gcloud run services describe rangoapp-backend \
  --region europe-west1 \
  --format="value(spec.template.spec.containers[0].env)"
```

---

## 🔒 Utiliser Google Secret Manager (Recommandé pour Production)

Pour les secrets sensibles comme `JWT_SECRET` et `MONGO_URI`, utilisez Secret Manager :

### 1. Créer les secrets
```bash
# Créer le secret JWT
echo -n "your-very-long-and-secure-secret-key" | \
  gcloud secrets create jwt-secret --data-file=-

# Créer le secret MongoDB URI
echo -n "mongodb+srv://user:password@cluster.mongodb.net/rangodb" | \
  gcloud secrets create mongo-uri --data-file=-
```

### 2. Donner l'accès au service Cloud Run
```bash
# Donner l'accès au secret JWT
gcloud secrets add-iam-policy-binding jwt-secret \
  --member="serviceAccount:YOUR_SERVICE_ACCOUNT@YOUR_PROJECT.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Donner l'accès au secret MongoDB
gcloud secrets add-iam-policy-binding mongo-uri \
  --member="serviceAccount:YOUR_SERVICE_ACCOUNT@YOUR_PROJECT.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

### 3. Utiliser les secrets dans Cloud Run
```bash
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-secrets JWT_SECRET=jwt-secret:latest,MONGO_URI=mongo-uri:latest
```

---

## 🐛 Déboguer les Variables d'Environnement

### Voir les logs du service
```bash
gcloud run services logs read rangoapp-backend \
  --region europe-west1 \
  --limit 50
```

### Activer le debug CORS
```bash
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars CORS_DEBUG="true"
```

Puis vérifiez les logs pour voir les détails CORS :
```bash
gcloud run services logs read rangoapp-backend \
  --region europe-west1 \
  --limit 100 | grep -i cors
```

---

## 📝 Notes Importantes

1. **ALLOWED_ORIGINS** : 
   - Doit contenir l'URL exacte de votre frontend
   - Pas de wildcards supportés (`*.vercel.app` ne fonctionne pas)
   - Pour plusieurs origines, séparez par des virgules : `origin1.com,origin2.com`

2. **JWT_SECRET** :
   - Minimum 32 caractères
   - Utilisez un générateur de secret fort
   - Ne le partagez jamais publiquement

3. **MONGO_URI** :
   - Assurez-vous que votre cluster MongoDB autorise les connexions depuis Cloud Run
   - Ajoutez `0.0.0.0/0` à la whitelist MongoDB pour tester (ou les IPs de Cloud Run)

4. **Après modification** :
   - Cloud Run redéploie automatiquement une nouvelle révision
   - Les changements prennent effet immédiatement
   - Vérifiez les logs pour confirmer que tout fonctionne

---

## 🚀 Commandes Rapides

### Mettre à jour uniquement CORS
```bash
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --update-env-vars ALLOWED_ORIGINS="https://rangoweb-ioelziq27-leenorshns-projects.vercel.app"
```

### Voir toutes les variables actuelles
```bash
gcloud run services describe rangoapp-backend \
  --region europe-west1 \
  --format="yaml(spec.template.spec.containers[0].env)"
```

### Supprimer une variable
```bash
gcloud run services update rangoapp-backend \
  --region europe-west1 \
  --remove-env-vars VARIABLE_NAME
```


































