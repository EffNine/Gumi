# Experiment 04 — Optimization Sensitivity & Coverage Study

Phase: 6
Date: 2026-08-26
Environment (controlled, unchanged across all points):
- GPU: RTX 5070 12 GB (driver 595.84) · CPU: Ryzen 7 5700X (16 threads; gumi pins 8) · 30 GB RAM
- Model: Qwen3-30B-A3B Q4_K_M (17.28 GiB, sha256-verified)
- Backend: llama-cli v10360 · temperature 0 · seed 42 · identical prompts/fixtures
- Method: one-variable-at-a-time around the anchor configuration
  `ngl=48,c=16384,kv=f16,fa,b=2048/512,exps-cpu` (= Phase-3 REFERENCE shape).
  Per point: 3 perf probes (1536-tok prompt / 160-tok gen) +, where noted,
  the full agentic capability battery. Warm-sample medians reported
  (first probe after load is a consistent cold outlier: e.g. anchor decode
  samples 20.6 → 29.3 → 30.1).

Raw artifacts: `~/reports/phase6/*.json` (+ progress log).

---

## 1. Measured matrix

| point | ctx | KV | offload | FA | batch b/u | dec t/s | prefill t/s | VRAM GB | RAM GB | capability |
|---|---|---|---|---|---|---|---|---|---|---|
| anchor | 16384 | f16 | 48+expsCPU | on | 2048/512 | **29.7** | **619** | 2.75 | 17.70 | 11/12 |
| ctx8k | 8192 | f16 | 48+expsCPU | on | 2048/512 | 31.5 | 645 | 2.00 | 17.68 | 11/12 |
| ctx32k | 32768 | f16 | 48+expsCPU | on | 2048/512 | 29.2 | 624 | 4.24 | 17.75 | – |
| ctx40k | 40960 | f16 | 48+expsCPU | on | 2048/512 | 28.7 | 610 | 4.99 | 17.77 | **12/12** |
| kv_q8 | 16384 | q8_0 | 48+expsCPU | on | 2048/512 | 26.9 | 594 | 2.05 | 17.70 | **10/12** |
| kv_q4 | 16384 | q4_0 | 48+expsCPU | on | 2048/512 | 28.9 | 617 | 1.67 | 17.69 | 11/12 |
| fa_off | 16384 | f16 | 48+expsCPU | **off** | 2048/512 | 25.4 | 540 | 3.61 | 17.77 | 11/12 |
| batch_small | 16384 | f16 | 48+expsCPU | on | **512/128** | 31.4 | **271** | 2.71 | 17.70 | – |
| batch_large | 16384 | f16 | 48+expsCPU | on | **4096/2048** | 31.0 | **964** | 2.99 | 17.75 | – |
| offload33 | 16384 | f16 | **33**+expsCPU | on | 2048/512 | 25.4 | 626 | 2.12 | **18.17** | 11/12 |
| chat (same config, chat battery) | 16384 | f16 | 48+expsCPU | on | 2048/512 | (22.9*) | (564*) | 2.75 | 17.70 | 3/4 |

\* chat perf point is warmup-contaminated (samples 12.7→19.1→26.7, ran last in
chain): its *speed* numbers are not used for workload conclusions; its suite
outcomes are.

## 2. Knob classifications (measured)

| knob | performance effect | capability effect | classification |
|---|---|---|---|
| Flash attention | OFF: decode −15%, prefill −13%, VRAM +0.86 GB | none measured | **HIGH IMPACT** (strictly dominant ON for this stack) |
| KV precision | **q8_0 slower than f16** (−9%); q4_0 ≈ parity (−3%); VRAM −0.7/−1.08 GB vs f16 @16k | q8_0 loses `retrieval_end` (10/12, reproduced from Phase 3/5); q4_0 clean (11/12) | **HIGH IMPACT + CAPABILITY RISK** — and q8_0 is *dominated*: no speed upside |
| GPU/CPU offload | 33/48 layers: decode −14%, RAM +0.47 GB; prefill unaffected at probe size | none at tested depth (11/12 same tasks) | **MEDIUM IMPACT** |
| Batch/ubatch | prefill 271 ↔ 964 t/s (**×3.6**); decode within ±6% (noise-level) | not battery-tested (prefill knob) | **HIGH IMPACT for prefill-bound workloads / LOW for decode-bound** |
| Context length | decode −9% over 8k→40k; VRAM linear ≈ +91 MB per 1k tokens; prefill flat | depth-capability improves with window (see §4) | **MEDIUM IMPACT + capability-enabling** |
| Expert placement | feasibility gate on this GPU (full-GPU = OOM by arithmetic, 17.3 > 11.4 GiB budget); with expsCPU resident, RAM constant 17.7 GB regardless of layer split | n/a (execution-only) | **HARDWARE/BUILD DEPENDENT** — mandatory enabler here, not a tuning axis |

mmap/mlock were not swept: both sit outside the candidate set's variance in
prior runs and neither can plausibly move capability; left LOW/no-test by
design (no speculative knobs).

## 3. Workload-specific effects (Task 3)

Mechanistic evidence, cross-checked against Phase-3/5 batteries:

- **agentic_coding** is prefill-and-depth bound: prompts/tool outputs are
  large (battery retrieval prompts reach ~14–24k tokens), so (a) batch size
  moves prompt-processing throughput ×3.6, (b) late-window recall
  (`kv_probe_deep`, `retrieval_end`) is the binding constraint, (c) context
  headroom converts directly into capability (40k passed the deepest probe).
- **chat** is decode-bound: short prompts make batch nearly irrelevant to
  experience; responsiveness rides on decode stability. Chat battery
  composition (reasoning/instruction/json) exercises different checks than
  agentic — observed single-run flakiness of `retrieval_mid` at 16k (pass in
  agentic run, fail in chat run, same config) also shows why the gate uses
  paired same-run comparisons rather than absolute thresholds.

Conclusion: **workload-specific optimization is justified by measurement** —
the two profiles weight different physical resources, and their hard
constraints differ accordingly (now encoded in the profile contract).

## 4. Context growth findings (Task 4)

8k → 16k → 32k → 40k at f16:

- VRAM grows linearly (~91 MB/1k tok incl. activations), +150% total.
- Decode tax is real but gentle (−9% end-to-end); prefill flat (±3%).
- Capability does NOT degrade with growth — it *improves where it matters*:
  the scaled deep probe passes only at the largest window (12/12 @40k vs
  11/12 below). No cliff found up to 40k on this model/hardware.
- Practical reading: recommend the smallest window that satisfies the
  workload floor plus measured recall margin — larger windows cost linear
  memory for mild speed loss but buy late-context reliability.

## 5. KV capability boundary (Task 5)

At fixed 16384 ctx: **the boundary is not "more quantization = worse"** —
it's that q8_0 sits at a bad point on this stack:

- q8_0: loses `retrieval_end` (again; third independent reproduction across
  phases/models) AND is 9% slower than f16 → strictly dominated.
- q4_0: preserves all tested capability (only shared miss is
  `kv_probe_deep`, which f16@16k also misses) while saving 1.08 GB.
- Cross-model confirmation: Llama-3.1-8B gen-run showed balanced(q8_0)
  decode 78.3 vs reference(f16) 79.0 — again no speed advantage.

Gumi can therefore *locate* the boundary empirically per model+hardware:
that is exactly what the paired gate now encodes. No universal KV ranking is
claimed.

## 6. Expert placement (Task 6)

On this GPU the placement flag is binary-feasibility: without `-ot
exps=CPU`, weights (17.3 GiB) exceed the safe VRAM budget outright — there
is no full-GPU operating point to compare against. With experts in RAM,
RAM residency is ~17.7 GB regardless of how many non-expert layers are
GPU-resident, and the remaining lever is the ordinary offload level (−14%
decode at 33/48). Placement materially matters as an enabler; as a *tuning*
axis it has no measurable interior on this hardware. Recorded, not removed —
on larger-VRAM cards the same flag becomes a genuine interior tradeoff.

## 7. Candidate-generator implications (Task 7)

Measured gap found and closed:

- BALANCED previously carried **q8_0** — now measured as dominated on two
  families (slower than f16 AND capability-lossy). The slot wasted one of
  five candidates on a config the gate would (correctly) distrust or users
  shouldn't prefer. **Change: BALANCED now uses q4_0** at the workload
  context with moderate batches; SPEED remains the aggressive-q4 point;
  QUALITY/REFERENCE remain the f16 line. Coverage after change:
  {f16 min-ctx, f16 grown-ctx, q4 min-ctx, q4 aggressive-batch} + conditional
  fifth — every slot maps to a distinct measured tradeoff axis.
- fa_off is never generated (always-on) — validated correct by §2.
- No missing operating points evidenced; no other generator changes.

## 8. deepseek2 KV deviation restated (Task 8)

From Phase 5 (unchanged): planner estimates ~324 KB/token from GGUF
attention geometry; llama.cpp b10360 allocates ~264 KB/token for deepseek2
(no MLA flag exists in that build). Backend/build-specific, MLA-related,
conservative direction (planner overestimates need ⇒ safe). This study
produced no new deepseek2 datapoint (Qwen-only sweep by design); automatic
calibration remains out of scope.

## 9. Profile contract (Task 9)

Now code-defined on each profile (`workload.Profile.Objective`,
`HardConstraints`, `PreferredMetrics`; rendered by `gumi profiles`):

- **agentic_coding** — objective: maximize usable context and prefill
  throughput at preserved long-context capability. Hard constraints:
  late-window retrieval parity (`retrieval_end`, `kv_probe_deep`); flash
  attention enabled; full-battery gate parity vs REFERENCE. Preferred
  metrics: prefill tok/s, context headroom, late-context retrieval rate.
- **chat** — objective: maximize interactive decode responsiveness at
  preserved instruction quality. Hard constraints: smoke formatting pass;
  reasoning/instruction parity; multi-turn context floor. Preferred metrics:
  decode tok/s, TTFT proxy, instruction pass rate.

No DSL; plain Go fields, tests enforce presence and divergence of the two
contracts.

## 10. Remaining uncertainties

- Single-model sweep (by design). The q8-domination result is confirmed on
  two families via prior runs, but interior-knob shapes (batch curve, context
  slope) are Qwen3-specific numbers until re-run per model.
- Cold-start bias means first-probe telemetry is excluded from medians;
  Gumi's own mean-of-3 slightly under-reports throughput versus steady state
  (documented; not changed this phase).
- Chat decode comparison was thermally contaminated and discarded — a clean
  chat-vs-agentic *speed* contrast still needs an interleaved A/B design.
- `kv_probe_deep` outcome varies with window size in a non-monotonic way
  (fails @8k/16k, passes @40k for f16): the fixture measures depth-recall at
  its own scale, so cross-size comparisons of that single task are not
  meaningful — only same-size paired comparisons are.
- Expert-placement "interior" behavior on >24 GB cards is extrapolation, not
  measurement.
