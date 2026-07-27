#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/c2cmarket-release-test.XXXXXX")"
STAGING_SHA="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
PRODUCTION_SHA="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

cleanup() {
  rm -rf "${TEST_ROOT}"
  rm -f \
    "/tmp/c2cmarket-release-staging-${STAGING_SHA}.tar.gz" \
    "/tmp/c2cmarket-release-production-${PRODUCTION_SHA}.tar.gz"
}
trap cleanup EXIT

fail() {
  echo "release test failed: $*" >&2
  exit 1
}

mkdir -p "${TEST_ROOT}/bin" "${TEST_ROOT}/shared" "${TEST_ROOT}/home"
touch "${TEST_ROOT}/shared/.env.production" "${TEST_ROOT}/shared/.env.staging"

cat >"${TEST_ROOT}/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'docker %s\n' "$*" >>"${CALL_LOG}"
printf 'docker-env BACKEND_PORT=%s\n' "${BACKEND_PORT:-}" >>"${CALL_LOG}"
if [[ -n "${FAIL_DOCKER_PATTERN:-}" && "$*" == *"${FAIL_DOCKER_PATTERN}"* ]]; then
  exit 42
fi
if [[ "$*" == *"exec -T postgres"* ]]; then
  printf 'fake-postgres-dump\n'
fi
EOF

cat >"${TEST_ROOT}/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'curl %s\n' "$*" >>"${CALL_LOG}"
EOF

cat >"${TEST_ROOT}/bin/rclone" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'rclone %s\n' "$*" >>"${CALL_LOG}"
EOF

chmod +x "${TEST_ROOT}/bin/docker" "${TEST_ROOT}/bin/curl" "${TEST_ROOT}/bin/rclone"
export PATH="${TEST_ROOT}/bin:${PATH}"
export HOME="${TEST_ROOT}/home"
export CALL_LOG="${TEST_ROOT}/calls.log"
export C2C_SHARED_DIR="${TEST_ROOT}/shared"
export C2C_HEALTH_RETRIES=1
export C2C_HEALTH_INTERVAL_SECONDS=0
export BACKUP_DIR="${TEST_ROOT}/backups"
export R2_REMOTE=test-r2
export R2_BUCKET=test-bucket
export LOCAL_RETENTION_DAYS=7

staging_image="ghcr.io/xiangrikuil/c2cmarket-backend:${STAGING_SHA}"
: >"${CALL_LOG}"
"${ROOT_DIR}/scripts/deploy-vps-backend.sh" staging "${staging_image}"
if grep -q '^rclone ' "${CALL_LOG}"; then
  fail "staging deployment must not run the production backup"
fi
grep -q -- '--profile app pull backend' "${CALL_LOG}" || fail "staging image was not pulled"
grep -q -- '--profile migrate run --rm migrate' "${CALL_LOG}" || fail "staging migration did not run"
grep -q -- '--profile app up -d --no-build backend' "${CALL_LOG}" || fail "staging backend was not started without a build"
grep -q '^docker-env BACKEND_PORT=8081$' "${CALL_LOG}" || fail "staging Compose did not receive port 8081"
grep -q '127.0.0.1:8081/readyz' "${CALL_LOG}" || fail "staging readiness did not use port 8081"

production_image="ghcr.io/xiangrikuil/c2cmarket-backend:${PRODUCTION_SHA}"
: >"${CALL_LOG}"
"${ROOT_DIR}/scripts/deploy-vps-backend.sh" production "${production_image}"
backup_line="$(grep -n '^rclone ' "${CALL_LOG}" | head -n 1 | cut -d: -f1)"
migration_line="$(grep -n -- '--profile migrate run --rm migrate' "${CALL_LOG}" | head -n 1 | cut -d: -f1)"
[[ -n "${backup_line}" && -n "${migration_line}" ]] || fail "production backup or migration call is missing"
((backup_line < migration_line)) || fail "production backup must finish before migration"
grep -q '^docker-env BACKEND_PORT=8080$' "${CALL_LOG}" || fail "production Compose did not receive port 8080"
grep -q '127.0.0.1:8080/readyz' "${CALL_LOG}" || fail "production readiness did not use port 8080"

: >"${CALL_LOG}"
if FAIL_DOCKER_PATTERN='--profile migrate run --rm migrate' \
  "${ROOT_DIR}/scripts/deploy-vps-backend.sh" staging "${staging_image}"; then
  fail "migration failure must fail the deployment"
fi

release_source="${TEST_ROOT}/release-source"
mkdir -p "${release_source}/scripts"
cat >"${release_source}/scripts/deploy-vps-backend.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s %s\n' "$1" "$2" >>"${INSTALL_LOG}"
if [[ "${INSTALL_DEPLOY_FAILURE:-0}" == "1" ]]; then
  exit 17
fi
EOF
cat >"${release_source}/scripts/backup-production-postgres.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "${release_source}/scripts/deploy-vps-backend.sh" "${release_source}/scripts/backup-production-postgres.sh"

staging_archive="/tmp/c2cmarket-release-staging-${STAGING_SHA}.tar.gz"
tar -czf "${staging_archive}" -C "${release_source}" .
export C2C_VPS_ROOT="${TEST_ROOT}/vps"
export INSTALL_LOG="${TEST_ROOT}/install.log"
mkdir -p "${C2C_VPS_ROOT}/old"
ln -s "${C2C_VPS_ROOT}/old" "${C2C_VPS_ROOT}/staging-current"
"${ROOT_DIR}/scripts/install-vps-release.sh" \
  staging "${STAGING_SHA}" "${staging_image}" "${staging_archive}"
expected_staging_release="${C2C_VPS_ROOT}/releases/staging/${STAGING_SHA}"
[[ "$(readlink "${C2C_VPS_ROOT}/staging-current")" == "${expected_staging_release}" ]] || \
  fail "successful staging install did not update staging-current"
[[ ! -e "${staging_archive}" ]] || fail "successful install did not remove the uploaded archive"

production_archive="/tmp/c2cmarket-release-production-${PRODUCTION_SHA}.tar.gz"
tar -czf "${production_archive}" -C "${release_source}" .
if INSTALL_DEPLOY_FAILURE=1 "${ROOT_DIR}/scripts/install-vps-release.sh" \
  production "${PRODUCTION_SHA}" "${production_image}" "${production_archive}"; then
  fail "failed deployment must fail the release installer"
fi
[[ ! -e "${C2C_VPS_ROOT}/current" ]] || fail "failed production install must not create current"
[[ -e "${production_archive}" ]] || fail "failed install must preserve the uploaded archive for diagnosis"

echo "VPS release script tests passed."
