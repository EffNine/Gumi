# GPU Benchmark Summary — qwen2.5-coder-7b-instruct (RTX 5070 12GB)

**Host:** WSL2 private worker (`afnan`), NVIDIA GeForce RTX 5070 12GB  
**Provider:** LM Studio @ `http://172.25.128.1:1234/v1` (Windows host gateway from WSL)  
**Model:** `qwen2.5-coder-7b-instruct` Q4_K_M, `context_length=8192`  
**Gumi:** `GUMI_PROVIDER_DEFAULT=lmstudio GUMI_LMSTUDIO_URL=http://172.25.128.1:1234/v1`  
**Date:** 2026-07-17T18:54:25Z

## Setup notes (WSL ↔ Windows LM Studio)

LM Studio listens on Windows `0.0.0.0:1234`. From WSL, `localhost:1234` does **not** reach it.
Use the Windows host gateway IP instead:

```bash
WIN_HOST=$(ip route show default | awk '{print $3}')
export LMSTUDIO_URL=http://$WIN_HOST:1234/v1
export GUMI_LMSTUDIO_URL=$LMSTUDIO_URL
GUMI_PROVIDER_DEFAULT=lmstudio GUMI_LMSTUDIO_URL=$LMSTUDIO_URL ./gumi start

# Load model (prefer modest context so 12GB VRAM stays fast):
curl -X POST http://$WIN_HOST:1234/api/v1/models/load \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen2.5-coder-7b-instruct","context_length":8192}'
```

Default max context (131k) filled ~12GB VRAM and slowed generation to ~56s for 8 tokens.
Reloading with `context_length: 8192` used ~5.4GB and restored ~0.14s for the same prompt.

## vs prior CPU baseline (llama3.2:3b Ollama)

| Suite | llama3.2:3b after (CPU) | qwen2.5-coder-7b GPU + Gumi |
|-------|-------------------------|-----------------------------|
| Local-model passed | 25 / 39 | **33 / 39** |
| Exact instruction following | 18 / 24 | **24 / 24** |
| Valid JSON (all JSON prompts) | 7 / 15 | **9 / 15** |
| D-GumiStabilized | 9 / 9 | **9 / 9** |
| E-GumiStructured | 3 / 3 | **3 / 3** |
| Agentic tool calls (Gumi) | n/a | **18 / 18** |
| Agentic tool calls (direct) | n/a | 0 / 9 |

## Local-model scorecard (3 attempts)

| Metric | Value |
|--------|-------|
| Passed | 33 / 39 |
| Exact instruction following | 24 / 24 |
| Valid JSON | 9 / 15 |
| Profile Doctor | Good baseline with lightweight caveat |
| Conclusion | Worth it — Gumi matches/exceeds direct quality |

Notable: Direct LM Studio JSON failed all 3 attempts (fenced/invalid); Gumi Direct/Stabilized/Structured JSON passed. Lightweight JSON still fails (expected; doctor recommends stronger JSON instructions).

## Agentic coding (3 attempts)

| Metric | Value |
|--------|-------|
| Passed | 24 / 36 |
| Valid tool calls | 18 / 24 |
| Valid JSON | 6 / 12 |
| Direct tool calls | 0 / 9 |
| Gumi tool calls | **18 / 18** (lightweight/stabilized/structured) |

Direct LM Studio failed all tool-call and JSON checks; Gumi recovered tool calling completely and JSON under stabilized/structured.

## Managed thinking (1 attempt)

| Metric | Value |
|--------|-------|
| Passed (non-empty + no leak) | 7 / 9 |
| No reasoning leak | 9 / 9 |
| Direct thinking-on | 3 / 3 nonempty |
| Gumi thinking-on | 2 / 3 (planning → 422 empty) |
| Gumi thinking-off | 2 / 3 (planning → 422 empty) |

Coder-7B is not a dedicated thinking model; no explicit reasoning markers leaked. Planning prompt returned empty under Gumi (422) — worth a follow-up.

## Artifacts

- `qwen2-5-coder-7b-instruct-local-20260717T182603Z.md` (+ JSON)
- `qwen2.5-coder-7b-instruct-agentic-20260717T184710Z.md` (+ JSON)
- `qwen2.5-coder-7b-instruct-thinking-20260717T185425Z.md` (+ JSON)
- `qwen2.5-coder-7b-instruct-profile-doctor-20260717T185425Z.txt`
