#!/usr/bin/env bash
# Keep a Cursor My Machines / self-hosted worker running inside WSL.
# Intended to be launched by the Windows Scheduled Task installed via
# install-worker-autostart.ps1 (or run manually in a tmux/long-lived shell).
set -euo pipefail

WORKER_DIR="${CURSOR_WORKER_DIR:-/home/dev/Gumi}"
WORKER_NAME="${CURSOR_WORKER_NAME:-gumi-windows}"
LOG_DIR="${CURSOR_WORKER_LOG_DIR:-$HOME/.gumi/logs}"
LOG_FILE="${LOG_DIR}/cursor-worker.log"
PID_FILE="${LOG_DIR}/cursor-worker.pid"
RESTART_DELAY_SEC="${CURSOR_WORKER_RESTART_DELAY_SEC:-5}"

mkdir -p "$LOG_DIR"
export PATH="${HOME}/.local/bin:${PATH}"

if ! command -v agent >/dev/null 2>&1; then
  echo "ERROR: 'agent' CLI not found on PATH. Install with:" >&2
  echo "  curl https://cursor.com/install -fsS | bash" >&2
  exit 1
fi

if [[ ! -d "$WORKER_DIR" ]]; then
  echo "ERROR: worker dir does not exist: $WORKER_DIR" >&2
  exit 1
fi

# Single-instance guard (same machine / same user).
if [[ -f "$PID_FILE" ]]; then
  old_pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "${old_pid:-}" ]] && kill -0 "$old_pid" 2>/dev/null; then
    # Another watchdog already owns the worker.
    if pgrep -af "cursor-agent/.*/index.js worker start" >/dev/null 2>&1 || \
       pgrep -af "agent worker start" >/dev/null 2>&1; then
      echo "$(date -Is) watchdog already running (pid=$old_pid); exiting" | tee -a "$LOG_FILE"
      exit 0
    fi
  fi
fi
echo $$ >"$PID_FILE"
trap 'rm -f "$PID_FILE"' EXIT

log() {
  echo "$(date -Is) $*" | tee -a "$LOG_FILE"
}

# If a bare worker is already up (e.g. interactive session), do not start a second.
if pgrep -af "cursor-agent/.*/index.js worker start" >/dev/null 2>&1; then
  log "agent worker already running; watchdog waiting for it to exit before restarting"
  while pgrep -af "cursor-agent/.*/index.js worker start" >/dev/null 2>&1; do
    sleep 15
  done
fi

log "starting Cursor agent worker dir=$WORKER_DIR name=$WORKER_NAME"

while true; do
  set +e
  agent worker start \
    --worker-dir "$WORKER_DIR" \
    --name "$WORKER_NAME" \
    --verbose \
    >>"$LOG_FILE" 2>&1
  code=$?
  set -e
  log "agent worker exited code=$code; restarting in ${RESTART_DELAY_SEC}s"
  sleep "$RESTART_DELAY_SEC"
done
