#!/usr/bin/env bash
# Universal installer for Gumi — downloads the latest GitHub Release and
# installs the binary + dashboard + profiles to a standard location.
#
# Usage:
#   ./scripts/gumi-install.sh              # install (asks sudo if needed)
#   ./scripts/gumi-install.sh --dry-run    # show what would happen
#   ./scripts/gumi-install.sh --uninstall  # remove installed files
#
# Environment overrides:
#   GUMI_INSTALL_DIR   — target directory (default: /usr/local/lib/gumi)
#   GUMI_BIN_DIR       — symlink directory (default: /usr/local/bin)
#   GUMI_OWNER/REPO    — GitHub owner/repo (default: EffNine/Gumi)

set -euo pipefail

OWNER="${GUMI_OWNER:-EffNine}"
REPO="${GUMI_REPO:-Gumi}"
API_URL="https://api.github.com/repos/${OWNER}/${REPO}"
INSTALL_DIR="${GUMI_INSTALL_DIR:-/usr/local/lib/gumi}"
BIN_DIR="${GUMI_BIN_DIR:-/usr/local/bin}"

# ── helpers ──────────────────────────────────────────────────────────
log()    { echo "[gumi-install] $*"; }
die()    { echo "error: $*" >&2; exit 1; }
info()   { echo "info:  $*"; }
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed."
}

# ── flags ────────────────────────────────────────────────────────────
DRY_RUN=false
UNINSTALL=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)   DRY_RUN=true; shift ;;
    --uninstall) UNINSTALL=true; shift ;;
    -h|--help)
      echo "Usage: $0 [--dry-run] [--uninstall]"
      exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

# ── uninstall ────────────────────────────────────────────────────────
if $UNINSTALL; then
  log "Uninstalling Gumi from ${INSTALL_DIR} ..."
  [ -L "${BIN_DIR}/gumi" ] && \
    ($DRY_RUN && info "[dry-run] Would remove symlink ${BIN_DIR}/gumi" || rm -f "${BIN_DIR}/gumi")
  if [ -d "${INSTALL_DIR}" ]; then
    $DRY_RUN && info "[dry-run] Would remove directory ${INSTALL_DIR}" || rm -rf "${INSTALL_DIR}"
  else
    info "Installation directory ${INSTALL_DIR} not found — nothing to remove."
  fi
  log "Done."
  exit 0
fi

# ── detect platform ──────────────────────────────────────────────────
require_cmd curl

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${OS}" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  msys*|cygwin*) OS="windows"; ARCH="amd64" ;;
  *) die "unsupported OS: ${OS}" ;;
esac
case "${ARCH}" in
  x86_64)   ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) die "unsupported architecture: ${ARCH}" ;;
esac

EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"
ASSET="gumi-${OS}-${ARCH}.${EXT}"

# ── fetch latest release ────────────────────────────────────────────
log "Fetching latest release from GitHub ..."
RELEASE_JSON=$(curl -fsSL --retry 3 --retry-delay 2 "${API_URL}/releases/latest" 2>/dev/null) ||
  die "failed to fetch releases from GitHub (check network / internet)"

TAG=$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed 's/.*"tag_name":"\([^"]*\)".*/\1/') ||
  die "could not parse release tag from GitHub API"
log "Latest release: ${TAG}"

# ── resolve asset URL & checksum ────────────────────────────────────
DOWNLOAD_URL=$(printf '%s' "$RELEASE_JSON" | grep -m1 "\"browser_download_url\":.*${ASSET}" | sed 's/.*"browser_download_url": "\([^"]*\)".*/\1/') || true

if [ -z "$DOWNLOAD_URL" ]; then
  DOWNLOAD_URL="https://github.com/${OWNER}/${REPO}/releases/download/${TAG}/${ASSET}"
  info "Asset not listed in API response — using constructed URL."
fi

# Try to find SHA256SUMS.txt from the release assets
CHECKSUM_URL=""
for cs_name in SHA256SUMS.txt sha256sums.txt; do
  _candidate="https://github.com/${OWNER}/${REPO}/releases/download/${TAG}/${cs_name}"
  if curl -fsSL --head "$_candidate" >/dev/null 2>&1; then
    CHECKSUM_URL="$_candidate"
    break
  fi
done

# ── check existing installation ─────────────────────────────────────
if [ -d "${INSTALL_DIR}" ] && [ -f "${INSTALL_DIR}/gumi" ]; then
  EXISTING_VER=$("${INSTALL_DIR}/gumi" version 2>/dev/null | head -1 || echo "unknown")
  if [ "$EXISTING_VER" != "unknown" ]; then
    echo ""
    echo "WARNING: Gumi is already installed at ${INSTALL_DIR} (${EXISTING_VER})"
    echo "This will be overwritten. Use --uninstall first to remove cleanly."
    echo ""
    read -rp "Continue anyway? [y/N] " confirm
    [[ "$confirm" =~ ^[Yy]$ ]] || die "aborted by user"
  fi
fi

# ── download & verify ───────────────────────────────────────────────
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE="${TMPDIR}/${ASSET}"

log "Downloading ${ASSET} ..."
curl -fsSL --retry 3 --retry-delay 2 -o "$ARCHIVE" "$DOWNLOAD_URL" ||
  die "download failed — check network or try again"

if [ -n "$CHECKSUM_URL" ]; then
  log "Verifying checksum ..."
  CHECKSUM_FILE="${TMPDIR}/checksums.txt"
  if curl -fsSL --retry 2 "$CHECKSUM_URL" -o "$CHECKSUM_FILE" 2>/dev/null; then
    EXPECTED=$(grep -E "(^|[[:space:]])${ASSET}" "$CHECKSUM_FILE" | awk '{print $1}' || true)
    if [ -n "$EXPECTED" ]; then
      ACTUAL=$(sha256sum "$ARCHIVE" | awk '{print $1}')
      if [ "$ACTUAL" = "$EXPECTED" ]; then
        log "Checksum OK."
      else
        die "checksum mismatch! expected=${EXPECTED} got=${ACTUAL}"
      fi
    else
      info "Could not find checksum for ${ASSET} — skipping verification."
    fi
  else
    info "Could not fetch checksums file — continuing without verification."
  fi
else
  info "No checksum URL found — skipping verification."
fi

# ── install ──────────────────────────────────────────────────────────
if $DRY_RUN; then
  info "[dry-run] Would extract ${ARCHIVE} → ${INSTALL_DIR}"
  info "[dry-run] Would create symlink ${BIN_DIR}/gumi → ${INSTALL_DIR}/gumi"
  info "[dry-run] Done."
  exit 0
fi

# Determine if we need sudo
NEED_SUDO=false
[ -d "${BIN_DIR}" ] && [ ! -w "${BIN_DIR}" ] && NEED_SUDO=true
[ -d "${INSTALL_DIR}" ] && [ ! -w "${INSTALL_DIR}" ] && NEED_SUDO=true
[ ! -d "${INSTALL_DIR}" ] && [ ! -w "$(dirname "${INSTALL_DIR}")" ] && NEED_SUDO=true

SUDO=""
if $NEED_SUDO; then
  require_cmd sudo
  SUDO="sudo"
fi

log "Installing to ${INSTALL_DIR} ..."
$SUDO mkdir -p "${INSTALL_DIR}"

if [ "$OS" = "windows" ]; then
  $SUDO unzip -oq "$ARCHIVE" -d "${INSTALL_DIR}" 2>/dev/null || \
  $SUDO 7z x "$ARCHIVE" -o"${INSTALL_DIR}" -y 2>/dev/null || \
  die "failed to extract archive — ensure 'unzip' or '7z' is available"
else
  $SUDO tar -xzf "$ARCHIVE" -C "${INSTALL_DIR}" --strip-components=1
fi

$SUDO chmod +x "${INSTALL_DIR}/gumi"

# Create symlink
if [ -L "${BIN_DIR}/gumi" ] || [ -f "${BIN_DIR}/gumi" ]; then
  $SUDO rm -f "${BIN_DIR}/gumi"
fi
$SUDO ln -sf "${INSTALL_DIR}/gumi" "${BIN_DIR}/gumi"

log "Gumi ${TAG} installed successfully!"
log "  binary:    ${INSTALL_DIR}/gumi"
log "  symlink:   ${BIN_DIR}/gumi"
log "Run 'gumi doctor' to verify the installation."
