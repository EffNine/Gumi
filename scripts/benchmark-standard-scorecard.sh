#!/usr/bin/env bash
set -euo pipefail

# Reproducible before/after benchmark for OpenAI-compatible chat endpoints.
# Default task is IFEval, which measures verifiable instruction following.
#
# Usage:
#   DIRECT_BASE_URL=http://192.168.0.164:1234/v1 \
#   GUMI_BASE_URL=http://127.0.0.1:8787/v1 \
#   ./scripts/benchmark-standard-scorecard.sh qwen/qwen3.5-9b
#
# Requirements:
#   pip install "lm-eval[api]"
#   The direct provider and Gumi must expose OpenAI-compatible chat APIs.

MODEL="${1:-}"
if [ -z "$MODEL" ]; then
  echo "Usage: $0 <model>" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BENCHMARK_DIR="${BENCHMARK_DIR:-$HOME/.gumi/benchmarks/standard-scorecard}"
DIRECT_BASE_URL="${DIRECT_BASE_URL:-http://localhost:1234/v1}"
GUMI_BASE_URL="${GUMI_BASE_URL:-http://localhost:8787/v1}"
DIRECT_API_KEY="${DIRECT_API_KEY:-local}"
GUMI_API_KEY="${GUMI_API_KEY:-gumi-local}"
GUMI_MODEL="${GUMI_MODEL:-lmstudio:$MODEL}"
TASKS="${STANDARD_TASKS:-ifeval}"
NUM_FEWSHOT="${NUM_FEWSHOT:-0}"
BATCH_SIZE="${BATCH_SIZE:-1}"
NUM_CONCURRENT="${NUM_CONCURRENT:-1}"
MAX_RETRIES="${MAX_RETRIES:-3}"
GEN_KWARGS="${GEN_KWARGS:-temperature=0,do_sample=false,max_gen_toks=512,reasoning_effort=none}"
GUMI_MODE="${GUMI_MODE:-stabilized}"
LM_EVAL_BIN="${LM_EVAL_BIN:-lm_eval}"
LIMIT="${LIMIT:-}"
RUN_ID="${RUN_ID:-$(date -u '+%Y%m%dT%H%M%SZ')}"

DIRECT_BASE_URL="${DIRECT_BASE_URL%/}"
GUMI_BASE_URL="${GUMI_BASE_URL%/}"
MODEL_SAFE="$(echo "$MODEL" | sed 's/[^a-zA-Z0-9_-]/-/g')"
RUN_DIR="$BENCHMARK_DIR/${MODEL_SAFE}-${RUN_ID}"
DIRECT_DIR="$RUN_DIR/direct"
GUMI_DIR="$RUN_DIR/gumi-${GUMI_MODE}"
SCORECARD_JSON="$REPO_DIR/benchmarks/reports/${MODEL_SAFE}-ifeval-${RUN_ID}.json"
SCORECARD_MD="$REPO_DIR/benchmarks/reports/${MODEL_SAFE}-ifeval-${RUN_ID}.md"

if ! command -v jq > /dev/null 2>&1; then
  echo "Error: jq is required." >&2
  exit 1
fi
if ! command -v "$LM_EVAL_BIN" > /dev/null 2>&1; then
  echo "Error: lm_eval is required. Install it with: pip install \"lm-eval[api]\"" >&2
  exit 1
fi

preflight() {
  local label="$1"
  local url="$2"
  local api_key="$3"
  if ! curl --max-time 10 -fsS -H "Authorization: Bearer $api_key" "$url/models" > /dev/null; then
    echo "Error: $label is not reachable at $url/models" >&2
    exit 1
  fi
}

run_eval() {
  local label="$1"
  local base_url="$2"
  local model="$3"
  local output_dir="$4"
  local api_key="$5"
  local -a args

  mkdir -p "$output_dir"

  echo "Running $label: tasks=$TASKS model=$model"
  args=(
    --model local-chat-completions
    --model_args "model=$model,base_url=$base_url/chat/completions,num_concurrent=$NUM_CONCURRENT,max_retries=$MAX_RETRIES"
    --tasks "$TASKS"
    --num_fewshot "$NUM_FEWSHOT"
    --batch_size "$BATCH_SIZE"
    --gen_kwargs "$GEN_KWARGS"
    --apply_chat_template
    --log_samples
    --output_path "$output_dir"
  )
  if [ -n "$LIMIT" ]; then
    args+=(--limit "$LIMIT")
  fi
  OPENAI_API_KEY="$api_key" "$LM_EVAL_BIN" "${args[@]}"
}

find_results() {
  local file
  while IFS= read -r file; do
    if jq -e 'has("results")' "$file" > /dev/null 2>&1; then
      echo "$file"
      return
    fi
  done < <(find "$1" -name '*.json' -type f | sort)
}

extract_scores() {
  local results_file="$1"
  jq '
    .results
    | to_entries
    | map({
        task: .key,
        metrics: (.value | with_entries(select(.value | type == "number"))),
        primary_metric: (
          .value
          | if has("inst_level_strict_acc,none") then "inst_level_strict_acc,none"
            elif has("inst_level_strict_acc") then "inst_level_strict_acc"
            elif has("exact_match,strict-match") then "exact_match,strict-match"
            elif has("exact_match,none") then "exact_match,none"
            elif has("acc,none") then "acc,none"
            elif has("acc") then "acc"
            else null
            end
        )
      })
    | map(. + {primary_score: (if .primary_metric == null then null else .metrics[.primary_metric] end)})
  ' "$results_file"
}

preflight "Direct provider" "$DIRECT_BASE_URL" "$DIRECT_API_KEY"
preflight "Gumi" "$GUMI_BASE_URL" "$GUMI_API_KEY"
mkdir -p "$RUN_DIR"

run_eval direct "$DIRECT_BASE_URL" "$MODEL" "$DIRECT_DIR" "$DIRECT_API_KEY"
run_eval gumi "$GUMI_BASE_URL" "$GUMI_MODEL" "$GUMI_DIR" "$GUMI_API_KEY"

DIRECT_RESULTS="$(find_results "$DIRECT_DIR")"
GUMI_RESULTS="$(find_results "$GUMI_DIR")"
if [ -z "$DIRECT_RESULTS" ] || [ -z "$GUMI_RESULTS" ]; then
  echo "Error: lm-eval did not produce results.json for both runs." >&2
  exit 1
fi

DIRECT_SCORES="$(extract_scores "$DIRECT_RESULTS")"
GUMI_SCORES="$(extract_scores "$GUMI_RESULTS")"

jq -n \
  --arg generated_at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --arg model "$MODEL" \
  --arg gumi_model "$GUMI_MODEL" \
  --arg direct_base_url "$DIRECT_BASE_URL" \
  --arg gumi_base_url "$GUMI_BASE_URL" \
  --arg gumi_mode "$GUMI_MODE" \
  --arg tasks "$TASKS" \
  --arg gen_kwargs "$GEN_KWARGS" \
  --arg direct_results "$DIRECT_RESULTS" \
  --arg gumi_results "$GUMI_RESULTS" \
  --argjson direct_scores "$DIRECT_SCORES" \
  --argjson gumi_scores "$GUMI_SCORES" \
  '{
    schema_version: 1,
    benchmark: "lm-evaluation-harness",
    generated_at: $generated_at,
    configuration: {
      model: $model,
      gumi_model: $gumi_model,
      direct_base_url: $direct_base_url,
      gumi_base_url: $gumi_base_url,
      gumi_mode: $gumi_mode,
      tasks: ($tasks | split(",")),
      generation: $gen_kwargs
    },
    artifacts: {direct_results: $direct_results, gumi_results: $gumi_results},
    comparisons: [
      $direct_scores[] as $direct
      | $gumi_scores[]
      | select(.task == $direct.task)
      | {
          task: $direct.task,
          primary_metric: ($direct.primary_metric // .primary_metric),
          direct: $direct.primary_score,
          gumi: .primary_score,
          delta: (if $direct.primary_score == null or .primary_score == null then null else (.primary_score - $direct.primary_score) end),
          direct_metrics: $direct.metrics,
          gumi_metrics: .metrics
        }
    ]
  }' > "$SCORECARD_JSON"

{
  echo "# Standard Before/After Scorecard"
  echo
  echo "- **Model:** $MODEL"
  echo "- **Gumi mode:** $GUMI_MODE"
  echo "- **Tasks:** $TASKS"
  echo "- **Generation:** $GEN_KWARGS"
  echo "- **Generated:** $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo
  echo "| Task | Primary metric | Direct | Gumi | Delta |"
  echo "|---|---|---:|---:|---:|"
  jq -r '.comparisons[] | "| \(.task) | \(.primary_metric // "not selected") | \(.direct // "N/A") | \(.gumi // "N/A") | \(.delta // "N/A") |"' "$SCORECARD_JSON"
  echo
  echo "Raw lm-eval output is retained under this directory. Compare only runs with the same model artifact, task version, generation settings, and few-shot count."
} > "$SCORECARD_MD"

echo ""
echo "Scorecard: $SCORECARD_MD"
echo "Machine-readable report: $SCORECARD_JSON"
