#!/bin/sh
# Dump the droplet Postgres volume. Keeps 14 days in ./backups.
# Cron runs this as root. Manual use: sh scripts/backup-droplet.sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f .env ]; then
  echo "Missing .env" >&2
  exit 1
fi

mkdir -p "$ROOT/backups"
chmod 700 "$ROOT/backups"
dest="$ROOT/backups/easystock-$(date +%F).dump"

docker compose --env-file .env -f docker-compose.digitalocean.yml exec -T postgres \
  pg_dump --format=custom --no-owner -U easystock_migrator -d easystock > "$dest"

find "$ROOT/backups" -name 'easystock-*.dump' -mtime +14 -delete
echo "backup written to $dest"
