#!/usr/bin/env bash
#
# Verify that the release archives produced by build-release.sh contain the
# expected binary, dashboard production build, model profiles, documentation,
# example config, and that their SHA256 checksums are valid.
#
# Usage:
#   ./scripts/check-release.sh [VERSION]

set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-}"
if [ -z "${VERSION}" ]; then
  RAW_VERSION=$(git describe --tags --always --dirty 2>/dev/null || true)
  if printf '%s' "${RAW_VERSION}" | grep -q '^v'; then
    VERSION="${RAW_VERSION}"
  else
    VERSION="0.1.0-alpha"
  fi
fi
RELEASE_DIR="dist/releases"
FAILED=0

fail() {
  echo "FAIL: $*" >&2
  FAILED=1
}

check_file_in_zip() {
  local zip="$1"
  local path="$2"
  local list
  list=$(unzip -Z1 "${zip}")
  printf '%s\n' "${list}" | grep -q "${path}" || fail "${zip} missing ${path}"
}

check_file_in_tar() {
  local tar="$1"
  local path="$2"
  local list
  list=$(tar -tzf "${tar}")
  printf '%s\n' "${list}" | grep -q "${path}" || fail "${tar} missing ${path}"
}

check_archive() {
  local os="$1"
  local arch="$2"
  local ext="$3"
  local name="novexa-${VERSION}-${os}-${arch}"

  if [ "${os}" = "windows" ]; then
    local archive="${RELEASE_DIR}/${name}.zip"
    if [ ! -f "${archive}" ]; then
      fail "archive not found: ${archive}"
      return
    fi
    check_file_in_zip "${archive}" "${name}/novexa.exe"
    check_file_in_zip "${archive}" "${name}/dashboard/dist/index.html"
    check_file_in_zip "${archive}" "${name}/profiles/generic-local.yaml"
    check_file_in_zip "${archive}" "${name}/README.md"
    check_file_in_zip "${archive}" "${name}/LICENSE"
    check_file_in_zip "${archive}" "${name}/CHANGELOG.md"
    check_file_in_zip "${archive}" "${name}/novexa.example.yaml"
  else
    local archive="${RELEASE_DIR}/${name}.tar.gz"
    if [ ! -f "${archive}" ]; then
      fail "archive not found: ${archive}"
      return
    fi
    check_file_in_tar "${archive}" "${name}/novexa${ext}"
    check_file_in_tar "${archive}" "${name}/dashboard/dist/index.html"
    check_file_in_tar "${archive}" "${name}/profiles/generic-local.yaml"
    check_file_in_tar "${archive}" "${name}/README.md"
    check_file_in_tar "${archive}" "${name}/LICENSE"
    check_file_in_tar "${archive}" "${name}/CHANGELOG.md"
    check_file_in_tar "${archive}" "${name}/novexa.example.yaml"
  fi
}

echo "verifying checksums ..."
(cd "${RELEASE_DIR}" && sha256sum -c SHA256SUMS.txt) || fail "checksum verification failed"

check_archive darwin arm64 ""
check_archive darwin amd64 ""
check_archive linux amd64 ""
check_archive linux arm64 ""
check_archive windows amd64 ".exe"

echo ""
if [ "${FAILED}" -eq 0 ]; then
  echo "all release archives look correct"
else
  echo "one or more release checks failed" >&2
  exit 1
fi
