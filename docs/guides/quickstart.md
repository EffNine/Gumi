# Quickstart — Local Inference Auto-Tuner

This quickstart takes about five minutes and covers building Gumi, inspecting a
model, probing hardware, and running a real tuning session. No dashboard, no
server, no cloud.

## 1. Build Gumi

```bash
git clone https://github.com/EffNine/Gumi.git
cd gumi
make build        # produces ./gumi
./gumi --help
./gumi tune --help
```

Requirements: Go 1.25+. No `llama.cpp` or model files needed to build or run
`--dry-run` plans.

For a real tuning run you need `llama-cli` (llama.cpp, CUDA build recommended)
on `PATH` or via `--backend-bin`. See [installation](./installation.md).

## 2. Get a GGUF model

Any GGUF file works. Download one with your preferred tool (e.g. `huggingface-cli`,
`wget`, LM Studio, or Ollama export). Example:

```bash
# Example — pick any GGUF you have; do not treat the filename as a requirement
ls ~/models/*.gguf
```

Inspect the metadata (no backend needed):

```bash
./gumi inspect ./model.gguf
./gumi inspect ./model.gguf --json
```

This prints architecture, parameter count, quantization, layers, training context,
and exact KV-cache geometry (e.g. Qwen3-30B-A3B = 96 KiB/token at f16).

## 3. Probe hardware

```bash
./gumi probe
./gumi probe --json
./gumi probe --bandwidth   # opt-in ~1s RAM bandwidth micro-benchmark
```

Probing reads `nvidia-smi` (GPU name/VRAM), CPU topology, RAM, and filesystem
capabilities. Unknown stays unknown — never fabricated. Measurement overrides
planning wherever they disagree.

## 4. Tune

The V1 product command:

```bash
# Default workload: agentic_coding (MinContext 16384, retain ≥75% decode)
./gumi tune ./model.gguf --workload agentic_coding

# Chat workload (MinContext 4096, retain ≥85% decode — decode-bound)
./gumi tune ./model.gguf --workload chat

# Explicit decode floor — absolute tok/s, gates frontier AND profiles
./gumi tune ./model.gguf --workload agentic_coding --min-decode 25

# Plan only — no llama.cpp needed
./gumi tune ./model.gguf --workload agentic_coding --dry-run

# Compare your current config through the same pipeline
./gumi tune ./model.gguf --baseline 'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128'
```

Typical run prints:

```
GUMI AUTO-TUNER
Model: ...  Hardware: ...  Workload: ...

Discovering backend capabilities...
Measuring REFERENCE configuration...
Searching context frontier...
  [PASS] 32K q4_0 — decode 28.9 tok/s meets target 24.6
Testing configuration variants...
Verifying frontier capability...
Final verification...

MAX PRACTICAL CONTEXT
  40K tokens — decode 26.1 tok/s, prefill 3105.2 tok/s

QUALITY / BALANCED / SPEED / MAX CONTEXT blocks with config, metrics, confidence.
RECOMMENDED ...
Full report: reports/<model>-<workload>-<timestamp>/
```

## 5. Read the report

Each run writes `reports/<model>-<workload>-<timestamp>/`:

- `report.md` — human-readable recommendation (start here)
- `report.json` — machine-readable evidence
- `candidates.json` — every candidate + every perf sample
- `hardware.json` — probed hardware snapshot

The report answers *what should I run?* — a recommended block (config, verified
tok/s with ± half-range, capability tier), per-candidate confidence
(HIGH/MEDIUM/LOW), and a **ranking confidence** that says when top candidates
are operationally tied rather than inventing a winner.

## 6. Export the winner

```bash
# List workload contracts and verification suites
./gumi profiles

# Render one saved candidate as a launch config
./gumi export --config reports/<run>/candidates.json --id balanced \
    --target llama.cpp --model ./model.gguf
# targets: llama.cpp | llama-server | lmstudio | ollama
```

Exports are static renders of the verified config — they never claim settings
the target cannot represent (e.g. LM Studio notes KV quantization as UI-only;
Ollama notes it as unsupported).

## Next steps

- [Installation](./installation.md) — detailed build, `llama-cli`, and platform notes.
- [System requirements](./system-requirements.md) — what V1 supports and what it does not.
- [Troubleshooting](./troubleshooting.md) — OOM, timeout, capability-gate failures.
- `docs/specs/26-gumi-v1-auto-tuner.md` — the full V1 product contract.
- `docs/experiments/` — real-hardware validation provenance.
