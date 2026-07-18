#!/usr/bin/env bash
# Keep a Cursor My Machines / self-hosted worker running inside WSL.
# Launched by scripts/windows/launch-worker.ps1 (Startup folder / Scheduled Task).
set -euo pipefail

WORKER_DIR="${CURSOR_WORKER_DIR:-/home/dev/Gumi}"
WORKER_NAME="${CURSOR_WORKER_NAME:-gumi-windows}"
LOG_DIR="${CURSOR_WORKER_LOG_DIR:-$HOME/.gumi/logs}"
LOG_FILE="${LOG_DIR}/cursor-worker.log"
PID_FILE="${LOG_DIR}/cursor-worker.pid"
LOCK_FILE="${LOG_DIR}/cursor-worker.lock"
RESTART_DELAY_SEC="${CURSOR_WORKER_RESTART_DELAY_SEC:-5}"

mkdir -p "$LOG_DIR"

# Use a clean Linux PATH. WSL login shells inject Windows paths that contain
# spaces / parentheses (e.g. Program Files (x86)) and break unquoted expansions.
export PATH="${HOME}/.local/bin:/usr/local/bin:/usr/bin:/bin"

log() {
  echo "$(date -Is) $*" | tee -a "$LOG_FILE"
}

if ! command -v agent >/dev/null 2>&1; then
  log "ERROR: 'agent' CLI not found on PATH=$PATH"
  log "Install with: curl https://cursor.com/install -fsS | bash"
  exit 1
fi

if [[ ! -d "$WORKER_DIR" ]]; then
  log "ERROR: worker dir does not exist: $WORKER_DIR"
  exit 1
fi

# Atomic single-instance guard (Startup folder + Scheduled Task can race).
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  log "another watchdog holds $LOCK_FILE; exiting"
  exit 0
fi
echo $$ >"$PID_FILE"
trap 'rm -f "$PID_FILE"; flock -u 9 2>/dev/null || true' EXIT

worker_running() {
  pgrep -af "cursor-agent/.*/index.js worker start" >/dev/null 2>&1
}

# If a worker is already up (manual session), wait — do not start a second one.
if worker_running; then
  log "agent worker already running; waiting for it to exit before owning restarts"
  while worker_running; do
    sleep 15
  done
  log "previous agent worker exited; taking over"
fi

log "starting Cursor agent worker dir=$WORKER_DIR name=$WORKER_NAME agent=$(command -v agent)"

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
