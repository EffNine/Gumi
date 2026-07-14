# Show HN: Gumi – An intelligence runtime that makes local AI models production-ready

**Title:** Show HN: Gumi – An intelligence runtime that makes local AI models production-ready

**Body:**

Local AI is private and cheap, but broken JSON, repeated output, and weak instruction-following make it unusable in real apps.

Gumi is a runtime layer that sits between your app and your local inference server (Ollama, LM Studio, or any OpenAI-compatible server). You point your app at Gumi's `/v1/chat/completions` endpoint and it stabilizes every request before it reaches the model.

What changes:

- JSON validity: 0% → 100% (Ornith 9B, agentic coding suite)
- JSON latency: 2,949ms → 352ms (8.4× faster)
- Instruction-following: 78% → 100% (14 constraint types auto-detected)
- Terminal-Bench JSON parser warnings: 11 → 1 (−91%)

These compound across 30+ agent turns — your agent stops getting stuck on bad JSON and wasted tool calls.

Key features:

- Agent mode — step budget enforcement, loop detection, tool-call JSON validation, context compaction
- Zero-VRAM memory engine — facts, episodes, model-fit tracking, survives session boundaries
- Agentic Coding Router — auto-selects model per step by task difficulty
- 17 validated model profiles (temperature, context length, thinking policy, provider quirks)
- Local dashboard + CLI diagnostics
- Written in Go, single binary, no GPU overhead

Quick start:

```
git clone https://github.com/EffNine/Gumi.git
cd Gumi && make build
./gumi start
```

Then point your app at `http://127.0.0.1:8787/v1` with API key `gumi-local`.

Works with OpenCode, Continue, Cline, Open WebUI, or any OpenAI SDK.

Repo: https://github.com/EffNine/Gumi

First release: v0.2.0-alpha. Feedback welcome.