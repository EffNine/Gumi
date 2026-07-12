#!/usr/bin/env python3
"""Agentic Coding Benchmark — 3 fast tests: JSON, Go, Python. Novexa vs Direct."""
import json, urllib.request, time, re, os, sys

NOVEXA_API = "http://127.0.0.1:8787/v1"
NOVEXA_KEY = "novexa-local"
DIRECT_API = "http://192.168.0.164:1234/v1"
DIRECT_KEY = "local"
OUT = os.path.expanduser("~/.novexa/benchmarks/agentic-coding")
os.makedirs(OUT, exist_ok=True)

def call(api, key, model, msg, label, timeout=90):
    p = {"model": model, "messages": [{"role": "user", "content": msg}],
         "temperature": 0.3, "max_tokens": 1536}
    if "8787" not in api:
        p["reasoning_effort"] = "none"
    t0 = time.time()
    req = urllib.request.Request(
        f"{api}/chat/completions",
        data=json.dumps(p).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {key}"}
    )
    with urllib.request.urlopen(req, timeout=timeout) as r:
        c = json.loads(r.read())["choices"][0]["message"]["content"]
    safe = re.sub(r'[^a-z0-9]+', '_', label.lower())[:50]
    with open(f"{OUT}/{safe}.txt", "w") as f:
        f.write(c)
    return c, time.time() - t0

def run(name, prompt, check_fn, api, key, ms, route, timeout=90):
    m = f"lmstudio:{ms}" if "8787" in api else ms
    print(f"\n{'='*50}")
    print(f"{name} ({route})")
    print('='*50)
    try:
        c, t = call(api, key, m, prompt, name, timeout)
        print(f"({len(c)}c, {t:.1f}s)")
        checks = check_fn(c)
        pts = sum(1 for ok, _ in checks if ok)
        for ok, lbl in checks:
            print(f"  {'OK' if ok else 'XX'} {lbl}")
        print(f"  >> {pts}/{len(checks)}")
        return pts
    except Exception as e:
        print(f"  ERROR: {e}")
        return 0

# --- TEST 1: JSON Transform ---
def chk_json(c):
    p = None
    try:
        p = json.loads(c)
    except:
        pass
    r = []
    r.append((p is not None, "valid JSON"))
    if p:
        r.append(("departments" in p, "has departments"))
        s = p.get("summary", {})
        r.append((s.get("total_employees") == 5, "total_employees=5"))
        r.append((s.get("total_salary") == 540000, "total_salary=540000"))
        r.append((p.get("complete") is True, "complete:true"))
    r.append(("```" not in c, "no markdown"))
    return r

P1 = """Transform this flat data into nested JSON:

Input: [{"id":1,"name":"Alice","dept":"Engineering","salary":120000},{"id":2,"name":"Bob","dept":"Engineering","salary":95000},{"id":3,"name":"Carol","dept":"Marketing","salary":110000},{"id":4,"name":"Dave","dept":"Engineering","salary":130000},{"id":5,"name":"Eve","dept":"Marketing","salary":85000}]

Output format: {"departments":[{"name":"Engineering","employees":[{"id":4,"name":"Dave","salary":130000},...],"total_salary":345000,"headcount":3},...],"summary":{"total_employees":5,"total_salary":540000,"average_salary":108000}}

Rules: No markdown fences. End with key "complete": true. Math must be correct."""

# --- TEST 2: Go Cache Interface ---
def chk_go(c):
    cl = c.strip()
    if cl.startswith("```"):
        cl = cl.split("\n", 1)[1].rsplit("```", 1)[0].strip()
    try:
        p = json.loads(cl)
    except:
        p = None
    code = p.get("code", "") if p else ""
    r = []
    r.append((p is not None, "valid JSON"))
    if p:
        r.append(("type Cache interface" in code, "Cache interface"))
        r.append(("Get(" in code, "Get()"))
        r.append(("Set(" in code, "Set()"))
        r.append(("Delete(" in code, "Delete()"))
        r.append(("Clear(" in code, "Clear()"))
        r.append(("RWMutex" in code, "sync.RWMutex"))
        r.append(("InMemoryCache" in code, "InMemoryCache struct"))
        r.append(("RedisCache" in code, "RedisCache struct"))
        r.append(("func New" in code, "constructor func"))
        r.append(("func Test" in code, "unit test"))
        r.append((p.get("complete") is True, "complete:true"))
    r.append((len(c) >= 400, "min 400 chars"))
    r.append(("```" not in c, "no markdown"))
    return r

P2 = """Design a Go cache interface. Return JSON with keys: code, explanation, complete.

Requirements:
- Cache interface with Get, Set, Delete, Clear methods
- InMemoryCache implementation using sync.RWMutex
- RedisCache implementation (mock Redis calls)
- Constructor functions: NewInMemoryCache(), NewRedisCache()
- Unit tests for each implementation (func Test*)
- No markdown fences
- At least 400 characters"""

# --- TEST 3: Python Async Pipeline ---
def chk_py(c):
    cl = c.strip()
    if cl.startswith("```"):
        cl = cl.split("\n", 1)[1].rsplit("```", 1)[0].strip()
    try:
        p = json.loads(cl)
    except:
        p = None
    code = p.get("code", "") if p else ""
    r = []
    r.append((p is not None, "valid JSON"))
    if p:
        r.append(("pipeline_step" in code, "@pipeline_step decorator"))
        r.append(("AsyncPipeline" in code, "AsyncPipeline class"))
        r.append(("async def" in code, "async def"))
        r.append(("extract" in code.lower(), "extract step"))
        r.append(("transform" in code.lower(), "transform step"))
        r.append(("load" in code.lower(), "load step"))
        r.append(("try" in code and "except" in code, "try/except"))
        r.append((p.get("complete") is True, "complete:true"))
    r.append((len(c) >= 400, "min 400 chars"))
    r.append(("```" not in c, "no markdown"))
    return r

P3 = """Build an async data pipeline with decorators. Return JSON: {code, usage, complete}.

Requirements:
- @pipeline_step decorator (logs start/end timing)
- AsyncPipeline class with add_step(name, coro) and run(data) methods
- @retry(max_attempts=3, delay=1.0) decorator
- 3 async generator steps: extract, transform, load
- try/except error handling
- No markdown fences
- At least 400 characters"""

if __name__ == "__main__":
    models = ["ornith-1.0-9b@q4_k_m", "qwen/qwen3.5-9b"]
    tests = [("JSON Transform", P1, chk_json),
             ("Go Cache", P2, chk_go),
             ("Python Pipeline", P3, chk_py)]
    results = []

    for api, key, route in [(NOVEXA_API, NOVEXA_KEY, "Novexa"),
                              (DIRECT_API, DIRECT_KEY, "Direct")]:
        for ms in models:
            for name, prompt, chk in tests:
                s = run(name, prompt, chk, api, key, ms, route)
                results.append({"model": ms, "test": name,
                                "route": route, "score": s})

    print("\n" + "="*60)
    print("SUMMARY")
    print("="*60)
    print(f"{'Model':20s} {'Test':20s} {'Nov':>4s} {'Dir':>4s} {'Δ':>4s}")
    print("-"*55)
    for ms in models:
        for name, _, _ in tests:
            n = next((r["score"] for r in results
                      if r["model"]==ms and r["test"]==name
                      and r["route"]=="Novexa"), 0)
            d = next((r["score"] for r in results
                      if r["model"]==ms and r["test"]==name
                      and r["route"]=="Direct"), 0)
            print(f"{ms[:20]:20s} {name:20s} {n:4d} {d:4d} {'+' if n>=d else ''}{n-d:+4d}")
        print("-"*55)

    with open(f"{OUT}/summary.json", "w") as f:
        json.dump(results, f, indent=2)
    print(f"Saved to {OUT}/summary.json")
    print("Done!")
