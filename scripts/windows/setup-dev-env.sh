#!/usr/bin/env bash
# Bootstrap a Gumi development environment inside WSL (Ubuntu/Debian).
#
# Installs Go 1.25+, Node.js 22+, make/git build tools, clones the repo if
# needed, builds the dashboard + runtime binary, and prints next steps for
# LM Studio / Ollama on the Windows host (GPU).
#
# Usage (from WSL):
#   curl -fsSL https://raw.githubusercontent.com/EffNine/Gumi/main/scripts/windows/setup-dev-env.sh | bash
#   # or, if you already have the repo:
#   bash scripts/windows/setup-dev-env.sh
#
# Env overrides:
#   GUMI_REPO_URL   default: https://github.com/EffNine/Gumi.git
#   GUMI_REPO_DIR   default: $HOME/Gumi
#   GUMI_GO_VERSION default: 1.25.0
#   GUMI_NODE_MAJOR default: 22
#   SKIP_BUILD=1    skip make build
#   SKIP_CLONE=1    do not clone; require existing GUMI_REPO_DIR
set -euo pipefail

REPO_URL="${GUMI_REPO_URL:-https://github.com/EffNine/Gumi.git}"
REPO_DIR="${GUMI_REPO_DIR:-$HOME/Gumi}"
GO_VERSION="${GUMI_GO_VERSION:-1.25.0}"
NODE_MAJOR="${GUMI_NODE_MAJOR:-22}"
SKIP_BUILD="${SKIP_BUILD:-0}"
SKIP_CLONE="${SKIP_CLONE:-0}"

log() { printf '\n==> %s\n' "$*"; }
have() { command -v "$1" >/dev/null 2>&1; }

require_debian_like() {
  if [[ ! -f /etc/os-release ]]; then
    echo "ERROR: /etc/os-release missing; this script targets Ubuntu/Debian WSL." >&2
    exit 1
  fi
  # shellcheck disable=SC1091
  . /etc/os-release
  case "${ID:-}:${ID_LIKE:-}" in
    ubuntu*|debian*|*:debian*|*:ubuntu*) ;;
    *)
      echo "WARNING: untested distro '${ID:-unknown}'. Continuing anyway." >&2
      ;;
  esac
}

apt_install() {
  log "Installing system packages (sudo may prompt)"
  sudo apt-get update -y
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
    build-essential curl ca-certificates git make unzip \
    pkg-config
}

go_version_ge() {
  # Return 0 if installed Go version >= required (both like 1.25.0).
  local required="$1" installed="$2"
  [[ "$(printf '%s\n%s\n' "$required" "$installed" | sort -V | head -1)" == "$required" ]]
}

install_go() {
  if have go; then
    local cur
    cur="$(go env GOVERSION 2>/dev/null || go version | awk '{print $3}')"
    cur="${cur#go}"
    if go_version_ge "$GO_VERSION" "$cur"; then
      log "Go already OK ($cur)"
      return 0
    fi
    log "Go $cur is older than required $GO_VERSION; upgrading"
  fi

  log "Installing Go ${GO_VERSION}"
  local arch tarball url tmp
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *)
      echo "ERROR: unsupported arch $(uname -m)" >&2
      exit 1
      ;;
  esac
  tarball="go${GO_VERSION}.linux-${arch}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  tmp="$(mktemp -d)"
  curl -fsSL "$url" -o "$tmp/$tarball"
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf "$tmp/$tarball"
  rm -rf "$tmp"

  # Persist PATH for interactive shells.
  if ! grep -q '/usr/local/go/bin' "$HOME/.bashrc" 2>/dev/null; then
    echo 'export PATH=/usr/local/go/bin:$PATH' >>"$HOME/.bashrc"
  fi
  if [[ -f "$HOME/.profile" ]] && ! grep -q '/usr/local/go/bin' "$HOME/.profile" 2>/dev/null; then
    echo 'export PATH=/usr/local/go/bin:$PATH' >>"$HOME/.profile"
  fi
  export PATH="/usr/local/go/bin:${PATH}"
  go version
}

install_node() {
  local need=1
  if have node; then
    local major
    major="$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0)"
    if [[ "$major" -ge "$NODE_MAJOR" ]]; then
      need=0
      log "Node already OK ($(node -v))"
    fi
  fi
  if [[ "$need" -eq 0 ]]; then
    have npm || {
      echo "ERROR: node is present but npm is missing" >&2
      exit 1
    }
    return 0
  fi

  log "Installing Node.js ${NODE_MAJOR}.x (NodeSource)"
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | sudo -E bash -
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y nodejs
  node -v
  npm -v
}

clone_or_update_repo() {
  if [[ -d "$REPO_DIR/.git" ]]; then
    log "Repo already at $REPO_DIR"
    return 0
  fi
  if [[ "$SKIP_CLONE" == "1" ]]; then
    echo "ERROR: SKIP_CLONE=1 but $REPO_DIR is not a git repo" >&2
    exit 1
  fi
  if [[ -e "$REPO_DIR" ]] && [[ ! -d "$REPO_DIR/.git" ]]; then
    echo "ERROR: $REPO_DIR exists but is not a git checkout" >&2
    exit 1
  fi
  log "Cloning $REPO_URL → $REPO_DIR"
  git clone "$REPO_URL" "$REPO_DIR"
}

build_gumi() {
  if [[ "$SKIP_BUILD" == "1" ]]; then
    log "Skipping build (SKIP_BUILD=1)"
    return 0
  fi
  log "Building Gumi (dashboard + runtime)"
  cd "$REPO_DIR"
  make build
  ./gumi version
}

print_next_steps() {
  cat <<EOF

────────────────────────────────────────────────────────
Gumi WSL development environment is ready.

Repo:    $REPO_DIR
Binary:  $REPO_DIR/gumi
Go:      $(go version 2>/dev/null || echo missing)
Node:    $(node -v 2>/dev/null || echo missing)

Day-to-day:
  cd $REPO_DIR
  make build && ./gumi start          # API :8787, dashboard :8788
  cd dashboard && npm run dev         # hot reload UI (runtime must be up)

Tests:
  cd $REPO_DIR/runtime && go test ./... && go vet ./...

GPU inference (Windows host — RTX recommended):
  1. Install LM Studio or Ollama on Windows and start the local server.
  2. Point Gumi at it from WSL, e.g.:

     export GUMI_PROVIDER_DEFAULT=lmstudio
     export GUMI_LMSTUDIO_URL=http://127.0.0.1:1234/v1
     ./gumi start

  If localhost forwarding fails, use the Windows host IP from:
     grep nameserver /etc/resolv.conf

Optional — keep a Cursor self-hosted worker alive on this PC:
  From Windows PowerShell (as your user):
    powershell -ExecutionPolicy Bypass -File \\\\wsl\$\\Ubuntu\\home\\\$env:USERNAME\\Gumi\\scripts\\windows\\install-worker-autostart.ps1
  Or after cloning into ~/Gumi inside WSL, see scripts/windows/install-worker-autostart.ps1
────────────────────────────────────────────────────────
EOF
}

main() {
  require_debian_like
  apt_install
  install_go
  install_node
  clone_or_update_repo
  build_gumi
  print_next_steps
}

main "$@"
