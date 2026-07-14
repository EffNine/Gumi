#!/usr/bin/env bash
#
# Install Gumi from a local release archive or source tree.
#
# This script detects the host OS/architecture and installs the binary plus the
# dashboard and profiles directories under /usr/local/lib/gumi. A symlink is
# created at /usr/local/bin/gumi so the runtime can still discover its assets
# relative to the real executable path.
#
# Usage:
#   ./scripts/install.sh
#
# Run from the root of an extracted release archive, or from the repository root
# after building with `make build`.

set -euo pipefail

cd "$(dirname "$0")/.."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "unsupported architecture: ${ARCH}" >&2; exit 1 ;;
esac

LIB_DIR="/usr/local/lib/gumi"
BIN_DIR="/usr/local/bin"

# Prefer a pre-built binary when running from an extracted release archive.
SOURCE_EXE=""
SOURCE_DASHBOARD=""
SOURCE_PROFILES=""

if [ -f "gumi" ] && [ -d "dashboard/dist" ] && [ -d "profiles" ]; then
  SOURCE_EXE="gumi"
  SOURCE_DASHBOARD="dashboard/dist"
  SOURCE_PROFILES="profiles"
elif [ -f "runtime/gumi" ] && [ -d "dashboard/dist" ] && [ -d "profiles" ]; then
  SOURCE_EXE="runtime/gumi"
  SOURCE_DASHBOARD="dashboard/dist"
  SOURCE_PROFILES="profiles"
else
  echo "No local gumi binary + dashboard/dist + profiles found."
  echo "Build first with: make build"
  exit 1
fi

echo "Installing Gumi for ${OS}/${ARCH} ..."
echo "  binary:    ${LIB_DIR}/gumi"
echo "  assets:    ${LIB_DIR}/dashboard/dist"
echo "  profiles:  ${LIB_DIR}/profiles"
echo "  symlink:   ${BIN_DIR}/gumi"

mkdir -p "${LIB_DIR}/dashboard" "${BIN_DIR}"
cp "${SOURCE_EXE}" "${LIB_DIR}/gumi"
chmod +x "${LIB_DIR}/gumi"
cp -r "${SOURCE_DASHBOARD}" "${LIB_DIR}/dashboard/"
cp -r "${SOURCE_PROFILES}" "${LIB_DIR}/profiles"

if [ -L "${BIN_DIR}/gumi" ] || [ -f "${BIN_DIR}/gumi" ]; then
  rm -f "${BIN_DIR}/gumi"
fi
ln -s "${LIB_DIR}/gumi" "${BIN_DIR}/gumi"

echo ""
echo "Gumi installed. Run: gumi version"
echo "Then start with:     gumi start"
