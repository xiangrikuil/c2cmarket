#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/output"
REF="${1:-HEAD}"

usage() {
  echo "Usage: scripts/package-source.sh [git-ref] [archive-name.tar.gz]" >&2
}

fail() {
  echo "Source package failed: $*" >&2
  exit 1
}

if (( $# > 2 )); then
  usage
  exit 1
fi

if ! git -C "${ROOT_DIR}" diff --quiet --ignore-submodules -- ||
  ! git -C "${ROOT_DIR}" diff --cached --quiet --ignore-submodules -- ||
  [[ -n "$(git -C "${ROOT_DIR}" ls-files --others --exclude-standard)" ]]; then
  fail "working tree must be clean before packaging"
fi

COMMIT="$(
  git -C "${ROOT_DIR}" rev-parse --verify --quiet --end-of-options "${REF}^{commit}"
)" || fail "Git ref does not resolve to a commit: ${REF}"
SHORT_COMMIT="$(git -C "${ROOT_DIR}" rev-parse --short=12 "${COMMIT}")"
ARCHIVE_NAME="${2:-c2cmarket-source-${SHORT_COMMIT}.tar.gz}"

if [[ ! "${ARCHIVE_NAME}" =~ ^[A-Za-z0-9._-]+\.tar\.gz$ ]]; then
  fail "archive name must be a basename ending in .tar.gz"
fi

mkdir -p "${OUT_DIR}"
ARCHIVE_PATH="${OUT_DIR}/${ARCHIVE_NAME}"
[[ ! -e "${ARCHIVE_PATH}" ]] || fail "archive already exists: ${ARCHIVE_PATH}"

TEMP_ARCHIVE="$(mktemp "${OUT_DIR}/.c2cmarket-source.tar.gz.XXXXXX")"
CONTENTS_FILE="$(mktemp "${TMPDIR:-/tmp}/c2cmarket-source-contents.XXXXXX")"
cleanup() {
  rm -f "${TEMP_ARCHIVE}" "${CONTENTS_FILE}"
}
trap cleanup EXIT

# git archive 固定输入提交；gzip -n 移除压缩头时间戳。
git -C "${ROOT_DIR}" archive \
  --format=tar \
  --prefix="c2cmarket-${SHORT_COMMIT}/" \
  "${COMMIT}" |
  gzip -n >"${TEMP_ARCHIVE}"

tar -tzf "${TEMP_ARCHIVE}" >"${CONTENTS_FILE}"

while IFS= read -r entry; do
  relative="${entry#*/}"
  relative="${relative%/}"
  [[ -n "${relative}" ]] || continue

  case "${relative}" in
    /* | ../* | */../*)
      fail "archive contains unsafe path: ${relative}"
      ;;
    .git | .git/* | .codex | .codex/* | output | output/* | tmp | tmp/* | \
      .trellis/tasks | .trellis/tasks/* | .trellis/workspace | .trellis/workspace/* | \
      node_modules | node_modules/* | */node_modules | */node_modules/* | \
      .pnpm-store | .pnpm-store/* | */.pnpm-store | */.pnpm-store/* | \
      .nuxt | .nuxt/* | */.nuxt | */.nuxt/* | \
      .output | .output/* | */.output | */.output/* | \
      .wrangler | .wrangler/* | */.wrangler | */.wrangler/* | \
      dist | dist/* | */dist | */dist/* | \
      build | build/* | */build | */build/* | \
      coverage | coverage/* | */coverage | */coverage/* | \
      .DS_Store | */.DS_Store | __MACOSX | __MACOSX/* | */__MACOSX | */__MACOSX/*)
      fail "archive contains forbidden path: ${relative}"
      ;;
  esac

  case "${relative}" in
    .env.example | .env.production.example | .env.staging.example)
      ;;
    .env | .env.* | */.env | */.env.*)
      fail "archive contains forbidden environment file: ${relative}"
      ;;
  esac
done <"${CONTENTS_FILE}"

mv "${TEMP_ARCHIVE}" "${ARCHIVE_PATH}"
if command -v sha256sum >/dev/null 2>&1; then
  SHA256="$(sha256sum "${ARCHIVE_PATH}" | awk '{print $1}')"
else
  SHA256="$(shasum -a 256 "${ARCHIVE_PATH}" | awk '{print $1}')"
fi
printf '%s  %s\n' "${SHA256}" "${ARCHIVE_NAME}" >"${ARCHIVE_PATH}.sha256"

printf 'archive=%s\n' "${ARCHIVE_PATH}"
printf 'commit=%s\n' "${COMMIT}"
printf 'sha256=%s\n' "${SHA256}"
