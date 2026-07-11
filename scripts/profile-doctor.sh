#!/usr/bin/env bash
set -euo pipefail

# Novexa Profile Doctor
# Usage: ./scripts/profile-doctor.sh <benchmark-report.json> [profile.yaml]
#
# Reads a benchmark JSON sidecar and recommends profile tuning changes.
# This script is intentionally read-only: it never edits profile YAML files.
#
# Scoring:
#   - B-NovexaDirect is diagnostic/proxy mode; failures there do not mark
#     the profile as "Needs tuning" if D and E pass.
#   - C-NovexaLightweight is a quality gate with relaxed tolerance. If it
#     fails but D/E pass, result is "Good baseline with lightweight caveat".
#   - D-NovexaStabilized and E-NovexaStructured are the main quality gates.
#     Failures in D or E result in "Needs tuning".

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
provider=$(jq -r '.provider // "unknown"' "$REPORT_JSON")
generated_at=$(jq -r '.generated_at // "unknown"' "$REPORT_JSON")
direct_mode=$(jq -r '.modes_tested[]? | select(startswith("A-"))' "$REPORT_JSON" | head -n 1)
if [ -z "$direct_mode" ]; then
  direct_mode=$(jq -r '.results[].mode | select(startswith("A-"))' "$REPORT_JSON" | head -n 1)
fi

# --- Direct provider stats ---
direct_total=$(jq '[.results[] | select(.mode | startswith("A-"))] | length' "$REPORT_JSON")
direct_pass=$(jq '[.results[] | select((.mode | startswith("A-")) and .passed == "true")] | length' "$REPORT_JSON")

# --- B-NovexaDirect (diagnostic only) ---
b_total=$(jq '[.results[] | select(.mode | startswith("B-"))] | length' "$REPORT_JSON")
b_pass=$(jq '[.results[] | select((.mode | startswith("B-")) and .passed == "true")] | length' "$REPORT_JSON")
b_exact_failures=$(jq '[.results[] | select((.mode | startswith("B-")) and (.prompt == "concise" or .prompt == "factual") and .passed != "true")] | length' "$REPORT_JSON")
b_json_failures=$(jq '[.results[] | select((.mode | startswith("B-")) and .prompt == "json" and (.json_valid != "true" or .json_keys != "true" or .no_fence != "true"))] | length' "$REPORT_JSON")
b_empty_failures=$(jq '[.results[] | select((.mode | startswith("B-")) and .note == "empty response")] | length' "$REPORT_JSON")
b_errors=$(jq '[.results[] | select((.mode | startswith("B-")) and .status != "200")] | length' "$REPORT_JSON")

# --- C-NovexaLightweight (quality gate with relaxed tolerance) ---
c_total=$(jq '[.results[] | select(.mode | startswith("C-"))] | length' "$REPORT_JSON")
c_pass=$(jq '[.results[] | select((.mode | startswith("C-")) and .passed == "true")] | length' "$REPORT_JSON")
c_exact_failures=$(jq '[.results[] | select((.mode | startswith("C-")) and (.prompt == "concise" or .prompt == "factual") and .passed != "true")] | length' "$REPORT_JSON")
c_json_failures=$(jq '[.results[] | select((.mode | startswith("C-")) and .prompt == "json" and (.json_valid != "true" or .json_keys != "true" or .no_fence != "true"))] | length' "$REPORT_JSON")
c_empty_failures=$(jq '[.results[] | select((.mode | startswith("C-")) and .note == "empty response")] | length' "$REPORT_JSON")
c_errors=$(jq '[.results[] | select((.mode | startswith("C-")) and .status != "200")] | length' "$REPORT_JSON")

# --- D-NovexaStabilized (main quality gate for normal prompts) ---
d_total=$(jq '[.results[] | select(.mode | startswith("D-"))] | length' "$REPORT_JSON")
d_pass=$(jq '[.results[] | select((.mode | startswith("D-")) and .passed == "true")] | length' "$REPORT_JSON")
d_exact_failures=$(jq '[.results[] | select((.mode | startswith("D-")) and (.prompt == "concise" or .prompt == "factual") and .passed != "true")] | length' "$REPORT_JSON")
d_json_failures=$(jq '[.results[] | select((.mode | startswith("D-")) and .prompt == "json" and (.json_valid != "true" or .json_keys != "true" or .no_fence != "true"))] | length' "$REPORT_JSON")
d_empty_failures=$(jq '[.results[] | select((.mode | startswith("D-")) and .note == "empty response")] | length' "$REPORT_JSON")
d_errors=$(jq '[.results[] | select((.mode | startswith("D-")) and .status != "200")] | length' "$REPORT_JSON")

# --- E-NovexaStructured (main quality gate for JSON) ---
e_total=$(jq '[.results[] | select(.mode | startswith("E-"))] | length' "$REPORT_JSON")
e_pass=$(jq '[.results[] | select((.mode | startswith("E-")) and .passed == "true")] | length' "$REPORT_JSON")
e_json_failures=$(jq '[.results[] | select((.mode | startswith("E-")) and .prompt == "json" and (.json_valid != "true" or .json_keys != "true" or .no_fence != "true"))] | length' "$REPORT_JSON")
e_empty_failures=$(jq '[.results[] | select((.mode | startswith("E-")) and .note == "empty response")] | length' "$REPORT_JSON")
e_errors=$(jq '[.results[] | select((.mode | startswith("E-")) and .status != "200")] | length' "$REPORT_JSON")

# --- Aggregate Novexa stats (all modes) ---
novexa_total=$(jq '[.results[] | select((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-")) or (.mode | startswith("E-")))] | length' "$REPORT_JSON")
novexa_pass=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-")) or (.mode | startswith("E-"))) and .passed == "true")] | length' "$REPORT_JSON")
novexa_empty=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-")) or (.mode | startswith("E-"))) and .note == "empty response")] | length' "$REPORT_JSON")
novexa_errors=$(jq '[.results[] | select(((.mode | startswith("B-")) or (.mode | startswith("C-")) or (.mode | startswith("D-")) or (.mode | startswith("E-"))) and .status != "200")] | length' "$REPORT_JSON")

# --- Latency ---
direct_p50=$(jq -r --arg mode "$direct_mode" '.latency_by_mode[$mode].p50 // empty' "$REPORT_JSON")
novexa_direct_p50=$(jq -r '.latency_by_mode["B-NovexaDirect"].p50 // empty' "$REPORT_JSON")
high_latency=false
if [[ "$direct_p50" =~ ^[0-9]+$ ]] && [[ "$novexa_direct_p50" =~ ^[0-9]+$ ]] && [ "$novexa_direct_p50" -gt $((direct_p50 * 2)) ]; then
  high_latency=true
fi

# --- Determine result ---
# Main quality gates: D (stabilized) and E (structured) must pass.
# C (lightweight) is relaxed tolerance: if it fails but D/E pass,
# result is "Good baseline with lightweight caveat".
# B (direct) failures are diagnostic only and do not trigger "Needs tuning".
has_de_data=false
de_has_failures=false
c_has_failures=false

if [ "$d_total" -gt 0 ] || [ "$e_total" -gt 0 ]; then
  has_de_data=true
fi

if [ "$d_exact_failures" -gt 0 ] || [ "$d_json_failures" -gt 0 ] || [ "$d_empty_failures" -gt 0 ] || [ "$d_errors" -gt 0 ]; then
  de_has_failures=true
fi
if [ "$e_json_failures" -gt 0 ] || [ "$e_empty_failures" -gt 0 ] || [ "$e_errors" -gt 0 ]; then
  de_has_failures=true
fi

if [ "$c_exact_failures" -gt 0 ] || [ "$c_json_failures" -gt 0 ] || [ "$c_empty_failures" -gt 0 ] || [ "$c_errors" -gt 0 ]; then
  c_has_failures=true
fi

result="Good baseline"
lightweight_caveat=false
if [ "$novexa_total" -eq 0 ]; then
  result="Insufficient data"
elif [ "$high_latency" = "true" ]; then
  result="Needs tuning"
elif ! $has_de_data; then
  # Only B/C data exists — treat as diagnostic, not a profile quality signal
  result="Good baseline"
elif $de_has_failures || [ "$novexa_empty" -gt 0 ] || [ "$novexa_errors" -gt 0 ]; then
  result="Needs tuning"
elif $c_has_failures; then
  result="Good baseline with lightweight caveat"
  lightweight_caveat=true
fi

# --- Output ---
echo "Novexa Profile Doctor"
echo ""
echo "Model: $model"
echo "Provider: $provider"
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

if [ "$direct_total" -gt 0 ]; then
  echo "- Direct provider passed $direct_pass/$direct_total checks."
fi

# B-NovexaDirect findings (diagnostic)
if [ "$b_total" -gt 0 ]; then
  if [ "$b_pass" -eq "$b_total" ]; then
    echo "- B-NovexaDirect (diagnostic): all $b_total passed."
  else
    echo "- B-NovexaDirect (diagnostic): $b_pass/$b_total passed."
    if [ "$b_exact_failures" -gt 0 ]; then
      echo "  - Exact instruction failures in direct mode: $b_exact_failures"
    fi
    if [ "$b_json_failures" -gt 0 ]; then
      echo "  - JSON failures in direct mode: $b_json_failures"
    fi
    if [ "$b_empty_failures" -gt 0 ]; then
      echo "  - Empty responses in direct mode: $b_empty_failures"
    fi
  fi
fi

# C-NovexaLightweight findings (quality gate with relaxed tolerance)
if [ "$c_total" -gt 0 ]; then
  if [ "$c_pass" -eq "$c_total" ]; then
    echo "- C-NovexaLightweight: all $c_total passed."
  else
    echo "- C-NovexaLightweight: $c_pass/$c_total passed."
    if [ "$c_exact_failures" -gt 0 ]; then
      echo "  - Exact instruction failures: $c_exact_failures"
    fi
    if [ "$c_json_failures" -gt 0 ]; then
      echo "  - JSON failures: $c_json_failures"
    fi
    if [ "$c_empty_failures" -gt 0 ]; then
      echo "  - Empty responses: $c_empty_failures"
    fi
  fi
fi

# D-NovexaStabilized findings (main quality gate for normal prompts)
if [ "$d_total" -gt 0 ]; then
  if [ "$d_pass" -eq "$d_total" ]; then
    echo "- D-NovexaStabilized: all $d_total passed."
  else
    echo "- D-NovexaStabilized: $d_pass/$d_total passed."
    if [ "$d_exact_failures" -gt 0 ]; then
      echo "  - Exact instruction failures: $d_exact_failures"
    fi
    if [ "$d_json_failures" -gt 0 ]; then
      echo "  - JSON failures: $d_json_failures"
    fi
    if [ "$d_empty_failures" -gt 0 ]; then
      echo "  - Empty responses: $d_empty_failures"
    fi
  fi
fi

# E-NovexaStructured findings (main quality gate for JSON)
if [ "$e_total" -gt 0 ]; then
  if [ "$e_pass" -eq "$e_total" ]; then
    echo "- E-NovexaStructured: all $e_total passed."
  else
    echo "- E-NovexaStructured: $e_pass/$e_total passed."
    if [ "$e_json_failures" -gt 0 ]; then
      echo "  - JSON failures: $e_json_failures"
    fi
    if [ "$e_empty_failures" -gt 0 ]; then
      echo "  - Empty responses: $e_empty_failures"
    fi
  fi
fi

if [ "$novexa_empty" -gt 0 ]; then
  echo "- Empty Novexa responses detected in $novexa_empty row(s)."
fi

if [ "$novexa_errors" -gt 0 ]; then
  echo "- Novexa HTTP/curl errors detected in $novexa_errors row(s)."
fi

if [ "$high_latency" = "true" ]; then
  echo "- Novexa direct p50 latency (${novexa_direct_p50}ms) is more than 2x direct provider p50 (${direct_p50}ms)."
elif [[ "$direct_p50" =~ ^[0-9]+$ ]] && [[ "$novexa_direct_p50" =~ ^[0-9]+$ ]]; then
  echo "- Latency overhead is acceptable: direct provider p50 ${direct_p50}ms, Novexa direct p50 ${novexa_direct_p50}ms."
else
  echo "- Latency comparison unavailable."
fi

# --- Direct mode note ---
if [ "$b_total" -gt 0 ] && [ "$b_pass" -lt "$b_total" ] && [ "$d_pass" -eq "$d_total" ] && [ "$e_pass" -eq "$e_total" ]; then
  echo ""
  echo "Note: Direct mode has failures, but stabilized/structured modes pass. This is acceptable because direct mode is intentionally thin."
fi

if $lightweight_caveat; then
  echo ""
  echo "Note: Lightweight mode (C) has failures, but stabilized (D) and structured (E) pass. Lightweight prompt is intentionally minimal; failures are expected for models that need stronger instructions."
fi

# --- Recommendations ---
echo ""
echo "Recommended profile changes:"

recommendation_count=0

# Only recommend profile changes if D or E modes fail.
if [ "$d_exact_failures" -gt 0 ]; then
  echo "- Add exact-format instruction:"
  echo '  "For one-word or exact-format prompts, output only the requested final content."'
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$c_json_failures" -gt 0 ] || [ "$d_json_failures" -gt 0 ] || [ "$e_json_failures" -gt 0 ]; then
  echo "- Add stronger JSON instruction:"
  echo '  "When the user asks for JSON, return ONLY the raw JSON object. No markdown fences, no code blocks, no explanation before or after."'
  echo "- Prefer structured mode for JSON-sensitive application calls."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$c_empty_failures" -gt 0 ] || [ "$d_empty_failures" -gt 0 ] || [ "$e_empty_failures" -gt 0 ]; then
  echo "- Set defaults.thinking: false if this is a thinking-capable model."
  echo "- Reduce defaults.max_tokens to 512 for small local models."
  echo "- Check provider output for reasoning-only responses."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$d_pass" -lt "$direct_pass" ] && [ "$direct_total" -gt 0 ] && [ "$d_total" -gt 0 ]; then
  echo "- Review profile prompt instructions because Novexa stabilized quality is below direct provider."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$high_latency" = "true" ]; then
  echo "- Reduce defaults.max_tokens and context.max_input_tokens."
  echo "- Keep profile prompt instructions shorter for this model."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$c_errors" -gt 0 ] || [ "$d_errors" -gt 0 ] || [ "$e_errors" -gt 0 ]; then
  echo "- Check provider availability and model aliases for this profile."
  recommendation_count=$((recommendation_count + 1))
fi

# B-only recommendations (use stabilized/structured mode instead of tuning profile)
if [ "$b_exact_failures" -gt 0 ] && [ "$d_exact_failures" -eq 0 ]; then
  echo "- Direct mode exact-format failures detected. Use stabilized mode (D) for production workloads requiring exact output."
  recommendation_count=$((recommendation_count + 1))
fi
if [ "$b_json_failures" -gt 0 ] && [ "$d_json_failures" -eq 0 ] && [ "$e_json_failures" -eq 0 ]; then
  echo "- Direct mode JSON failures detected. Use stabilized mode (D) or structured mode (E) for JSON-sensitive calls."
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
