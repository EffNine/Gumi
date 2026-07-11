#!/usr/bin/env bash
set -euo pipefail

# Novexa Profile Doctor
# Usage: ./scripts/profile-doctor.sh <benchmark-report.json> [profile.yaml]
#
# Reads a benchmark JSON sidecar and recommends profile tuning changes.
# This script is intentionally read-only: it never edits profile YAML files.

REPORT_JSON="${1:-}"
PROFILE_PATH="${2:-}"

if [ -z "$REPORT_JSON" ]; then
  echo "Usage: $0 <benchmark-report.json> [profile.yaml]" >&2
  exit 1
fi

if ! command -v jq > /dev/null 2>&1; then
  echo "Error: jq is required to run Profile Doctor." >&2
  exit 1
fi

if [ ! -f "$REPORT_JSON" ]; then
  echo "Error: benchmark report not found: $REPORT_JSON" >&2
  exit 1
fi

if ! jq -e '.schema_version == 1 and (.results | type == "array")' "$REPORT_JSON" > /dev/null 2>&1; then
  echo "Error: unsupported benchmark JSON report. Expected schema_version 1 with results array." >&2
  exit 1
fi

model=$(jq -r '.model // "unknown"' "$REPORT_JSON")
generated_at=$(jq -r '.generated_at // "unknown"' "$REPORT_JSON")

novexa_total=$(jq '[.results[] | select((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-")))] | length' "$REPORT_JSON")
novexa_pass=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-"))) and .passed == "true")] | length' "$REPORT_JSON")
ollama_total=$(jq '[.results[] | select(.mode == "A-OllamaDirect")] | length' "$REPORT_JSON")
ollama_pass=$(jq '[.results[] | select(.mode == "A-OllamaDirect" and .passed == "true")] | length' "$REPORT_JSON")

exact_failures=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-"))) and (.prompt == "concise" or .prompt == "factual") and .passed != "true")] | length' "$REPORT_JSON")
json_failures=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-"))) and .prompt == "json" and (.json_valid != "true" or .json_keys != "true" or .no_fence != "true"))] | length' "$REPORT_JSON")
empty_failures=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-"))) and .note == "empty response")] | length' "$REPORT_JSON")
novexa_errors=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-"))) and .status != "200")] | length' "$REPORT_JSON")

ollama_p50=$(jq -r '.latency_by_mode["A-OllamaDirect"].p50 // empty' "$REPORT_JSON")
novexa_direct_p50=$(jq -r '.latency_by_mode["B-NovexaDirect"].p50 // empty' "$REPORT_JSON")
high_latency=false
if [[ "$ollama_p50" =~ ^[0-9]+$ ]] && [[ "$novexa_direct_p50" =~ ^[0-9]+$ ]] && [ "$novexa_direct_p50" -gt $((ollama_p50 * 2)) ]; then
  high_latency=true
fi

result="Good baseline"
if [ "$novexa_total" -eq 0 ]; then
  result="Insufficient data"
elif [ "$novexa_pass" -lt "$novexa_total" ] || [ "$novexa_pass" -lt "$ollama_pass" ] || [ "$exact_failures" -gt 0 ] || [ "$json_failures" -gt 0 ] || [ "$empty_failures" -gt 0 ] || [ "$novexa_errors" -gt 0 ] || [ "$high_latency" = "true" ]; then
  result="Needs tuning"
fi

echo "Novexa Profile Doctor"
echo ""
echo "Model: $model"
echo "Report: $REPORT_JSON"
echo "Generated: $generated_at"
if [ -n "$PROFILE_PATH" ]; then
  echo "Profile: $PROFILE_PATH"
fi
echo "Result: $result"
echo ""
echo "Findings:"

if [ "$novexa_total" -eq 0 ]; then
  echo "- No Novexa benchmark rows were found."
else
  echo "- Novexa passed $novexa_pass/$novexa_total checks."
fi

if [ "$ollama_total" -gt 0 ]; then
  echo "- Direct Ollama passed $ollama_pass/$ollama_total checks."
fi

if [ "$exact_failures" -eq 0 ]; then
  echo "- Exact instruction following passed across Novexa modes."
else
  echo "- Exact instruction following failed in $exact_failures Novexa row(s)."
fi

if [ "$json_failures" -eq 0 ]; then
  echo "- JSON output is valid across Novexa modes."
else
  echo "- JSON output failed validation in $json_failures Novexa row(s)."
fi

if [ "$empty_failures" -eq 0 ]; then
  echo "- No empty Novexa responses detected."
else
  echo "- Empty Novexa responses detected in $empty_failures row(s)."
fi

if [ "$novexa_errors" -eq 0 ]; then
  echo "- No Novexa HTTP/curl errors detected."
else
  echo "- Novexa HTTP/curl errors detected in $novexa_errors row(s)."
fi

if [ "$high_latency" = "true" ]; then
  echo "- Novexa direct p50 latency (${novexa_direct_p50}ms) is more than 2x direct Ollama p50 (${ollama_p50}ms)."
elif [[ "$ollama_p50" =~ ^[0-9]+$ ]] && [[ "$novexa_direct_p50" =~ ^[0-9]+$ ]]; then
  echo "- Latency overhead is acceptable: Ollama p50 ${ollama_p50}ms, Novexa direct p50 ${novexa_direct_p50}ms."
else
  echo "- Latency comparison unavailable."
fi

echo ""
echo "Recommended profile changes:"

recommendation_count=0
if [ "$exact_failures" -gt 0 ]; then
  echo "- Add exact-format instruction:"
  echo '  "For one-word or exact-format prompts, output only the requested final content."'
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$json_failures" -gt 0 ]; then
  echo "- Add stronger JSON instruction:"
  echo '  "When the user asks for JSON, return ONLY the raw JSON object. No markdown fences, no code blocks, no explanation before or after."'
  echo "- Prefer structured mode for JSON-sensitive application calls."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$empty_failures" -gt 0 ]; then
  echo "- Set defaults.thinking: false if this is a thinking-capable model."
  echo "- Reduce defaults.max_tokens to 512 for small local models."
  echo "- Check provider output for reasoning-only responses."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$novexa_pass" -lt "$ollama_pass" ] && [ "$ollama_total" -gt 0 ]; then
  echo "- Review profile prompt instructions because Novexa quality is below direct Ollama."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$high_latency" = "true" ]; then
  echo "- Reduce defaults.max_tokens and context.max_input_tokens."
  echo "- Keep profile prompt instructions shorter for this model."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$novexa_errors" -gt 0 ]; then
  echo "- Check provider availability and model aliases for this profile."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$recommendation_count" -eq 0 ]; then
  echo "- No required changes."
fi

if [ "$recommendation_count" -gt 0 ]; then
  echo ""
  echo "Suggested YAML patch snippet:"
  cat <<'YAMLEOF'
defaults:
  max_tokens: 512
  thinking: false

context:
  max_input_tokens: 4000

prompt:
  instructions:
    - For one-word or exact-format prompts, output only the requested final content.
    - Do not wrap normal plain-text answers in JSON unless JSON or structured output is explicitly requested.
    - When the user asks for JSON, return ONLY the raw JSON object. No markdown fences, no code blocks, no explanation before or after.
YAMLEOF
fi
