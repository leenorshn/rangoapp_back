# Déploiement DigitalOcean

EasyStock publie l’API HTTP (`GET /health`, `POST /graphql`). Le processus gRPC, le worker de rappels et PostgreSQL restent sur le réseau privé. Le port 7443 n’est pas exposé.

Deux chemins sont prêts :

- **Droplet** — un serveur, Docker Compose, Caddy. C’est le chemin à utiliser.
- **App Platform** — spécification `.do/app.yaml`, base managée, API, gRPC et worker de rappels.

## Droplet

### 1. Créer le serveur

Ubuntu LTS, clé SSH, région proche des utilisateurs. 2 Go de RAM si l’image est construite sur le serveur. 1 Go suffit quand l’image vient de GitHub Actions.

### 2. Installer

```sh
git clone https://github.com/leenorshn/rangoapp_back.git /opt/easystock
cd /opt/easystock
sudo sh scripts/setup-droplet.sh
```

Le script installe Docker, ouvre les ports 22, 80 et 443, ajoute 1 Go de swap, génère `/opt/easystock/.env` (mots de passe hexadécimaux, permissions `600`) et programme une sauvegarde quotidienne.

### 3. Compléter `.env`

Fichier modèle : `env.digitalocean.example`.

- `AUTH_JWT_SECRET` et `AUTH_OTP_SECRET` (au moins 32 caractères, générés par `setup-droplet.sh`)
- `SMS_PROVIDER=uzashop`, `SMS_UZA_TOKEN`, et au besoin `SMS_UZA_SENDER` et `SMS_UZA_BASE_URL`. L'ancien mode `SMS_PROVIDER=http` reste disponible avec `SMS_HTTP_URL` et `SMS_HTTP_TOKEN`. `PUSH_PROVIDER=http`, `PUSH_HTTP_URL`, `PUSH_HTTP_TOKEN`
- `AUTH_CORS_ORIGINS`, liste explicite des origines du client web
- `PHONE_DEFAULT_REGION=CD`
- Spaces, si les photos et reçus sont utilisés : `S3_ENDPOINT` (`https://<région>.digitaloceanspaces.com`), `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_USE_PATH_STYLE=false`
- `SITE_ADDRESS=:80` pour HTTP sur l’adresse IP du droplet. Avec un nom de domaine (`api.example.com`), Caddy obtient le certificat. Le DNS doit pointer vers le droplet avant le redéploiement.

Les mots de passe sont placés dans les URL PostgreSQL : hexadécimal, sans espace ni `$`.

`GRAPHQL_PLAYGROUND` reste absent en production.

### 4. Démarrer

```sh
sudo sh /opt/easystock/scripts/deploy.sh
```

Le script construit `Dockerfile.digitalocean`, applique les migrations, pose les mots de passe des rôles `easystock_app`, `easystock_identity` et `easystock_job`, puis démarre Postgres, Redis, gRPC, l’API, le worker et Caddy.

Vérification :

```sh
curl -fsS http://127.0.0.1/health
```

Réponse attendue : `ok`. Avec un domaine : `curl -fsS https://api.example.com/health`.

URL publiques :

- `POST /graphql`
- `GET /health`

`APP_ENV=production` refuse `SMS_PROVIDER=dev` et `PUSH_PROVIDER=dev`. `SMS_PROVIDER=uzashop` exige `SMS_UZA_TOKEN`. `SMS_PROVIDER=http` exige `SMS_HTTP_URL`. Sans cela, l’API et le worker ne démarrent pas.

### 5. GitHub Actions

Le workflow `.github/workflows/deploy-digitalocean-droplet.yml` lance les tests à chaque push sur `main`. Le déploiement ne part que manuellement, ou automatiquement si la variable de dépôt `DIGITALOCEAN_AUTO_DEPLOY` vaut `true`.

Secrets du dépôt :

| Secret | Valeur |
|---|---|
| `DIGITALOCEAN_ACCESS_TOKEN` | Jeton API DigitalOcean (lecture/écriture registry) |
| `DIGITALOCEAN_REGISTRY_NAME` | Nom du Container Registry |
| `DROPLET_IP` | Adresse du droplet |
| `DROPLET_USER` | `root`, ou un utilisateur du groupe `docker` |
| `DROPLET_SSH_KEY` | Clé privée SSH |

Avant le premier lancement : le registry existe, `setup-droplet.sh` a déjà créé `.env`, et le dépôt est cloné dans `/opt/easystock`. L’action met à jour les fichiers de déploiement, remplace `EASYSTOCK_IMAGE` par `registry.digitalocean.com/<registry>/easystock:<sha>`, puis exécute `scripts/deploy.sh`.

Actions → **Deploy to DigitalOcean Droplet** → **Run workflow**.

### Sauvegardes

Chaque nuit, `scripts/backup-droplet.sh` écrit un dump custom dans `/opt/easystock/backups` et supprime ceux de plus de 14 jours. Restauration, Postgres arrêté aux écritures :

```sh
cd /opt/easystock
docker compose --env-file .env -f docker-compose.digitalocean.yml exec -T postgres \
  pg_restore --clean --if-exists --no-owner -U easystock_migrator -d easystock < backups/easystock-YYYY-MM-DD.dump
```

### Conteneurs

`docker compose --env-file .env -f docker-compose.digitalocean.yml ps`

Journaux : `docker compose --env-file .env -f docker-compose.digitalocean.yml logs -f api grpc`.

Si un ancien conteneur `rangoapp-backend` tourne encore : `docker rm -f rangoapp-backend`.

## App Platform

Importer `.do/app.yaml` (Apps → Create → Edit app spec, ou `doctl apps create --spec .do/app.yaml`). Le dépôt GitHub est `leenorshn/rangoapp_back`, branche `main`.

La base `easystock-db` est une base de développement PostgreSQL 16. Passez `production: true` dans la spec pour une base de production.

Renseigner les secrets avant d’attendre un déploiement sain. `DATABASE_URL` de la migration est lié à l’utilisateur administrateur de la base managée. Les trois mots de passe de rôles servent à `easystock migrate` (`SET_ROLE_PASSWORDS=true`). Les mêmes mots de passe entrent dans les URL de l’API, avec l’hôte, le port et le nom de base affichés dans le panneau DigitalOcean (souvent `defaultdb`, port `25060`) :

```text
postgres://easystock_app:<APP_DB_PASSWORD>@<hôte>:25060/defaultdb?sslmode=require
postgres://easystock_identity:<IDENTITY_DB_PASSWORD>@<hôte>:25060/defaultdb?sslmode=require
```

`DATABASE_URL_APP` (service gRPC et service API) utilise `easystock_app`. `DATABASE_URL_IDENTITY` utilise `easystock_identity`. `DATABASE_URL_JOB` (worker) utilise `easystock_job`. `COMMERCE_GRPC_ADDR` est `grpc:7443`. Le port gRPC est interne à l’application. `GRPC_INSECURE_DEV=true` est posé par la spec parce que l’écoute n’est pas en loopback ; le port n’a pas de route publique.

`INTERNAL_CONTEXT_SECRET` fait au moins 32 caractères, identique pour l’API et gRPC.

`GRAPHQL_PLAYGROUND=true` sur le service API ouvre `GET /playground` et l’introspection, le temps du test du MVP. Retirer la variable ensuite.

Spaces : mêmes variables que sur le droplet, région du bucket, `S3_USE_PATH_STYLE=false`.

Le workflow **Deploy to DigitalOcean App Platform** est manuel. Secret supplémentaire : `DIGITALOCEAN_APP_ID`. Il reconstruit l’application déjà créée, sans remplacer les secrets.

## Vérifier

- [ ] `GET /health` répond `ok` (le processus API écoute ; cette route ne vérifie pas PostgreSQL)
- [ ] Les ports 5432 et 7443 ne répondent pas depuis l’extérieur
- [ ] `AUTH_JWT_SECRET`, `AUTH_OTP_SECRET`, `SMS_PROVIDER=uzashop` avec `SMS_UZA_TOKEN`, et `PUSH_PROVIDER=http` sont renseignés
- [ ] Le worker journalise `easystock worker ticking every 5 minutes`
- [ ] Le bucket Spaces existe dans la même région que `S3_REGION`
- [ ] `docker compose ... logs grpc` affiche `easystock grpc listening`
- [ ] Une sauvegarde a été écrite dans `/opt/easystock/backups`
