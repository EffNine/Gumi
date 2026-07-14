#!/usr/bin/env bash
#
# Cross-compile Gumi release archives for all supported platforms and package
# them with the dashboard production build, model profiles, documentation, and
# example configuration.
#
# Usage:
#   ./scripts/build-release.sh [VERSION] [COMMIT] [BUILD_DATE]
#
# The Makefile calls this script with the current git tag, short commit, and
# UTC build date. Run manually without arguments to produce a development
# release named "0.2.0-alpha".

set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-}"
if [ -z "${VERSION}" ]; then
  RAW_VERSION=$(git describe --tags --always --dirty 2>/dev/null || true)
  if printf '%s' "${RAW_VERSION}" | grep -q '^v'; then
    VERSION="${RAW_VERSION}"
  else
    VERSION="0.2.0-alpha"
  fi
fi
COMMIT="${2:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${3:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

RELEASE_DIR="dist/releases"
RELEASE_DIR_ABS="$(pwd)/${RELEASE_DIR}"
STAGING_DIR="dist/staging"
DASHBOARD_DIR="dashboard/dist"
PROFILES_DIR="profiles"

LDFLAGS="-s -w \
  -X github.com/EffNine/gumi/runtime/internal/version.Version=${VERSION} \
  -X github.com/EffNine/gumi/runtime/internal/version.Commit=${COMMIT} \
  -X github.com/EffNine/gumi/runtime/internal/version.BuildDate=${BUILD_DATE}"

require_dashboard() {
  if [ ! -f "${DASHBOARD_DIR}/index.html" ]; then
    echo "Dashboard build missing. Building dashboard now..."
    (cd dashboard && npm ci && npm run build)
  fi
}

require_files() {
  for f in README.md LICENSE CHANGELOG.md gumi.example.yaml; do
    if [ ! -f "$f" ]; then
      echo "missing required release file: $f" >&2
      exit 1
    fi
  done
}

build_target() {
  local os="$1"
  local arch="$2"
  local ext="$3"
  local name="gumi-${VERSION}-${os}-${arch}"
  local dir="$(pwd)/${STAGING_DIR}/${name}"

  echo "building ${os}/${arch} ..."
  mkdir -p "${dir}"

  (cd runtime && GOOS="${os}" GOARCH="${arch}" CGO_ENABLED=0 \
    go build -ldflags "${LDFLAGS}" -o "${dir}/gumi${ext}" ./cmd/gumi)

  mkdir -p "${dir}/dashboard"
  cp -r "${DASHBOARD_DIR}" "${dir}/dashboard/"
  cp -r "${PROFILES_DIR}" "${dir}/profiles/"
  cp README.md LICENSE CHANGELOG.md gumi.example.yaml "${dir}/"

  mkdir -p "${RELEASE_DIR_ABS}"
  if [ "${os}" = "windows" ]; then
    (cd "${STAGING_DIR}" && zip -rq "${RELEASE_DIR_ABS}/${name}.zip" "${name}")
  else
    tar -czf "${RELEASE_DIR_ABS}/${name}.tar.gz" -C "${STAGING_DIR}" "${name}"
  fi
}

main() {
  require_dashboard
  require_files

  rm -rf "${RELEASE_DIR}" "${STAGING_DIR}"
  mkdir -p "${RELEASE_DIR}"

  build_target darwin arm64 ""
  build_target darwin amd64 ""
  build_target linux amd64 ""
  build_target linux arm64 ""
  build_target windows amd64 ".exe"

  echo "generating SHA256SUMS ..."
  (cd "${RELEASE_DIR}" && sha256sum gumi-* > SHA256SUMS.txt)

  echo ""
  echo "release artifacts ready in ${RELEASE_DIR}:"
  ls -1 "${RELEASE_DIR}"
}

main "$@"
