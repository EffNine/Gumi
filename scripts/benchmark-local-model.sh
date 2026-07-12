#!/usr/bin/env bash
set -euo pipefail

# Novexa Local Model Benchmark
# Usage: ./scripts/benchmark-local-model.sh <model_name>
#
# Runs sequential requests against a local model in five modes:
#   A. Ollama direct with think:false
#   B. Novexa direct mode with model profile
#   C. Novexa lightweight mode with model profile
#   D. Novexa stabilized mode with model profile
#   E. Novexa structured JSON mode with model profile
#
# Requirements:
#   - ollama must be running on http://localhost:11434
#   - novexa must be running on http://localhost:8787
#   - The requested model must be pulled in ollama
#   - jq must be installed

MODEL="${1:-}"
if [ -z "$MODEL" ]; then
  echo "Usage: $0 <model_name>"
  echo "Example: $0 qwen3.5:2b"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"
LMSTUDIO_URL="${LMSTUDIO_URL:-http://localhost:1234/v1}"
NOVEXA_URL="${NOVEXA_URL:-http://localhost:8787/v1}"
NOVEXA_HEALTH_URL="${NOVEXA_HEALTH_URL:-${NOVEXA_URL%/v1}/health}"

# Override with environment variable
ATTEMPTS="${ATTEMPTS:-3}"
BENCHMARK_TIMEOUT_SECONDS="${BENCHMARK_TIMEOUT_SECONDS:-60}"
BENCHMARK_PROVIDER="${BENCHMARK_PROVIDER:-ollama}"
BENCHMARK_DISABLE_THINKING="${BENCHMARK_DISABLE_THINKING:-true}"

case "$BENCHMARK_PROVIDER" in
  ollama)
    DIRECT_MODE_LABEL="A-OllamaDirect"
    NOVEXA_MODEL="ollama:$MODEL"
    ;;
  lmstudio)
    DIRECT_MODE_LABEL="A-LMStudioDirect"
    NOVEXA_MODEL="lmstudio:$MODEL"
    LMSTUDIO_URL="${LMSTUDIO_URL%/}"
    ;;
  *)
    echo "Error: unsupported BENCHMARK_PROVIDER '$BENCHMARK_PROVIDER'. Use 'ollama' or 'lmstudio'." >&2
    exit 1
    ;;
esac

# Test suite: three prompts covering different use cases
PROMPT_CONCISE="What is 2+2? Answer in one word."
PROMPT_FACTUAL="What is the capital of France? Answer in one word."
PROMPT_JSON='Return a JSON object with keys "name" and "value". Use "test" and 42.'

pass=0
fail=0
total=0

results_json='[]'

preflight_checks() {
  if ! command -v jq > /dev/null 2>&1; then
    echo "Error: jq is required to run this benchmark." >&2
    exit 1
  fi
  if ! command -v curl > /dev/null 2>&1; then
    echo "Error: curl is required to run this benchmark." >&2
    exit 1
  fi
  case "$BENCHMARK_PROVIDER" in
    ollama)
      if ! curl --max-time 5 -fsS "$OLLAMA_URL/api/tags" > /dev/null 2>&1; then
        echo "Error: Ollama is not reachable at $OLLAMA_URL." >&2
        echo "Suggestion: start Ollama and make sure the model is pulled: ollama pull $MODEL" >&2
        exit 1
      fi
      ;;
    lmstudio)
      if ! curl --max-time 5 -fsS "$LMSTUDIO_URL/models" > /dev/null 2>&1; then
        echo "Error: LM Studio is not reachable at $LMSTUDIO_URL." >&2
        echo "Suggestion: start LM Studio server and enable OpenAI-compatible API." >&2
        exit 1
      fi
      ;;
  esac
  if ! curl --max-time 5 -fsS "$NOVEXA_HEALTH_URL" > /dev/null 2>&1; then
    echo "Error: Novexa is not reachable at $NOVEXA_HEALTH_URL." >&2
    echo "Suggestion: start Novexa with: cd runtime && go run ./cmd/novexa start" >&2
    exit 1
  fi
}

# Scoring helpers
score_exact_match() {
  local content="$1"
  local expected="$2"
  local cleaned
  cleaned=$(echo "$content" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]//g')
  local expected_clean
  expected_clean=$(echo "$expected" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]//g')
  if [ "$cleaned" = "$expected_clean" ]; then
    echo "true"
  else
    echo "false"
  fi
}

score_json_valid() {
  local content="$1"
  if echo "$content" | jq -e . > /dev/null 2>&1; then
    echo "true"
  else
    echo "false"
  fi
}

score_json_has_keys() {
  local content="$1"
  shift
  local all_have=true
  for key in "$@"; do
    if ! echo "$content" | jq -e ". | has(\"$key\")" > /dev/null 2>&1; then
      all_have=false
      break
    fi
  done
  echo "$all_have"
}

score_no_markdown_fence() {
  local content="$1"
  if echo "$content" | grep -qE '^```'; then
    echo "false"
  else
    echo "true"
  fi
}

record_result() {
  local mode="$1"
  local prompt_label="$2"
  local attempt="$3"
  local status="$4"
  local latency_ms="$5"
  local passed_validation="$6"
  local exact_match="$7"
  local json_valid="$8"
  local json_keys="$9"
  local no_fence="${10}"
  local note="${11}"

  total=$((total + 1))
  if [ "$passed_validation" = "true" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
  fi

  results_json=$(echo "$results_json" | jq \
    --arg mode "$mode" \
    --arg prompt "$prompt_label" \
    --arg attempt "$attempt" \
    --arg status "$status" \
    --arg latency "$latency_ms" \
    --arg passed "$passed_validation" \
    --arg exact "$exact_match" \
    --arg json_valid "$json_valid" \
    --arg json_keys "$json_keys" \
    --arg no_fence "$no_fence" \
    --arg note "$note" \
    '. + [{mode: $mode, prompt: $prompt, attempt: $attempt, status: $status, latency: $latency, passed: $passed, exact_match: $exact, json_valid: $json_valid, json_keys: $json_keys, no_fence: $no_fence, note: $note}]')
}

warm_up_ollama() {
  echo ""
  echo "=== Warming up Ollama: $MODEL ==="
  local payload
  payload=$(jq -n \
    --arg model "$MODEL" \
    '{model: $model, messages: [{role: "user", content: "warm up"}], stream: false, options: {num_predict: 10}, think: false}')
  curl --max-time "$BENCHMARK_TIMEOUT_SECONDS" -s -X POST "$OLLAMA_URL/api/chat" \
    -H "Content-Type: application/json" \
    -d "$payload" > /dev/null 2>&1 || true
  sleep 1
}

warm_up_lmstudio() {
  echo ""
  echo "=== Warming up LM Studio: $MODEL ==="
  local payload
  payload=$(jq -n \
    --arg model "$MODEL" \
    '{model: $model, messages: [{role: "user", content: "warm up"}], stream: false, max_tokens: 10}')
  if [ "$BENCHMARK_DISABLE_THINKING" = "true" ]; then
    payload=$(echo "$payload" | jq '.reasoning_effort = "none"')
  fi
  curl --max-time "$BENCHMARK_TIMEOUT_SECONDS" -s -X POST "$LMSTUDIO_URL/chat/completions" \
    -H "Content-Type: application/json" \
    -d "$payload" > /dev/null 2>&1 || true
  sleep 1
}

warm_up_novexa() {
  echo ""
  echo "=== Warming up Novexa ==="
  local payload
  payload=$(jq -n \
    --arg model "$NOVEXA_MODEL" \
    '{model: $model, messages: [{role: "user", content: "warm up"}], max_tokens: 10}')
  if [ "$BENCHMARK_DISABLE_THINKING" = "true" ]; then
    payload=$(echo "$payload" | jq '.novexa.thinking.enabled = false')
  fi
  curl --max-time "$BENCHMARK_TIMEOUT_SECONDS" -s -X POST "$NOVEXA_URL/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer novexa-local" \
    -d "$payload" > /dev/null 2>&1 || true
  sleep 1
}

run_ollama_direct() {
  local prompt="$1"
  local prompt_label="$2"
  local attempt="$3"

  local payload
  payload=$(jq -n \
    --arg model "$MODEL" \
    --arg content "$prompt" \
    '{model: $model, messages: [{role: "user", content: $content}], stream: false, options: {num_predict: 512}, think: false}')

  local start end latency_ms status response_content response_len

  start=$(date +%s%N)
  response=$(curl --max-time "$BENCHMARK_TIMEOUT_SECONDS" -s -w "\n%{http_code}" -X POST "$OLLAMA_URL/api/chat" \
    -H "Content-Type: application/json" \
    -d "$payload" 2>/dev/null) || {
    record_result "A-OllamaDirect" "$prompt_label" "$attempt" "error" "0" "false" "false" "false" "false" "false" "curl failed or timed out after ${BENCHMARK_TIMEOUT_SECONDS}s"
    return
  }
  end=$(date +%s%N)
  latency_ms=$(( (end - start) / 1000000 ))

  status=$(echo "$response" | tail -1)
  body=$(echo "$response" | sed '$d')

  if [ "$status" != "200" ]; then
    record_result "A-OllamaDirect" "$prompt_label" "$attempt" "$status" "$latency_ms" "false" "false" "false" "false" "false" "HTTP $status"
    return
  fi

  response_content=$(echo "$body" | jq -r '.message.content // ""')
  response_len=${#response_content}

  if [ -z "$response_content" ] || [ "$response_content" = "null" ]; then
    record_result "A-OllamaDirect" "$prompt_label" "$attempt" "$status" "$latency_ms" "false" "false" "false" "false" "false" "empty response"
    return
  fi

  local exact="false"
  local json_valid="false"
  local json_keys="false"
  local no_fence="false"

  no_fence=$(score_no_markdown_fence "$response_content")

  if [ "$prompt_label" = "concise" ]; then
    exact=$(score_exact_match "$response_content" "4")
  elif [ "$prompt_label" = "factual" ]; then
    exact=$(score_exact_match "$response_content" "paris")
  elif [ "$prompt_label" = "json" ]; then
    json_valid=$(score_json_valid "$response_content")
    if [ "$json_valid" = "true" ]; then
      json_keys=$(score_json_has_keys "$response_content" "name" "value")
    fi
  fi

  local passed_validation="false"
  if [ "$prompt_label" = "concise" ] || [ "$prompt_label" = "factual" ]; then
    passed_validation="$exact"
  elif [ "$prompt_label" = "json" ] && [ "$json_valid" = "true" ] && [ "$json_keys" = "true" ] && [ "$no_fence" = "true" ]; then
    passed_validation="true"
  fi

  record_result "A-OllamaDirect" "$prompt_label" "$attempt" "$status" "$latency_ms" "$passed_validation" "$exact" "$json_valid" "$json_keys" "$no_fence" "ok (${response_len} chars)"
}

run_lmstudio_direct() {
  local prompt="$1"
  local prompt_label="$2"
  local attempt="$3"

  local payload
  payload=$(jq -n \
    --arg model "$MODEL" \
    --arg content "$prompt" \
    '{model: $model, messages: [{role: "user", content: $content}], stream: false, max_tokens: 512}')
  if [ "$BENCHMARK_DISABLE_THINKING" = "true" ]; then
    payload=$(echo "$payload" | jq '.reasoning_effort = "none"')
  fi

  local start end latency_ms status response_content response_len

  start=$(date +%s%N)
  response=$(curl --max-time "$BENCHMARK_TIMEOUT_SECONDS" -s -w "\n%{http_code}" -X POST "$LMSTUDIO_URL/chat/completions" \
    -H "Content-Type: application/json" \
    -d "$payload" 2>/dev/null) || {
    record_result "A-LMStudioDirect" "$prompt_label" "$attempt" "error" "0" "false" "false" "false" "false" "false" "curl failed or timed out after ${BENCHMARK_TIMEOUT_SECONDS}s"
    return
  }
  end=$(date +%s%N)
  latency_ms=$(( (end - start) / 1000000 ))

  status=$(echo "$response" | tail -1)
  body=$(echo "$response" | sed '$d')

  if [ "$status" != "200" ]; then
    record_result "A-LMStudioDirect" "$prompt_label" "$attempt" "$status" "$latency_ms" "false" "false" "false" "false" "false" "HTTP $status"
    return
  fi

  response_content=$(echo "$body" | jq -r '.choices[0].message.content // ""')
  response_len=${#response_content}

  if [ -z "$response_content" ] || [ "$response_content" = "null" ]; then
    record_result "A-LMStudioDirect" "$prompt_label" "$attempt" "$status" "$latency_ms" "false" "false" "false" "false" "false" "empty response"
    return
  fi

  local exact="false"
  local json_valid="false"
  local json_keys="false"
  local no_fence="false"

  no_fence=$(score_no_markdown_fence "$response_content")

  if [ "$prompt_label" = "concise" ]; then
    exact=$(score_exact_match "$response_content" "4")
  elif [ "$prompt_label" = "factual" ]; then
    exact=$(score_exact_match "$response_content" "paris")
  elif [ "$prompt_label" = "json" ]; then
    json_valid=$(score_json_valid "$response_content")
    if [ "$json_valid" = "true" ]; then
      json_keys=$(score_json_has_keys "$response_content" "name" "value")
    fi
  fi

  local passed_validation="false"
  if [ "$prompt_label" = "concise" ] || [ "$prompt_label" = "factual" ]; then
    passed_validation="$exact"
  elif [ "$prompt_label" = "json" ] && [ "$json_valid" = "true" ] && [ "$json_keys" = "true" ] && [ "$no_fence" = "true" ]; then
    passed_validation="true"
  fi

  record_result "A-LMStudioDirect" "$prompt_label" "$attempt" "$status" "$latency_ms" "$passed_validation" "$exact" "$json_valid" "$json_keys" "$no_fence" "ok (${response_len} chars)"
}

run_direct_provider() {
  case "$BENCHMARK_PROVIDER" in
    ollama)
      run_ollama_direct "$@"
      ;;
    lmstudio)
      run_lmstudio_direct "$@"
      ;;
  esac
}

run_novexa_mode() {
  local mode="$1"
  local prompt="$2"
  local prompt_label="$3"
  local attempt="$4"
  local mode_label="$5"

  local payload
  if [ "$mode" = "structured" ]; then
    payload=$(jq -n \
      --arg model "$NOVEXA_MODEL" \
      --arg content "$prompt" \
      '{model: $model, messages: [{role: "user", content: $content}], response_format: {type: "json_object"}, max_tokens: 512}')
  else
    payload=$(jq -n \
      --arg model "$NOVEXA_MODEL" \
      --arg content "$prompt" \
      '{model: $model, messages: [{role: "user", content: $content}], max_tokens: 512}')
  fi

  if [ "$mode" = "direct" ]; then
    payload=$(echo "$payload" | jq '.novexa.mode = "direct"')
  elif [ "$mode" = "lightweight" ]; then
    payload=$(echo "$payload" | jq '.novexa.mode = "lightweight"')
  fi

  if [ "$BENCHMARK_DISABLE_THINKING" = "true" ]; then
    payload=$(echo "$payload" | jq '.novexa.thinking.enabled = false')
  fi

  local start end latency_ms status response_content response_len

  start=$(date +%s%N)
  response=$(curl --max-time "$BENCHMARK_TIMEOUT_SECONDS" -s -w "\n%{http_code}" -X POST "$NOVEXA_URL/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer novexa-local" \
    -d "$payload" 2>/dev/null) || {
    record_result "$mode_label" "$prompt_label" "$attempt" "error" "0" "false" "false" "false" "false" "false" "curl failed or timed out after ${BENCHMARK_TIMEOUT_SECONDS}s"
    return
  }
  end=$(date +%s%N)
  latency_ms=$(( (end - start) / 1000000 ))

  status=$(echo "$response" | tail -1)
  body=$(echo "$response" | sed '$d')

  if [ "$status" != "200" ]; then
    error_code=$(echo "$body" | jq -r '.error.code // "unknown"')
    record_result "$mode_label" "$prompt_label" "$attempt" "$status" "$latency_ms" "false" "false" "false" "false" "false" "error: $error_code"
    return
  fi

  response_content=$(echo "$body" | jq -r '.choices[0].message.content // ""')
  response_len=${#response_content}

  if [ -z "$response_content" ] || [ "$response_content" = "null" ]; then
    record_result "$mode_label" "$prompt_label" "$attempt" "$status" "$latency_ms" "false" "false" "false" "false" "false" "empty response"
    return
  fi

  local exact="false"
  local json_valid="false"
  local json_keys="false"
  local no_fence="false"

  no_fence=$(score_no_markdown_fence "$response_content")

  if [ "$prompt_label" = "concise" ]; then
    exact=$(score_exact_match "$response_content" "4")
  elif [ "$prompt_label" = "factual" ]; then
    exact=$(score_exact_match "$response_content" "paris")
  elif [ "$prompt_label" = "json" ]; then
    json_valid=$(score_json_valid "$response_content")
    if [ "$json_valid" = "true" ]; then
      json_keys=$(score_json_has_keys "$response_content" "name" "value")
    fi
  fi

  local passed_validation="false"
  if [ "$prompt_label" = "concise" ] || [ "$prompt_label" = "factual" ]; then
    passed_validation="$exact"
  elif [ "$prompt_label" = "json" ] && [ "$json_valid" = "true" ] && [ "$json_keys" = "true" ] && [ "$no_fence" = "true" ]; then
    passed_validation="true"
  fi

  record_result "$mode_label" "$prompt_label" "$attempt" "$status" "$latency_ms" "$passed_validation" "$exact" "$json_valid" "$json_keys" "$no_fence" "ok (${response_len} chars)"
}

# Aggregate stats helper
compute_stats() {
  local field="$1"
  echo "$results_json" | jq -r "[.[] | select(.status == \"200\") | .$field | select(. != \"\") | tonumber] | sort" | jq -r \
    'if length == 0 then {} else {
      count: length,
      min: .[0],
      max: .[-1],
      p50: .[(length * 0.50) | floor],
      p95: .[(length * 0.95) | floor],
      avg: (add / length)
    } end'
}

echo "============================================"
echo "  Novexa Local Model Benchmark"
echo "  Model: $MODEL"
echo "  Provider: $BENCHMARK_PROVIDER"
echo "  Attempts per prompt: $ATTEMPTS"
echo "  Date:  $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "============================================"

preflight_checks

case "$BENCHMARK_PROVIDER" in
  ollama)
    warm_up_ollama
    ;;
  lmstudio)
    warm_up_lmstudio
    ;;
esac
warm_up_novexa

# Mode A: direct provider
echo ""
echo "=== Mode A: $BENCHMARK_PROVIDER Direct ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_direct_provider "$PROMPT_CONCISE" "concise" "$i"
  run_direct_provider "$PROMPT_FACTUAL" "factual" "$i"
  run_direct_provider "$PROMPT_JSON" "json" "$i"
done

# Mode B: Novexa direct mode
echo ""
echo "=== Mode B: Novexa Direct Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "direct" "$PROMPT_CONCISE" "concise" "$i" "B-NovexaDirect"
  run_novexa_mode "direct" "$PROMPT_FACTUAL" "factual" "$i" "B-NovexaDirect"
  run_novexa_mode "direct" "$PROMPT_JSON" "json" "$i" "B-NovexaDirect"
done

# Mode C: Novexa lightweight mode
echo ""
echo "=== Mode C: Novexa Lightweight Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "lightweight" "$PROMPT_CONCISE" "concise" "$i" "C-NovexaLightweight"
  run_novexa_mode "lightweight" "$PROMPT_FACTUAL" "factual" "$i" "C-NovexaLightweight"
  run_novexa_mode "lightweight" "$PROMPT_JSON" "json" "$i" "C-NovexaLightweight"
done

# Mode D: Novexa stabilized mode
echo ""
echo "=== Mode D: Novexa Stabilized Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "stabilized" "$PROMPT_CONCISE" "concise" "$i" "D-NovexaStabilized"
  run_novexa_mode "stabilized" "$PROMPT_FACTUAL" "factual" "$i" "D-NovexaStabilized"
  run_novexa_mode "stabilized" "$PROMPT_JSON" "json" "$i" "D-NovexaStabilized"
done

# Mode E: Novexa structured JSON mode
echo ""
echo "=== Mode E: Novexa Structured JSON Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "structured" "$PROMPT_JSON" "json" "$i" "E-NovexaStructured"
done

# Print per-request table
echo ""
echo "============================================"
echo "  Per-Request Results"
echo "============================================"
echo ""
echo "| Mode | Prompt | # | Status | Lat(ms) | Pass | Exact | JSON | Keys | NoFence | Note |"
echo "|------|--------|---|--------|---------|------|-------|------|------|---------|------|"
echo "$results_json" | jq -r '.[] | "| \(.mode) | \(.prompt) | \(.attempt) | \(.status) | \(.latency) | \(.passed) | \(.exact_match) | \(.json_valid) | \(.json_keys) | \(.no_fence) | \(.note) |"'

# Aggregate quality metrics
echo ""
echo "============================================"
echo "  Aggregate Quality Metrics"
echo "============================================"
echo ""
echo "| Metric | Value |"
echo "|--------|-------|"

total_requests=$(echo "$results_json" | jq 'length')
passed_requests=$(echo "$results_json" | jq '[.[] | select(.passed == "true")] | length')
failed_requests=$(echo "$results_json" | jq '[.[] | select(.passed == "false")] | length')
exact_count=$(echo "$results_json" | jq '[.[] | select(.exact_match == "true")] | length')
json_valid_count=$(echo "$results_json" | jq '[.[] | select(.json_valid == "true")] | length')
json_keys_count=$(echo "$results_json" | jq '[.[] | select(.json_keys == "true")] | length')
no_fence_count=$(echo "$results_json" | jq '[.[] | select(.no_fence == "true")] | length')
empty_count=$(echo "$results_json" | jq '[.[] | select(.note == "empty response")] | length')
error_count=$(echo "$results_json" | jq '[.[] | select(.status != "200")] | length')

echo "| Total requests | $total_requests |"
echo "| Passed | $passed_requests |"
echo "| Failed | $failed_requests |"
echo "| Empty responses | $empty_count |"
echo "| HTTP/curl errors | $error_count |"
echo "| Exact instruction following | $exact_count / $(echo "$results_json" | jq '[.[] | select(.prompt == "concise" or .prompt == "factual")] | length') |"
echo "| Valid JSON (all JSON prompts) | $json_valid_count / $(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length') |"
echo "| JSON with required keys | $json_keys_count / $(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length') |"
echo "| No markdown fences | $no_fence_count / $total_requests |"

# Latency stats
echo ""
echo "**Latency Statistics (successful requests only):**"
echo ""
echo "| Statistic | Value (ms) |"
echo "|-----------|-----------|"
lat_stats=$(compute_stats "latency")
echo "$lat_stats" | jq -r 'to_entries[] | "| \(.key) | \(.value | tostring) |"'

# Per-mode latency
echo ""
echo "**Per-Mode Latency (p50 / p95):**"
echo ""
echo "| Mode | p50 (ms) | p95 (ms) | Count |"
echo "|------|----------|----------|-------|"
for mode_label in "$DIRECT_MODE_LABEL" "B-NovexaDirect" "C-NovexaLightweight" "D-NovexaStabilized" "E-NovexaStructured"; do
  mode_stats=$(echo "$results_json" | jq -r "[.[] | select(.mode == \"$mode_label\" and .status == \"200\") | .latency | tonumber] | sort")
  count=$(echo "$mode_stats" | jq 'length')
  if [ "$count" -gt 0 ]; then
    p50=$(echo "$mode_stats" | jq '.[(length * 0.50) | floor]')
    p95=$(echo "$mode_stats" | jq '.[(length * 0.95) | floor]')
    echo "| $mode_label | $p50 | $p95 | $count |"
  else
    echo "| $mode_label | N/A | N/A | 0 |"
  fi
done

# Repair/retry summary (Novexa modes only)
echo ""
echo "**Novexa Repair & Retry Summary:**"
echo ""
echo "| Mode | Repairs | Retries |"
echo "|------|---------|---------|"
for mode_label in "B-NovexaDirect" "C-NovexaLightweight" "D-NovexaStabilized" "E-NovexaStructured"; do
  repairs=$(echo "$results_json" | jq -r "[.[] | select(.mode == \"$mode_label\") | .note | select(contains(\"repair\") or contains(\"retry\"))] | length")
  echo "| $mode_label | $repairs | - |"
done

# Failed requests detail
if [ "$fail" -gt 0 ]; then
  echo ""
  echo "**Failed requests detail:**"
  echo "$results_json" | jq -r '.[] | select(.passed == "false") | "  - \(.mode)/\(.prompt) attempt \(.attempt): \(.note)"'
fi

echo ""
echo "---"
echo "Benchmark completed at $(date -u '+%Y-%m-%dT%H:%M:%SZ')"

successful_requests=$(echo "$results_json" | jq '[.[] | select(.status == "200")] | length')
if [ "$successful_requests" -eq 0 ]; then
  echo "No successful benchmark requests were recorded. Report export skipped." >&2
  exit 1
fi

# ─────────────────────────────────────────────────────────────────
#  Markdown report export
# ─────────────────────────────────────────────────────────────────

model_safe_name=$(echo "$MODEL" | sed 's/[^a-zA-Z0-9_-]/-/g')
timestamp=$(date -u '+%Y%m%dT%H%M%SZ')
completed_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
raw_dir="$HOME/.novexa/benchmarks/local-model"
report_dir="$REPO_DIR/benchmarks/reports"
report_file="${report_dir}/${model_safe_name}-local-${timestamp}.md"
report_json_file="${raw_dir}/${model_safe_name}-${timestamp}.json"

mkdir -p "$raw_dir" "$report_dir"

direct_pass_rate=$(echo "$results_json" | jq --arg mode "$DIRECT_MODE_LABEL" '[.[] | select(.mode == $mode and .passed == "true")] | length')
direct_total=$(echo "$results_json" | jq --arg mode "$DIRECT_MODE_LABEL" '[.[] | select(.mode == $mode)] | length')
novexa_pass=$(echo "$results_json" | jq '[.[] | select((.mode | startswith("B-") or startswith("C-") or startswith("D-") or startswith("E-")) and .passed == "true")] | length')
novexa_total=$(echo "$results_json" | jq '[.[] | select((.mode | startswith("B-") or startswith("C-") or startswith("D-") or startswith("E-")))] | length')

direct_p50=$(echo "$results_json" | jq -r --arg mode "$DIRECT_MODE_LABEL" '[.[] | select(.mode == $mode and .status == "200") | .latency | tonumber] | sort | .[(length * 0.50) | floor] // "N/A"')
novexa_direct_p50=$(echo "$results_json" | jq -r '[.[] | select(.mode == "B-NovexaDirect" and .status == "200") | .latency | tonumber] | sort | .[(length * 0.50) | floor] // "N/A"')

conclusion=""
if [ "$novexa_total" -gt 0 ] && [ "$direct_total" -gt 0 ]; then
  if [ "$novexa_pass" -ge "$direct_pass_rate" ] 2>/dev/null; then
    if [ "$novexa_direct_p50" != "N/A" ] && [ "$direct_p50" != "N/A" ] && [ "$novexa_direct_p50" -le $((direct_p50 * 2)) ] 2>/dev/null; then
      conclusion="Worth it — Novexa modes match or exceed direct provider quality with acceptable latency overhead."
    else
      conclusion="Needs tuning — Novexa quality is acceptable but latency overhead is high."
    fi
  else
    conclusion="Needs tuning — Novexa modes underperform direct provider on quality. Review profile settings and prompt instructions."
  fi
else
  conclusion="Insufficient data — one or more modes produced no results."
fi

latency_by_mode=$(echo "$results_json" | jq --arg direct_mode "$DIRECT_MODE_LABEL" '
  def stats:
    if length == 0 then {count: 0, min: null, max: null, p50: null, p95: null, avg: null}
    else sort | {count: length, min: .[0], max: .[-1], p50: .[(length * 0.50) | floor], p95: .[(length * 0.95) | floor], avg: (add / length)}
    end;
  {
    ($direct_mode): ([.[] | select(.mode == $direct_mode and .status == "200") | .latency | tonumber] | stats),
    "B-NovexaDirect": ([.[] | select(.mode == "B-NovexaDirect" and .status == "200") | .latency | tonumber] | stats),
    "C-NovexaLightweight": ([.[] | select(.mode == "C-NovexaLightweight" and .status == "200") | .latency | tonumber] | stats),
    "D-NovexaStabilized": ([.[] | select(.mode == "D-NovexaStabilized" and .status == "200") | .latency | tonumber] | stats),
    "E-NovexaStructured": ([.[] | select(.mode == "E-NovexaStructured" and .status == "200") | .latency | tonumber] | stats)
  }')

jq -n \
  --arg model "$MODEL" \
  --arg provider "$BENCHMARK_PROVIDER" \
  --arg direct_mode "$DIRECT_MODE_LABEL" \
  --arg generated_at "$completed_at" \
  --argjson attempts "$ATTEMPTS" \
  --arg conclusion "$conclusion" \
  --argjson results "$results_json" \
  --argjson latency_by_mode "$latency_by_mode" \
  --argjson total_requests "$total_requests" \
  --argjson passed_requests "$passed_requests" \
  --argjson failed_requests "$failed_requests" \
  --argjson empty_count "$empty_count" \
  --argjson error_count "$error_count" \
  --argjson exact_count "$exact_count" \
  --argjson exact_total "$(echo "$results_json" | jq '[.[] | select(.prompt == "concise" or .prompt == "factual")] | length')" \
  --argjson json_valid_count "$json_valid_count" \
  --argjson json_total "$(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length')" \
  --argjson json_keys_count "$json_keys_count" \
  --argjson no_fence_count "$no_fence_count" \
  '{
    schema_version: 1,
    model: $model,
    provider: $provider,
    generated_at: $generated_at,
    attempts_per_prompt: $attempts,
    modes_tested: [$direct_mode, "B-NovexaDirect", "C-NovexaLightweight", "D-NovexaStabilized", "E-NovexaStructured"],
    quality: {
      total_requests: $total_requests,
      passed: $passed_requests,
      failed: $failed_requests,
      empty_responses: $empty_count,
      http_or_curl_errors: $error_count,
      exact_instruction_following: {passed: $exact_count, total: $exact_total},
      valid_json: {passed: $json_valid_count, total: $json_total},
      json_with_required_keys: {passed: $json_keys_count, total: $json_total},
      no_markdown_fences: {passed: $no_fence_count, total: $total_requests}
    },
    latency_by_mode: $latency_by_mode,
    conclusion: $conclusion,
    results: $results
  }' > "$report_json_file"

cat > "$report_file" << REPORTEOF
# Benchmark Report

**Model:** $MODEL
**Provider:** $BENCHMARK_PROVIDER
**Date:** $completed_at
**Attempts per prompt:** $ATTEMPTS
**Modes tested:** $DIRECT_MODE_LABEL, B-NovexaDirect, C-NovexaLightweight, D-NovexaStabilized, E-NovexaStructured

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | $total_requests |
| Passed | $passed_requests |
| Failed | $failed_requests |
| Empty responses | $empty_count |
| HTTP/curl errors | $error_count |
| Exact instruction following | $exact_count / $(echo "$results_json" | jq '[.[] | select(.prompt == "concise" or .prompt == "factual")] | length') |
| Valid JSON (all JSON prompts) | $json_valid_count / $(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length') |
| JSON with required keys | $json_keys_count / $(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length') |
| No markdown fences | $no_fence_count / $total_requests |

## Per-Mode Latency

| Mode | p50 (ms) | p95 (ms) | Count |
|------|----------|----------|-------|
REPORTEOF

for mode_label in "$DIRECT_MODE_LABEL" "B-NovexaDirect" "C-NovexaLightweight" "D-NovexaStabilized" "E-NovexaStructured"; do
  mode_stats=$(echo "$results_json" | jq -r "[.[] | select(.mode == \"$mode_label\" and .status == \"200\") | .latency | tonumber] | sort")
  count=$(echo "$mode_stats" | jq 'length')
  if [ "$count" -gt 0 ]; then
    p50=$(echo "$mode_stats" | jq '.[(length * 0.50) | floor]')
    p95=$(echo "$mode_stats" | jq '.[(length * 0.95) | floor]')
    echo "| $mode_label | $p50 | $p95 | $count |" >> "$report_file"
  else
    echo "| $mode_label | N/A | N/A | 0 |" >> "$report_file"
  fi
done

cat >> "$report_file" << REPORTEOF

## Per-Request Results

| Mode | Prompt | # | Status | Lat(ms) | Pass | Exact | JSON | Keys | NoFence | Note |
|------|--------|---|--------|---------|------|-------|------|------|---------|------|
REPORTEOF

echo "$results_json" | jq -r '.[] | "| \(.mode) | \(.prompt) | \(.attempt) | \(.status) | \(.latency) | \(.passed) | \(.exact_match) | \(.json_valid) | \(.json_keys) | \(.no_fence) | \(.note) |"' >> "$report_file"

cat >> "$report_file" << REPORTEOF

## Conclusion

$conclusion
REPORTEOF

echo ""
echo "Markdown report saved to $report_file"
echo "JSON report saved to $report_json_file"
