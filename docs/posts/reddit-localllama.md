# Reddit — r/LocalLLaMA

**Title:** I built Novexa – a local AI runtime that fixes broken JSON, repeated output, and weak instruction-following from local models

**Body:**

Hey r/LocalLLaMA —

Like a lot of you, I run local models for privacy and cost reasons. But every time I tried to use them in actual apps (agent loops, tool calling, structured output), the same things kept breaking:

- JSON that's invalid half the time
- Output that loops or repeats
- Instruction constraints the model just ignores
- Provider quirks that need per-model tuning

I'd get a model working, then switch tasks and it'd fall apart. The model wasn't bad — the layer around it was.

So I built **Novexa**. It's a runtime that sits between your app (OpenCode, Continue, Cline, anything OpenAI-compatible) and your local server (Ollama, LM Studio, etc.). It validates and repairs JSON, enforces instruction constraints, detects loops, and manages model profiles — all before the response gets back to your app.

Concrete numbers from benchmarks with Ornith 9B:

| Metric | Direct | With Novexa |
|---|---|---|
| JSON validity | 0% | 100% |
| JSON latency (p50) | 2,949ms | 352ms (8.4× faster) |
| Instruction-following | 78% | 100% |
| Terminal-Bench JSON warnings | 11 | 1 (−91%) |

Tool call accuracy stayed at 100% — nothing broke, it just got more reliable.

The other thing: these gains compound. When your agent runs 30+ turns on a task, a 0% → 100% JSON fix means it stops getting stuck on malformed tool calls and wasted cycles.

What's in it:

- **Agent mode** — step budgets, loop detection, tool-call validation, context compaction
- **Memory engine** — zero-VRAM persistent memory (facts, episodes, model-fit), survives across sessions
- **Agentic Coding Router** — picks the right model per step based on task difficulty
- **17 validated model profiles** — temperature, context length, thinking policy, provider quirks tuned per model
- **Local dashboard + CLI diagnostics** so you can actually see what's happening
- Written in Go, single binary, no extra VRAM

Quick start:

```
git clone https://github.com/EffNine/Novexa.git
cd Novexa && make build
./novexa start
```

Point your app at `http://127.0.0.1:8787/v1`, API key `novexa-local`.

Repo: https://github.com/EffNine/Novexa
First release: v0.2.0-alpha

I'd really appreciate feedback — especially from anyone running agentic workloads or multi-model setups. What breaks for you that I haven't covered? What models should I profile next?

Happy to answer questions about how the JSON repair, instruction assist, or memory engine work under the hood.