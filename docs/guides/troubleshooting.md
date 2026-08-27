# Troubleshooting — Local Inference Auto-Tuner

## `llama-cli` not found

**Symptom:**

```
backend unavailable: llama-cli not found on PATH
```

**Fix:**

```bash
which llama-cli
# or pass explicitly
./gumi tune ./model.gguf --backend-bin /path/to/llama-cli
```

`gumi inspect`, `gumi probe`, `gumi profiles`, and `gumi tune --dry-run` do not need a backend.

## Capability gate rejects everything

**Symptom:** all variants show `REJECTED · capability regression: 9/12 < 10/12`.

This is **intended behavior** — a faster configuration that regresses vs REFERENCE must be rejected. This is the core differentiator (`internal/verify.Gate`, exercised in `internal/optimize/pipeline_test.go`).

**Checks:**

1. Read the **REFERENCE CONFIGURATION / Why selected** section in `report.md` — it documents the quality baseline you are being compared against (f16 KV where feasible, greedy, seed 42).
2. Check `candidates.json` → `gate.reason` for the exact regression (smoke must be perfect; Tier-2 rate ≥ reference − `--gate-slack`).
3. If REFERENCE itself looks degraded, the model/backend/workload may be mismatched for this machine — try `--tier smoke` to confirm the smoke tier, or a different workload.

Do not treat verification as proof of "same intelligence" — it is a **bounded, empirical screening** vs REFERENCE on the included test battery. Honest wording is `SCREENED` / `VERIFIED` / `RECOMMENDED` / `REJECTED` / `UNKNOWN`.

## OOM during frontier sweep

**Symptom:** `  [OOM] 64K …` then refinement continues below the wall.

This is normal. The coarse sweep records the memory wall at `hi = level` and refinement continues below it. The **maximum practical context** steps down through measured passing levels (≤ 3 attempts) before anchoring; it is never the theoretical arithmetic alone.

If every level OOMs, the frontier anchors at the workload minimum (`16384` for `agentic_coding`, `4096` for `chat`).

## Timeout

**Symptom:** `timeout` in `candidates[].error`, peak timings missing.

- Per-run isolation uses process-group `SIGKILL` on timeout (default `--timeout 10` minutes).
- Increase `--timeout`, reduce `--perf-runs`, or reduce the probed context.

Non-classified backend errors yield `UNKNOWN` rows — insufficient evidence, never fabricated verdicts.

## No verified winner / TARGET NOT ACHIEVED

**Symptom:** `TARGET NOT ACHIEVED` in the report, exit code `1`.

Gumi reports this plainly when no verified configuration satisfies the declared performance objective (`--min-decode` or `DecodeRetention`). It names the best verified configuration, writes all artifacts, and exits non-zero — it does not fake a winner.

Fix: relax `--min-decode`, switch workload, or resolve the capability bottleneck shown in the per-candidate table.

## Backend flag drift

**Symptom:** unknown-argument errors on a newer/older llama.cpp build.

Gumi probes `llama-cli --help` once at startup to select flash-attention syntax, KV type flags, `-ot`, `-ngl`/`-b`/`-ub`, `mmap`/`mlock`, `--single-turn`. Unsupported dimensions are suppressed upstream with recorded reasons (`report.json` → `backend_capabilities.suppressed[]` + `policy.declined_slots[]` + `LIMITATIONS`). Defense-in-depth `validateAgainstCaps` refuses to run a config demanding an unsupported evidence-critical flag rather than silently measuring something else; the unknown-argument retry chain is the final arbiter.

## `gumi probe` shows wrong GPU count or name

**Symptom:** `gumi probe --json` listed a ghost GPU like `00.0 VGA compatible controller` (fixed in `27-gumi-v1-release-audit.md` §7.2).

Fixed in `internal/hardware/gpu.go`: `lspci` fallback no longer duplicates vendors already represented by a VRAM-backed probe and extracts names correctly. Verify with:

```bash
./gumi probe --json | python3 -m json.tool | grep -A2 gpus
```

Without `nvidia-smi` (e.g. WSL without driver), `lspci` may still list a vendor entry without VRAM — harmless and harmlessly reported.

## Performance variability

- Reported tok/s values are **means ± half-range** over `--perf-runs` samples (default 3). A single-sample value is reported without `±`.
- Ranking confidence (`internal/confidence.RankConfidence`) compares only the top two gate-passing candidates. Zero observed variance is treated as UNKNOWN noise floor — never a separation claim; ties fall back to the safer operating margin.
- If REFERENCE OOMs, context halves once and retries; reference failure aborts the run (nothing rankable without the anchor) after writing partial artifacts.

## Malformed GGUF or missing model

```
inspect: GGUF header magic mismatch
tune: open model: no such file
```

`gguf.Inspect` fails fast and the pipeline aborts with a clear message.

## Stale documentation references

If you encounter mentions of `gumi start`, `gumi doctor`, `gumi stop`, `8787`/`8788`, dashboard, or runtime pipeline — those belong to the **frozen pre-pivot** Gumi (specs `00`–`22` + `GEP_v1`, code at `runtime/` + `dashboard/`). They are retained for provenance and not the current product. The V1 product commands are `gumi tune` / `inspect` / `probe` / `profiles` / `export` (`optimize` is an alias).
