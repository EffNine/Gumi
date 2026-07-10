#!/usr/bin/env bash
set -euo pipefail

# Novexa Local Model Benchmark
# Usage: ./scripts/benchmark-local-model.sh <model_name>
#
# Runs sequential requests against a local model in four modes:
#   A. Ollama direct with think:false
#   B. Novexa direct mode with model profile
#   C. Novexa stabilized mode with model profile
#   D. Novexa structured JSON mode with model profile
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

OLLAMA_URL="http://localhost:11434"
NOVEXA_URL="http://localhost:8787/v1"

# Override with environment variable
ATTEMPTS="${ATTEMPTS:-3}"

# Test suite: three prompts covering different use cases
PROMPT_CONCISE="What is 2+2? Answer in one word."
PROMPT_FACTUAL="What is the capital of France? Answer in one word."
PROMPT_JSON='Return a JSON object with keys "name" and "value". Use "test" and 42.'

pass=0
fail=0
total=0

results_json='[]'

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
  curl -s -X POST "$OLLAMA_URL/api/chat" \
    -H "Content-Type: application/json" \
    -d "$payload" > /dev/null 2>&1 || true
  sleep 1
}

warm_up_novexa() {
  echo ""
  echo "=== Warming up Novexa ==="
  local payload
  payload=$(jq -n \
    --arg model "$MODEL" \
    '{model: $model, messages: [{role: "user", content: "warm up"}], max_tokens: 10}')
  curl -s -X POST "$NOVEXA_URL/chat/completions" \
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
  response=$(curl -s -w "\n%{http_code}" -X POST "$OLLAMA_URL/api/chat" \
    -H "Content-Type: application/json" \
    -d "$payload" 2>/dev/null) || {
    record_result "A-OllamaDirect" "$prompt_label" "$attempt" "error" "0" "false" "false" "false" "false" "false" "curl failed"
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

run_novexa_mode() {
  local mode="$1"
  local prompt="$2"
  local prompt_label="$3"
  local attempt="$4"
  local mode_label="$5"

  local payload
  if [ "$mode" = "structured" ]; then
    payload=$(jq -n \
      --arg model "ollama:$MODEL" \
      --arg content "$prompt" \
      '{model: $model, messages: [{role: "user", content: $content}], response_format: {type: "json_object"}, max_tokens: 512}')
  else
    payload=$(jq -n \
      --arg model "ollama:$MODEL" \
      --arg content "$prompt" \
      '{model: $model, messages: [{role: "user", content: $content}], max_tokens: 512}')
  fi

  if [ "$mode" = "direct" ]; then
    payload=$(echo "$payload" | jq '.novexa = {mode: "direct"}')
  fi

  local start end latency_ms status response_content response_len

  start=$(date +%s%N)
  response=$(curl -s -w "\n%{http_code}" -X POST "$NOVEXA_URL/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer novexa-local" \
    -d "$payload" 2>/dev/null) || {
    record_result "$mode_label" "$prompt_label" "$attempt" "error" "0" "false" "false" "false" "false" "false" "curl failed"
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
echo "  Attempts per prompt: $ATTEMPTS"
echo "  Date:  $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
echo "============================================"

# Warm up
warm_up_ollama
warm_up_novexa

# Mode A: Ollama direct with think:false
echo ""
echo "=== Mode A: Ollama Direct (think:false) ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_ollama_direct "$PROMPT_CONCISE" "concise" "$i"
  run_ollama_direct "$PROMPT_FACTUAL" "factual" "$i"
  run_ollama_direct "$PROMPT_JSON" "json" "$i"
done

# Mode B: Novexa direct mode
echo ""
echo "=== Mode B: Novexa Direct Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "direct" "$PROMPT_CONCISE" "concise" "$i" "B-NovexaDirect"
  run_novexa_mode "direct" "$PROMPT_FACTUAL" "factual" "$i" "B-NovexaDirect"
  run_novexa_mode "direct" "$PROMPT_JSON" "json" "$i" "B-NovexaDirect"
done

# Mode C: Novexa stabilized mode
echo ""
echo "=== Mode C: Novexa Stabilized Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "stabilized" "$PROMPT_CONCISE" "concise" "$i" "C-NovexaStabilized"
  run_novexa_mode "stabilized" "$PROMPT_FACTUAL" "factual" "$i" "C-NovexaStabilized"
  run_novexa_mode "stabilized" "$PROMPT_JSON" "json" "$i" "C-NovexaStabilized"
done

# Mode D: Novexa structured JSON mode
echo ""
echo "=== Mode D: Novexa Structured JSON Mode ==="
for i in $(seq 1 "$ATTEMPTS"); do
  run_novexa_mode "structured" "$PROMPT_JSON" "json" "$i" "D-NovexaStructured"
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
for mode_label in "A-OllamaDirect" "B-NovexaDirect" "C-NovexaStabilized" "D-NovexaStructured"; do
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
for mode_label in "B-NovexaDirect" "C-NovexaStabilized" "D-NovexaStructured"; do
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
