# Experiment 02 — Model-Family Generalization

Phase: 5
Date: 2026-08-25
Machine: RTX 5070 12 GB · Ryzen 7 5700X · 30 GB RAM (same as Experiment 01)
Backend: llama-cli v10360 (identical binary for every model and configuration)
Workload: `agentic_coding` (16-task suite), paired gating vs each run's own REFERENCE
Reduced protocol per Task 2: one full optimization flow per additional family,
no cross-family comparisons of absolute capability.

---

## Artifacts (all source-verified before use)

| Field | DeepSeek-V2-Lite | Meta-Llama-3.1-8B-Instruct |
|---|---|---|
| Role | alternate **MoE** family (`deepseek2`, whitelist member) | **dense** family (`llama`) |
| Source repo | HF `mradermacher/DeepSeek-V2-Lite-GGUF` | HF `bartowski/Meta-Llama-3.1-8B-Instruct-GGUF` |
| File | `DeepSeek-V2-Lite.Q4_K_M.gguf` | `Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf` |
| Size | 10,364,416,736 B (9.65 GiB) | 4,920,739,232 B (4.58 GiB) |
| SHA256 | `70985c39…b89ad1a` ✔ verified | `7b064f58…863033557c` ✔ verified |
| `gumi inspect` | deepseek2 · 15.7B · Q4_K_M · 27 layers · KV 16×192 · train ctx 163840 · yarn ×40 · MoE 64 total / 6 active / ffn 1408 · experts 8.87 GB | llama · 8.0B · Q4_K_M · 32 layers · KV 8×128 (GQA) · train ctx 131072 · no MoE |

Baseline specs (competent-user, family-appropriate):
- deepseek2: `ngl=27,c=8192,kv=q8_0,fa,b=512,ub=128,exps-cpu`
- llama: `ngl=max,c=8192,kv=q8_0,fa,b=512,ub=128`

## Results

### Meta-Llama-3.1-8B-Instruct (dense)

Full pipeline green on the first attempt:

| Config | ctx | KV | Tier2 | decode t/s | verdict |
|---|---|---|---|---|---|
| CURRENT-BASELINE | 8192 | q8_0 | 9/12 | 113.5 | VERIFIED |
| REFERENCE | 16384 | f16 | 9/12 | 79.0 | VERIFIED (anchor) |
| QUALITY | 32768 | f16 | 10/12 | 79.5 | VERIFIED |
| HIGH-CONTEXT ⭐ | 32768 | q8_0 | 10/12 | 79.0 | RECOMMENDED |
| BALANCED | 16384 | q8_0 | 9/12 | 78.3 | VERIFIED |
| SPEED | 16384 | q4_0 | 9/12 | 78.3 | VERIFIED |

- Dense planning path exercised end-to-end: no expert split anywhere, full
  32/32 offload, HIGH-CONTEXT fifth candidate correctly substituted for
  EXPERT-SPLIT, `-ngl max` baseline sentinel handled.
- Predicted VRAM within ~10% of measured peaks, conservative direction.
- Ranking confidence = LOW with explicit tie note (decode deltas ≤0.6%
  inside 10.4% spread) — semantics held on a completely different model.
- Failures were genuine model limits, not harness friction:
  `math_mult` answered 3891 (wrong arithmetic), `kv_probe_deep` picked a
  distractor, v1 `late_instruction` filler-continued.

### DeepSeek-V2-Lite (MoE, MLA architecture)

First run exposed two family-specific findings:

1. **KV-cache sizing (documented family rule).** Gumi computes
   324 KB/token from GGUF geometry; llama.cpp b10360 allocates ≈264 KB/token
   for deepseek2 (no MLA flag exists in this build — verified via --help).
   The mismatch is *conservative* (planner overestimates memory need), so
   planned configurations remain safe and runnable, but context ceilings are
   understated for this family. Rule recorded; no silent generalization.
2. **Chat-scaffold pollution.** The template renders bare `User:` /
   `Assistant:` lines into stdout, and degenerate temp-0 outputs can repeat
   turns. Fix applied to the harness (family-agnostic): CleanOutput strips
   bare turn-marker lines. Re-run below.

Re-run after the fix (artifacts `~/reports/gen-deepseek2`):

(see §Results addendum — filled from the completed re-run)

## Candidate-generation audit (Task 3)

| Family | Findings |
|---|---|
| llama (dense) | No defects: full-offload math, quality growth to 32768, HIGH-CONTEXT generation, sentinel handling all correct. Nothing changed. |
| deepseek2 (MoE) | Planning sane under the conservative KV rule: all candidates 27/27 layers + experts in RAM, contexts at workload minimums, nothing impossible or redundant generated. KV estimate deviation documented above; planner change deferred until a real failure (not just slack) demonstrates need. |

## Verdict criteria check

- GGUF parsing across archs: ✔ (deepseek2 MoE metadata incl. yarn scaling;
  llama GQA)
- Candidate generation sane: ✔ both families
- Hardware fitting sane: ✔ (predicted vs measured within tolerance)
- Backend flags valid: ✔ (no unknown-flag retries triggered on either model)
- Verification works: ✔ (suites ran; objective evaluators executed)
- Capability gate works: ✔ — on deepseek2 it correctly left only REFERENCE
  verified when sibling configs produced degraded output
- Reports honest: ✔ statuses, ranking ties, limitations sections rendered

(Verdict in §Verdict below, after the DeepSeek re-run numbers.)

---

## Results addendum — DeepSeek-V2-Lite re-run (post scaffold-cleanup)

| Config | ctx | KV | Tier2 | decode t/s | status |
|---|---|---|---|---|---|
| REFERENCE | 16384 | f16 | 5/12 | 18.9 | RECOMMENDED (only verified config) |
| QUALITY | 16384 | f16 | 5/12 | 18.0 | REJECTED · smoke 1/3 |
| BALANCED | 16384 | q8_0 | 5/12 | 21.0 | REJECTED · smoke 1/3 |
| SPEED | 16384 | q4_0 | 6/12 | 22.7 | REJECTED · smoke 0/3 |
| CURRENT-BASELINE | 8192 | q8_0 | 5/12 | 22.2 | REJECTED · smoke 1/3 |

Reading (honest, per the SIGNAL>DIFFICULTY rule):

- The pipeline functioned identically to Llama/Qwen: inspect → plan (27/27
  layers + experts RAM) → flags → measure → gate → statuses/ranking.
- Remaining failures are genuine model behavior, not harness artifacts:
  DSV2-Lite (15.7B-total / ~2.4B-active) exhibits degenerate temp-0
  formatting on short prompts (smoke_json/smoke_format), skips fences
  entirely on code tasks ("empty answer"), and on the recalibrated
  `late_instruction` produced the right words in the right order but
  dropped the required `## ` prefixes — a near-miss that v2's positional
  grader measured precisely.
- The capability gate did exactly its job: only REFERENCE remained VERIFIED;
  faster siblings with degraded smoke were REJECTED with recorded reasons.
  LOW confidence on every row reflects the sub-50% absolute rate — no
  overclaiming anywhere.

## Verdict

**PARTIALLY GENERALIZED**

Core pipeline (GGUF parse → geometry planning → candidate generation →
backend execution → paired verification → evidence reporting) works across
three families — qwen3moe, deepseek2, llama — with zero architecture-specific
code changes. Two documented family-level gaps keep it from full
generalization:

1. **deepseek2 KV sizing**: planner uses GGUF-declared attention geometry
   (~324 KB/token); b10360 allocates ≈264 KB/token (no MLA flag exists).
   Conservative direction (safe), but context ceilings are understated.
   Exact MLA-aware sizing is documented, not implemented.
2. **Template-scaffold sensitivity**: turn-marker stripping is generic, but
   small-active MoE models whose templates emit heavier scaffolding can
   still fail extraction-based checks in ways that lower measured capability
   (measured honestly rather than masked).

Both are bounded, documented requirements — neither is a Qwen3-specific
assumption baked into the architecture.

## Remaining uncertainties

- Only one additional MoE family executed (deepseek2); mixtral/qwen2moe are
  whitelisted but unexercised on real hardware.
- deepseek2 KV deviation quantified on ONE llama.cpp build (b10360); other
  builds with MLA support may behave differently.
- Absolute capability rates are suite-version-specific (Phase 4 vs Phase 5
  fixtures differ); only within-run paired comparisons are meaningful.
- `multi_file_reasoning` / `subtle_bug_diagnosis` still show no failures on
  any tested model — discriminating power remains unproven.
- Dense-model evidence rests on one 8B model; very large dense models
  (CPU-offload-heavy paths) are untested on this GPU class.
