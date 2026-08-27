# 25 — Evidence Semantics — Capability, Stability, Ranking (V1 Component)

Version: 1.0
Status: **Implemented — preserved inside V1 auto-tuner**
Phase: 4 (extends `24-verification-confidence-phase2.md` and Experiment 01). See `26-gumi-v1-auto-tuner.md` for the V1 product framing.

Gumi's reports now answer three separate questions. Conflating them is how
optimizers overclaim.

---

# 1. The three questions

| Question | Answered by | Source of truth |
|---|---|---|
| Does this configuration preserve model capability? | **Capability confidence** (`Confidence` per candidate) | paired Tier-1/Tier-2 suites vs REFERENCE |
| Is its performance measurement trustworthy? | **Performance stability** (repeated perf probes, `± half-range`) | `--perf-runs` samples |
| Do we have enough evidence to say it beats the other passing candidates? | **Ranking confidence** (`Ranking` per report) | pairwise mean/range comparison of the top two passers |

PASS/REJECT verdicts come only from the capability gate. Ranking statements
come only from measured performance evidence — and are allowed to say
"we cannot tell."

# 2. Capability confidence (unchanged rules, clarified scope)

Deterministic HIGH/MEDIUM/LOW from `internal/confidence.Assess`: gate
verdict, suite completeness, repeated-run success, OOM/timeout events,
VRAM headroom, experimental-placement flag. Unknown evidence is neutral.
This rating NEVER claims one candidate is faster than another.

# 3. Performance stability

Each candidate's decode tok/s is reported as `mean ± (max−min)/2` with the
run count when ≥2 probes succeeded. A missing or single-sample value is
reported without a ± marker rather than inventing a stability figure.

# 4. Ranking confidence (`internal/confidence.RankConfidence`)

Compares ONLY the top two gate-passing candidates, using their perf-probe
samples. Deterministic thresholds on separation-vs-noise ratio, where
separation = |Δmean| and noise = max(max−min range of either candidate):

- **HIGH** — both metrics' ratios ≥ 2: "no overlap in measured ranges".
- **MEDIUM** — both metrics' ratios ∈ [0.5, 2): means ordered, ranges touch.
- **LOW / operationally indistinguishable** — either metric below 0.5,
  or the two metrics favor different candidates, or fewer than 2 samples
  exist for either side.

Zero observed variance is treated as an UNKNOWN noise floor, never as zero
noise: a metric that returned bit-identical values provides no evidence for
a separation claim.

## 4.1 Recommendation policy under ties

When the top two passers are operationally indistinguishable, Gumi does not
manufacture a speed winner. Winner selection falls back to the safer
operating margin, in deterministic order:

1. higher capability rate;
2. larger measured VRAM headroom against the planning budget;
3. fewer error events;
4. otherwise the scored order stands.

The report renders this as a tie note on the recommendation and marks the
runner-up as "**operationally tied with the recommendation**" in
Alternatives. `report.json` carries `ranking.{level, indistinguishable,
note, winner_id, runner_up_id}`.

# 5. Hardened agentic-coding fixtures (Task 3/4)

Added to the `agentic_coding` capability tier (suite now 16 tasks total:
3 smoke + 13 capability; target was 10–20):

| Task | Category | What it detects | Evaluation |
|---|---|---|---|
| `kv_probe_deep` | context_retrieval | **KV/context degradation at depth** — Experiment 01 showed q8_0 KV losing end-of-window recall while throughput looked healthy | seeded distractor access-codes fill ~85% of the window; the true tag appears once at ~97% depth; last-match exact check resists prompt echo |
| `multi_file_reasoning` | repository_reasoning | single-file shortcut reasoning | computed value spans two files; distractor constants in a third; exact numeric match (value never appears verbatim) |
| `subtle_bug_diagnosis` | bug_diagnosis | superficial patch-instead-of-diagnosis behavior | failing-test output implicates the less obvious of two functions; final-word exact name match |
| `late_instruction` | instruction_following | dropped instructions far from conversation start | structural constraint placed after ~60% seeded filler via `BuildLateInstruction`; exact multi-line format check |

Existing fixtures are unchanged. All new fixtures are code-defined, seeded
(`haystackSeed`-derived), network-free, LLM-judge-free, and bounded by the
engine's per-task token budgets plus run timeouts. Exec fixtures keep their
documented toolchain-skip behavior; everything else runs anywhere.

# 6. Regression protection (Task 6)

Pipeline-level proofs added in `internal/optimize/pipeline_test.go`:

- fast-but-dumb → REJECT (pre-existing, retained);
- slower-but-capable stays eligible and listed among Alternatives;
- overlapping measurement ranges ⇒ `Ranking.Level=LOW`, `indistinguishable`
  set, explicit tie note rendered;
- stable superior performance ⇒ HIGH ranking citing non-overlap;
- single repetition ⇒ ranking suppressed (LOW/indistinguishable);
- indistinguishable tie ⇒ safer margin (lower VRAM peak) wins without
  manufacturing a speed claim.

Unit-level proofs in `internal/confidence/ranking_test.go` cover threshold
boundaries, metric-direction conflicts, insufficient telemetry, and
determinism.

# 7. Real-hardware validation of the hardened suite

One full optimization run on RTX 5070 + Qwen3-30B-A3B Q4_K_M
(`~/reports/exp02-battery`, llama-cli v10360):

| Config | ctx | KV | Tier 2 (/12) | verdict |
|---|---|---|---|---|
| CURRENT-BASELINE | 8192 | q8_0 | 10/12 | passed |
| REFERENCE | 16384 | f16 | 10/12 | anchor |
| QUALITY ⭐ | 40960 | f16 | **11/12** | RECOMMENDED |
| SPEED | 16384 | q4_0 | 10/12 | passed |
| BALANCED | 16384 | q8_0 | **9/12** | **REJECTED** |

Findings:

- **`kv_probe_deep` discriminates sharply.** Every configuration except
  f16-at-40k picked the *same distractor code* (`KVX-3145`) instead of the
  true late-declared tag — deterministic temp-0 confirmation that quantized/
  shorter-context KV loses precisely the late-window recall this fixture
  targets. The older word-salad `retrieval_end` had passed these same
  configurations in Experiment 01; the distractor-hardened probe catches
  what that one missed.
- **BALANCED rejected for the third consecutive run** (now also losing an
  additional retrieval point): the capability gate remains the stable,
  trustworthy signal across suite revisions.
- **Ranking confidence refused to overclaim end-to-end**: the recommended
  QUALITY beats everything on prefill (+55%) but trails on decode (−13%);
  `RankConfidence` flagged conflicting metrics → `LOW /
  indistinguishable`, and the report states plainly that QUALITY and
  REFERENCE are operationally tied on overall performance despite
  QUALITY's capability edge. Gate says yes; ranking says "cannot claim
  faster"; both statements appear verbatim in the artifact.
- **`late_instruction` does not discriminate yet**: all configurations
  failed it identically (the model continued filler text instead of
  honoring the buried constraint). Uniform failure keeps the paired
  comparison fair but adds no signal; recalibration (stronger salience
  markers, adjusted depth) is queued as follow-up work, not silently
  left in place.
- `multi_file_reasoning` and `subtle_bug_diagnosis` passed everywhere in
  this run — objective and cheap, but their discriminating power within
  this model class remains unproven until a failing configuration appears.
