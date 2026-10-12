#!/bin/bash

# Script de configuration automatique d'un Droplet DigitalOcean
# Usage: ./setup-droplet.sh

set -e

echo "🚀 Configuration du Droplet DigitalOcean pour RangoApp Backend"

# Couleurs pour les messages
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Vérifier que le script est exécuté en root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}❌ Ce script doit être exécuté en tant que root${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Mise à jour du système...${NC}"
apt update && apt upgrade -y

echo -e "${GREEN}✅ Installation de Docker...${NC}"
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
rm get-docker.sh

echo -e "${GREEN}✅ Installation de Docker Compose...${NC}"
apt install docker-compose -y

echo -e "${GREEN}✅ Installation de Nginx...${NC}"
apt install nginx -y

echo -e "${GREEN}✅ Installation de Certbot (pour SSL)...${NC}"
apt install certbot python3-certbot-nginx -y

echo -e "${GREEN}✅ Configuration du firewall...${NC}"
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 8080/tcp
ufw --force enable

echo -e "${GREEN}✅ Création du dossier de l'application...${NC}"
mkdir -p /opt/rangoapp
cd /opt/rangoapp

echo -e "${YELLOW}⚠️  Créez maintenant le fichier .env avec vos variables d'environnement${NC}"
echo -e "${YELLOW}   nano /opt/rangoapp/.env${NC}"

echo -e "${GREEN}✅ Configuration terminée!${NC}"
echo ""
echo "Prochaines étapes:"
echo "1. Créez le fichier .env: nano /opt/rangoapp/.env"
echo "2. Ajoutez vos variables d'environnement (DATABASE_URL, CLERK_JWKS_URL, INTERNAL_CONTEXT_SECRET, etc.)"
echo "3. Configurez Nginx pour votre domaine"
echo "4. Configurez SSL avec: certbot --nginx -d votre-domaine.com"
echo "5. Le déploiement se fera automatiquement via GitHub Actions"
