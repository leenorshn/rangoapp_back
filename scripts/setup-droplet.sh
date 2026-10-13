#!/bin/sh
# Prepare an Ubuntu droplet for EasyStock. Run as root from a clone of this repository:
#   sudo sh scripts/setup-droplet.sh

set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Run this script as root." >&2
  exit 1
fi

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get upgrade -y
apt-get install -y ca-certificates curl openssl ufw

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com -o /tmp/get-docker.sh
  sh /tmp/get-docker.sh
  rm -f /tmp/get-docker.sh
fi
systemctl enable --now docker
docker compose version

if [ ! -f /swapfile ]; then
  fallocate -l 1G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=1024
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  grep -q '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

if [ ! -f "$ROOT/.env" ]; then
  umask 077
  pg="$(openssl rand -hex 24)"
  app="$(openssl rand -hex 24)"
  identity="$(openssl rand -hex 24)"
  job="$(openssl rand -hex 24)"
  secret="$(openssl rand -hex 32)"
  cat > "$ROOT/.env" <<EOF
POSTGRES_PASSWORD=$pg
APP_DB_PASSWORD=$app
IDENTITY_DB_PASSWORD=$identity
JOB_DB_PASSWORD=$job
INTERNAL_CONTEXT_SECRET=$secret
CLERK_WEBHOOK_SECRET=
CLERK_JWKS_URL=
CLERK_ISSUER=
CLERK_AUTHORIZED_PARTIES=
S3_ENDPOINT=
S3_REGION=nyc3
S3_BUCKET=
S3_ACCESS_KEY_ID=
S3_SECRET_ACCESS_KEY=
S3_USE_PATH_STYLE=false
SITE_ADDRESS=:80
EASYSTOCK_IMAGE=easystock:local
EASYSTOCK_PULL_POLICY=missing
EOF
  chmod 600 "$ROOT/.env"
  echo "Wrote $ROOT/.env with generated database passwords."
else
  echo "Kept existing $ROOT/.env"
fi

cat > /etc/cron.d/easystock <<EOF
SHELL=/bin/sh
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
0 3 * * * root $ROOT/scripts/backup-droplet.sh >> /var/log/easystock-backup.log 2>&1
EOF
chmod 644 /etc/cron.d/easystock

echo
echo "Next:"
echo "1. Edit $ROOT/.env (Clerk, Spaces, SITE_ADDRESS)."
echo "2. sudo sh $ROOT/scripts/deploy.sh"
echo "3. Check http://<droplet-ip>/health"
