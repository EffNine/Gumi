#!/usr/bin/env bash
set -euo pipefail

# Compare the same Terminal-Bench agent against a direct OpenAI-compatible
# provider and Gumi. This evaluates the complete agent harness, not just the
# language model. Docker Desktop must be running before execution.
#
# Usage:
#   DIRECT_BASE_URL=http://192.168.0.164:1234/v1 \
#   GUMI_BASE_URL=http://127.0.0.1:8787/v1 \
#   ./scripts/benchmark-terminal-bench.sh qwen/qwen3.5-9b

MODEL="${1:-}"
if [ -z "$MODEL" ]; then
  echo "Usage: $0 <model>" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TB_BIN="${TB_BIN:-$REPO_DIR/.venv-terminal/bin/tb}"
DIRECT_BASE_URL="${DIRECT_BASE_URL:-http://localhost:1234/v1}"
GUMI_BASE_URL="${GUMI_BASE_URL:-http://localhost:8787/v1}"
DIRECT_API_KEY="${DIRECT_API_KEY:-local}"
GUMI_API_KEY="${GUMI_API_KEY:-gumi-local}"
GUMI_MODEL="${GUMI_MODEL:-lmstudio:$MODEL}"
DATASET="${TERMINAL_BENCH_DATASET:-terminal-bench-core==0.1.1}"
TASKS="${TERMINAL_BENCH_TASKS:-5}"
AGENT="${TERMINAL_BENCH_AGENT:-terminus-2}"
MAX_EPISODES="${MAX_EPISODES:-30}"
RUN_ID="${RUN_ID:-$(date -u '+%Y%m%dT%H%M%SZ')}"
OUTPUT_DIR="${TERMINAL_BENCH_OUTPUT_DIR:-$HOME/.gumi/benchmarks/terminal-bench/${RUN_ID}}"

DIRECT_BASE_URL="${DIRECT_BASE_URL%/}"
GUMI_BASE_URL="${GUMI_BASE_URL%/}"

if [ ! -x "$TB_BIN" ]; then
  echo "Error: Terminal-Bench CLI not found at $TB_BIN." >&2
  echo "Create it with: python3.13 -m venv .venv-terminal && .venv-terminal/bin/pip install terminal-bench" >&2
  exit 1
fi
if ! command -v docker > /dev/null 2>&1 || ! docker info > /dev/null 2>&1; then
  echo "Error: Docker Desktop must be installed and running for Terminal-Bench." >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

run_benchmark() {
  local label="$1"
  local model_name="$2"
  local api_base="$3"
  local api_key="$4"
  local run_dir="$OUTPUT_DIR/$label"

  echo "Running Terminal-Bench $label with $AGENT on $model_name"
  OPENAI_API_KEY="$api_key" "$TB_BIN" run \
    --dataset "$DATASET" \
    --agent "$AGENT" \
    --model "openai/$model_name" \
    --agent-kwarg "api_base=$api_base" \
    --agent-kwarg "max_episodes=$MAX_EPISODES" \
    --n-tasks "$TASKS" \
    --n-concurrent 1 \
    --output-path "$run_dir" \
    --run-id "$label"
}

run_benchmark direct "$MODEL" "$DIRECT_BASE_URL" "$DIRECT_API_KEY"
# Gumi run: set generous timeouts so long model generations don't get
# cut off by the HTTP server or provider adapter.
GUMI_SERVER_TIMEOUT_SECONDS=600 \
GUMI_PROVIDER_TIMEOUT_SECONDS=300 \
run_benchmark gumi "$GUMI_MODEL" "$GUMI_BASE_URL" "$GUMI_API_KEY"

cat <<EOF

Terminal-Bench runs completed.
Direct: $OUTPUT_DIR/direct
Gumi: $OUTPUT_DIR/gumi

Compare the official run artifacts only when dataset version, task list, agent,
max episodes, model artifact, and concurrency match exactly.
EOF
