> **Historical — Pre-Pivot Architecture (Frozen)**
> This document describes the pre-pivot Gumi runtime / dashboard / benchmark architecture. It is **not** the current V1 product. The current product is the **local inference auto-tuner** specified in `26-gumi-v1-auto-tuner.md` (with `23-optimization-engine-mvp.md` · `24-verification-confidence-phase2.md` · `25-evidence-hardening.md` · `27-gumi-v1-release-audit.md`). This file is retained for provenance — do not extend it.

---

# Auto-Fit Engine Specification

Version: 0.1
Status: **Proposed**
Scope: Hardware-aware automatic model fitting for Gumi Runtime — probe the
machine, generate feasible load configurations ("fit plans"), apply them via
provider model-management APIs, verify with micro-benchmarks, and monitor at
runtime.

---

# 1. Purpose

Local models that fit in a 12 GB VRAM budget are weak. The models that are
strong enough for agentic coding (30B-class MoE) do not fit — unless the
runtime knows how to place them: which tensors on GPU, what KV cache
configuration, what context length, what batch size.

Today users find these configurations by searching online, guessing, and
failing with out-of-memory errors. Gumi should do this automatically:

> **Gumi probes your actual hardware, computes which model placements fit,
> applies the best one through the provider's model management API, verifies
> it with a short benchmark, and monitors it while you work.**

The Auto-Fit Engine is not a fine-tuning tool, not a sampler-tuner (that is a
later quality-tier effort), and not a router change. It tunes **load-time
placement**, not request-time behavior.

---

# 2. The Problem

## 2.1 Fit Decisions Are Currently Manual and Error-Prone

A user with a 12 GB GPU + 32 GB RAM asking "can I run Qwen3-30B-A3B?" must
know:

| Question | Requires |
|----------|----------|
| Does the quantized file even fit? | Model size tables from Hugging Face |
| Expert-offload placement? | llama.cpp tensor-override syntax, per architecture |
| What context length is affordable? | KV-cache arithmetic per model geometry |
| Flash attention on or off? | Backend knowledge (required for KV-V quant) |
| Will it be fast enough? | RAM bandwidth reality (DDR4 vs DDR5) |

No mainstream tool answers all five automatically. LM Studio exposes sliders;
Ollama exposes almost nothing; llama.cpp exposes raw flags.

## 2.2 The Insight

> *"Fit planning is mostly deterministic arithmetic plus empirical
> verification — not machine learning."*

KV cache size is exactly computable from model geometry (layers × KV heads ×
head_dim × bytes). Weight residency is exactly computable from file size +
placement policy. The only unknowns (RAM bandwidth, real VRAM overhead) are
measured once by a 60-second micro-benchmark after load. This makes a reliable
v1 achievable without any learning loop.

## 2.3 Why This Matters Strategically

The hero outcome writes itself:

> "Gumi detected your 12 GB GPU and 32 GB RAM and configured
> Qwen3-30B-A3B with expert weights in system RAM. You are now running a 30B
> model that outclasses every 9B dense model that used to be your ceiling."

This is differentiated: LM Studio gives manual sliders, Ollama gives nothing,
and no one does closed-loop fit verification.

---

# 3. Audit: Existing Assets and Gaps (2026-08-24)

This proposal builds directly on infrastructure that already exists in the
runtime.

## 3.1 What Exists

| Asset | Location | Relevance |
|-------|----------|-----------|
| `ModelManager` interface — `LoadModel`, `UnloadModel`, `ListAvailableModels`, `BuildPerModelConfig` | `runtime/internal/provider/lmstudio_mgmt.go` | The actuator surface already exists for LM Studio |
| Load knobs: `ContextLength`, `FlashAttention`, `OffloadKVCache`, `EvalBatchSize`, `NumExperts` | `LMStudioLoadRequest` (same file) | Four of six MVP knobs already plumbed end-to-end |
| Management API routes | `runtime/internal/gateway/routes.go` (`POST /v1/gumi/providers/lmstudio/models/load\|unload`) | HTTP control plane exists; CLI (`internal/cli/lmstudio.go`) too |
| Per-model config map + defaults | `config.LMStudioMgmtConfig.ModelConfig` (`runtime/internal/config/config.go`) | Natural home for committed fit plans |
| Opt-in feature precedent | `Routing.Enabled=false`, `Memory.Enabled=false`, `SelfTuning.Enabled=false` | Auto-Fit follows the same opt-in pattern |
| Observation-driven self-tuning precedent | `config.SelfTuningConfig` (router Phase 3) | Architectural template for verify/commit loops |
| Local SQLite telemetry | `runtime/internal/storage/schema.go` (requests, pipeline_events, provider_health, …) | Extend with hardware + plan telemetry |
| Benchmark runner/scorer/degradation checks | separate `benchmark/` Go module (`runner/`, `scorer/`) | Reusable as the verification probe harness |
| Profiles | `profiles/*.yaml` (capabilities, defaults, context strategy) | Extended with per-model geometry metadata |

## 3.2 What Is Missing (the gaps this spec fills)

1. **Hardware prober** — nothing in the runtime detects GPU VRAM, system RAM
   (total/free), or RAM bandwidth. `gumi doctor` only checks provider
   reachability (`runtime/internal/gateway/handlers.go`, `handleDoctor`).
2. **Model catalog** — profiles carry capabilities but no geometry (layer
   count, KV heads, head_dim), no MoE flag, no per-quant sizes.
3. **Fit-plan concept** — no representation of "this model loaded *this way*
   on *this* hardware".
4. **Verification protocol** — loads happen without measuring t/s or VRAM
   peak; no regression gate.
5. **Runtime monitoring** — no VRAM/RAM pressure telemetry; OOM failures are
   indistinguishable from other provider errors.
6. **Actuator coverage** — expert-CPU offload (`-ot "exps=CPU"`) and KV-cache
   quantization are llama.cpp-server flags; neither is reachable through the
   LM Studio v1 REST API today.

---

# 4. Architecture

## 4.1 Position in Runtime

```
gumi start / gumi autofit plan
        ↓
Hardware Prober          (NEW — internal/hardware)
        ↓
Model Catalog            (NEW — extends profiles)
        ↓
Fit Planner              (NEW — internal/autofit)
        ↓
Provider Actuator        (EXISTING — ModelManager interface, extended)
        ↓
Probe Verifier           (NEW — timed micro-benchmark through normal pipeline)
        ↓
Commit Plan → config/profiles + SQLite telemetry
        ↓
Runtime Monitor          (NEW — watches pressure/OOM, triggers degrade)
```

Auto-Fit operates at **load time**. It never bypasses the Pipeline Engine:
verification probes run through the same `/v1/chat/completions` path as real
traffic.

## 4.2 Components

```
Auto-Fit Engine
├── Hardware Prober      ← GPU VRAM, RAM total/free, bandwidth estimate
├── Model Catalog        ← geometry metadata + MoE family whitelist
├── Fit Planner          ← deterministic constraint solver over plans
├── Actuator Adapters    ← per-provider capability matrix (thin)
├── Probe Verifier       ← load → timed prompts → accept/reject
├── Plan Store           ← committed plans in config + telemetry DB
└── Runtime Monitor      ← VRAM/RAM pressure watch, auto-degrade hook
```

---

# 5. Hardware Prober

## 5.1 Signals Collected

| Signal | Source (per OS) | Fallback |
|--------|-----------------|----------|
| GPU name + VRAM total/free | NVIDIA Mgmt Library (nvml) / `nvidia-smi`; Metal via `system_profiler` | Provider-reported (`GET /api/v1/models` load info) |
| System RAM total | `/proc/meminfo`, `sysctl`, `wmic` | hard requirement — abort if unknown |
| System RAM available | same | required for expert-offload feasibility |
| RAM bandwidth class | measured (see 5.2) | conservative DDR4 prior |

No new heavy dependencies: prefer direct syscalls/files; nvml bindings only if
cgo-free options prove insufficient.

## 5.2 Bandwidth Is Measured, Never Assumed

Expert-offload speed is RAM-bandwidth-bound. DDR5 dual-channel yields usable
generation speed; old DDR4 can crawl. After any expert-offload load, the Probe
Verifier measures generation throughput and rejects plans below threshold
(§9). The prober stores the measured bandwidth class in telemetry so later
planner runs start from data instead of priors.

## 5.3 Output Schema

```go
type Hardware struct {
    GPUs         []GPU     // name, vramTotalBytes, vramFreeBytes
    RamTotalBytes uint64
    RamAvailBytes uint64
    BandwidthGBps float64 // 0 = unknown until first measurement
    OS           string
    Backend      string   // detected serving stack (lmstudio, llama-server, …)
}
```

---

# 6. Model Catalog

## 6.1 Profile Extension

Profiles gain an optional `fit:` section. Absent `fit:` ⇒ Auto-Fit skips the
model (safe default).

```yaml
# profiles/qwen3-30b-a3b.yaml (new)
id: qwen3-30b-a3b
family: qwen3-moe
fit:
  moe: true
  experts_active: 8
  layers: 48
  kv_heads: 4
  head_dim: 128
  quants:
    q4_k_m: { file_gb: 18.6 }
    iq3_xxs: { file_gb: 14.1 }
  tensor_pattern: ".ffn_.*_exps."   # verified for this family
  min_ctx_tokens: 8192
```

## 6.2 KV-Cache Arithmetic (exact, planner input)

```
kv_bytes_per_token = 2 × layers × kv_heads × head_dim × bytes_per_elem
```

Example — Qwen3-30B-A3B (48 L, 4 KV-H, 128 D, f16):
96 KiB/token → 32k ctx ≈ 3.0 GiB, q8_0 ≈ 1.5 GiB, q4_0 ≈ 0.75 GiB.

The planner uses exact formulas where geometry is known and refuses to plan
when it is not — no silent estimates.

## 6.3 MoE Family Whitelist (V1)

Tensor-name patterns for expert offload differ per architecture. V1 ships
verified patterns only:

| Family | Pattern status |
|--------|---------------|
| Qwen3-MoE (30B-A3B variants) | verified `.ffn_.*_exps.` |
| DeepSeek-V3/Lite-style | verify before adding |
| gpt-oss | different naming — verify before adding |

Unknown families are planned **without** expert offload (dense-style partial
GPU offload only).

---

# 7. Fit Plans

## 7.1 Representation

```go
type FitPlan struct {
    ModelID       string
    Quant         string            // e.g. "q4_k_m"
    Placement     PlacementPolicy   // all_gpu | experts_cpu | partial_offload
    ContextLength int
    FlashAttention bool
    KVCacheQuant  string            // f16 | q8_0 | q4_0 (actuator permitting)
    EvalBatchSize int
    NumExperts    *int              // optional limiting knob
    EstVRAMBytes  uint64
    EstRAMBytes   uint64
}
```

## 7.2 Planner Algorithm (deterministic)

For each candidate (quant, placement, ctx, kv_quant):

1. Compute `est_vram = non_expert_weights + kv_cache(ctx, kv_quant) + activations(batch)`
   and `est_ram = expert_weights` (placement-dependent).
2. Reject if `est_vram > 0.92 × vram_free` or `est_ram > ram_avail − headroom(4 GiB)`.
3. Score survivors: `quality_prior(quant) × ctx_satisfaction × speed_prior(bandwidth_class)`.
4. Return ranked list; top plan is the candidate for verification.

Quality priors come from existing profile benchmark scores (e.g., the
validated-profiles table in README); speed priors start conservative and are
replaced by measurements after each probe.

## 7.3 Commit Semantics

An accepted plan is written to `providers.<p>.model_management.model_config[<model>]`
(existing config structure — zero schema migration) and tagged in telemetry
with the hardware fingerprint that produced it. Plans are re-evaluated when
hardware fingerprint changes.

---

# 8. Actuator Capability Matrix

| Knob | LM Studio v1 API | llama.cpp server | Ollama |
|------|------------------|------------------|--------|
| context_length | ✅ today | ✅ flag | ✅ num_ctx |
| flash_attention | ✅ today | ✅ flag | ❌ |
| offload_kv_to_gpu | ✅ today | n/a (flag-driven) | ❌ |
| eval_batch_size | ✅ today | ✅ flag | ❌ |
| num_experts | ✅ today | ❌ | ❌ |
| KV cache quant | ❌ investigate | ✅ `--cache-type-k/v` | ❌ |
| expert tensors → CPU | ❌ investigate GPU-layer param | ✅ `-ot` override | ❌ |

**Phase 1 actuator:** LM Studio only — everything in the "✅ today" column
ships with no new backend integration.
**Phase 2 actuator:** runtime-managed `llama-server` subprocess unlocks
expert-CPU offload and KV quant — the full hero feature. This adds process
lifecycle ownership to Gumi; it is scoped deliberately to Phase 2.
**Ollama:** right-sizing of `num_ctx` only; explicitly second-class.

Adapters stay thin per development rules: each actuator implements one narrow
interface (`LoadWithPlan(ctx, model, FitPlan) (AppliedPlan, error)`), layered
on the existing `ModelManager`.

---

# 9. Probe Verifier

After applying a plan, Gumi runs a fixed micro-probe through the normal chat
path (~60 s):

| Measurement | Pass condition (defaults) |
|-------------|--------------------------|
| Load success | no error, applied config echoes requested plan |
| Prompt processing t/s | ≥ 200 t/s (agent contexts are prompt-heavy) |
| Generation t/s | ≥ 8 t/s (reject crawling expert offloads) |
| VRAM peak | ≤ budget; no OOM retries |
| Sanity completion | one structured JSON task scores valid |

Failures fall back to the next-ranked plan automatically; results persist to
telemetry regardless. All thresholds configurable; `--dry-run` prints ranked
plans without loading anything.

---

# 10. Runtime Monitoring & Auto-Degrade

While a plan is active, the runtime records per-request latency and watches
for degradation signals (latency cliff, provider OOM errors, RAM pressure).
On repeated signals it can demote to the next-ranked plan (e.g., halve ctx
before unloading). Auto-degrade is **off by default** in Phase 1; Phase 1 only
records the signals.

---

# 11. Configuration

Follows existing conventions (YAML + `GUMI_*` env overrides, opt-in default):

```yaml
fit:
  enabled: false                # opt-in like routing/memory
  mode: "plan"                  # plan | apply | monitor
  target_vram_gb: 11            # safety margin below physical
  min_gen_tps: 8
  min_prompt_tps: 200
  auto_degrade: false
```

Env: `GUMI_FIT_ENABLED=1`, `GUMI_FIT_MODE=apply`.

---

# 12. CLI / API Surface

```bash
gumi fit scan                     # show hardware probe result
gumi fit plan [--model X]         # print ranked fit plans (--json supported)
gumi fit apply --model X          # plan + load + verify + commit
gumi fit status                   # active plan, last probe metrics
```

HTTP (dashboard-ready, mirrors existing route style):

```text
GET  /v1/gumi/fit/hardware
GET  /v1/gumi/fit/plans?model=X
POST /v1/gumi/fit/apply
GET  /v1/gumi/fit/status
```

Telemetry additions: `hardware_snapshots` and `fit_probes` tables (probe
metrics per plan attempt), extending `runtime/internal/storage/schema.go`.

---

# 13. Delivery Phases

| Phase | Scope | Exit criteria |
|-------|-------|---------------|
| **P1 — Right-Sizing (LM Studio)** | Hardware prober, catalog for 1 MoE family + dense fallback, planner, LM Studio actuator (existing knobs), probe verifier, CLI + routes, telemetry tables | On a 12 GB test box: `gumi fit apply --model lmstudio:<moE>` picks FA-on + max-feasible-ctx plan, passes probe, commits plan; unit tests for planner math |
| **P2 — Expert Offload (llama-server)** | Managed llama.cpp subprocess actuator, `-ot exps=CPU` plans, KV quant knobs, bandwidth measurement loop | Qwen3-30B-A3B q4 runs ≥ 8 t/s on 12 GB GPU + 32 GB DDR4/DDR5 RAM, hero demo reproducible from clean state |
| **P3 — Monitor & Auto-Degrade** | Pressure watcher, auto-degrade, plan re-eval on hardware change, dashboard Fit page | 24 h soak with no manual intervention on simulated pressure |
| **P4 — Catalog Growth** | More MoE families (DeepSeek-style, gpt-oss), speculative-draft plans using spare VRAM | Each new family lands with verified pattern + fixture tests |

Each phase is independently shippable; P1 alone delivers honest value
(right-sized context, correct FA/KV settings, no more OOM roulette).

---

# 14. Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| LM Studio API gaps (no KV quant / expert offload) | P1 scope limited to supported knobs; P2 owns the rest via llama-server |
| Per-arch tensor patterns break silently | Whitelist + probe sanity check (structured task must still pass) |
| RAM bandwidth disappoints on DDR4 | Measured, not assumed; verifier gates on t/s; plans ranked accordingly |
| Windows pagefile masks OOM until too late | Headroom constant + free-RAM probing; conservative `target_vram_gb` |
| Planner math wrong for exotic geometries | Refuse-to-plan when geometry unknown; never estimate silently |
| Scope creep into sampler tuning | Explicitly out of scope (§1); future separate spec |

---

# 15. Non-Goals (V1)

- Sampling/thinking auto-tuning (quality tier — separate spec)
- Cloud providers, billing, external telemetry (development rules 2, 3, 7)
- Multi-GPU tensor splitting beyond trivial detection
- Fine-tuning, LoRA, or training of any kind
- Non-local operating systems beyond current Gumi platform targets

---

# 16. Compliance with Development Rules

1. Local-first — everything runs on-device. ✅
2–3. No cloud/billing. ✅
4. Pipeline Engine untouched — probes traverse normal request path. ✅
5. Provider adapters stay thin — one small interface addition per actuator. ✅
6–7. No prompts/responses stored; probes log metrics only. ✅
8. Benchmark-before-tuning is literally automated here. ✅
