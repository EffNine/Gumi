#!/usr/bin/env bash
#
# Install Novexa from a local release archive or source tree.
#
# This script detects the host OS/architecture and installs the binary plus the
# dashboard and profiles directories under /usr/local/lib/novexa. A symlink is
# created at /usr/local/bin/novexa so the runtime can still discover its assets
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

LIB_DIR="/usr/local/lib/novexa"
BIN_DIR="/usr/local/bin"

# Prefer a pre-built binary when running from an extracted release archive.
SOURCE_EXE=""
SOURCE_DASHBOARD=""
SOURCE_PROFILES=""

if [ -f "novexa" ] && [ -d "dashboard/dist" ] && [ -d "profiles" ]; then
  SOURCE_EXE="novexa"
  SOURCE_DASHBOARD="dashboard/dist"
  SOURCE_PROFILES="profiles"
elif [ -f "runtime/novexa" ] && [ -d "dashboard/dist" ] && [ -d "profiles" ]; then
  SOURCE_EXE="runtime/novexa"
  SOURCE_DASHBOARD="dashboard/dist"
  SOURCE_PROFILES="profiles"
else
  echo "No local novexa binary + dashboard/dist + profiles found."
  echo "Build first with: make build"
  exit 1
fi

echo "Installing Novexa for ${OS}/${ARCH} ..."
echo "  binary:    ${LIB_DIR}/novexa"
echo "  assets:    ${LIB_DIR}/dashboard/dist"
echo "  profiles:  ${LIB_DIR}/profiles"
echo "  symlink:   ${BIN_DIR}/novexa"

mkdir -p "${LIB_DIR}/dashboard" "${BIN_DIR}"
cp "${SOURCE_EXE}" "${LIB_DIR}/novexa"
chmod +x "${LIB_DIR}/novexa"
cp -r "${SOURCE_DASHBOARD}" "${LIB_DIR}/dashboard/"
cp -r "${SOURCE_PROFILES}" "${LIB_DIR}/profiles"

if [ -L "${BIN_DIR}/novexa" ] || [ -f "${BIN_DIR}/novexa" ]; then
  rm -f "${BIN_DIR}/novexa"
fi
ln -s "${LIB_DIR}/novexa" "${BIN_DIR}/novexa"

echo ""
echo "Novexa installed. Run: novexa version"
echo "Then start with:     novexa start"
