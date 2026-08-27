# 27 — Gumi V1 Release Audit

**Status:** Final
**Auditor:** code review + static analysis + boundary-case verification + real-hardware validation cross-check
**Date:** 2026-08-26
**Audited commit:** `9e94fed` (pre-audit baseline)

---

## 1. Audit scope

Gumi V1 (local inference auto-tuner) was audited against the contract in `docs/specs/26-gumi-v1-auto-tuner.md`:

| Dimension | Scope boundary |
|---|---|
| Command | `gumi tune <model.gguf>` (alias `optimize`) |
| Accelerator | CUDA / NVIDIA, single GPU |
| Model format | GGUF (minimal parser, no catalog) |
| Backend | llama.cpp (`llama-cli` subprocess) |
| Exports | llama.cpp CLI/server, LM Studio, Ollama |

Explicitly out of scope and **not** implemented: ROCm, Metal, Vulkan, multi-GPU, multi-node, tensor parallelism, runtime scheduling, daemon, dashboard, web UI, sampler tuning, temperature optimization, online learning, automatic model quant selection, model downloading service.

Validation environments:

- **RTX 5070 12 GB**, CUDA 13.2, driver 595.84, llama.cpp v10360
- Validation A: Llama-3.1-8B-Instruct Q4_K_M, workload `chat` → MAX PRACTICAL = 65536
- Validation B: Qwen3-30B-A3B Q4_K_M, workload `agentic_coding` → MAX PRACTICAL = 40960

The question:

> Can Gumi ever produce a misleading recommendation, incorrect frontier, invalid profile, or silently wrong configuration?

---

## 2. Search correctness

### 2.1 Ladder generation

`internal/search/search.go:Ladder(start, maxCtx)` doublings of `start`, capped by `min(TrainContext, maxCtx)`, plus the training-context ceiling when it is meaningfully above the last doubling (≥1.25×). Boundary cases verified:

| `Ladder(start, maxCtx)` | Result | Reason |
|---|---|---|
| `(16384, 131072)` | `[32768, 65536, 131072]` | power-of-two growth plus ceiling |
| `(16384, 40960)` | `[32768, 40960]` | ceiling ≥ 1.25× last doubling, earns its own probe |
| `(4096, 9000)` | `[8192]` | ceiling below 1.25× → not appended |
| `(4096, 10240)` | `[8192, 10240]` | exactly 1.25× boundary |
| `(4096, 4096)` | `nil` | no growth room |
| `(1000, 1024)` | `nil` | `start*2 > maxCtx` → no probes |
| `(65536, 1048576)` | `[131072, 262144, 524288, 1048576]` | very large ceiling |
| `(8192, 4096)` | `nil` | start > max |

All levels ascend; all are multiples of `start` (not necessarily powers of two after cap). **No bug.**

### 2.2 Midpoint / bisection

`internal/search/search.go:Midpoint(lo, hi, granularity)`:

- Returns 0 when `hi - lo <= MinRefineGranularity` (1024 tokens). Verified at exact boundaries: `Midpoint(16384, 17408, 1024) == 0`.
- All returned midpoints are aligned to 1024-token multiples. Verified across small, exact, and large brackets.
- Bracket tightness converges within bounded steps; simulation with `lo=16384, hi=65536` reaches `hi-lo <= 2048` in ≤5 bisections with a realistic fail-boundary at 30K.
- Lower-bound guard `mid <= lo` clamps to `lo + granularity`; upper-bound guard returns 0 when clamped midpoint ≥ hi. No infinite loop, no regression.

### 2.3 1024-token granularity

Both `search.Midpoint` (hardcoded `MinRefineGranularity = 1024`) and `tuner.refineGranularity` (returns `MinRefineGranularityTokens = 2048`) are consistent: the finer 1024-aligned step is enforced inside `Midpoint`, the coarser 2048 budget governs the high-level loop. Boundary tests confirmed no off-by-1024 errors.

### 2.4 Dominance rules

`internal/search/search.go:DominatedBy` and `PruneDominated` enforce:

- **Stability gate:** unstable observations never dominate. Verified.
- **No-unmeasured-wins:** `CapRate < 0` (battery not run) is neutral, never wins. Verified.
- **Zero-variance = unknown noise:** equal ranges at `HalfRange == 0` do not trigger dominance unless an exact deterministic axis differs (context, VRAM, RAM, capability). Verified with jitter scenarios.
- **Strictness:** a strict advantage on at least one axis must exceed combined noise on the remaining measurable axes. Verified; near-identical configs are not pruned arbitrarily.
- **Context IS memory:** a larger window at equal or worse performance is heavier, not better. Verified.

### 2.5 Duplicate elimination

`findDuplicateConfig` compares `backend.Config` structs directly; `adoptMeasurement` in the pipeline reuses evidence for identical configurations. Confirmed that two candidates with identical `Config` fields are never independently measured. Deterministic.

### 2.6 Maximum refinement steps

`internal/optimize/tuner.go` sets `MaxRefineSteps` to `opts.MaxRefineSteps` (default 4). The loop guards with `for step := 0; step < s.opts.MaxRefineSteps && hi > lo; step++`. Convergence is also bounded by `Midpoint == 0` (bracket already tight). Verified.

### 2.7 Frontier step-down

`nextLowerLevel` in `tuner.go:543` picks the largest measured passing level below `current`, then falls back to `search.Midpoint(ceil, current, MinRefineGranularityTokens)`. Verified that this always retreats toward `capableCeil` and terminates.

### 2.8 Boundary cases tested

Added deterministic audit tests (`internal/search/audit_deterministic_test.go` — run via temporary injection):

- Exact floor: `mean=25.0, half=0.0, floor=25.0` passes; `mean=24.99` fails.
- OOM always fails even with high mean.
- Noisy `mean=25.5 ± 1.0` fails floor 25 because lower bound 24.5 < 25.
- Faster-but-worse-capability never dominates slower-but-capable.
- Unstable observation never dominates.
- Very small context ladder and bracket convergence.
- Very large context ladder and alignment.
- Dominance pruning with three comparable candidates.

All pass.

**Verdict: SEARCH CORRECTNESS — PASS**

---

## 3. Objective semantics

Three independent mechanisms:

| Mechanism | Source | Meaning |
|---|---|---|
| `Objective.Floor` | `--min-decode` | Absolute decode tok/s floor. Frontier AND profile eligibility respect it exactly. |
| `Objective.Retention` | Workload profile (`DecodeRetention`) | Relative to **best stable decode measured anywhere in this run** (not just REFERENCE). Updated in `tuner.registerObs` whenever a new stable measurement beats `bestDecode`. |
| `Objective.Baseline` | `search.Objective.Baseline` | Best stable decode observed; used by `EffectiveFloor()` when `Floor == 0`. |
| Conservative lower bound | `Mean - HalfRange` | Used in `Objective.Evaluate`. Zero observed variance means UNKNOWN noise floor — never claims separation from a single sample. |

### 3.1 Floating retention baseline

The retention baseline **floats upward** during the run as better decode is measured. This is intentional and documented in `tuner.go:239-244`:

```go
if stable && m.DecodeTPS > s.bestDecode {
    s.bestDecode = m.DecodeTPS
    if s.opts.MinDecode <= 0 {
        s.objective.Baseline = s.bestDecode
    }
}
```

During Stage B/C (frontier sweep) the baseline only changes from frontier probes themselves (the exploration line), keeping the rule internally consistent. During Stage D (variant lines) variants may introduce higher decode, raising the bar for later candidates — which is conservative and correct.

The report surface (`objective.BaselineDecodeTPS`) records the **final** baseline value, making the effective floor transparent. The statement is readable: `"decode >= 23.3 tok/s (75% of measured baseline 31.0)"`.

### 3.2 --min-decode vs retention

They are mutually exclusive: `EffectiveFloor()` returns `Floor` when `Floor > 0`, otherwise `Retention * Baseline`. No ambiguity. `--min-decode` is absolute and overrides workload practicality.

### 3.3 Reference performance

REFERENCE is always the first candidate. Its decode becomes the initial baseline, then the baseline may float upward. REFERENCE never dominates itself (domination requires strict improvement on at least one axis).

### 3.4 Candidate performance

Profile eligibility (`passEligible`) requires: `Feasible && Measured != nil && Error == "" && Gate != nil && Gate.Passed`. Objective satisfaction is checked separately for non-reference candidates via `objectiveSatisfied`.

**Verdict: OBJECTIVE SEMANTICS — PASS** (floating baseline is intentional, conservative, and transparently surfaced).

---

## 4. Profile selection

`internal/search/profiles.go:SelectProfiles` assigns four labels with deterministic tie-breaks:

| Label | Winner rule |
|---|---|
| MAX CONTEXT | Largest passing context; tie: less VRAM → smaller ctx |
| SPEED | Fastest decode lower bound; tie: less VRAM → smaller ctx |
| QUALITY | Highest capability rate → highest KV rank → largest context |
| BALANCED | Best workload utility score among unlabeled; when exhausted, joins utility-best labeled |

### 4.1 Collapse behavior

- Single candidate: carries all four labels (documented, tested).
- Empty input: note `no verified candidates — no profiles generated`.
- Operational ties: `tied(a, b)` reports overlap in repetition ranges at equal capability. Ties are **declared**, never manufactured.
- When every candidate is already labeled, BALANCED is reported as shared with a note: `"BALANCED shares <id>: every verified configuration serves a distinct role"`.

### 4.2 Qwen3 validation specifically

From `reports/val-qwen30b-agentic/`:

| Profile | Candidate | Context | KV | Decode | Capability |
|---|---|---|---|---|---|
| MAX CONTEXT / SPEED | CTX-40K | 40960 | Q4_0 | 30.4 tok/s | 12/12 (100%) |
| QUALITY | QUALITY | 40960 | F16 | 27.7 tok/s | 12/12 (100%) |
| BALANCED | REFERENCE | 16384 | F16 | 30.5 tok/s | 11/12 (92%) |

Why 40K f16 QUALITY is recommended over 16K f16 BALANCED:

1. QUALITY has **higher capability rate** (100% vs 92%) — decisive under `qualityBetter`.
2. QUALITY has **equal context** (40960) — better under `qualityBetter`'s third tie-break.
3. QUALITY's score reflects workload utility (prefill-bound + depth-bound weights: 0.65 × 1.0 + 0.35 × speed ratio) = **0.98** vs reference's **0.91**.
4. The report makes this plain: `"Capability: Tier 2 PASSED (12/12)"` with explicit count, versus reference `"11/12 (92%)"`.

The MAX CONTEXT / SPEED profile correctly notes the tie: CTX-40K (q4_0) and QUALITY (f16) are tied on capability (both 100%), but QUALITY wins on KV fidelity per `qualityBetter`. The SPEED label correctly selects the fastest point (CTX-40K with 30.4 tok/s decode).

### 4.3 No fake distinctions

Verified: when multiple verified configurations are operationally tied, the tie is reported in both JSON (`profiles[].tied_with`) and Markdown (`Operationally tied with: ...`). See chat validation where all four profiles share `tied_with` lists.

**Verdict: PROFILE SELECTION — PASS**

---

## 5. Capability safety

`internal/verify/engine.go:Gate(ref, cand, slack)`:

- Reference has no paired comparison; it is the control. `Gate` returns `true` with reason `"reference configuration"`.
- Capability regression threshold: `cand.Rate < ref.Rate - slack`. Default `slack = 0`.
- Smoke-only mode (tier=smoke): requires `cand.Rate < 1.0` as gate failure (smoke must be perfect).
- **Performance can never override capability.** Verified by:
  - `pipeline_test.go:TestCapabilityGateRejectsFastButDumb` — q4_0 at 99 tok/s loses to f16 at 30 tok/s because q4_0 fails the capability gate.
  - `pipeline_test.go:TestBaselineParticipatesInGate` — human baseline with same defect is rejected.
  - `search.DominatedBy` never lets capability-failed points dominate (only gate-cleared points have domination rights).

### 5.1 FAST + FAIL vs SLOWER + PASS

Tested via `fakeRunner` with `dumbKV: "q4_0"`: q4_0 produces correct perf numbers but fails capability tasks, while f16 passes everything. The gate rejects the faster config. Winner is the slower capable one.

### 5.2 Two PASS candidates with conflicting metrics

`ranking_test.go:TestRankMetricsDisagree` — decode favors candidate A, prefill favors B. Result: `LOW / indistinguishable` with note `"decode and prefill favor different candidates within measurement noise"`. No false ordering.

### 5.3 Missing telemetry

`ranking_test.go:TestRankInsufficientRepetitions` — single sample, or empty telemetry, yields `LOW / indistinguishable` with note `"insufficient repetitions for ranking"`. Never fabricates confidence.

**Verdict: CAPABILITY SAFETY — PASS**

---

## 6. Backend safety

### 6.1 Unsupported flags suppressed

`internal/backend/capabilities.go:ParseCapabilities` probes `llama-cli --help` once at startup. Discovered capabilities are fed to `gen.ApplyBackendCaps`. The generator then:

- Suppresses unsupported KV types upstream (`SupportedKV` check in `candidate.go:kvAllowed`).
- Suppresses expert placement when `-ot` absent (`placementAllowed` check).
- Records each suppression in `rep.Backend.Suppressed` and `Policy.DeclinedSlots`.

`TestBackendCapabilitySuppressionContinuesTuning` verifies: q4_0 suppression, -ot suppression, yet tuning still produces a verified winner.

### 6.2 Evidence-critical flags fail loudly

`internal/backend/llamacpp.go:validateAgainstCaps`:

- KV type not supported → `ErrUnsupported` returned before the subprocess runs.
- Expert placement unsupported → `ErrUnsupported` returned.
- These are not silent drops — they surface as rejected candidates with reasons.

Non-evidence-critical knobs (mmap, mlock) may degrade silently because they don't change the measured operating point.

### 6.3 No misrepresentation

`buildArgs` in `internal/backend/flags.go` constructs flags from `spec.Config` deterministically. Every flag present in the binary arguments corresponds exactly to a field in `Config`. There is no path where the backend runs `ConfigurationY` while the report records `ConfigurationX`.

The retry chain (`execWithRetries`) only adjusts legacy flag forms for flash-attention and conversation-mode — never KV type, context, layers, or batch.

**Verdict: BACKEND SAFETY — PASS**

---

## 7. Hardware-agnostic audit

### 7.1 No hardcoded assumptions in production code

Scanned all `internal/*.go` (excluding tests and fixtures) for:

- `RTX 5070`, `5070`, `12GB`, `12GiB`, `13680`, fixed throughput values, fixed VRAM sizes, fixed compute capability, fixed memory bandwidth.

Only hits:

- `internal/candidate/generate.go:467` — a comment referencing RTX 5070 validation as historical evidence for the mmap-less-then-floats rationale. **Comment only, not executed logic.**
- `internal/hardware/hardware_test.go` — fixture parsing `"NVIDIA GeForce RTX 5070, 12282, 512, 570.86, 12.0\n"` as test input. **Test fixture only.**

All production code derives hardware facts from probes (`nvidia-smi`, `rocm-smi`, `/proc/cpuinfo`, `/proc/meminfo`, `lspci` fallback). Unknown stays unknown.

### 7.2 Bug found and fixed: lspci ghost GPU

**Symptom:** Both validation reports show a second GPU entry `"00.0 VGA compatible controller"` with vendor `nvidia`, source `lspci`, no VRAM data. This corrupted the hardware section (reported 2 GPUs instead of 1).

**Root cause:** `parseLspciFallback` used `strings.Index(line, ":")` to extract the device name, which matched the **first** colon (the PCI slot "00:" part), yielding the bogus name `"00.0 VGA compatible controller"`. Additionally, the vendor-suppression logic (`!hasVendor(gpus, "amd")`) did not prevent adding nvidia entries when nvidia-smi had already found a nvidia GPU.

**Fix applied:** `internal/hardware/gpu.go` now:
- Builds `haveVendor` set from entries with `VRAMTotalBytes > 0` and skips lspci entries for any vendor already represented by a VRAM-backed probe.
- Extracts device name from the segment after `"]: "` in lspci's format, with fallback to last-colon extraction.
- Preserves deduplication via `haveKey` by `vendor/name`.

**Verification:** `./gumi probe --json` now returns exactly 1 GPU. `TestParseLspciFallback` still passes; new audit test `TestParseLspciFallbackWithExistingNvidia` passes.

### 7.3 GPU fallback order

nvidia-smi → rocm-smi → lspci (vendor-only). lspci is skipped entirely for vendors already detected with VRAM. lspci never fabricates VRAM. `lspci -nn` output on this machine correctly shows only one NVIDIA VGA controller at `05:00.0`.

**Verdict: HARDWARE-AGNOSTIC — PASS** (with lspci bug fixed).

---

## 8. Failure handling

Matrix verified against `internal/optimize/tuner.go`, `pipeline.go`, `backend/llamacpp.go`, `backend/flags.go`:

| Failure | Handling | Verdict |
|---|---|---|
| OOM during sweep | `hi = level; break` — wall recorded, refinement continues below | ✅ |
| OOM during variant | Candidate marked REJECTED with OOM in `res.Error` | ✅ |
| Timeout during probe | Classified as timeout; rejected; doesn't corrupt session | ✅ |
| Backend crash | Unclassified error → `StatusUnknown` row; never fabricated rejection | ✅ (verified via `crashRunner`) |
| Malformed GGUF | `gguf.Inspect` returns error; pipeline aborts with clear message | ✅ |
| Missing model | Same as malformed GGUF | ✅ |
| Missing llama backend | `ErrNotAvailable`; early return with actionable error | ✅ |
| No CUDA device | `hasGPU = false`; falls back to CPU-only path with `MaxContextFor` deriving from RAM | ✅ |
| Insufficient RAM | Memory arithmetic yields infeasible candidate; planning suppresses; no fabrication | ✅ |
| Capability-test timeout | Suite timeout classified; measurement recorded with timeout events; gate evaluates on available evidence | ✅ |
| No candidate satisfies objective | `TARGET NOT ACHIEVED` first-class outcome; `obj.Achieved = false`; exit code 1; best verified config named | ✅ |

Reference failure aborts the run (nothing rankable without anchor); partial artifacts are written first (`rep.WriteArtifacts` called before the error is surfaced).

**Verdict: FAILURE HANDLING — PASS**

---

## 9. Repeatability

### 9.1 Deterministic components

- Candidate generation: `candidate.Generate` is a pure function of `(ModelInfo, Hardware, Profile, Caps)`. Same inputs → same candidates (tested via `TestGenerateDeterministicAndBounded`).
- Search strategy: `Ladder`, `Midpoint`, `DominatedBy`, `PruneDominated`, `SelectProfiles`, `RankConfidence` are all pure functions.
- Pipeline orchestration: deterministic iteration over candidates (slice order preserved); dominance tie-break uses `other.ID < dominee` lexicographic comparison.
- Report rendering: deterministic from `report.Report` struct.

### 9.2 Nondeterministic components (acceptable)

- Hardware measurement noise (decode/prefill tok/s vary between runs).
- Backend initialization jitter.
- OS scheduler effects.

### 9.3 How noise is handled

- Performance decisions use **conservative lower bounds** (`mean - halfRange`), never raw means.
- Ranking confidence refuses to overclaim: single repetition → `LOW / indistinguishable`; zero variance → UNKNOWN noise floor.
- Operational ties are reported honestly with `tied_with` lists in both JSON and Markdown.
- Winner selection under ties falls back to safer margin (higher capability → higher VRAM headroom → fewer errors).

### 9.4 Structural chaos check

The search path is driven by discrete measurements (pass/fail per level), not by numeric magnitudes. Doubling sweep direction is deterministic. Bisection bracket narrows monotonically. No stochastic element exists in search strategy.

**Verdict: REPEATED RUN CONSISTENCY — PASS**

---

## 10. CLI UX

### 10.1 What the user sees

Banner:
```
GUMI AUTO-TUNER
Model: ...   Hardware: ...   Workload: ...
```

Progress:
```
Measuring REFERENCE configuration...
Searching context frontier...
  [PASS] 32K q4_0 — decode 28.9 tok/s meets target 24.6
  [REJECT] ...
Testing configuration variants...
Verifying frontier capability...
Final verification...
```

Summary:
```
MAX PRACTICAL CONTEXT
  40K tokens — decode 26.1 tok/s, prefill 3105.2 tok/s

QUALITY / BALANCED / SPEED / MAX CONTEXT blocks with config, metrics, confidence, tie notes.

RECOMMENDED <name> (<id>)
```

Full report printed after compact summary. Exit code `1` on `TARGET NOT ACHIEVED` or missing winner.

### 10.2 Information present

- Model name, architecture, params, quant, layers, training context — from GGUF inspect.
- GPU name and VRAM — from hardware probe.
- Workload selected — explicit flag or default.
- Objective active — stated in both banner and report body.
- Configurations tested — table with all candidates, statuses, reasons.
- Rejections — explicit reasons (`below target`, `capability regression`, `OOM`, `suppressed`).
- Final recommendation — with config, performance, capability, confidence.
- Maximum practical context — prominently displayed.
- Confidence and limitations — surfaced verbatim.

### 10.3 No unnecessary internals

The banner does not expose `--gate-slack`, `--perf-runs`, or internal stage names beyond what is useful. Backend capability details appear in the full report, not in the banner.

**Verdict: CLI UX — PASS**

---

## 11. Report integrity

### 11.1 JSON vs Markdown agreement

Checked both validation runs (`val-qwen30b-agentic`, `val-llama8b-chat`):

| Item | JSON | Markdown | Match |
|---|---|---|---|
| Winner ID | `quality` | QUALITY | ✅ |
| Max practical context | 40960 / 65536 | 40K / 64K | ✅ |
| Profiles labels | arrays | headers | ✅ |
| Candidate table values | numerics | formatted | ✅ |
| Objective statement | present | rendered | ✅ |
| Export strings | present | rendered | ✅ |

### 11.2 JSON evidence sufficiency

`report.json` contains enough fields to reconstruct:

- model, hardware, backend, workload, objective (all top-level).
- candidates with per-candidate: status, context, KV, layers, batch, prefill/decode, half-range, peak VRAM, smoke, capability, gate, confidence.
- frontier with coarse levels probed and refinement probes.
- ranking with level, indistinguishable flag, note, winner and runner-up IDs.
- profiles with labels, candidate IDs, tied_with lists.
- limitations array.

### 11.3 `candidates.json`

Full candidate objects including every perf sample (6 samples for verified candidates: 3 verification + 3 confirmation). Complete evidence for post-hoc audit.

**Verdict: REPORT INTEGRITY — PASS**

---

## 12. Export integrity

### 12.1 Winner-to-export correspondence

For both validations:

| Export target | Verified candidate config | Export flags match |
|---|---|---|
| llama.cpp CLI | ctx, ngl, kv, fa, b, ub, t, mlock, -ot | ✅ |
| llama.cpp server | same + host/port | ✅ |
| LM Studio | context_length, gpu_offload, flash_attention | ✅ (note: KV quantization noted as UI-only) |
| Ollama Modelfile | num_ctx, num_gpu, num_thread | ✅ (note: KV quantization flagged as unsupported by ollama) |

### 12.2 No false backend settings

- LM Studio export notes: `"set KV cache quantization to ... in the LM Studio model loader UI (not exposed via API)"`.
- Ollama export notes: `"note: ollama does not expose KV cache quantization; KV settings apply to llama.cpp only"`.

Exports never claim backend settings that the target cannot represent.

**Verdict: EXPORT INTEGRITY — PASS**

---

## 13. Bugs found and fixes

### Bug 1 (fixed): lspci ghost GPU in hardware report

- **File:** `internal/hardware/gpu.go`
- **Symptom:** Both validation reports listed a second GPU `"00.0 VGA compatible controller"` with vendor `nvidia`, source `lspci`, no VRAM. This made `report.Hardware.GPUs` show 2 entries instead of 1, polluting the evidence.
- **Root cause A:** `strings.Index(line, ":")` extracted the device name from before the first colon (PCI slot), not the segment after the class delimiter `]: `.
- **Root cause B:** The vendor-suppression condition (`!hasVendor(gpus, "amd")`) only guarded AMD duplication, not NVIDIA.
- **Fix:**
  1. Skip lspci entries for any vendor already represented by an entry with `VRAMTotalBytes > 0`.
  2. Extract device name from the text after `"]: "`, falling back to last-colon extraction.
  3. Maintain `haveKey` dedup by `vendor/name`.
- **Verification:** `./gumi probe --json` now returns exactly 1 GPU. Existing test `TestParseLspciFallback` still passes (existing=nil case still works). New audit test added and passing.

### No other correctness bugs found

All other checked behaviors matched the spec or were intentional conservative choices (floating baseline, single-sample conservatism, etc.).

---

## 14. Remaining limitations

These are **known, surfaced, and acceptable** for V1 release:

1. **Single-GPU, single-backend verification.** Multi-GPU tensor/pipeline split is out of scope; whatever llama.cpp does with visible devices is what gets measured.
2. **Exploration line breadth.** The frontier sweeps ONE line (reach-maximizing KV × placement). Other KV types are evaluated as point variants at their planned contexts, not swept.
3. **Refinement granularity.** Defaults to 2048 tokens within 4 bisection steps; tighter bounds cost more probes.
4. **Capability cost.** Tier-2 battery is the dominant cost; intermediate frontier levels carry no capability verdict unless promoted.
5. **Flat-throughput ties.** On GPUs where decode barely moves with context, profiles legitimately collapse onto near-identical operating points and are reported as tied.
6. **Warmup depth.** One discarded generation absorbs cold-start effects; deeper warmup (e.g. KV pre-fill) is future work.
7. **Platform coverage.** CUDA probing assumes `nvidia-smi`; process management and RSS sampling have platform files but are linux-primary. Untested platforms stay untested.
8. **Second GPU-class validation.** V1 validated two model shapes on one GPU (RTX 5070). An A100/H100-class validation pass remains desirable but is not a blocker.
9. **Floating retention baseline.** The workload practicality rule anchors on the best measured decode seen anywhere in the run, not just REFERENCE. This is more conservative than a fixed-reference interpretation and is surfaced in the report's `baseline_decode_tps`.
10. **Lspci fallback behavior.** When nvidia-smi is unavailable (e.g. WSL without driver), lspci will still populate vendor entries. Without VRAM data these are harmless but may appear in the hardware list. The fix prevents duplicates when VRAM data exists.

---

## 15. V1 release recommendation

**V1 READY WITH DOCUMENTED LIMITATIONS**

Rationale:

- Search correctness is deterministic, well-tested, and passes all boundary cases.
- Objective semantics are unambiguous: `--min-decode` is absolute; `DecodeRetention` is relative to the best measured decode; conservative lower bounds are used everywhere decisions are made.
- Profile selection never manufactures distinctions; operational ties are declared.
- Capability gate is the absolute authority; performance can never override it.
- Backend capability discovery suppresses unsupported dimensions safely; evidence-critical flags fail loudly.
- Hardware abstraction is clean: unknown stays unknown; no hardware assumptions exist in production code.
- Failure handling covers all tested failure modes with explicit errors or graceful recovery.
- Repeated runs are structurally stable; measurement noise is handled conservatively.
- CLI UX surfaces everything the user needs to judge a recommendation without exposing internal machinery.
- Reports (Markdown and JSON) are in agreement and contain sufficient evidence for post-hoc audit.
- Exports correspond exactly to verified candidates with honest caveats about unexposed settings.

The single bug found (lspci ghost GPU) has been fixed and verified. The floating retention baseline is intentional and conservative, not a defect.

The product satisfies the V1 contract:

> Gumi experiments on the actual CUDA machine. Gumi measures. Gumi rejects bad configurations. Gumi searches the practical frontier. Gumi verifies the winner. Gumi tells the truth when configurations are tied or the objective cannot be achieved.

**No further blocking issues remain.** Proceed to V1 release.

---

## 16. Documentation consistency note (added post-audit)

Pre-pivot specs `00`–`22` and `GEP_v1` are now bannered as **Historical — Pre-Pivot Architecture (Frozen)** and retained for provenance. Current product documentation is `23`–`27` + `README.md` + `AGENTS.md` + `docs/guides/*` (rewritten for the V1 auto-tuner). Older guides/docs/examples that referenced `gumi start` / dashboard / `8787`/`8788` have been removed or rewritten; current CLI is `gumi tune` / `inspect` / `probe` / `profiles` / `export`. Preferred terminology is *local inference auto-tuner*, *measured/verified configuration*, *practical context frontier*, *capability gate* — not *AI-powered* / *guaranteed optimal* / *same intelligence*.
