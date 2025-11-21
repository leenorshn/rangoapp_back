# Déploiement sur Google Cloud Run

Ce guide explique comment déployer l'application RangoApp Backend sur Google Cloud Run.

## Prérequis

1. **Google Cloud SDK** installé et configuré
   ```bash
   # Installer gcloud CLI
   # macOS
   brew install google-cloud-sdk
   
   # Ou télécharger depuis: https://cloud.google.com/sdk/docs/install
   ```

2. **Authentification Google Cloud**
   ```bash
   gcloud auth login
   gcloud config set project YOUR_PROJECT_ID
   ```

3. **Activer les APIs nécessaires**
   ```bash
   gcloud services enable cloudbuild.googleapis.com
   gcloud services enable run.googleapis.com
   gcloud services enable containerregistry.googleapis.com
   ```

## Configuration des Variables d'Environnement

Cloud Run nécessite que vous configuriez les variables d'environnement. Vous pouvez le faire de deux façons:

### Option 1: Via la Console Google Cloud

1. Allez sur [Cloud Run Console](https://console.cloud.google.com/run)
2. Sélectionnez votre service
3. Cliquez sur "EDIT & DEPLOY NEW REVISION"
4. Dans l'onglet "Variables and Secrets", ajoutez:
   - `MONGO_URI` - Votre URI MongoDB
   - `MONGO_DB_NAME` - Nom de la base de données (par défaut: `rangodb`)
   - `JWT_SECRET` - Secret JWT (minimum 32 caractères)
   - `PORT` - Port (Cloud Run définit automatiquement, mais vous pouvez le forcer à 8080)
   - `DB_TIMEOUT_SECONDS` - (optionnel, défaut: 5)
   - `DB_CONNECT_TIMEOUT_SECONDS` - (optionnel, défaut: 10)
   - `DB_MAX_RETRIES` - (optionnel, défaut: 3)
   - `HEALTH_CHECK_INTERVAL_SECONDS` - (optionnel, défaut: 30)
   - `LOG_LEVEL` - (optionnel, défaut: INFO)

### Option 2: Via gcloud CLI

Créez un fichier `env.yaml`:

```yaml
MONGO_URI: "mongodb+srv://user:password@cluster.mongodb.net/rangodb?retryWrites=true&w=majority"
MONGO_DB_NAME: "rangodb"
JWT_SECRET: "your-very-long-and-secure-secret-key-at-least-32-characters-long"
PORT: "8080"
DB_TIMEOUT_SECONDS: "5"
DB_CONNECT_TIMEOUT_SECONDS: "10"
DB_MAX_RETRIES: "3"
HEALTH_CHECK_INTERVAL_SECONDS: "30"
LOG_LEVEL: "INFO"
```

Puis utilisez-le lors du déploiement (voir ci-dessous).

## Méthodes de Déploiement

### Méthode 1: Déploiement Direct avec Docker

```bash
# 1. Build l'image Docker
docker build -t gcr.io/YOUR_PROJECT_ID/rangoapp-backend:latest .

# 2. Push l'image vers Google Container Registry
docker push gcr.io/YOUR_PROJECT_ID/rangoapp-backend:latest

# 3. Déployer sur Cloud Run
gcloud run deploy rangoapp-backend \
  --image gcr.io/YOUR_PROJECT_ID/rangoapp-backend:latest \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --port 8080 \
  --memory 512Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 10 \
  --timeout 300 \
  --concurrency 80 \
  --env-vars-file env.yaml
```

### Méthode 2: Déploiement avec Cloud Build (Recommandé)

```bash
# 1. Soumettre le build à Cloud Build
gcloud builds submit --config cloudbuild.yaml

# 2. Le déploiement se fait automatiquement via cloudbuild.yaml
```

### Méthode 3: Déploiement Source-Based (Buildpack)

```bash
# Cloud Run peut builder directement depuis le code source
gcloud run deploy rangoapp-backend \
  --source . \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --port 8080 \
  --memory 512Mi \
  --env-vars-file env.yaml
```

## Mise à Jour du Service

Pour mettre à jour le service avec de nouvelles variables d'environnement:

```bash
gcloud run services update rangoapp-backend \
  --region us-central1 \
  --update-env-vars MONGO_URI="new-uri",JWT_SECRET="new-secret"
```

## Vérification du Déploiement

1. **Obtenir l'URL du service**
   ```bash
   gcloud run services describe rangoapp-backend --region us-central1 --format 'value(status.url)'
   ```

2. **Tester l'endpoint de santé**
   ```bash
   curl https://YOUR-SERVICE-URL/health
   ```

3. **Tester GraphQL Playground**
   Ouvrez dans votre navigateur: `https://YOUR-SERVICE-URL/`

## Configuration Recommandée pour Production

### Ressources
- **Memory**: 512Mi (minimum) à 1Gi (recommandé pour production)
- **CPU**: 1 (minimum) à 2 (recommandé pour production)
- **Concurrency**: 80 (défaut Cloud Run, ajustez selon vos besoins)
- **Timeout**: 300s (5 minutes, maximum Cloud Run)

### Scaling
- **Min Instances**: 0 (pour économiser) ou 1 (pour éviter cold starts)
- **Max Instances**: 10+ selon votre trafic

### Sécurité
- Utilisez **Secrets Manager** pour les secrets sensibles:
  ```bash
  # Créer un secret
  echo -n "your-jwt-secret" | gcloud secrets create jwt-secret --data-file=-
  
  # Utiliser dans Cloud Run
  gcloud run services update rangoapp-backend \
    --update-secrets JWT_SECRET=jwt-secret:latest
  ```

## Monitoring et Logs

1. **Voir les logs**
   ```bash
   gcloud run services logs read rangoapp-backend --region us-central1
   ```

2. **Monitoring dans la Console**
   - Allez sur [Cloud Run Console](https://console.cloud.google.com/run)
   - Sélectionnez votre service
   - Onglet "LOGS" pour les logs
   - Onglet "METRICS" pour les métriques

## Endpoints Disponibles

Une fois déployé, votre service expose:

- **GraphQL Playground**: `https://YOUR-SERVICE-URL/`
- **GraphQL Endpoint**: `https://YOUR-SERVICE-URL/query`
- **Health Check**: `https://YOUR-SERVICE-URL/health`
- **Readiness**: `https://YOUR-SERVICE-URL/health/ready`
- **Liveness**: `https://YOUR-SERVICE-URL/health/live`

## Dépannage

### Erreur: "Container failed to start"
- Vérifiez les logs: `gcloud run services logs read rangoapp-backend`
- Vérifiez que toutes les variables d'environnement sont définies
- Vérifiez que `MONGO_URI` est correct et accessible depuis Cloud Run

### Erreur: "Connection timeout"
- Vérifiez que votre cluster MongoDB autorise les connexions depuis Cloud Run
- Ajoutez l'IP de Cloud Run à la whitelist MongoDB (ou utilisez 0.0.0.0/0 pour tester)

### Erreur: "Port already in use"
- Cloud Run définit automatiquement `PORT`, ne le modifiez pas dans votre code
- Le serveur utilise déjà `os.Getenv("PORT")` avec fallback sur 8080

## Coûts Estimés

Cloud Run facture:
- **CPU**: Seulement quand le service traite des requêtes
- **Memory**: Seulement quand le service traite des requêtes
- **Requests**: Par million de requêtes

Avec `min-instances: 0`, vous ne payez que lorsque le service est actif.

## Support

Pour plus d'informations:
- [Documentation Cloud Run](https://cloud.google.com/run/docs)
- [Pricing Cloud Run](https://cloud.google.com/run/pricing)

