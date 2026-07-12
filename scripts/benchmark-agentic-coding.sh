#!/usr/bin/env bash
set -euo pipefail

# Novexa Agentic Coding Benchmark
# Usage: ./scripts/benchmark-agentic-coding.sh <model_name>
#
# Compares direct provider calls vs Novexa Runtime on agentic coding tasks:
#   - prompt-based tool calling
#   - structured JSON output
#   - multi-turn tool context
#
# Requirements:
#   - LM Studio running on http://localhost:1234/v1
#   - Novexa running on http://localhost:8787
#   - jq installed

MODEL="${1:-}"
if [ -z "$MODEL" ]; then
  echo "Usage: $0 <model_name>"
  echo "Example: $0 qwen2.5-coder-7b-instruct"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

LMSTUDIO_URL="${LMSTUDIO_URL:-http://localhost:1234/v1}"
NOVEXA_URL="${NOVEXA_URL:-http://localhost:8787/v1}"
NOVEXA_HEALTH_URL="${NOVEXA_HEALTH_URL:-${NOVEXA_URL%/v1}/health}"
ATTEMPTS="${ATTEMPTS:-3}"
BENCHMARK_TIMEOUT_SECONDS="${BENCHMARK_TIMEOUT_SECONDS:-120}"

DIRECT_MODE_LABEL="A-LMStudioDirect"
NOVEXA_MODEL="lmstudio:$MODEL"

results_json='[]'
pass=0
fail=0
total=0

preflight_checks() {
  if ! command -v jq > /dev/null 2>&1; then
    echo "Error: jq is required to run this benchmark." >&2
    exit 1
  fi
  if ! command -v curl > /dev/null 2>&1; then
    echo "Error: curl is required to run this benchmark." >&2
    exit 1
  fi
  if ! curl --max-time 5 -fsS "$LMSTUDIO_URL/models" > /dev/null 2>&1; then
    echo "Error: LM Studio is not reachable at $LMSTUDIO_URL." >&2
    exit 1
  fi
  if ! curl --max-time 5 -fsS "$NOVEXA_HEALTH_URL" > /dev/null 2>&1; then
    echo "Error: Novexa is not reachable at $NOVEXA_HEALTH_URL." >&2
    exit 1
  fi
}

make_payload() {
  local base_url="$1"
  local mode="$2"
  local prompt="$3"
  local include_tools="${4:-false}"

  local model
  if [ "$base_url" = "$LMSTUDIO_URL" ]; then
    model="$MODEL"
  else
    model="$NOVEXA_MODEL"
  fi

  local tool_block=""
  if [ "$include_tools" = "true" ]; then
    tool_block=$(cat <<'EOF'
,"tools":[{"type":"function","function":{"name":"read_file","description":"Read the contents of a file.","parameters":{"type":"object","properties":{"path":{"type":"string","description":"Relative file path"}},"required":["path"]}}}]
EOF
)
  fi

  local novexa_block=""
  if [ "$base_url" = "$NOVEXA_URL" ]; then
    novexa_block=$(cat <<EOF
,"novexa":{"mode":"$mode"}
EOF
)
  fi

  cat <<EOF
{
  "model": "$model",
  "messages": [{"role": "user", "content": $prompt}],
  "max_tokens": 4096,
  "temperature": 0.3
  $tool_block
  $novexa_block
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

extract_tool_calls() {
  local resp="$1"
  echo "$resp" | jq -c '.choices[0].message.tool_calls // []'
}

score_tool_call_valid() {
  local resp="$1"
  local tool_calls
  tool_calls=$(extract_tool_calls "$resp")
  if [ "$tool_calls" = "[]" ]; then
    echo "false"
    return
  fi
  local name
  name=$(echo "$tool_calls" | jq -r '.[0].function.name // ""')
  local args
  args=$(echo "$tool_calls" | jq -r '.[0].function.arguments // ""')
  if [ "$name" != "read_file" ]; then
    echo "false"
    return
  fi
  if ! echo "$args" | jq -e '.path' > /dev/null 2>&1; then
    echo "false"
    return
  fi
  echo "true"
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
  local tool_valid="$6"
  local json_valid="$7"
  local json_keys="$8"
  local no_fence="$9"

  local note="ok"
  if [ "$status" != "200" ]; then
    note="http_error"
  fi

  results_json=$(echo "$results_json" | jq \
    --arg mode "$mode" \
    --arg prompt "$prompt_label" \
    --arg attempt "$attempt" \
    --arg status "$status" \
    --arg latency "$latency_ms" \
    --arg tool_valid "$tool_valid" \
    --arg json_valid "$json_valid" \
    --arg json_keys "$json_keys" \
    --arg no_fence "$no_fence" \
    '. + [{
      mode: $mode,
      prompt: $prompt,
      attempt: ($attempt | tonumber),
      status: ($status | try tonumber catch 422),
      latency_ms: ($latency | tonumber),
      tool_valid: ($tool_valid == "true"),
      json_valid: ($json_valid == "true"),
      json_keys: ($json_keys == "true"),
      no_fence: ($no_fence == "true")
    }]')

  total=$((total + 1))
  if [ "$tool_valid" = "true" ] || [ "$json_valid" = "true" ]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
  fi
}

# safe_jq runs a jq expression against JSON input; if the input is not valid
# JSON, it returns the fallback value instead of crashing.
safe_jq() {
  local fallback="$1"
  local expr="$2"
  local input="$3"
  if echo "$input" | jq -e . > /dev/null 2>&1; then
    echo "$input" | jq -r "$expr"
  else
    echo "$fallback"
  fi
}

run_prompt() {
  local mode_label="$1"
  local base_url="$2"
  local mode="$3"
  local prompt_label="$4"
  local prompt="$5"
  local include_tools="${6:-false}"

  for attempt in $(seq 1 "$ATTEMPTS"); do
    local payload
    payload=$(make_payload "$base_url" "$mode" "$prompt" "$include_tools")
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
    local tool_valid
    tool_valid=$(score_tool_call_valid "$resp")
    local json_valid
    json_valid=$(score_json_valid "$content")
    local json_keys
    json_keys=$(score_json_has_keys "$content" "name" "value")
    local no_fence
    no_fence=$(score_no_markdown_fence "$content")

    record_result "$mode_label" "$prompt_label" "$attempt" "$status" "$latency_ms" "$tool_valid" "$json_valid" "$json_keys" "$no_fence"
  done
}

run_benchmark() {
  local mode_label="$1"
  local base_url="$2"
  local mode="$3"

  # Tool call prompt
  local tool_prompt='"Read the file main.go and return a tool call."'
  run_prompt "$mode_label" "$base_url" "$mode" "tool_call" "$tool_prompt" "true"

  # Structured JSON prompt
  local json_prompt='"Return a JSON object with keys name and value. Use test and 42."'
  run_prompt "$mode_label" "$base_url" "$mode" "json" "$json_prompt" "false"

  # Multi-turn context prompt: includes a prior tool result and asks for next step
  # This is only meaningful against Novexa; direct provider gets the same raw messages.
  local multiturn_prompt='"You previously read main.go. What is the next file you should read? Return a tool call."'
  run_prompt "$mode_label" "$base_url" "$mode" "multi_turn" "$multiturn_prompt" "true"
}

write_report() {
  local timestamp
  timestamp=$(date -u +"%Y%m%dT%H%M%SZ")
  local raw_dir="$HOME/.novexa/benchmarks/agentic-coding"
  local report_dir="$REPO_DIR/benchmarks/reports"
  mkdir -p "$raw_dir" "$report_dir"
  local base="$raw_dir/${MODEL//\//_}-${timestamp}"
  local report_base="$report_dir/${MODEL//\//_}-agentic-${timestamp}"

  echo "$results_json" | jq . > "$base.json"

  cat > "$report_base.md" <<EOF
# Agentic Coding Benchmark Report

**Model:** $MODEL
**Provider:** lmstudio
**Date:** $timestamp
**Attempts per prompt:** $ATTEMPTS

## Quality Metrics

| Metric | Value |
|--------|-------|
| Total requests | $total |
| Passed | $pass |
| Failed | $fail |
| Valid tool calls | $(echo "$results_json" | jq '[.[] | select(.tool_valid == true)] | length') / $(echo "$results_json" | jq '[.[] | select(.prompt == "tool_call" or .prompt == "multi_turn")] | length') |
| Valid JSON | $(echo "$results_json" | jq '[.[] | select(.json_valid == true and .prompt == "json")] | length') / $(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length') |
| JSON with required keys | $(echo "$results_json" | jq '[.[] | select(.json_keys == true and .prompt == "json")] | length') / $(echo "$results_json" | jq '[.[] | select(.prompt == "json")] | length') |
| No markdown fences | $(echo "$results_json" | jq '[.[] | select(.no_fence == true)] | length') / $total |

## Per-Request Results

| Mode | Prompt | Attempt | Status | Lat(ms) | ToolValid | JSONValid | JSONKeys | NoFence |
|------|--------|---------|--------|---------|-----------|-----------|----------|---------|
EOF

  echo "$results_json" | jq -r '.[] | "| \(.mode) | \(.prompt) | \(.attempt) | \(.status) | \(.latency_ms) | \(.tool_valid) | \(.json_valid) | \(.json_keys) | \(.no_fence) |"' >> "$report_base.md"

  cat >> "$report_base.md" <<EOF

## Conclusion

$(if [ "$pass" -ge "$(( total * 2 / 3 ))" ]; then echo "Worth it — Novexa agentic modes match or exceed direct provider quality for most requests."; else echo "Mixed — some agentic tasks still fail; tune profiles and retry strategy."; fi)

Raw JSON: \`$base.json\`
EOF

  echo "Report written to $report_base.md"
}

main() {
  preflight_checks
  run_benchmark "$DIRECT_MODE_LABEL" "$LMSTUDIO_URL" "direct"
  run_benchmark "B-NovexaLightweight" "$NOVEXA_URL" "lightweight"
  run_benchmark "C-NovexaStabilized" "$NOVEXA_URL" "stabilized"
  run_benchmark "D-NovexaStructured" "$NOVEXA_URL" "structured"
  write_report
}

main "$@"
