# Guide de Déploiement vers DigitalOcean

Ce guide vous explique comment déployer votre application RangoApp Backend vers DigitalOcean, une alternative beaucoup plus économique que Google Cloud Run.

## 💰 Comparaison des Coûts

### Google Cloud Run
- **Minimum**: ~$10-20/mois (même avec peu de trafic)
- **Facturation**: Par requête + CPU/Mémoire utilisée
- **Problème**: Coûts élevés même avec <20 utilisateurs

### DigitalOcean Options

#### Option 1: App Platform (Recommandé pour débuter)
- **Coût**: $5-12/mois (basic-xxs ou basic-xs)
- **Avantages**:
  - Déploiement automatique depuis GitHub
  - Scaling automatique
  - HTTPS inclus
  - Monitoring intégré
- **Parfait pour**: <20 utilisateurs, trafic modéré

#### Option 2: Droplet avec Docker (Le plus économique)
- **Coût**: $4-6/mois (Basic Droplet 1GB)
- **Avantages**:
  - Contrôle total
  - Coût fixe prévisible
  - Pas de facturation par requête
- **Parfait pour**: Budget serré, contrôle maximum

---

## 🚀 Option 1: Déploiement sur App Platform (Recommandé)

### Prérequis
1. Compte DigitalOcean
2. Repository GitHub
3. Token d'accès DigitalOcean

### Étapes

#### 1. Créer un Token d'Accès DigitalOcean

1. Allez sur https://cloud.digitalocean.com/account/api/tokens
2. Cliquez sur "Generate New Token"
3. Nommez-le (ex: "rangoapp-deploy")
4. Copiez le token (vous ne le verrez qu'une fois)

#### 2. Configurer les Secrets GitHub

1. Allez dans votre repository GitHub
2. Settings → Secrets and variables → Actions
3. Ajoutez les secrets suivants:

```
DIGITALOCEAN_ACCESS_TOKEN: votre_token_digitalocean
```

#### 3. Configurer App Platform via Interface Web

1. Allez sur https://cloud.digitalocean.com/apps
2. Cliquez sur "Create App"
3. Connectez votre repository GitHub
4. Sélectionnez votre repository `rangoapp_back`
5. Configurez:
   - **Type**: Web Service
   - **Dockerfile Path**: `Dockerfile.digitalocean`
   - **HTTP Port**: `8080`
   - **Instance Size**: `Basic XXS` ($5/mois) ou `Basic XS` ($12/mois)
   - **Instance Count**: `1`

#### 4. Configurer les Variables d'Environnement

Dans App Platform, ajoutez ces variables d'environnement:

**Obligatoires:**
```
MONGO_URI=mongodb+srv://user:password@cluster.mongodb.net/rangodb?retryWrites=true&w=majority
MONGO_DB_NAME=rangodb
JWT_SECRET=votre-secret-jwt-tres-long-et-securise
ALLOWED_ORIGINS=https://votre-frontend.vercel.app
```

**Optionnelles:**
```
PORT=8080
LOG_LEVEL=INFO
DB_TIMEOUT_SECONDS=5
DB_CONNECT_TIMEOUT_SECONDS=10
DB_MAX_RETRIES=3
```

#### 5. Configurer le Health Check

Dans App Platform:
- **HTTP Path**: `/health`
- **Initial Delay**: `10s`
- **Period**: `10s`
- **Timeout**: `5s`
- **Success Threshold**: `1`
- **Failure Threshold**: `3`

#### 6. Déployer

1. Cliquez sur "Create Resources"
2. DigitalOcean va:
   - Construire votre image Docker
   - Déployer votre application
   - Configurer HTTPS automatiquement
   - Vous donner une URL (ex: `rangoapp-backend-xxxxx.ondigitalocean.app`)

#### 7. Configurer le Domaine Personnalisé (Optionnel)

1. Dans App Platform → Settings → Domains
2. Ajoutez votre domaine
3. Suivez les instructions DNS

---

## 🐳 Option 2: Déploiement sur Droplet avec Docker (Plus Économique)

### Prérequis
1. Compte DigitalOcean
2. Droplet créé (Ubuntu 22.04 LTS, 1GB RAM minimum)
3. Container Registry DigitalOcean

### Étapes

#### 1. Créer un Droplet

1. Allez sur https://cloud.digitalocean.com/droplets/new
2. Configurez:
   - **Image**: Ubuntu 22.04 LTS
   - **Plan**: Basic ($4-6/mois pour 1GB RAM)
   - **Region**: Choisissez la plus proche de vos utilisateurs
   - **Authentication**: SSH keys (recommandé)
3. Créez le Droplet

#### 2. Créer un Container Registry

1. Allez sur https://cloud.digitalocean.com/registry
2. Créez un nouveau registry
3. Notez le nom du registry

#### 3. Configurer les Secrets GitHub

Ajoutez ces secrets dans GitHub:

```
DIGITALOCEAN_ACCESS_TOKEN: votre_token_digitalocean
DIGITALOCEAN_REGISTRY_NAME: nom_de_votre_registry
DROPLET_IP: ip_de_votre_droplet
DROPLET_USER: root (ou votre utilisateur)
DROPLET_SSH_KEY: votre_clé_ssh_privée
```

#### 4. Préparer le Droplet

Connectez-vous au Droplet via SSH:

```bash
ssh root@VOTRE_DROPLET_IP
```

Installez Docker:

```bash
# Mettre à jour le système
apt update && apt upgrade -y

# Installer Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Installer Docker Compose
apt install docker-compose -y

# Vérifier l'installation
docker --version
docker-compose --version
```

Créez le dossier pour l'application:

```bash
mkdir -p /opt/rangoapp
cd /opt/rangoapp
```

Créez le fichier `.env`:

```bash
nano .env
```

Ajoutez toutes vos variables d'environnement (MONGO_URI, JWT_SECRET, etc.)

#### 5. Configurer le Firewall

```bash
# Autoriser le port 8080
ufw allow 8080/tcp
ufw allow 22/tcp
ufw enable
```

#### 6. Configurer Nginx comme Reverse Proxy (Recommandé)

Installez Nginx:

```bash
apt install nginx -y
```

Créez la configuration:

```bash
nano /etc/nginx/sites-available/rangoapp
```

Ajoutez:

```nginx
server {
    listen 80;
    server_name votre-domaine.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

Activez la configuration:

```bash
ln -s /etc/nginx/sites-available/rangoapp /etc/nginx/sites-enabled/
nginx -t
systemctl restart nginx
```

#### 7. Configurer SSL avec Let's Encrypt (Recommandé)

```bash
apt install certbot python3-certbot-nginx -y
certbot --nginx -d votre-domaine.com
```

#### 8. Déployer via GitHub Actions

Le workflow GitHub Actions va automatiquement:
1. Construire l'image Docker
2. La pousser vers le Container Registry
3. Se connecter au Droplet via SSH
4. Puller la nouvelle image
5. Redémarrer le container

#### 9. Vérifier le Déploiement

```bash
# Vérifier que le container tourne
docker ps

# Voir les logs
docker logs rangoapp-backend

# Tester le health check
curl http://localhost:8080/health
```

---

## 🔄 Workflow CI/CD

### Déploiement Automatique

Les workflows GitHub Actions sont configurés pour:
- **Déployer automatiquement** à chaque push sur `main` ou `master`
- **Exécuter les tests** avant le déploiement
- **Construire l'image Docker** optimisée
- **Déployer** vers DigitalOcean

### Déploiement Manuel

Vous pouvez aussi déclencher un déploiement manuel:
1. Allez dans Actions → Deploy to DigitalOcean
2. Cliquez sur "Run workflow"

---

## 📊 Monitoring et Logs

### App Platform
- Logs disponibles dans l'interface DigitalOcean
- Monitoring automatique des métriques
- Alertes configurables

### Droplet
```bash
# Voir les logs en temps réel
docker logs -f rangoapp-backend

# Voir l'utilisation des ressources
docker stats rangoapp-backend

# Voir les logs système
journalctl -u docker -f
```

---

## 🔧 Maintenance

### Mettre à jour l'Application

1. Faites vos modifications
2. Committez et pushez vers `main`
3. Le déploiement se fait automatiquement

### Redémarrer l'Application

**App Platform:**
- Interface web → Restart

**Droplet:**
```bash
docker restart rangoapp-backend
```

### Mettre à jour les Variables d'Environnement

**App Platform:**
- Interface web → Settings → App-Level Environment Variables

**Droplet:**
```bash
# Éditer le fichier .env
nano /opt/rangoapp/.env

# Redémarrer le container
docker restart rangoapp-backend
```

---

## 💡 Optimisations

### Pour Réduire les Coûts

1. **Utilisez un Droplet Basic 1GB** ($4/mois) si vous avez <20 utilisateurs
2. **Désactivez le scaling automatique** si pas nécessaire
3. **Utilisez le Container Registry** (gratuit jusqu'à 500MB)
4. **Configurez les backups** seulement si nécessaire

### Pour Améliorer les Performances

1. **Utilisez un Droplet avec plus de RAM** si vous avez des pics de trafic
2. **Configurez un CDN** pour les assets statiques
3. **Utilisez un Load Balancer** si vous avez plusieurs instances

---

## 🆘 Dépannage

### L'application ne démarre pas

```bash
# Vérifier les logs
docker logs rangoapp-backend

# Vérifier les variables d'environnement
docker exec rangoapp-backend env

# Vérifier la connexion MongoDB
docker exec rangoapp-backend ping -c 3 your-mongodb-host
```

### Problèmes de connexion

```bash
# Vérifier que le port est ouvert
netstat -tulpn | grep 8080

# Vérifier le firewall
ufw status

# Tester localement
curl http://localhost:8080/health
```

### Problèmes de déploiement

1. Vérifiez les secrets GitHub
2. Vérifiez les logs GitHub Actions
3. Vérifiez la connexion SSH au Droplet
4. Vérifiez les permissions du Container Registry

---

## 📝 Checklist de Déploiement

### Avant le Déploiement
- [ ] Token DigitalOcean créé
- [ ] Secrets GitHub configurés
- [ ] Variables d'environnement préparées
- [ ] Dockerfile testé localement
- [ ] Tests passent

### App Platform
- [ ] App créée dans DigitalOcean
- [ ] Repository GitHub connecté
- [ ] Variables d'environnement configurées
- [ ] Health check configuré
- [ ] Domaine configuré (optionnel)

### Droplet
- [ ] Droplet créé
- [ ] Docker installé
- [ ] Container Registry créé
- [ ] Fichier .env créé
- [ ] Nginx configuré
- [ ] SSL configuré
- [ ] Firewall configuré

### Après le Déploiement
- [ ] Health check fonctionne
- [ ] Application accessible
- [ ] Logs vérifiés
- [ ] Monitoring configuré
- [ ] Backups configurés (optionnel)

---

## 🎯 Recommandation Finale

Pour **<20 utilisateurs**, je recommande:

1. **Début**: App Platform Basic XXS ($5/mois) - Le plus simple
2. **Budget serré**: Droplet Basic 1GB ($4/mois) - Le plus économique
3. **Croissance**: Passez à App Platform Basic XS ($12/mois) ou Droplet 2GB ($12/mois)

**Économies estimées**: 50-75% par rapport à Cloud Run! 💰

---

## 📞 Support

- Documentation DigitalOcean: https://docs.digitalocean.com
- Support DigitalOcean: https://cloud.digitalocean.com/support
- GitHub Issues: Pour les problèmes de déploiement
