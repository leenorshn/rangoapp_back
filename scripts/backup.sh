#!/bin/sh
set -eu
if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi
dest="${1:-easystock.dump}"
pg_dump --format=custom --no-owner --file "$dest" "$DATABASE_URL"
echo "backup written to $dest"
