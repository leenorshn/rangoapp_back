#!/bin/sh
# Build or pull EasyStock, migrate, then start api, grpc and Caddy.
# Run from the droplet, in the repository root: sh scripts/deploy.sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f .env ]; then
  echo "Missing .env. Run scripts/setup-droplet.sh or copy env.digitalocean.example to .env." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
. ./.env
set +a

if [ -z "${INTERNAL_CONTEXT_SECRET:-}" ] || [ "${#INTERNAL_CONTEXT_SECRET}" -lt 32 ]; then
  echo "INTERNAL_CONTEXT_SECRET must be at least 32 characters." >&2
  exit 1
fi

compose() {
  docker compose --env-file .env -f docker-compose.digitalocean.yml "$@"
}

compose pull postgres caddy

policy="${EASYSTOCK_PULL_POLICY:-missing}"
if [ "$policy" = "always" ]; then
  if [ -n "${DIGITALOCEAN_ACCESS_TOKEN:-}" ]; then
    printf '%s' "$DIGITALOCEAN_ACCESS_TOKEN" | docker login registry.digitalocean.com -u "$DIGITALOCEAN_ACCESS_TOKEN" --password-stdin
  fi
  compose --profile migrate pull migrate api grpc
else
  compose build api
fi

compose up -d --wait postgres
compose --profile migrate run --rm --no-deps migrate
compose up -d --wait --no-deps --force-recreate --remove-orphans api grpc caddy

if ! compose ps --status running --services | grep -qx grpc; then
  echo "grpc is not running" >&2
  compose logs --tail=80 grpc
  exit 1
fi

echo
if ! compose exec -T api wget -q -O - http://127.0.0.1:8080/health; then
  echo
  echo "API health check failed" >&2
  compose logs --tail=80 api grpc
  exit 1
fi
echo

if [ "${SITE_ADDRESS:-:80}" = ":80" ]; then
  i=0
  until out="$(curl -fsS http://127.0.0.1/health)"; do
    i=$((i + 1))
    if [ "$i" -gt 20 ]; then
      echo "Caddy did not answer /health" >&2
      compose logs --tail=40 caddy api
      exit 1
    fi
    sleep 1
  done
  printf '%s\n' "$out"
fi

compose ps
