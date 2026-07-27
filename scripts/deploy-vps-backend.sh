#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_ENVIRONMENT="${1:-}"
BACKEND_IMAGE="${2:-}"
SHARED_DIR="${C2C_SHARED_DIR:-/opt/c2cmarket/shared}"
HEALTH_RETRIES="${C2C_HEALTH_RETRIES:-30}"
HEALTH_INTERVAL_SECONDS="${C2C_HEALTH_INTERVAL_SECONDS:-2}"

case "${DEPLOY_ENVIRONMENT}" in
  production)
    COMPOSE_PROJECT="c2c-prod"
    ENV_FILE="${SHARED_DIR}/.env.production"
    BACKEND_PORT="8080"
    ;;
  staging)
    COMPOSE_PROJECT="c2c-staging"
    ENV_FILE="${SHARED_DIR}/.env.staging"
    BACKEND_PORT="8081"
    ;;
  *)
    echo "Deployment environment must be production or staging: ${DEPLOY_ENVIRONMENT}" >&2
    exit 2
    ;;
esac

if [[ ! "${BACKEND_IMAGE}" =~ ^ghcr\.io/xiangrikuil/c2cmarket-backend:[0-9a-f]{40}$ ]]; then
  echo "Backend image must use the immutable GHCR commit tag: ${BACKEND_IMAGE}" >&2
  exit 2
fi
if [[ ! "${HEALTH_RETRIES}" =~ ^[1-9][0-9]*$ ]]; then
  echo "C2C_HEALTH_RETRIES must be a positive integer: ${HEALTH_RETRIES}" >&2
  exit 2
fi
if [[ ! "${HEALTH_INTERVAL_SECONDS}" =~ ^[0-9]+$ ]]; then
  echo "C2C_HEALTH_INTERVAL_SECONDS must be a non-negative integer: ${HEALTH_INTERVAL_SECONDS}" >&2
  exit 2
fi
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Deployment environment file not found: ${ENV_FILE}" >&2
  exit 1
fi

for command_name in docker curl; do
  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "Required deployment command not found: ${command_name}" >&2
    exit 1
  fi
done

export BACKEND_IMAGE BACKEND_PORT
compose=(
  docker compose
  -p "${COMPOSE_PROJECT}"
  --env-file "${ENV_FILE}"
  -f "${ROOT_DIR}/compose.yaml"
  -f "${ROOT_DIR}/compose.prod.yaml"
)

echo "Validating ${DEPLOY_ENVIRONMENT} Compose configuration."
"${compose[@]}" config --quiet

echo "Starting ${DEPLOY_ENVIRONMENT} PostgreSQL."
"${compose[@]}" up -d --wait postgres

if [[ "${DEPLOY_ENVIRONMENT}" == "production" ]]; then
  echo "Backing up production PostgreSQL before migration."
  ENV_FILE="${ENV_FILE}" \
    COMPOSE_PROJECT="${COMPOSE_PROJECT}" \
    "${ROOT_DIR}/scripts/backup-production-postgres.sh"
fi

echo "Pulling immutable backend image ${BACKEND_IMAGE}."
"${compose[@]}" --profile app pull backend

echo "Applying ${DEPLOY_ENVIRONMENT} database migrations."
"${compose[@]}" --profile migrate run --rm migrate

echo "Starting ${DEPLOY_ENVIRONMENT} backend without a local build."
"${compose[@]}" --profile app up -d --no-build backend

health_url="http://127.0.0.1:${BACKEND_PORT}/health"
ready_url="http://127.0.0.1:${BACKEND_PORT}/readyz"
for ((attempt = 1; attempt <= HEALTH_RETRIES; attempt += 1)); do
  if curl -fsS "${health_url}" >/dev/null && curl -fsS "${ready_url}" >/dev/null; then
    echo "${DEPLOY_ENVIRONMENT} backend is healthy and ready on port ${BACKEND_PORT}."
    exit 0
  fi
  if ((attempt < HEALTH_RETRIES)); then
    sleep "${HEALTH_INTERVAL_SECONDS}"
  fi
done

echo "${DEPLOY_ENVIRONMENT} backend failed health/readiness checks after ${HEALTH_RETRIES} attempts." >&2
"${compose[@]}" --profile app ps >&2
exit 1
