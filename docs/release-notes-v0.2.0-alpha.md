# Release v0.2.0-alpha — Agent Mode, Memory Engine & LM Studio Management

**Tag:** `v0.2.0-alpha`
**Date:** 2026-07-14
**Status:** Pre-release (alpha)
**License:** Apache-2.0

---

Novexa is an intelligence runtime that makes local AI models more stable,
reliable, and production-ready. It sits between your app (OpenCode, Continue,
Cline) and your local inference engine (Ollama, LM Studio), hardening every
turn so local models behave like cloud-grade ones.

`v0.2.0-alpha` is the first tagged release. It bundles 13 sprints of work into
a single runtime that turns rough local models into reliable coding agents.

---

## 🎯 Headline metrics

Tested on Ornith 9B and Qwen 3.5 9B via LM Studio. Full report:
[`benchmarks/reports/SUMMARY-20260712.md`](../benchmarks/reports/SUMMARY-20260712.md).

| Metric | Before (direct) | After (Novexa) | Change |
|---|---|---|---|
| JSON validity (agentic) | 0% | **100%** | +100% |
| JSON + required keys | 0% | **100%** | +100% |
| Tool-call accuracy | 100% | 100% | maintained |
| Latency p50 (JSON) | 2,949 ms | **352 ms** | **8.4× faster** |
| HTTP errors | ~50% | **0%** | eliminated |
| Repetition false positives | 113 | **0** | eliminated |
| Terminal-Bench parser warnings | 11 | **1** | −91% |
| Instruction following (structured) | 67% | **100%** | +33% |

These per-turn gains compound across multi-turn agent loops. When an agent
makes 30+ turns, a single broken JSON response can stall the whole run —
Novexa makes that failure mode disappear.

---

## ✨ What's new in v0.2.0-alpha

### Agent mode
A dedicated runtime mode for agentic coding loops with step-budget
enforcement, tool-call loop detection, tool-call JSON validation, and
sliding-window context compaction that actually trims old messages instead of
just hinting.

### Agentic coding router
Automatic per-step model selection. The router classifies every agent step
by difficulty (trivial → novel) and task type (fix, refactor, feature, test,
review, docs, search, plan) using structural heuristics — no AI inference
needed. A "fix typo" step routes to a tiny model; the next "implement payment
handler" escalates to a strong one. Opt-in, agent-mode only.

### Memory engine
Zero-VRAM persistent memory for coding agents. Facts, episode summaries, and
model-fit data are stored in SQLite — shared across all models, surviving
model swaps and session boundaries. Feeds the router's feedback loop.

### LM Studio model management
Load, unload, and list models via LM Studio's v1 REST API. Per-model config
overrides (context length, flash attention, KV-cache offload, eval batch size,
num experts). Auto-unload previous model on switch.

### Instruction-following assist
Auto-detects 14 constraint types from user prompts (sentence count, word
limits, forbidden words, end-with, JSON, line count, bullets, and more) and
injects explicit numbered reminders. Post-generation validation with automatic
retry.

### Managed thinking
Controlled reasoning for local reasoning models. Token budget split into
output + reasoning. Reasoning stripped from final responses. Automatically
disabled for JSON and tool-calling workflows.

### Reliability fixes (P0–P3)
- Exponential backoff for LM Studio model-loading 502s (~50% → 0% errors)
- Validation telemetry now writes issue code + message + location (was `{}`)
- Repetition detection skips JSON/structured output (113 false positives → 0)
- JSON repair handles any language-tagged code fence (```python, ```javascript)

---

## 📦 Release artifacts

Pre-built archives for five platforms (the GitHub Actions workflow creates a
**draft** release — publish it after reviewing):

| Platform | Archive |
|---|---|
| macOS arm64 | `novexa_v0.2.0-alpha_darwin_arm64.tar.gz` |
| macOS amd64 | `novexa_v0.2.0-alpha_darwin_amd64.tar.gz` |
| Linux amd64 | `novexa_v0.2.0-alpha_linux_amd64.tar.gz` |
| Linux arm64 | `novexa_v0.2.0-alpha_linux_arm64.tar.gz` |
| Windows amd64 | `novexa_v0.2.0-alpha_windows_amd64.zip` |

Each archive contains the binary, embedded dashboard, starter model profiles,
`novexa.example.yaml`, `README.md`, `LICENSE`, and `CHANGELOG.md`. SHA256
checksums are published alongside the archives.

### Install from source

```bash
git clone https://github.com/EffNine/Novexa.git
cd Novexa
make build
./novexa start
```

### Docker

```bash
docker build -t novexa:0.2.0-alpha .
docker run -d --name novexa \
  -p 127.0.0.1:8787:8787 \
  -p 127.0.0.1:8788:8788 \
  -v novexa-data:/data \
  novexa:0.2.0-alpha
```

---

## ⚡ Quick start

```bash
./novexa start
```

Point any OpenAI-compatible client at Novexa:

```text
base_url: http://127.0.0.1:8787/v1
api_key:  novexa-local
model:    lmstudio:qwen2.5-coder-7b-instruct
```

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "Write a Go function that adds two ints. Code only."}]
  }'
```

Dashboard: `http://127.0.0.1:8788`

---

## 🧪 Tested models

| Model | Role | Profile |
|---|---|---|
| Ornith 9B (q4_k_m) | Quality alternative / agentic | `ornith-1.0-9b-q4-km` |
| Qwen 2.5 Coder 7B | Primary fast coder | `qwen2.5-coder-7b` |
| Qwen 3.5 9B | Complex reasoning fallback | `qwen3.5-9b` |
| Essential AI RNJ-1 | Reasoning model | `essentialai-rnj-1` |
| Gemma 3 1B / 4B | Fast chat | `gemma3-1b`, `gemma3-4b` |
| Llama 3.2 3B | Ollama fast chat | `llama3.2-3b` |

---

## ⚠️ Alpha limitations

- Continue tab autocomplete should use LM Studio directly for now.
- Docker image verification may vary by host.
- Profile Doctor is read-only.
- APIs may change before v1.

---

## 📝 Full changelog

See [`CHANGELOG.md`](../CHANGELOG.md) for the complete sprint-by-sprint history
(Sprints 0–13).

---

## 🙏 Acknowledgements

Thanks to the LM Studio, Ollama, and local-AI communities whose tools make this
runtime layer possible.

---

**Downloads:** [GitHub Releases](https://github.com/EffNine/Novexa/releases/tag/v0.2.0-alpha)