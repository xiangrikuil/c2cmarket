#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER_BIN="${DOCKER_BIN:-docker}"
GO_BIN="${GO_BIN:-go}"

POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:18-alpine@sha256:96d56f7f57c6aacd1fcb908bc83b345ec5f83231ee486dd66a1baadce274db88}"
MIGRATE_IMAGE="${MIGRATE_IMAGE:-migrate/migrate:v4.18.3@sha256:39b59b389634e43bb3f2d4e94bc1edef0775ec2a9a3540ce6a2cf330e5daae55}"
EXPECTED_MIGRATION_VERSION="${EXPECTED_MIGRATION_VERSION:-64}"

POSTGRES_USER="c2c_prelaunch"
POSTGRES_PASSWORD="c2c_prelaunch_test_password"
GENERAL_DATABASE="c2c_prelaunch_test"
QUOTA_DATABASE="c2c_quota_test"
REPUTATION_DATABASE="c2c_reputation_test_main"

run_id="c2c-prelaunch-$$"
network_name="${run_id}-network"
postgres_container="${run_id}-postgres"

fail() {
  echo "PostgreSQL integration gate failed: $*" >&2
  exit 1
}

cleanup() {
  if [[ "${postgres_container}" == c2c-prelaunch-*-postgres ]]; then
    "${DOCKER_BIN}" rm --force "${postgres_container}" >/dev/null 2>&1 || true
  fi
  if [[ "${network_name}" == c2c-prelaunch-*-network ]]; then
    "${DOCKER_BIN}" network rm "${network_name}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

command -v "${DOCKER_BIN}" >/dev/null 2>&1 ||
  fail "Docker executable is unavailable: ${DOCKER_BIN}"
command -v "${GO_BIN}" >/dev/null 2>&1 ||
  fail "Go executable is unavailable: ${GO_BIN}"

"${DOCKER_BIN}" network create "${network_name}" >/dev/null
"${DOCKER_BIN}" run --detach \
  --name "${postgres_container}" \
  --network "${network_name}" \
  --publish "127.0.0.1::5432" \
  --env "POSTGRES_DB=postgres" \
  --env "POSTGRES_USER=${POSTGRES_USER}" \
  --env "POSTGRES_PASSWORD=${POSTGRES_PASSWORD}" \
  "${POSTGRES_IMAGE}" >/dev/null

ready=false
for _ in $(seq 1 60); do
  if "${DOCKER_BIN}" exec "${postgres_container}" \
    pg_isready --quiet --username "${POSTGRES_USER}" --dbname postgres; then
    ready=true
    break
  fi
  sleep 1
done
[[ "${ready}" == "true" ]] || fail "PostgreSQL did not become ready within 60 seconds"

published_port="$("${DOCKER_BIN}" port "${postgres_container}" 5432/tcp)"
host_port="${published_port##*:}"
[[ "${host_port}" =~ ^[0-9]+$ ]] || fail "could not resolve the published PostgreSQL port"

for database in "${GENERAL_DATABASE}" "${QUOTA_DATABASE}" "${REPUTATION_DATABASE}"; do
  "${DOCKER_BIN}" exec "${postgres_container}" \
    createdb --username "${POSTGRES_USER}" "${database}"

  "${DOCKER_BIN}" run --rm \
    --network "${network_name}" \
    --volume "${ROOT_DIR}/backend/migrations:/migrations:ro" \
    "${MIGRATE_IMAGE}" \
    -path=/migrations \
    -database="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${postgres_container}:5432/${database}?sslmode=disable" \
    up

  migration_state="$(
    "${DOCKER_BIN}" exec "${postgres_container}" \
      psql --no-psqlrc --tuples-only --no-align \
      --username "${POSTGRES_USER}" \
      --dbname "${database}" \
      --command "SELECT version::text || ':' || dirty::text FROM schema_migrations"
  )"
  [[ "${migration_state}" == "${EXPECTED_MIGRATION_VERSION}:false" ]] ||
    fail "${database} migration state is ${migration_state}, expected ${EXPECTED_MIGRATION_VERSION}:false"
done

database_url() {
  local database="$1"
  printf 'postgres://%s:%s@127.0.0.1:%s/%s?sslmode=disable' \
    "${POSTGRES_USER}" "${POSTGRES_PASSWORD}" "${host_port}" "${database}"
}

run_go_test() {
  local database="$1"
  local package="$2"
  local pattern="$3"

  (
    cd "${ROOT_DIR}/backend"
    C2C_TEST_DATABASE_URL="$(database_url "${database}")" \
      "${GO_BIN}" test -count=1 "${package}" -run "${pattern}"
  )
}

run_go_test "${GENERAL_DATABASE}" ./internal/database \
  '^TestOpenPostgresWithOptionsAppliesPoolAndSessionTimeouts$'
run_go_test "${GENERAL_DATABASE}" ./internal/store/postgres \
  '^(TestPostgresOAuthIdentityOwnershipAndConcurrency|TestPostgresBootstrapAdminIsCreateOnlyAndProvenanced|TestPostgresBootstrapAdminRejectsConflictsAndRollsBack|TestPostgresSessionRenewalUpdatesExactlyOnceAtBoundary|TestPostgresContactReencryptDryRunAndApply|TestPostgresDataLifecycleAppliesRetentionAndPreservesAuditHistory|TestPostgresDataLifecycleSkipsWhenAdvisoryLockIsHeld|TestSlowActiveQueryCountIntegration|TestPostgresEmailVerificationChallengeLifecycle|TestPostgresIdempotencyLifecycleBoundsBodiesAndRejectsStaleGeneration)$'
run_go_test "${GENERAL_DATABASE}" ./internal/server \
  '^(TestPostgresAdminOfficialPriceRecordFlow|TestPostgresProductCatalogReadAPIs|TestPostgresAPIServiceFlow|TestPostgresAPIServiceIntegrityConstraints|TestPostgresAPIPurchaseIntentFlow|TestPostgresAPIOrderReleasesPurchaseIntent|TestPostgresAPIPurchaseIntentIntegrityConstraints|TestPostgresIdempotencyProcessingReplay|TestPostgresContactSessionFlow|TestPostgresContactIntegrityConstraints|TestPostgresCarpoolMembershipIntegrityConstraints|TestPostgresOfficialPriceAdminRecordSideEffectsAreIdempotent|TestPostgresCarpoolApplicationFlow)$'

run_go_test "${QUOTA_DATABASE}" ./internal/store/postgres \
  '^TestAPIQuotaPostgres'
run_go_test "${QUOTA_DATABASE}" ./internal/server \
  '^TestPostgresAPIQuotaHTTPFlow$'

run_go_test "${REPUTATION_DATABASE}" ./internal/store/postgres \
  '^(TestReputationEnginePostgres|TestReputationPostgres|TestSourceAuthorVerificationPostgres|TestEffectiveSourceAuthorVerificationTracks|TestTransactionReviewPostgres)'

echo "PostgreSQL 18 migration and integration gate passed."
