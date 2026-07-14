#!/usr/bin/env bash
set -euo pipefail

# Mega Prompt Benchmark v2 — Real-World Hard Scenarios
# Tests a model's ability to follow complex, realistic instructions.

BASE_URL="${BASE_URL:-http://127.0.0.1:8787/v1}"
API_KEY="${API_KEY:-gumi-local}"
MODEL="${MODEL:-lmstudio:qwen/qwen3.5-9b}"
OUTPUT_DIR="${OUTPUT_DIR:-$HOME/.gumi/benchmarks/mega-prompt}"

mkdir -p "$OUTPUT_DIR"

run_test() {
  local label="$1"
  local prompt="$2"
  local constraints="$3"
  local outfile="$OUTPUT_DIR/${label// /_}.json"

  echo ""
  echo "────────────────────────────────────────────────────────"
  echo " TEST: $label"
  echo "────────────────────────────────────────────────────────"

  RESPONSE=$(curl -s "$BASE_URL/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d "$(python3 -c "
import json
prompt_text = '''$prompt'''
payload = {
    'model': '$MODEL',
    'messages': [{'role': 'user', 'content': prompt_text}],
    'temperature': 0.3,
    'max_tokens': 2048
}
print(json.dumps(payload))
")")

  CONTENT=$(echo "$RESPONSE" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    if 'error' in data:
        print('ERROR:' + data['error'].get('message', ''))
        sys.exit(0)
    print(data['choices'][0]['message']['content'])
except Exception as e:
    print('PARSE_ERROR:' + str(e))
")

  CONTENT_LEN=${#CONTENT}
  echo "  Response: ${CONTENT_LEN}c"
  echo "$CONTENT" > "$outfile"
  echo ""

  # Validate via Python
  python3 -c "
import json, sys, re, os

content = open(os.devnull, 'r').read() if False else '''$CONTENT'''
content = '''$CONTENT'''
constraints_text = open('/dev/stdin', 'r').read() if False else '''$constraints'''
constraints = json.loads('''$constraints''')

passed = 0
total = len(constraints)

for r in constraints:
    key = r['key']
    label = r['label']
    expected = r.get('expected', True)

    if key == 'json_valid':
        ok = False
        try:
            json.loads(content)
            ok = True
        except:
            ok = False
    elif key == 'json_has_keys':
        try:
            parsed = json.loads(content)
            ok = all(k in parsed for k in expected)
        except:
            ok = False
    elif key == 'no_word':
        ok = expected.lower() not in content.lower()
    elif key == 'has_word':
        ok = expected.lower() in content.lower()
    elif key == 'min_chars':
        ok = len(content) >= int(expected)
    elif key == 'no_markdown':
        ok = '\`\`\`' not in content
    elif key == 'no_commas':
        ok = ',' not in content
    elif key == 'ends_with':
        ok = content.strip().endswith(expected)
    elif key == 'line_count':
        lines = [l for l in content.split(chr(10)) if l.strip()]
        ok = len(lines) == int(expected)
    elif key == 'min_lines':
        lines = [l for l in content.split(chr(10)) if l.strip()]
        ok = len(lines) >= int(expected)
    elif key == 'capital_start':
        lines = [l for l in content.split(chr(10)) if l.strip()]
        ok = all(l[0].isupper() for l in lines if l.strip())
    elif key == 'contains_all':
        ok = all(e.lower() in content.lower() for e in expected)
    elif key == 'not_contains':
        ok = all(e.lower() not in content.lower() for e in expected)
    elif key == 'regex':
        ok = bool(re.search(expected, content))
    elif key == 'min_words_per_line':
        lines = [l for l in content.split(chr(10)) if l.strip()]
        ok = all(len(l.split()) >= int(expected) for l in lines)
    elif key == 'sections':
        count = len(re.findall(r'(?:^|\\n)#+\s+\w+|(?:^|\\n)\d+\.\s+\w+', content))
        ok = count >= int(expected)
    else:
        ok = True

    mark = '\u2713' if ok else '\u2717'
    print(f'  {mark} {label}')
    if ok:
        passed += 1

print(f'')
print(f'  Score: {passed}/{total}  ({100 * passed // total}%)')
"
}

# ─────────────────────────────────────────────────────────
# SCENARIO 1: Code Review with Structured Output
# ─────────────────────────────────────────────────────────
PROMPT_1=$(cat << 'END'
I have a Python function with several bugs. Review it carefully, then return ONLY a JSON object with these exact keys:

1. "bugs": array of objects, each with "line" (int), "description" (string), "severity" ("critical"|"major"|"minor")
2. "fixed_code": the corrected function as a single string
3. "summary": one-sentence summary of the changes

The code:

def calculate_stats(numbers):
    result = {}
    result["sum"] = sum(numbers)
    result["mean"] = sum(numbers) / len(numbers)
    result["median"] = sorted(numbers)[len(numbers) // 2]
    result["variance"] = sum((x - result["mean"]) ** 2 for x in numbers) / len(numbers)
    result["std_dev"] = result["variance"] ** 0.5
    return result

def process_data(filepath):
    data = []
    with open(filepath) as f:
        for line in f.readlines():
            data.append(int(line.strip()))
    stats = calculate_stats(data)
    print("Sum: " + stats["sum"])
    print("Mean: " + str(stats["mean"]))
    return stats

Rules:
- Do NOT use markdown fences around the JSON
- Do NOT use the word "sample" anywhere
- The JSON must end with the key "complete": true
- At least 500 characters total
- No commas in the summary field
- Each of the 3 bug objects must have all 3 fields
END
)

CONSTRAINTS_1='[
  {"key":"json_valid","label":"valid JSON"},
  {"key":"json_has_keys","label":"has bugs, fixed_code, summary","expected":["bugs","fixed_code","summary"]},
  {"key":"no_word","label":"no sample","expected":"sample"},
  {"key":"has_word","label":"has complete key","expected":"complete"},
  {"key":"min_chars","label":"min 500 chars","expected":"500"},
  {"key":"no_markdown","label":"no markdown fences"},
  {"key":"min_lines","label":"at least 3 bug objects","expected":"3"}
]'

# ─────────────────────────────────────────────────────────
# SCENARIO 2: Data Pipeline Architecture
# ─────────────────────────────────────────────────────────
PROMPT_2=$(cat << 'END'
Design a data processing pipeline for ingesting 10 TB/day of JSON log files. Write exactly 6 lines. Use dash bullet points. Each line must start with a capital letter. Do not use the word "pipeline". End with the word "scale". At least 400 characters. Do not use any commas. No markdown formatting. Each line must have at least 10 words. Do not rhyme.
END
)

CONSTRAINTS_2='[
  {"key":"line_count","label":"exactly 6 lines","expected":"6"},
  {"key":"has_word","label":"dash bullets","expected":"-"},
  {"key":"capital_start","label":"capital start each line"},
  {"key":"no_word","label":"no pipeline","expected":"pipeline"},
  {"key":"ends_with","label":"ends with scale","expected":"scale"},
  {"key":"min_chars","label":"min 400 chars","expected":"400"},
  {"key":"no_commas","label":"no commas"},
  {"key":"no_markdown","label":"no markdown"},
  {"key":"min_words_per_line","label":"min 10 words per line","expected":"10"}
]'

# ─────────────────────────────────────────────────────────
# SCENARIO 3: API Documentation
# ─────────────────────────────────────────────────────────
PROMPT_3=$(cat << 'END'
Write API documentation for a /v1/users endpoint with these requirements:

- Exactly 4 sections: Overview, Authentication, Endpoints, Error Codes
- Highlight each section title with markdown ##
- At least 2 endpoints documented (GET /v1/users, POST /v1/users)
- Each endpoint must have: Description, Request body (if applicable), Response format
- At least 600 characters total
- Do not use the word "simple"
- End the entire document with the word "scalable"
- No commas
- Do not wrap in code fences (no ```)
- Each sentence must be on its own line
- Each line must start with a capital letter
- At least 30 lines total
- Do not rhyme
END
)

CONSTRAINTS_3='[
  {"key":"sections","label":"at least 4 ## sections","expected":"4"},
  {"key":"has_word","label":"has GET /v1/users","expected":"GET"},
  {"key":"has_word","label":"has POST /v1/users","expected":"POST"},
  {"key":"min_chars","label":"min 600 chars","expected":"600"},
  {"key":"no_word","label":"no simple","expected":"simple"},
  {"key":"ends_with","label":"ends with scalable","expected":"scalable"},
  {"key":"no_commas","label":"no commas"},
  {"key":"no_markdown","label":"no code fences"},
  {"key":"capital_start","label":"capital start each line"},
  {"key":"min_lines","label":"min 30 lines","expected":"30"}
]'

run_test "Code Review Audit" "$PROMPT_1" "$CONSTRAINTS_1"
run_test "Data Pipeline Design" "$PROMPT_2" "$CONSTRAINTS_2"
run_test "API Documentation" "$PROMPT_3" "$CONSTRAINTS_3"

cat <<EOF

────────────────────────────────────────────────────────
Results saved to: $OUTPUT_DIR/
Model: $MODEL
────────────────────────────────────────────────────────
EOF
