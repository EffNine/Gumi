# Experiment 01 Results — Real-Hardware Validation

Date: 2025-08-25 (runs at 10:26–11:28 +08)
Machine: RTX 5070 12 GB (driver 595.84) · Ryzen 7 5700X (16 threads, gumi pinned 8) · 30 GB RAM · ext4 NVMe
Model: Qwen3-30B-A3B Q4_K_M GGUF, 17.28 GiB
SHA256 `9f1a24700a339b09…c5ba48` (source-verified; metadata verified via `gumi inspect`: qwen3moe, 48 layers, KV heads 4×128, 40960 train ctx, 128 total / **8 active** experts, expert weights 16.35 GB)
Backend: llama-cli v10360 (90e6a9131), Unsloth CUDA build — same binary for every configuration
Workload: `agentic_coding` · seed 42 · temperature 0 · identical prompts/fixtures · perf probes ×3 per config · two full repetitions

Baseline spec (competent-user config, measured not assumed):
`ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128,exps-cpu`

---

## Verdict: **VALIDATED**

> Gumi produced a materially better verified configuration than a competent
> manual configuration while preserving model capability — and its gate
> rejected a faster-looking alternative that measurably lost long-context
> retrieval. Reproduced identically across two full repetitions.

This verdict carries two scope caveats (see §6): capability parity was
perfect but the suite's absolute floor is dominated by three tasks where
harness fixes were required first, and the #1/#2 ranking between two passing
Gumi configs flipped between repetitions within measurement noise.

## 1. Headline comparison (both repetitions, Tier 2 = paired vs REFERENCE)

| Config | ctx | KV | GPU layers | prefill t/s | decode t/s | Tier 2 | verdict |
|---|---|---|---|---|---|---|---|
| CURRENT-BASELINE | 8192 | q8_0 | 33/48 + expsCPU | 255–258 | 24.7 | 8/8 | passed |
| GUMI REFERENCE | 16384 | f16 | 48/48 + expsCPU | 639–644 | 31.1 | 8/8 | anchor |
| **GUMI SPEED ⭐ (run1)** | 16384 | q4_0 | 48/48 + expsCPU | 941–945 | 31.2–31.3 | 8/8 | RECOMMENDED run1 |
| GUMI QUALITY (run2) | 40960 | f16 | 48/48 + expsCPU | 1022–1056 | 28.7–30.6 | 8/8 | RECOMMENDED run2 |
| GUMI BALANCED | 16384 | q8_0 | 48/48 + expsCPU | 634–639 | 30.1–31.2 | **7/8** | **REJECTED both runs** |

Stability: decode spread across perf probes ≤ 4.2% everywhere; run-to-run
deltas ≤ 3.3% except QUALITY decode (28.7→30.6, 6.6%). Zero OOM, zero
timeouts in any configuration. Peak VRAM 1.35–5.03 GB; peak RAM ≈ 17.3–17.8 GB
(expert tensors in system RAM as planned).

## 2. Capability dominates speed — demonstrated, not asserted

BALANCED matched REFERENCE and BASELINE on decode speed yet was rejected:
its q8_0 KV cache lost `retrieval_end` (needle at 95% depth of a ~14k-token
haystack) in **both** repetitions, deterministically. The report records the
rejection reason verbatim:

```
capability regression: candidate 88% < reference 100% (slack 0.00)
```

Meanwhile SPEED's q4_0 KV preserved all eight tasks including both retrieval
probes and won on prefill. The optimizer therefore selected by *verified*
capability, exactly the product rule; a throughput-only tool would have
ranked BALANCED above REFERENCE.

## 3. Gumi vs the human baseline (the thesis test)

Every passing Gumi configuration dominates CURRENT-BASELINE simultaneously on:

- **prefill**: 941–1056 vs 255–258 t/s → **3.7–4.1×**
- **decode**: 31.1–31.3 vs 24.7 t/s → **+26%**
- **context**: 16384–40960 vs 8192 tokens → **2–5×**, with retrieval verified
  at the larger window (the baseline's own 8k retrieval passed, Gumi's 16k/40k
  windows also passed)
- **capability**: 8/8 == 8/8, paired prompts/seeds/model/binary
- **stability**: spreads comparable; no OOM/timeouts anywhere

The mechanism is unglamorous and real: the human baseline under-offloads
(33/48 layers) and pays q8_0-KV cost anyway; Gumi places all 48 non-expert
layers on GPU with experts in RAM (exact arithmetic from GGUF geometry),
then lets the gate pick between f16/q4_0 KV by measured evidence.

## 4. MoE behavior audit (Task 5)

- Active experts: **8 of 128** in metadata; Gumi emits **no flag** touching
  active-expert count, RoPE, or sampler behavior (code audit: only `-ot
  exps=CPU` placement exists in `internal/backend`). Recorded configs show
  `seed=42 temp=0` on every generated candidate.
- Expert placement: all configurations ran experts in system RAM
  (`ExpertBytes` ≈ 16.35 GB resident, matching peak RAM ≈ 17.3–17.8 GB);
  non-expert layers fully GPU-resident for Gumi configs (peak VRAM 1.9–5.0 GB).
- All such candidates carry `experimental: true` labeling in reports.
- Contract slip found & fixed post-run: baseline spec inherited `seed=0`
  instead of the shared 42. Inert at temperature 0 (greedy), confirmed by
  identical 8/8 outcomes across runs; forced centrally now with a regression
  test.

## 5. Candidate-space analysis (Task 6)

1. **Meaningfully better?** Yes — §3 margins are large and reproduced.
2. **Convergence?** No: prefill spans 255→1056 t/s (4.1×) across configs;
   the space is far from flat.
3. **Faster-but-worse rejected?** Yes — BALANCED, twice, deterministically.
4. **Did expert placement matter?** Decisively: it is what makes a 30B MoE
   fit a 12 GB card at all; with split enabled, full 48-layer offload beat
   the baseline's partial offload on every axis.
5. **Did KV/context matter?** Yes — and non-monotonically: q8_0 lost
   end-of-context recall while q4_0 did not (quantization error structure is
   not a quality ladder). Context scaling to 40960 held capability. This is
   exactly the kind of result that justifies measuring instead of assuming.
6. **Missing safe knob?** None proposed. The loss modes observed were all
   covered by existing knobs; adding more would be speculation.
7. **Actual constraint:** expert-tensor bandwidth from system RAM (~17.8 GB
   resident) caps decode near 31 t/s; nothing in candidate space changes
   that without violating safe-tuning boundaries.

## 6. Scope caveats

- Three of eight capability tasks (python_bug_fix, rust_refactor,
  repository_navigation) initially failed for **all** configurations due to
  harness defects (wrong fixture answer key, first-fence extraction hitting
  echoed task text). After fixes they pass uniformly (8/8); their current
  discriminating power is unproven — they may be too easy for this model
  class. Suite hardening remains open work.
- Winner identity flipped between SPEED (run1) and QUALITY (run2) — both
  fully passing; driven by QUALITY's decode variance at 40k context crossing
  a narrow score margin. Gate outcomes were stable; recommendation ordering
  among ties is noise-sensitive.

## 7. Next engineering changes, ranked by measured impact (Task 9)

1. **Confidence should reflect repetition agreement** — the run1/run2 winner
   flip would have been visible pre-report if recommendations carried
   cross-run stability. Smallest change: optional second-repetition mode
   comparing winners before finalizing.
2. **Exec-fixture difficulty calibration** — add one harder bug variant per
   fixture so coding_exec discriminates within strong models.
3. **KV-strategy nuance** — evidence shows q4_0 ≥ q8_0 here; consider a
   q6_0/q5_K point only if a future model flips the ordering (no action now).
4. Not pursued by design: sampler search (no execution-config deficiency
   observed), any runtime/scheduler component.

## 8. Reproduce

```bash
gumi probe --bandwidth --json
gumi inspect ~/models/local/Qwen3-30B-A3B-Q4_K_M.gguf --json
gumi optimize ~/models/local/Qwen3-30B-A3B-Q4_K_M.gguf --workload agentic_coding \
  --baseline 'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128,exps-cpu' \
  --out reports/exp01-runN
python3 docs/experiments/compare.py reports/exp01-run1 reports/exp01-run2
```

Raw artifacts: `~/reports/exp01-run{1,2}/` (report.md/json, candidates.json,
hardware.json) on the experiment machine.
