# Example: `gumi tune` report (V1 format — illustrative)

Illustrative output of `gumi tune qwen3-30b-a3b-q4_k_m.gguf --workload agentic_coding` (`optimize` is an alias). The dry-run form is identical minus the verified numbers;
performance, capability, and confidence sections appear only after a real
verification run. Illustrative — not a performance guarantee; real throughput varies by hardware, driver, and llama.cpp build.

```markdown
# GUMI OPTIMIZATION REPORT

_Generated 2026-08-24T15:02:11Z by gumi 0.2.0_

**Workload:** agentic_coding

## Model

- **qwen3-30b-a3b-q4_k_m.gguf** (qwen3moe, ~30.5B, Q4_K_M)
  - MoE: 8/128 active experts
  - Layers: 48, Training context: 40960, File: 18.6 GB

## Hardware

- GPU: NVIDIA GeForce RTX 5070 12GB
- RAM: 32 GB total (26 GB available)
- CPU: AMD Ryzen 7 5700X 8-Core Processor (16 threads)
- Storage: ext4

## REFERENCE CONFIGURATION

**REFERENCE**: context 16384, KV cache F16, GPU offload 29/48, experts in system RAM

**Why selected:**

- memory safe: context capped at the workload minimum (16384 tokens), f16 KV cache accounted exactly from GGUF geometry, planned within a 95% VRAM budget
- highest quality settings: f16 KV precision, greedy decoding (temperature 0), fixed seed, flash attention
- stable execution: partial GPU offload (29/48 layers) as required by memory safety
- Expert tensors placed in system RAM for memory safety (qwen3moe family is whitelist-approved for expert placement).
- used for paired comparison: every candidate runs identical prompts and seeds against this configuration

## Verified candidates

| Config | Context | KV | GPU layers | Batch | Prefill tok/s | Decode tok/s | Capability | Confidence | Verdict |
|---|---|---|---|---|---|---|---|---|---|
| REFERENCE *(experimental)* | 16384 | F16 | 29/48 | 2048/512 | 812.4 | 41.2 | 9/9 (100%) | MEDIUM | passed |
| QUALITY | 16384 | F16 | 0 (cpu) | 2048/512 | - | - | - | - | infeasible |
| BALANCED *(experimental)* | 16384 | Q8_0 | 32/48 | 1024/512 | 1105.7 | 58.9 | 9/9 (100%) | MEDIUM | **RECOMMENDED** |
| SPEED *(experimental)* | 16384 | Q4_0 | 33/48 | 4096/2048 | 1493.1 | 77.4 | 5/9 (56%) | LOW | capability regression: candidate 56% < reference 100% (slack 0.00) |

## RECOMMENDED ⭐

### BALANCED

Optimized memory use: q8_0 KV cache (requires flash attention), practical context, full or expert-split offload. Expert tensors placed in system RAM.

**Configuration:**

- Context: 16384 tokens
- KV cache: Q8_0
- GPU offload: 32/48
- Batch: 1024 (ubatch 512)
- Expert placement: experts in system RAM *(experimental)*

**Performance (verified):**

- Prefill: 1105.7 tok/s
- Decode: 58.9 tok/s
- Peak VRAM: 10.9 GB

**Capability:**

Tier 2 PASSED (9/9)

**Confidence:** MEDIUM

Evidence:

- + capability verification passed (Tier 2)
- + 3/3 stable perf runs
- + stable decode latency (spread 4%)
- − experimental expert placement active

### Alternatives

- **SPEED**: decode 31% faster, lower KV precision; experimental expert placement *(rejected by the capability gate — shown for transparency)*
- **REFERENCE**: decode 30% slower, higher KV precision; experimental expert placement

## Exports

**llama.cpp (cli):**
```bash
llama-cli -m qwen3-30b-a3b-q4_k_m.gguf -c 16384 -ngl 32 -ctk q8_0 -ctv q8_0 -fa on -b 1024 -ub 512 -t 16 -ot exps=CPU
```

**LM Studio / Ollama:** analogous load settings and Modelfile.
```

Notes on reading the report:

- **MEDIUM confidence** here is honest, not hedging: BALANCED relies on
  experimental MoE expert placement. A dense model with the same evidence
  would rate HIGH.
- **SPEED stays visible** even when rejected so users can see exactly what
  the gate refused and why.
- The REFERENCE section is the audit trail for "what did we compare
  against?" — it is policy-selected, never a random default.
