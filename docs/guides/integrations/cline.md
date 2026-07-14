# Cline + Gumi

Connect [Cline](https://github.com/cline/cline) to a local LLM through
[Gumi](https://gumi.dev) in under 30 seconds. Gumi handles profile tuning,
thinking policy, provider quirks, JSON validation, and prompt optimization so you
do not have to.

**What you get:**

- A coding agent backed by `qwen2.5-coder-7b-instruct` running on your own
  machine via LM Studio.
- Cline creates and edits files, runs terminal commands, and searches your
  codebase — all local, no cloud API keys, no rate limits.
- Gumi lightweight mode is optimised for agentic coding workloads — minimal
  prompt overhead, 24 % faster than Gumi stabilised mode, 70 % fewer prompt
  tokens.
- Gumi agent mode (v0.2.0+) provides stricter governance for coding agents:
  step budget enforcement, tool-call loop detection, tool-call JSON validation,
  and context compaction hints. Use `"gumi":{"mode":"agent"}` in your request
  to enable it. Lightweight mode remains the recommended default for most
  agentic clients.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed and running.
- The model loaded: `qwen2.5-coder-7b-instruct`
- [Gumi](https://gumi.dev) built or downloaded
- [Cline](https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev) VS Code extension installed

## Step 1 — Start Gumi

Run Gumi with LM Studio as the provider:

```bash
GUMI_PROVIDER_DEFAULT=lmstudio \
GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
GUMI_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
GUMI_PROVIDER_TIMEOUT_SECONDS=120 \
./gumi start
```

You should see:

```text
Gumi Runtime 0.1.0

API        http://127.0.0.1:8787/v1
Dashboard  http://127.0.0.1:8788
Mode       stabilized
Provider   lmstudio
Model      qwen2.5-coder-7b-instruct

Status     ready
```

Leave this terminal open. Gumi runs until you press `Ctrl+C`.

## Step 2 — Configure Cline

Open Cline settings in VS Code (the Cline extension settings panel):

| Setting | Value |
|---------|-------|
| API Provider | `OpenAI Compatible` |
| Base URL | `http://127.0.0.1:8787/v1` |
| API Key | `gumi-local` |
| Model ID | `lmstudio:qwen2.5-coder-7b-instruct` |

That is it. Cline will now send OpenAI-compatible chat requests through Gumi,
which routes them to LM Studio, applies the validated `qwen2.5-coder-7b`
profile, runs repair and validation on failures, and returns clean,
agent-friendly output.

**No parameter tuning needed.** Do not set `temperature`, `top_p`, `max_tokens`,
or `thinking` inside Cline. Gumi applies the correct values from the validated
model profile automatically. Cline sees a normal OpenAI-compatible API and does
not need to know about LM Studio, reasoning_effort, or JSON format mapping.

## Step 3 — Quick verification

Open Cline in VS Code and send a message:

```
Create a small TypeScript function that validates an email address. Return code only.
```

Expect a clean code response with no preamble or reasoning blocks.

## How Gumi modes work for Cline

| Mode | Label | Use with Cline |
|------|-------|----------------|
| Lightweight | `C-GumiLightweight` | **Recommended for Cline.** Minimal prompt, fastest response. Works well for file editing and codebase search. |
| Direct | `B-GumiDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |
| Stabilized | `D-GumiStabilized` | Full quality gate. Slower but catches more edge cases. Use if you see failures in lightweight mode. |
| Structured | `E-GumiStructured` | Strict JSON/schema output. Use when Cline must receive valid structured data from the model. |

You do not need to configure the mode in Cline. The benchmark and Profile
Doctor tools determine which mode a model can safely use.

## Troubleshooting

### Cline cannot connect

- Verify Gumi is running:
  `curl http://127.0.0.1:8787/v1/models -H "Authorization: Bearer gumi-local"`
- Check **Base URL** in Cline settings — must end with `/v1`.
- Check **API Key** matches `gumi-local`.

### Model not found

- Confirm the model is loaded in LM Studio:
  `curl http://192.168.0.164:1234/v1/models`
- The model ID in Cline must match the Gumi model identifier:
  `lmstudio:qwen2.5-coder-7b-instruct`

### LM Studio unreachable

- Test LM Studio directly:
  `curl http://192.168.0.164:1234/v1/models`
- Ensure LM Studio's local API server is enabled (Settings → Local API Server → Enable).

### Port 8787 already in use

```bash
lsof -i :8787
kill -9 <PID>
# Or start on a different port
GUMI_PROVIDER_DEFAULT=lmstudio GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
  GUMI_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
  ./gumi start --port 8790
```

### Slow responses

| Cause | Fix |
|-------|-----|
| LM Studio on CPU only | Use a quantised model (e.g., `q4_k_m`) or GPU-backed setup. |
| `max_tokens` too high | Remove `max_tokens` from Cline settings — Gumi profiles cap it. |
| Large context window | Reduce `context_length` in LM Studio's model settings. |

### Empty or reasoning-only output

- Restart Gumi with `GUMI_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Run `./gumi doctor` to check provider health.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct`

## Recommended model choices

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |
