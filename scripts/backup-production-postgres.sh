#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ENV_FILE:-${ROOT_DIR}/.env.production}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-c2c-prod}"
BACKUP_DIR="${BACKUP_DIR:-${XDG_STATE_HOME:-${HOME}/.local/state}/c2cmarket/backups/production}"
R2_REMOTE="${R2_REMOTE:-c2cmarket-r2}"
R2_BUCKET="${R2_BUCKET:-c2cmarket-backups}"
R2_PREFIX="${R2_PREFIX:-postgres/production}"
LOCAL_RETENTION_DAYS="${LOCAL_RETENTION_DAYS:-7}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Production env file not found: ${ENV_FILE}" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 1
fi
if ! command -v rclone >/dev/null 2>&1; then
  echo "rclone is required" >&2
  exit 1
fi

mkdir -p "${BACKUP_DIR}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_name="c2cmarket-production-${timestamp}.dump"
backup_path="${BACKUP_DIR}/${backup_name}"
checksum_path="${backup_path}.sha256"

# pg_dump 在生产 PostgreSQL 容器内执行，凭据只从本机忽略的 env 文件读取。
docker compose \
  -p "${COMPOSE_PROJECT}" \
  --env-file "${ENV_FILE}" \
  -f "${ROOT_DIR}/compose.yaml" \
  -f "${ROOT_DIR}/compose.prod.yaml" \
  exec -T postgres sh -c \
  'exec pg_dump --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --format=custom --no-owner --no-privileges' \
  >"${backup_path}"

if [[ ! -s "${backup_path}" ]]; then
  echo "pg_dump produced an empty backup: ${backup_path}" >&2
  exit 1
fi

(
  cd "${BACKUP_DIR}"
  shasum -a 256 "${backup_name}" >"${backup_name}.sha256"
)

# 上传失败时 set -e 会保留本地 dump 和校验文件，便于排障后重传。
rclone copyto "${backup_path}" "${R2_REMOTE}:${R2_BUCKET}/${R2_PREFIX}/${backup_name}"
rclone copyto "${checksum_path}" "${R2_REMOTE}:${R2_BUCKET}/${R2_PREFIX}/${backup_name}.sha256"

# 仅在远端上传全部成功后清理过期本地副本；R2 的 30 天保留由 Bucket Lifecycle 管理。
find "${BACKUP_DIR}" -type f \( -name 'c2cmarket-production-*.dump' -o -name 'c2cmarket-production-*.dump.sha256' \) -mtime "+${LOCAL_RETENTION_DAYS}" -delete

echo "Uploaded ${backup_name} to ${R2_REMOTE}:${R2_BUCKET}/${R2_PREFIX}/"
