# Experiment 05 — Evidence → Deterministic Heuristics

Phase: 7
Date: 2026-08-26
Question: *Can Gumi use what it has learned from measurement to make better
candidate decisions on a model/hardware combination that was not manually
tuned?*

Environment (same controlled rig as Exp 04):
- GPU: RTX 5070 12 GB · CPU Ryzen 7 5700X (gumi pins 8 threads) · ~30 GB RAM
- Backend: llama-cli v10360 · temperature 0 · seed 42 · identical prompts/seeds
- Blind target: **Meta-Llama-3.1-8B-Instruct Q4_K_M** (llama family, dense,
  32 layers, KV geometry 128 KiB/token @f16, train ctx 131072)
- Human baseline admitted via `--baseline 'ngl=99,c=8192,kv=q8_0,fa,b=512,ub=128'`
  in both comparison runs (identical treatment through measure → gate → rank)

Boundary kept: no ML, no online learning, no external "best settings", no
sampler search, no automatic quantization switching, no new runtime surfaces.
The heuristic layer is one pure Go package (`internal/policy`, ~450 lines with
tests) evaluated fresh per run.

---

## 1. Existing evidence carried in

From Exp 04 (Qwen3-30B-A3B sweep, one-variable-at-a-time):

| knob | measured classification |
|---|---|
| Flash attention | HIGH IMPACT — OFF strictly dominated when supported |
| KV precision | HIGH IMPACT + CAPABILITY RISK — q8_0 lost late-window recall AND was slower than f16 (dominated on that stack); q4_0 ≈ parity |
| Batch/ubatch | HIGH for prefill-bound workloads (×3.6 prefill spread), noise-level decode |
| Offload | MEDIUM (−14% decode at 33/48); interior explored ⇒ strictly worse |
| Context | MEDIUM + capability-enabling; VRAM ≈ linear; depth recall improved at largest window |
| Expert placement | feasibility mechanism on 12 GB, not an interior tuning axis |

And workload analysis (Exp 04 §3): `agentic_coding` is prefill-and-depth
bound; `chat` is decode-bound.

These are **observations on one stack**, not laws. Phase 7's job was to
convert them into priors that allocate attention — while the paired
capability gate stays the only authority on what is safe.

## 2. Fact / measurement / heuristic separation (Task 1)

Four knowledge categories are now separated in code, not just prose:

| category | lives in | examples |
|---|---|---|
| A Hardware facts | probed (`internal/hardware`), never fabricated | VRAM total/free, RAM available, threads, bandwidth |
| B Model facts | parsed GGUF geometry (`internal/gguf`) | layers, kv heads × head_dim, train ctx, MoE expert share, weight bytes |
| C Measured facts | prior controlled experiments + this run's probes | Exp 04 table above; perf samples recorded per candidate |
| D Derived heuristics | `internal/policy` outputs | which axes deserve slots, in what priority |

Category C enters generation **only as cited motivation for heuristics**
(source label `heuristic` with an explicit reference such as
"docs/experiments/04 §2"), never as a rule that decides outcomes.
Deterministic arithmetic (KV bytes/token, feasibility budgets) remains in
`internal/candidate`; the policy layer receives *constraint outcomes* (e.g.
`FullOffloadFeasible`, `FitHeadroomFraction`) so memory formulas exist in
exactly one place and policy never re-derives them.

Every policy decision carries a machine-readable source:

```
hardware_fact | model_fact | deterministic_formula | workload_contract | heuristic
```

and every report now renders it (see §3 and Task 11 output below), so a
reader can tell whether a choice came from a fact, a formula, the workload
contract, or a heuristic prior.

## 3. The heuristic policy (Task 2)

`internal/policy.Evaluate(Input) *Plan` — a pure function. Same input, same
plan, forever (enforced by test). It emits six axis decisions, each with
impact ∈ {forced, high, medium, low, none}, a source label, and a reason;
then allocates at most four conditional candidate slots beyond REFERENCE,
recording an explicit reason for every declined slot.

The rules, in full:

```
IF flash attention:            enable on every candidate [heuristic]
                               (prior: OFF strictly dominated when supported;
                                quantized KV requires it; support probed at
                                verification — unsupported builds fail loudly)
IF depth-bound workload:       context growth = HIGH priority
ELSE:                          moderate growth via quality line only
IF f16 does not fit min ctx:   quantized KV = FORCED (capability-enabling) [formula]
ELIF fit headroom < 25%:       quantized KV rung = HIGH (OOM insurance)
ELSE:                          standard medium-priority ladder rung
IF prefill-bound workload:     dedicated large-batch contrast slot
IF decode-bound workload:      hold batches at baseline; spend NO slot
IF full offload infeasible:    partial offload = FORCED by arithmetic;
                               interior offload levels not explored
IF MoE ∧ split changes layer count (whitelisted family):
                               expert placement = FORCED enabling mechanism
ELSE IF dense:                 placement = none (model fact)
```

What the policy deliberately does **not** do:

- No precision is presumed good or bad. `high_context_q8` is framed in code
  and reports as *"a capability-risk candidate … kept only if paired
  verification holds"* (Task 4 language). BALANCED's rationale states any KV
  type is presumed risky until gated.
- Placement derives from memory geometry + the verified-family whitelist —
  never from a GPU name or VRAM number (Task 4).
- Heuristics may prioritize, eliminate impossible candidates, allocate slots,
  and choose axes. They cannot declare capability safe, bypass verification,
  or claim quality preservation (Task 3). Pipeline order is unchanged:
  FACTS → CONSTRAINTS → POLICY → MEASURE → GATE → RANK → REPORT.

Workload sensitivity is declared as three plain booleans on each profile
(`PrefillBound`, `DecodeBound`, `DepthBound`; Exp 04 §3 justification; no
DSL), surfaced by `gumi profiles` and enforced-divergent by test (Task 10).

Decision trace in reports (Task 11) — every report now contains:

```markdown
## WHY THESE CANDIDATES (generation policy)
- objective: maximize usable context and prefill throughput ...
- sensitivity: prefill-bound, depth-bound
- hard context floor: 16384 tokens (candidates never plan below it)

| Axis | Impact | Source | Decision |
| flash_attention | high | heuristic | enable ... — prior measurement ... |
| context | high | workload_contract | grow toward the largest feasible window ... |
| batch | high | workload_contract | dedicated large-batch contrast candidate |
| kv_memory | medium | heuristic | include quantized-KV rung ... |
| offload | none | deterministic_formula | full offload comfortable; no slot spent |
| expert_placement | none | model_fact | no expert tensors (dense model) |

Candidate slots used: context_growth → QUALITY ...
Candidate slots declined: aggressive_batch — hold batches at the baseline ...
_Heuristics decided what to test; every capability claim comes from paired
verification only._
```

## 4. Candidate budget policy (Task 5)

REFERENCE + up to 4 slots, admitted in priority order, each mapped to exactly
one candidate; converging configurations are dropped deterministically with a
recorded reason (never silently verified twice). Slots:

| slot | candidate | admitted when |
|---|---|---|
| `expert_split` | EXPERT-SPLIT | whitelisted MoE where split raises feasible layers (else absorbed into REFERENCE) |
| `context_growth` | QUALITY | a larger f16 window actually fits — otherwise declined instead of emitting a near-duplicate |
| `kv_memory_rung` | BALANCED | always (cheap hedge; halves KV without execution change) |
| `aggressive_batch` | SPEED | prefill-bound workload only |
| `high_context_q8` | HIGH-CONTEXT | depth-bound workload ∧ training ctx ≥ 2× floor |

Two hygiene changes came out of this budget review:

1. **BALANCED batches aligned to baseline (2048/512).** Previously BALANCED
   differed from REFERENCE in both KV *and* batch, conflating two variables;
   a gate rejection could not be attributed cleanly. Now REFERENCE→BALANCED
   isolates KV precision and BALANCED→SPEED isolates batch.
2. **Infeasible growth declines the slot** instead of verifying a duplicate:
   on DeepSeek-V2-Lite (dry-run) QUALITY used to collapse to
   REFERENCE+`--mlock`; it is now declined with reason *"context growth
   infeasible…"*.

## 5. Blind generalization experiment (Task 6)

Target: Llama-3.1-8B Instruct Q4_K_M — different family, dense, never used to
tune the generator. Disclosure: Exp 04 §5 cites one prior cross-check
datapoint on this artifact (balanced-q8 decode ≈ reference decode); no
generator logic was derived from it, but it is not fully untouched.

Procedure was exactly the standard pipeline — inspect → probe → generate via
the heuristic policy → run → capability-gate → rank → report — with no
manually supplied settings for this model:

```bash
gumi optimize Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf --workload agentic_coding \
  --baseline 'ngl=99,c=8192,kv=q8_0,fa,b=512,ub=128'
# plus a chat-workload run to validate the reduced chat plan end-to-end
```

Coverage inspection ("were the important axes actually covered?"):

- Flash attention ON everywhere ✓ (axis covered; support probed OK).
- KV contrast ✓ f16 vs q4_0 vs q8_0 all measured.
- Batch contrast ✓ (prefill-bound contract admits the slot).
- Context growth ✓ 16k → 32k measured.
- Offload / placement: correctly identified as **no-interior** from geometry
  alone (dense model, weights+KV comfortable) → zero slots wasted ✓.
- Axis magnitudes were NOT assumed: measured batch effect on this model was
  only ~+5% prefill (vs ×3.6 on Qwen3) — the policy covered the axis without
  encoding the magnitude, exactly as designed.

Raw artifacts: `~/reports/phase7/{old-gen,new-gen}-llama-agentic`,
`~/reports/phase7/new-gen-llama-chat` (+ dry-run plans under
`/tmp/opencode/phase7-baseline/` during the session).

## 6. Results (Task 6 continued)

Real-hardware, agentic_coding, heuristic generator (mean of 3 probes;
capability = Tier-2 battery):

| candidate | ctx | KV | b/u | dec t/s | prefill t/s | cap | verdict |
|---|---|---|---|---|---|---|---|
| REFERENCE | 16384 | f16 | 2048/512 | 81.6 | 4370 | 10/12 | VERIFIED |
| QUALITY | 32768 | f16 | 2048/512 | 82.5 | 4577 | 11/12 | VERIFIED |
| BALANCED | 16384 | q4_0 | 2048/512 | 67.5 ±13.9* | 4209 | 10/12 | VERIFIED |
| SPEED | 16384 | q4_0 | 4096/2048 | 76.7 | 4422 | 10/12 | VERIFIED |
| HIGH-CONTEXT | 32768 | q8_0 | 2048/512 | 77.5 | 4198 | **11/12** | **RECOMMENDED** |
| CURRENT-BASELINE | 8192 | q8_0 | 512/128 | **113.7** | 4106 | 10/12 | VERIFIED |

\* one transient outlier probe (49.2 t/s between 76.2 and 77.0). The same
q4 shape measured 78.4 ± 0.25 in the adjacent run; first/third probes here
are normal. This is a measurement event, not a config property — and the
stability-evidence machinery did exactly its job: BALANCED's half-range
flagged it and ranking refused to trust orderings around it.

Chat workload (validating the reduced plan end-to-end): 3 candidates only,
all VERIFIED, winner QUALITY (f16@16k, 85.2 t/s dec, ranking MEDIUM). The
two conserved slots cost nothing measurable.

Key findings:

1. **All six configurations passed the paired capability gate** on a family
   the generator was never tuned for. The q8_0 long-window probe was included
   as a capability-risk candidate and *earned* its recommendation through the
   gate (11/12, tied-best with f16@32k) rather than by rule.
2. Winner selection reproduced across generators and runs: HIGH-CONTEXT
   (77.1 vs 77.5 t/s across runs — within noise), ranking confidence MEDIUM.
3. The human baseline (c=8192) again passed the gate at **113.7 t/s — ~47%
   faster decode than the recommendation** with paired parity (10/12 both).
   Both generated ladders sit at ≥16k because the `agentic_coding` contract
   floors context at 16384. This is now a measured, twice-reproduced tension
   between the declared floor and real latency upside (see §9).

## 7. Existing vs heuristic generator comparison (Task 7)

Same model, hardware, workload, backend, prompts, seeds; baseline included in
both real runs; chat compared via plans + a real validation run of the new
plan.

| dimension | existing generator | heuristic generator |
|---|---|---|
| agentic candidate set | 5 (ref, quality, balanced b1024, speed b4096, highctx) | 5 — same axes; BALANCED now single-variable vs REFERENCE |
| agentic best verified (dec) | 77.1 t/s (HIGH-CONTEXT) | 77.5 t/s (HIGH-CONTEXT) — tie within noise |
| agentic capability pass set | all 6 VERIFIED | all 6 VERIFIED (identical) |
| wasted slots (agentic) | 0 | 0 (plus near-duplicate risk removed on tight shapes, §4.2) |
| wasted slots (chat, roomy dense) | **2 of 5**: SPEED spent on batch (measured noise-level for decode-bound); HIGHCTX q8@8k despite non-depth contract | **0 of 3**: both declined with recorded reasons; real run confirmed nothing lost |
| attribution hygiene | gate outcomes can conflate KV+batch changes | one variable per rung by construction |
| traceability | rationale prose only | sourced decision table + slot ledger in every report |
| ranking confidence | MEDIUM | MEDIUM (unchanged machinery) |

Honest reading: on the agentic blind target the heuristic generator matched
the existing one rather than beating it — same winner, same pass set. The
demonstrated improvements are structural (slot economy on chat/tight shapes,
single-variable contrasts, full decision traceability), plus one prevented
waste class (infeasible-growth duplicates). No capability or performance
regression was observed anywhere.

## 8. Failure analysis (Task 8)

No generalization failure occurred on the blind target, so **no
family-specific rule was added**. Two events were analyzed before being
dismissed as failures:

- BALANCED outlier probe → root cause: transient system event during probe 2
  of 3; not architecture-specific; handled by existing repetition/stability
  evidence (half-range flagging). Category: *measurement noise*, no action.
- DeepSeek-V2-Lite QUALITY near-duplicate → root cause: *insufficient
  candidate space discipline* (slot spent on infeasible growth), fixed
  generically by declining the slot — not a deepseek2 special case.

Pre-existing explicit rules remain explicit and documented: the MoE
placement whitelist (`internal/candidate/generate.go`) is family-specific,
evidence-backed, and fails conservative (unknown families keep default
placement; users can apply `-ot exps=CPU` manually from exports).

## 9. Calibration boundary & remaining uncertainties (Tasks 9 + open items)

Calibration boundary, stated explicitly (deepseek2 case unchanged):

- **Analytically knowable:** KV bytes/token from GGUF geometry (exact),
  weight residency splits, safe budgets. Planner estimates deepseek2 at
  ~324 KiB/token f16; llama.cpp b10360 allocates ~264 KiB/token (MLA-related,
  build-specific). Overestimate ⇒ conservative direction ⇒ safe.
- **Only knowable by measurement:** actual speed/capability effects of every
  knob (this is what verification is for).
- **Backend/build dependent:** flag syntax, KV allocation behavior, flash
  attention support. Unknown values stay UNKNOWN; conservative estimates are
  acceptable; fabricated precision is not. **No auto-calibration introduced**
  — Phase 7 produced no evidence that the bias ever bites in the unsafe
  direction.

Remaining uncertainties:

1. **Blind coverage is n=1 hardware target, dense-only.** The MoE placement
   path was exercised on hardware in earlier phases (Qwen3) and by dry-run
   here (DeepSeek-V2-Lite plans look correct: forced placement/KV decisions
   with sources), but no *new* MoE family was run blind on hardware.
2. **The workload-floor question.** Twice now, an admitted human baseline at
   c=8192 passed the full agentic gate while decoding ~47% faster than every
   generated candidate (which are floored at c≥16384 by the contract). Either
   the floor is over-conservative for some models, or the battery is not
   sensitive enough below it to catch regressions that matter. Resolving this
   requires a dedicated floor-justification experiment — explicitly *not*
   snuck into the heuristic layer, since MinContext is a hard constraint and
   heuristics must not override contracts.
3. Interior-knob shapes (batch curve, context slope) remain
   model/build-specific numbers until re-run per model; the policy cites them
   as motivation only.
4. Single-run chat speed comparison remains thermally confounded (carried
   over from Exp 04); interleaved A/B design still owed.

---

## Verdict

**HEURISTICS PARTIALLY GENERALIZED.**

On the one blind target tested end-to-end on real hardware (Llama-3.1-8B on
RTX 5070), the deterministic policy selected the right axes from geometry and
workload contracts alone, spent no slot on inapplicable knobs, produced the
same gate-passing set and statistically identical winner as the manually
evolved generator, and added traceability and slot economy without any
capability or performance regression. That justifies keeping the policy as
the candidate-selection mechanism.

It is not yet a claim of full generalization: one family/GPU pair is thin
evidence, the blind MoE path has not been run on hardware, and a real,
twice-measured gap (contract floor vs baseline latency) sits outside the
policy's authority by design. Those are the next experiments — not new rules.
