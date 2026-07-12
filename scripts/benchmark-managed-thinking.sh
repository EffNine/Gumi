#!/usr/bin/env bash
set -euo pipefail

# Novexa Managed Thinking Benchmark
# Usage: ./scripts/benchmark-managed-thinking.sh <model_name>
#
# Compares three configurations:
#   A. Direct LM Studio with thinking enabled
#   B. Novexa with managed thinking (thinking enabled via request override)
#   C. Novexa with thinking disabled (default)
#
# Requirements:
#   - LM Studio running on http://localhost:1234/v1
#   - Novexa running on http://localhost:8787/v1
#   - jq installed

MODEL="${1:-}"
if [ -z "$MODEL" ]; then
  echo "Usage: $0 <model_name>"
  echo "Example: $0 qwen/qwen3.5-9b"
  exit 1
fi
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
LMSTUDIO_URL="${LMSTUDIO_URL:-http://localhost:1234/v1}"
NOVEXA_URL="${NOVEXA_URL:-http://127.0.0.1:8787/v1}"
NOVEXA_HEALTH_URL="${NOVEXA_HEALTH_URL:-${NOVEXA_URL%/v1}/health}"
ATTEMPTS="${ATTEMPTS:-1}"
BENCHMARK_TIMEOUT_SECONDS="${BENCHMARK_TIMEOUT_SECONDS:-180}"

NOVEXA_MODEL="lmstudio:$MODEL"
results_json='[]'
pass=0
fail=0
total=0

preflight_checks() {
  if ! command -v jq > /dev/null 2>&1; then
    echo "Error: jq is required." >&2
    exit 1
  fi
  if ! curl --max-time 5 -fsS "$LMSTUDIO_URL/models" > /dev/null 2>&1; then
    echo "Error: LM Studio not reachable at $LMSTUDIO_URL." >&2
    exit 1
  fi
  if ! curl --max-time 5 -fsS "$NOVEXA_HEALTH_URL" > /dev/null 2>&1; then
    echo "Error: Novexa not reachable at $NOVEXA_HEALTH_URL." >&2
    exit 1
  fi
}

make_payload() {
  local base_url="$1"
  local prompt="$2"
  local thinking="$3"

  local model
  if [ "$base_url" = "$LMSTUDIO_URL" ]; then
    model="$MODEL"
  else
    model="$NOVEXA_MODEL"
  fi

  local thinking_block=""
  if [ "$base_url" = "$NOVEXA_URL" ]; then
    thinking_block=$(cat <<EOF
,"novexa":{"thinking":{"enabled":$thinking}}
EOF
)
  fi

  cat <<EOF
{
  "model": "$model",
  "messages": [{"role": "user", "content": $prompt}],
  "max_tokens": 4096,
  "temperature": 0.4
  $thinking_block
}
EOF
}

call_api() {
  local url="$1"
  local payload="$2"
  if [ "$url" = "$NOVEXA_URL" ]; then
    curl -sS --max-time "$BENCHMARK_TIMEOUT_SECONDS" -X POST "$url/chat/completions" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer novexa-local" \
      -d "$payload"
  else
    curl -sS --max-time "$BENCHMARK_TIMEOUT_SECONDS" -X POST "$url/chat/completions" \
      -H "Content-Type: application/json" \
      -d "$payload"
  fi
}

extract_content() {
  local resp="$1"
  echo "$resp" | jq -r '.choices[0].message.content // ""'
}

has_reasoning_leak() {
  local content="$1"
  # Only flag explicit reasoning/thinking markers, not normal prose that
  # happens to contain words like "think" or "reason".
  if echo "$content" | grep -qiE '(<thinking>|</thinking>|<reasoning>|</reasoning>|```thinking|```reasoning|\[thinking\]|\[/thinking\]|\[reasoning\]|\[/reasoning\])'; then
    echo "true"
  else
    echo "false"
  fi
}

score_response_nonempty() {
  local content="$1"
  if [ -n "$content" ]; then
    echo "true"
  else
    echo "false"
  fi
}

record_result() {
  local mode="$1"
  local prompt_label="$2"
  local attempt="$3"
  local status="$4"
  local latency_ms="$5"
  local nonempty="$6"
  local has_leak="$7"
  local content_length="$8"
  local content_preview="$9"

  results_json=$(echo "$results_json" | jq \
    --arg mode "$mode" \
    --arg prompt "$prompt_label" \
    --arg attempt "$attempt" \
    --arg status "$status" \
    --arg latency "$latency_ms" \
    --arg nonempty "$nonempty" \
    --arg has_leak "$has_leak" \
    --arg content_length "$content_length" \
    --arg content_preview "$content_preview" \
    '. + [{
      mode: $mode,
      prompt: $prompt,
      attempt: ($attempt | tonumber),
      status: ($status | try tonumber catch 422),
      latency_ms: ($latency | tonumber),
      response_nonempty: ($nonempty == "true"),
      no_reasoning_leak: ($has_leak == "false"),
      content_length: ($content_length | tonumber),
      content_preview: $content_preview
    }]')

  total=$((total + 1))
  if [ "$nonempty" = "true" ] && [ "$has_leak" = "false" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
  fi
}

run_prompt() {
  local mode_label="$1"
  local base_url="$2"
  local prompt_label="$3"
  local prompt="$4"
  local thinking="$5"

  for attempt in $(seq 1 "$ATTEMPTS"); do
    local payload
    payload=$(make_payload "$base_url" "$prompt" "$thinking")
    local start
    start=$(date +%s%N)
    local resp
    resp=$(call_api "$base_url" "$payload") || resp='{"error":"curl_failed"}'
    local end
    end=$(date +%s%N)
    local latency_ms=$(( (end - start) / 1000000 ))

    local status
    if echo "$resp" | jq -e '.error' > /dev/null 2>&1; then
      status=$(echo "$resp" | jq -r '.error.code // 422')
      if [ "$status" = "null" ] || [ -z "$status" ]; then
        status=422
      fi
    else
      status=$(echo "$resp" | jq -r 'if .choices[0].message != null then 200 else 500 end')
    fi
    if [ -z "$status" ] || [ "$status" = "null" ]; then
      status=500
    fi

    local content
    content=$(extract_content "$resp")
    local nonempty
    nonempty=$(score_response_nonempty "$content")
    local has_leak
    has_leak=$(has_reasoning_leak "$content")

    record_result "$mode_label" "$prompt_label" "$attempt" "$status" "$latency_ms" "$nonempty" "$has_leak" "${#content}" "$(echo "$content" | head -c 80 | jq -sR .)"
  done
}

run_benchmark() {
  local mode_label="$1"
  local base_url="$2"
  local thinking="$3"

  # Complex debugging prompt
  local debug_prompt='"You are reviewing a Go function that occasionally returns wrong results under concurrent load. The function uses a shared map without synchronization. Explain the likely bug, why it happens, and suggest a minimal fix."'
  run_prompt "$mode_label" "$base_url" "debugging" "$debug_prompt" "$thinking"

  # Multi-step planning prompt
  local plan_prompt='"Plan a refactor to split a 500-line monolithic HTTP handler into three smaller handlers with shared middleware. List the files you would create and the responsibilities of each."'
  run_prompt "$mode_label" "$base_url" "planning" "$plan_prompt" "$thinking"

  # JSON prompt (managed thinking should be disabled here)
  local json_prompt='"Return a JSON object with keys name and value. Use test and 42."'
  run_prompt "$mode_label" "$base_url" "json" "$json_prompt" "$thinking"
}

write_report() {
  local timestamp
  timestamp=$(date -u +"%Y%m%dT%H%M%SZ")
  local raw_dir="$HOME/.novexa/benchmarks/managed-thinking"
  local report_dir="$REPO_DIR/benchmarks/reports"
  mkdir -p "$raw_dir" "$report_dir"
  local base="$raw_dir/${MODEL//\//_}-${timestamp}"
  local report_base="$report_dir/${MODEL//\//_}-thinking-${timestamp}"

  echo "$results_json" | jq . > "$base.json"

  cat > "$report_base.md" <<EOF
# Managed Thinking Benchmark Report

**Model:** $MODEL
**Provider:** lmstudio
**Date:** $timestamp
**Attempts per prompt:** $ATTEMPTS

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | $total |
| Passed (non-empty + no leak) | $pass |
| Failed | $fail |
| Non-empty responses | $(echo "$results_json" | jq '[.[] | select(.response_nonempty == true)] | length') / $total |
| No reasoning leak | $(echo "$results_json" | jq '[.[] | select(.no_reasoning_leak == true)] | length') / $total |

## Per-Request Results

| Mode | Prompt | Attempt | Status | Lat(ms) | NonEmpty | NoLeak | Len | Preview |
|------|--------|---------|--------|---------|----------|--------|-----|---------|
EOF

  echo "$results_json" | jq -r '.[] | "| \(.mode) | \(.prompt) | \(.attempt) | \(.status) | \(.latency_ms) | \(.response_nonempty) | \(.no_reasoning_leak) | \(.content_length) | \(.content_preview // "") |"' >> "$report_base.md"

  cat >> "$report_base.md" <<EOF

## Conclusion

$(if [ "$pass" -ge "$(( total * 2 / 3 ))" ]; then echo "Managed thinking infrastructure is working: responses are non-empty and no explicit reasoning markers (\\<thinking\\>, \\<reasoning\\>, fenced blocks) leak to the client."; else echo "Mixed results — some responses are empty or contain explicit reasoning markers. Tune the profile or increase max_tokens."; fi)

**Important:** This benchmark only detects explicit reasoning/thinking markers. Local models may still emit reasoning prose inside the main content. Novexa strips reasoning when the provider returns it in a separate field (reasoning_content) or when the model wraps it in explicit markers. Models that emit reasoning as plain prose inside content are not fully stripped yet.

Raw JSON: \`$base.json\`
EOF

  echo "Report written to $report_base.md"
}

main() {
  preflight_checks
  run_benchmark "A-LMStudioDirect-ThinkingOn" "$LMSTUDIO_URL" "true"
  run_benchmark "B-Novexa-ThinkingOn" "$NOVEXA_URL" "true"
  run_benchmark "C-Novexa-ThinkingOff" "$NOVEXA_URL" "false"
  write_report
}

main "$@"
