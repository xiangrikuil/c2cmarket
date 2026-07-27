#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ENVIRONMENT="${1:-}"
GIT_SHA="${2:-}"
BACKEND_IMAGE="${3:-}"
ARCHIVE_PATH="${4:-}"
VPS_ROOT="${C2C_VPS_ROOT:-/opt/c2cmarket}"

case "${DEPLOY_ENVIRONMENT}" in
  production)
    CURRENT_LINK="${VPS_ROOT}/current"
    ;;
  staging)
    CURRENT_LINK="${VPS_ROOT}/staging-current"
    ;;
  *)
    echo "Release environment must be production or staging: ${DEPLOY_ENVIRONMENT}" >&2
    exit 2
    ;;
esac

if [[ ! "${GIT_SHA}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Release Git SHA must contain 40 lowercase hexadecimal characters: ${GIT_SHA}" >&2
  exit 2
fi
expected_image="ghcr.io/xiangrikuil/c2cmarket-backend:${GIT_SHA}"
if [[ "${BACKEND_IMAGE}" != "${expected_image}" ]]; then
  echo "Release image does not match the tested Git SHA: ${BACKEND_IMAGE}" >&2
  exit 2
fi
expected_archive="/tmp/c2cmarket-release-${DEPLOY_ENVIRONMENT}-${GIT_SHA}.tar.gz"
if [[ "${ARCHIVE_PATH}" != "${expected_archive}" ]]; then
  echo "Unexpected release archive path: ${ARCHIVE_PATH}" >&2
  exit 2
fi
if [[ ! -f "${ARCHIVE_PATH}" ]]; then
  echo "Release archive not found: ${ARCHIVE_PATH}" >&2
  exit 1
fi
if [[ -e "${CURRENT_LINK}" && ! -L "${CURRENT_LINK}" ]]; then
  echo "Current release path must be absent or a symlink: ${CURRENT_LINK}" >&2
  exit 1
fi

release_dir="${VPS_ROOT}/releases/${DEPLOY_ENVIRONMENT}/${GIT_SHA}"
mkdir -p "${release_dir}"
tar -xzf "${ARCHIVE_PATH}" -C "${release_dir}"

deploy_script="${release_dir}/scripts/deploy-vps-backend.sh"
if [[ ! -f "${deploy_script}" ]]; then
  echo "Release archive is missing the backend deployment script: ${deploy_script}" >&2
  exit 1
fi
chmod 750 "${deploy_script}" "${release_dir}/scripts/backup-production-postgres.sh"

"${deploy_script}" "${DEPLOY_ENVIRONMENT}" "${BACKEND_IMAGE}"

ln -sfn "${release_dir}" "${CURRENT_LINK}"
rm -f "${ARCHIVE_PATH}"
echo "Installed ${DEPLOY_ENVIRONMENT} release ${GIT_SHA} at ${CURRENT_LINK}."
