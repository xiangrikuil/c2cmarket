#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER_BIN="${DOCKER_BIN:-docker}"

usage() {
  echo "Usage: scripts/build-backend-image.sh <git-ref> <version> <image>" >&2
}

fail() {
  echo "Backend image build failed: $*" >&2
  exit 1
}

[[ $# -eq 3 ]] || {
  usage
  exit 1
}

REF="$1"
VERSION="$2"
IMAGE="$3"

[[ "${VERSION}" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$ ]] ||
  fail "version contains unsupported characters"
[[ -n "${IMAGE}" && ! "${IMAGE}" =~ [[:space:]] ]] ||
  fail "image must be a non-empty Docker reference without whitespace"

if ! git -C "${ROOT_DIR}" diff --quiet --ignore-submodules -- ||
  ! git -C "${ROOT_DIR}" diff --cached --quiet --ignore-submodules -- ||
  [[ -n "$(git -C "${ROOT_DIR}" ls-files --others --exclude-standard)" ]]; then
  fail "working tree must be clean before building a release image"
fi

COMMIT="$(
  git -C "${ROOT_DIR}" rev-parse --verify --quiet --end-of-options "${REF}^{commit}"
)" || fail "Git ref does not resolve to a commit: ${REF}"
BUILD_TIME="$(git -C "${ROOT_DIR}" show -s --format=%cI "${COMMIT}")"

BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/c2cmarket-backend-build.XXXXXX")"
cleanup() {
  rm -rf "${BUILD_DIR}"
}
trap cleanup EXIT

# Docker context 只来自目标提交，不读取当前工作目录文件。
git -C "${ROOT_DIR}" archive "${COMMIT}" | tar -xf - -C "${BUILD_DIR}"
[[ -f "${BUILD_DIR}/backend/Dockerfile" ]] ||
  fail "target commit does not contain backend/Dockerfile"

"${DOCKER_BIN}" build \
  --file "${BUILD_DIR}/backend/Dockerfile" \
  --build-arg "APP_VERSION=${VERSION}" \
  --build-arg "GIT_COMMIT=${COMMIT}" \
  --build-arg "BUILD_TIME=${BUILD_TIME}" \
  --tag "${IMAGE}" \
  "${BUILD_DIR}/backend"

actual_version="$(
  "${DOCKER_BIN}" image inspect \
    --format '{{ index .Config.Labels "org.opencontainers.image.version" }}' \
    "${IMAGE}"
)"
actual_commit="$(
  "${DOCKER_BIN}" image inspect \
    --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' \
    "${IMAGE}"
)"
actual_build_time="$(
  "${DOCKER_BIN}" image inspect \
    --format '{{ index .Config.Labels "org.opencontainers.image.created" }}' \
    "${IMAGE}"
)"

[[ "${actual_version}" == "${VERSION}" ]] ||
  fail "image version label mismatch: ${actual_version}"
[[ "${actual_commit}" == "${COMMIT}" ]] ||
  fail "image revision label mismatch: ${actual_commit}"
[[ "${actual_build_time}" == "${BUILD_TIME}" ]] ||
  fail "image created label mismatch: ${actual_build_time}"

printf 'image=%s\n' "${IMAGE}"
printf 'version=%s\n' "${VERSION}"
printf 'commit=%s\n' "${COMMIT}"
printf 'build_time=%s\n' "${BUILD_TIME}"
