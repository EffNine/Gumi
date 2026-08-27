#!/usr/bin/env python3
"""Render the Phase-3 three-way comparison from a gumi run's candidates.json.

Usage: compare.py <run_dir> [run_dir2 ...]
Prints per-config measurements, per-task outcomes, and stability evidence.
Read-only; no numbers are invented — everything comes from artifacts.
"""
import json, sys, glob, os

def load(run_dir):
    with open(os.path.join(run_dir, "candidates.json")) as f:
        return json.load(f)

def perf_stats(c):
    m = c.get("measured") or {}
    samples = m.get("perf_samples") or []
    dec = [s["decode_tps"] for s in samples if s.get("decode_tps")]
    pre = [s["prefill_tps"] for s in samples if s.get("prefill_tps")]
    def spread(v):
        if len(v) < 2 or max(v) <= 0:
            return None
        return (max(v) - min(v)) / max(v) * 100
    return {
        "prefill_mean": sum(pre)/len(pre) if pre else 0.0,
        "decode_mean": sum(dec)/len(dec) if dec else 0.0,
        "decode_spread_pct": spread(dec),
        "runs_ok": m.get("runs_ok", 0),
        "oom": m.get("oom_events", 0),
        "timeouts": m.get("timeouts", 0),
        "peak_vram_gb": round(m.get("peak_vram_bytes", 0)/2**30, 2),
        "peak_ram_gb": round(m.get("peak_ram_bytes", 0)/2**30, 2),
    }

def cap_groups(c):
    m = c.get("measured") or {}
    out = {}
    for suite in ("smoke", "capability"):
        s = m.get(suite)
        if not s:
            continue
        for o in s.get("outcomes", []):
            g = out.setdefault(o["category"], [0, 0])
            g[1] += 1
            if o["passed"]:
                g[0] += 1
    return out

def main():
    runs = sys.argv[1:]
    all_cands = [load(r) for r in runs]
    ids = [c["id"] for c in all_cands[0]]
    print("=" * 100)
    print(f"RUNS: {', '.join(os.path.basename(r) for r in runs)}")
    print("=" * 100)
    for i, cid in enumerate(ids):
        name = all_cands[0][i]["name"]
        print(f"\n### {name} ({cid})")
        for run_dir, cands in zip(runs, all_cands):
            c = next(x for x in cands if x["id"] == cid)
            p = perf_stats(c)
            conf = (c.get("confidence") or {}).get("level", "-")
            gate = "PASS" if c.get("gate_passed") else "FAIL"
            cfg = c["config"]
            print(f"  [{os.path.basename(run_dir)}] "
                  f"ctx={cfg['ContextTokens']} kv={cfg['KVCacheType']} ngl={cfg['GPULayers']} "
                  f"expsCPU={cfg['ExpertsOnCPU']} b={cfg.get('BatchSize')}/{cfg.get('UBatchSize')}")
            print(f"    prefill={p['prefill_mean']:.1f} t/s  decode={p['decode_mean']:.1f} t/s  "
                  f"spread={p['decode_spread_pct'] if p['decode_spread_pct'] is None else round(p['decode_spread_pct'],1)}%  "
                  f"runsOK={p['runs_ok']}/3 oom={p['oom']} to={p['timeouts']} "
                  f"vram={p['peak_vram_gb']}GB ram={p['peak_ram_gb']}GB")
            print(f"    gate={gate} confidence={conf}")
            if c.get("gate_reason"):
                print(f"    reason: {c['gate_reason'][:110]}")
            groups = cap_groups(c)
            if groups:
                gs = "  ".join(f"{k}:{v[0]}/{v[1]}" for k, v in sorted(groups.items()))
                print(f"    tasks: {gs}")
            # failed task detail (first repetition only)
            if run_dir == runs[0]:
                m = c.get("measured") or {}
                for suite in ("smoke", "capability"):
                    s = m.get(suite)
                    if not s:
                        continue
                    for o in s.get("outcomes", []):
                        if not o["passed"]:
                            print(f"      FAIL[{suite}] {o['task_id']}: {(o.get('error') or '')[:90]}")
    print()

if __name__ == "__main__":
    main()
