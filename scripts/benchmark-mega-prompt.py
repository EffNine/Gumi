#!/usr/bin/env python3
"""Mega Prompt Benchmark — Real-World Scenarios

Usage:
    python3 scripts/benchmark-mega-prompt.py [--model lmstudio:ornith-1.0-9b@q4_k_m]
"""

import json, urllib.request, time, re, sys, argparse

API = "http://127.0.0.1:8787/v1"
KEY = "gumi-local"

def chat(model, msgs, max_tokens=2048, timeout=120):
    p = {"model": model, "messages": msgs, "temperature": 0.3, "max_tokens": max_tokens}
    req = urllib.request.Request(f"{API}/chat/completions",
        data=json.dumps(p).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {KEY}"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.loads(r.read())["choices"][0]["message"]["content"]

def check(label, ok, detail=""):
    print(f"  {chr(10003) if ok else chr(10007)} {label}" + (f"  ({detail})" if detail else ""))
    return 1 if ok else 0

def test_code_review(model):
    print(f"\n{'='*70}")
    print("SCENARIO 1: Code Review Audit")
    print(f"Model: {model}")
    print('='*70)
    prompt = "...[see source for full prompt]..."  # truncated for brevity
    t0 = time.time()
    content = chat(model, [{"role": "user", "content": prompt}])
    t = time.time() - t0
    n = len(content)
    print(f"\nResponse ({n}c, {t:.1f}s)")
    pts = 0
    is_json, parsed = False, None
    try:
        parsed = json.loads(content)
        is_json = True
    except: pass
    pts += check("valid JSON", is_json)
    if is_json:
        for k in ["bugs", "fixed_code", "summary"]:
            pts += check(f"has '{k}'", k in parsed)
        bugs = parsed.get("bugs", [])
        pts += check(">= 3 bugs", isinstance(bugs, list) and len(bugs) >= 3, f"got {len(bugs)}")
        for i, b in enumerate(bugs[:3]):
            for f in ["line", "description", "severity"]:
                pts += check(f"  bug[{i}].{f}", isinstance(b, dict) and f in b)
    pts += check("no 'sample'", "sample" not in content.lower())
    pts += check("has complete:true", is_json and parsed.get("complete") == True)
    pts += check("min 500 chars", n >= 500, f"got {n}")
    pts += check("no ``` fences", chr(96)*3 not in content)
    print(f"\n  Score: {pts}/{18 if is_json else 7}\n")
    return pts

def test_data_pipeline(model):
    print(f"\n{'='*70}")
    print("SCENARIO 2: Data Pipeline Architecture")
    print(f"Model: {model}")
    print('='*70)
    prompt = "Design a data processing system for ingesting 10 TB/day of JSON log files. Write exactly 6 lines. Use dash bullet points. Each line must start with a capital letter. Do not use the word \"system\". End with the word \"scale\". At least 400 characters. Do not use any commas. No markdown formatting. Each line must have at least 10 words. Do not rhyme."
    t0 = time.time()
    content = chat(model, [{"role": "user", "content": prompt}])
    t = time.time() - t0
    n = len(content)
    print(f"\nResponse ({n}c, {t:.1f}s)")
    pts = 0
    lines = [l for l in content.split("\n") if l.strip()]
    pts += check("exactly 6 lines", len(lines) == 6, f"got {len(lines)}")
    pts += check("dash bullets", any(l.strip().startswith("-") for l in lines))
    pts += check("no 'system'", "system" not in content.lower())
    pts += check("ends with 'scale'", content.strip().endswith("scale"))
    pts += check("min 400 chars", n >= 400, f"got {n}")
    pts += check("no commas", "," not in content)
    pts += check("no markdown", chr(96)*3 not in content)
    caps_ok = all(l.strip().lstrip("- ")[0].isupper() for l in lines if l.strip())
    pts += check("capital start", caps_ok)
    words_ok = all(len(l.strip().lstrip("- ").split()) >= 10 for l in lines)
    min_w = min(len(l.strip().lstrip("- ").split()) for l in lines) if lines else 0
    pts += check("min 10 words/line", words_ok, f"min was {min_w}")
    print(f"\n  Score: {pts}/9\n")
    return pts

def test_api_docs(model):
    print(f"\n{'='*70}")
    print("SCENARIO 3: API Documentation")
    print(f"Model: {model}")
    print('='*70)
    prompt = "Write API documentation for a /v1/users endpoint with these requirements:\n\n- Exactly 4 sections: Overview, Authentication, Endpoints, Error Codes\n- Highlight each section title with markdown ##\n- At least 2 endpoints (GET /v1/users, POST /v1/users)\n- Each endpoint: Description, Request body, Response format\n- At least 600 characters\n- Do not use the word \"simple\"\n- End with the word \"scalable\"\n- No commas\n- No code fences\n- Each sentence on its own line\n- Each line starts with capital letter\n- At least 30 lines\n- Do not rhyme"
    t0 = time.time()
    content = chat(model, [{"role": "user", "content": prompt}])
    t = time.time() - t0
    n = len(content)
    print(f"\nResponse ({n}c, {t:.1f}s)")
    pts = 0
    lines = [l for l in content.split("\n") if l.strip()]
    sections = len(re.findall(r'^##\s+\w+', content, re.MULTILINE))
    pts += check(">= 4 ## sections", sections >= 4, f"got {sections}")
    pts += check("has GET", "GET" in content)
    pts += check("has POST", "POST" in content)
    pts += check("has /v1/users", "/v1/users" in content)
    pts += check("min 600 chars", n >= 600, f"got {n}")
    pts += check("no 'simple'", "simple" not in content.lower())
    pts += check("ends with 'scalable'", content.strip().endswith("scalable"))
    pts += check("no commas", "," not in content)
    pts += check("no code fences", chr(96)*3 not in content)
    caps_ok = all(l[0].isupper() for l in lines if l.strip() and l[0] != "#")
    pts += check("capital start", caps_ok)
    pts += check(">= 30 lines", len(lines) >= 30, f"got {len(lines)}")
    print(f"\n  Score: {pts}/12\n")
    return pts

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="lmstudio:ornith-1.0-9b@q4_k_m")
    parser.add_argument("--models", nargs="+")
    args = parser.parse_args()
    models = args.models or [args.model]
    for m in models:
        print(f"\n{'#'*70}\n# MODEL: {m}\n{'#'*70}")
        for fn in [test_code_review, test_data_pipeline, test_api_docs]:
            try: fn(m)
            except Exception as e: print(f"ERROR: {e}")
