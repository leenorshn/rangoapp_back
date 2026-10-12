#!/bin/sh
set -eu
if [ -z "${DATABASE_URL:-}" ] || [ -z "${1:-}" ]; then
  echo "usage: DATABASE_URL=... scripts/restore.sh easystock.dump" >&2
  exit 1
fi
pg_restore --clean --if-exists --no-owner --dbname "$DATABASE_URL" "$1"
echo "restore finished"
