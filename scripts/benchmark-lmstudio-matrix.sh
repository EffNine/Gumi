#!/usr/bin/env bash
set -euo pipefail

# Novexa LM Studio Benchmark Matrix Runner
# Usage: ./scripts/benchmark-lmstudio-matrix.sh
#
# Runs benchmarks across multiple LM Studio models and produces a summary table.
#
# Environment variables:
#   MODELS          Space-separated list of model IDs (default: auto-detected from LM Studio)
#   LMSTUDIO_URL    LM Studio server URL (default: http://localhost:1234/v1)
#   ATTEMPTS        Attempts per prompt (default: 1)
#   BENCHMARK_TIMEOUT_SECONDS  Request timeout (default: 60)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BENCHMARK_DIR="$REPO_DIR/benchmarks"
MATRIX_SCRIPT="$SCRIPT_DIR/benchmark-lmstudio-matrix.sh"
BENCHMARK_SCRIPT="$SCRIPT_DIR/benchmark-local-model.sh"
DOCTOR_SCRIPT="$SCRIPT_DIR/profile-doctor.sh"

LMSTUDIO_URL="${LMSTUDIO_URL:-http://localhost:1234/v1}"
LMSTUDIO_URL="${LMSTUDIO_URL%/}"
ATTEMPTS="${ATTEMPTS:-1}"
BENCHMARK_TIMEOUT_SECONDS="${BENCHMARK_TIMEOUT_SECONDS:-60}"

if [ ! -f "$BENCHMARK_SCRIPT" ]; then
  echo "Error: benchmark script not found at $BENCHMARK_SCRIPT" >&2
  exit 1
fi

if [ ! -f "$DOCTOR_SCRIPT" ]; then
  echo "Error: profile doctor script not found at $DOCTOR_SCRIPT" >&2
  exit 1
fi

if ! command -v jq > /dev/null 2>&1; then
  echo "Error: jq is required." >&2
  exit 1
fi

# ── Resolve model list ──────────────────────────────────────────────
if [ -n "${MODELS:-}" ]; then
  IFS=' ' read -ra model_list <<< "$MODELS"
else
  echo "Querying LM Studio models from $LMSTUDIO_URL/models ..."
  model_json=$(curl --max-time 10 -fsS "$LMSTUDIO_URL/models" 2>/dev/null || true)
  if [ -z "$model_json" ]; then
    echo "Error: cannot reach LM Studio at $LMSTUDIO_URL" >&2
    exit 1
  fi
  mapfile -t model_list < <(echo "$model_json" | jq -r '.data[].id // empty')
  if [ ${#model_list[@]} -eq 0 ]; then
    echo "Error: no models found in LM Studio response." >&2
    exit 1
  fi
fi

echo "=============================================="
echo " LM Studio Benchmark Matrix"
echo "=============================================="
echo "LM Studio URL: $LMSTUDIO_URL"
echo "Attempts per prompt: $ATTEMPTS"
echo "Models: ${model_list[*]}"
echo ""

# ── Run matrix ──────────────────────────────────────────────────────
matrix_timestamp=$(date -u '+%Y%m%dT%H%M%SZ')
results_dir="$BENCHMARK_DIR"
mkdir -p "$results_dir"

declare -a summary_rows=()
overall_pass=0
overall_fail=0
overall_total=0

for model in "${model_list[@]}"; do
  echo "──────────────────────────────────────────────"
  echo "Benchmarking model: $model"
  echo "──────────────────────────────────────────────"

  model_safe=$(echo "$model" | sed 's/[^a-zA-Z0-9_-]/-/g')
  before_files=$(ls "$results_dir"/*.json 2>/dev/null | sort || true)

  set +e
  BENCHMARK_PROVIDER=lmstudio \
  LMSTUDIO_URL="$LMSTUDIO_URL" \
  ATTEMPTS="$ATTEMPTS" \
  BENCHMARK_TIMEOUT_SECONDS="$BENCHMARK_TIMEOUT_SECONDS" \
  BENCHMARK_DISABLE_THINKING=true \
  "$BENCHMARK_SCRIPT" "$model"
  bench_exit=$?
  set -e

  # Find the JSON report that was just created
  after_files=$(ls "$results_dir"/*.json 2>/dev/null | sort || true)
  json_report=""
  while IFS= read -r f; do
    case "$before_files" in
      *"$f"*) ;;
      *) json_report="$f" ;;
    esac
  done <<< "$after_files"

  if [ -z "$json_report" ] || [ ! -f "$json_report" ]; then
    json_report=$(ls -t "$results_dir"/"${model_safe}"-*.json 2>/dev/null | head -1 || true)
  fi

  if [ -z "$json_report" ] || [ ! -f "$json_report" ]; then
    echo "WARNING: no JSON report found for model '$model'. Skipping doctor analysis." >&2
    summary_rows+=("$model|N/A|N/A|N/A|N/A|N/A|N/A|N/A|N/A")
    overall_total=$((overall_total + 1))
    overall_fail=$((overall_fail + 1))
    continue
  fi

  # Run Profile Doctor
  doctor_result=""
  doctor_output=""
  if [ -f "$DOCTOR_SCRIPT" ]; then
    set +e
    doctor_output=$("$DOCTOR_SCRIPT" "$json_report" 2>&1)
    doctor_exit=$?
    set -e
    doctor_result=$(echo "$doctor_output" | grep "^Result:" | sed 's/^Result: *//')
    if [ -z "$doctor_result" ]; then
      doctor_result="unknown"
    fi
  else
    doctor_result="doctor script not found"
  fi

  # Extract stats from JSON report
  direct_pass=$(jq '[.results[] | select((.mode | startswith("A-")) and .passed == "true")] | length' "$json_report")
  stabilized_pass=$(jq '[.results[] | select((.mode | startswith("C-")) and .passed == "true")] | length' "$json_report")
  structured_pass=$(jq '[.results[] | select((.mode | startswith("D-")) and .passed == "true")] | length' "$json_report")

  direct_p50=$(jq -r '[.latency_by_mode | to_entries[] | select(.key | startswith("A-")) | .value.p50 // empty] | first // "N/A"' "$json_report")
  stabilized_p50=$(jq -r '.latency_by_mode["C-NovexaStabilized"].p50 // "N/A"' "$json_report")
  structured_p50=$(jq -r '.latency_by_mode["D-NovexaStructured"].p50 // "N/A"' "$json_report")

  summary_rows+=("$model|$json_report|$doctor_result|$direct_pass|$stabilized_pass|$structured_pass|$direct_p50|$stabilized_p50|$structured_p50")

  overall_total=$((overall_total + 1))
  if [ "$bench_exit" -eq 0 ]; then
    overall_pass=$((overall_pass + 1))
  else
    overall_fail=$((overall_fail + 1))
  fi

  echo ""
done

# ── Generate summary markdown ──────────────────────────────────────
summary_file="$BENCHMARK_DIR/lmstudio-matrix-${matrix_timestamp}.md"

{
  echo "# LM Studio Benchmark Matrix"
  echo ""
  echo "**Generated:** $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "**LM Studio URL:** $LMSTUDIO_URL"
  echo "**Attempts per prompt:** $ATTEMPTS"
  echo "**Models tested:** ${#model_list[@]}"
  echo "**Passed:** $overall_pass / $overall_total"
  echo ""
  echo "## Summary"
  echo ""
  echo "| Model | JSON Report | Doctor Result | Direct Pass | Stabilized Pass | Structured Pass | Direct p50 (ms) | Stabilized p50 (ms) | Structured p50 (ms) |"
  echo "|-------|-------------|---------------|-------------|-----------------|-----------------|-----------------|---------------------|----------------------|"
  for row in "${summary_rows[@]}"; do
    IFS='|' read -r model path doctor d_pass s_pass st_pass d_p50 s_p50 st_p50 <<< "$row"
    echo "| $model | $path | $doctor | $d_pass | $s_pass | $st_pass | $d_p50 | $s_p50 | $st_p50 |"
  done
  echo ""
  echo "## Legend"
  echo ""
  echo "- **Direct Pass**: Number of passing requests in direct provider mode (A-*)"
  echo "- **Stabilized Pass**: Number of passing requests in Novexa stabilized mode (C-NovexaStabilized)"
  echo "- **Structured Pass**: Number of passing requests in Novexa structured mode (D-NovexaStructured)"
  echo "- **p50 (ms)**: Median latency in milliseconds for each mode"
  echo "- **Doctor Result**: Profile Doctor assessment (Good baseline / Needs tuning / Insufficient data)"
} > "$summary_file"

echo "=============================================="
echo " Matrix complete: $overall_pass / $overall_total models passed"
echo " Summary: $summary_file"
echo "=============================================="
