#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/output"
ARCHIVE_NAME="${1:-c2cmarket-source-$(date -u +%Y%m%dT%H%M%SZ).tar.gz}"

if [[ "${ARCHIVE_NAME}" == */* ]]; then
  echo "Archive name must not contain path separators: ${ARCHIVE_NAME}" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"
ARCHIVE_PATH="${OUT_DIR}/${ARCHIVE_NAME}"
MANIFEST_PATH="$(mktemp "${TMPDIR:-/tmp}/c2cmarket-source-manifest.XXXXXX")"
trap 'rm -f "${MANIFEST_PATH}"' EXIT

# Package tracked source from the current working tree and still exclude known
# transient paths even if an old generated asset was accidentally committed.
while IFS= read -r -d '' source_path; do
  case "${source_path}" in
    .git|.git/*|output|output/*|tmp|tmp/*|__MACOSX|__MACOSX/*|*/__MACOSX|*/__MACOSX/*|.DS_Store|*/.DS_Store|node_modules|node_modules/*|*/node_modules|*/node_modules/*|dist|dist/*|*/dist|*/dist/*|build|build/*|*/build|*/build/*|coverage|coverage/*|*/coverage|*/coverage/*)
      continue
      ;;
  esac
  printf '%s\0' "${source_path}" >> "${MANIFEST_PATH}"
done < <(git -C "${ROOT_DIR}" ls-files -z --cached --others --exclude-standard)

tar --null -T "${MANIFEST_PATH}" -czf "${ARCHIVE_PATH}" -C "${ROOT_DIR}"

forbidden_pattern='(^|/)(\.git|output|tmp|__MACOSX|node_modules|dist|build|coverage)(/|$)|(^|/)\.DS_Store$'
if tar -tzf "${ARCHIVE_PATH}" | grep -E "${forbidden_pattern}" >/tmp/c2cmarket-package-forbidden.txt; then
  echo "Archive contains forbidden paths:" >&2
  cat /tmp/c2cmarket-package-forbidden.txt >&2
  rm -f /tmp/c2cmarket-package-forbidden.txt
  exit 1
fi
rm -f /tmp/c2cmarket-package-forbidden.txt

echo "${ARCHIVE_PATH}"
