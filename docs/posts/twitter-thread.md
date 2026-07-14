# Twitter/X Thread

**Tweet 1:**

Local AI is private and cheap. But broken JSON, repeated output, and empty responses make it unusable in real apps. I built Gumi to fix that. 🧵

**Tweet 2:**

The problem isn't the model — it's the layer around it. Local models emit invalid JSON, ignore instruction constraints, loop on repeated output, and need per-provider tuning. Every agent run that hits 30+ turns amplifies these failures until the task falls apart.

**Tweet 3:**

Gumi is a runtime that sits between your app (OpenCode, Continue, Cline, any OpenAI SDK) and your local server (Ollama, LM Studio). Point your app at its /v1 endpoint. It validates JSON, enforces constraints, detects loops, and manages profiles — before responses come back.

**Tweet 4:**

Benchmark results with Ornith 9B:

• JSON validity: 0% → 100%
• JSON latency: 2,949ms → 352ms (8.4× faster)
• Instruction-following: 78% → 100%
• Terminal-Bench JSON warnings: −91%

Tool call accuracy: 100% maintained. Nothing broke. 📊

**Tweet 5:**

Agent mode: step budgets, loop detection, tool-call validation, context compaction.

Memory engine: zero-VRAM persistent memory (facts, episodes, model-fit) that survives session boundaries.

Agentic Coding Router: auto-selects model per step by task difficulty.

**Tweet 6:**

Quick start:

git clone github.com/EffNine/Gumi
cd Gumi && make build
./gumi start

Point your app at http://127.0.0.1:8787/v1 — API key gumi-local. Works with Ollama, LM Studio, any OpenAI-compatible server. Single Go binary, no extra VRAM.

**Tweet 7:**

17 validated model profiles. Local dashboard + CLI diagnostics. First release: v0.2.0-alpha.

If you're running local models in real apps, give it a try and tell me what breaks.

🔗 https://github.com/EffNine/Gumi

Stars and feedback appreciated. 🙏