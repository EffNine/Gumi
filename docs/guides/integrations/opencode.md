# OpenCode + Gumi

Connect [OpenCode](https://opencode.ai) to a local LLM through
[Gumi](https://gumi.dev) in under 30 seconds. Gumi handles profile tuning,
thinking policy, provider quirks, JSON validation, and prompt optimization so you
do not have to.

**What you get:**

- A coding agent backed by `qwen2.5-coder-7b-instruct` running on your own
  machine via LM Studio.
- No API keys, no cloud credits, no rate limits.
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
- A terminal with `curl`

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

**Custom LM Studio URL.** If your LM Studio is on a different host or port,
change `GUMI_LMSTUDIO_URL`. Default is `http://localhost:1234/v1`.

## Step 2 — Quick verification

In another terminal, confirm Gumi is reachable:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "Write \"hello world\" in Rust."}]
  }'
```

Expect an OpenAI-compatible response with generated Rust code.

## Step 3 — Configure OpenCode

Add the following to your OpenCode configuration (`opencode.json` or
`opencode.jsonc`):

```jsonc
{
  "model": {
    "provider": "openai",
    "base_url": "http://127.0.0.1:8787/v1",
    "api_key": "gumi-local",
    "model": "lmstudio:qwen2.5-coder-7b-instruct"
  }
}
```

That is it. OpenCode will now send every request through Gumi, which routes
them to LM Studio, applies the validated `qwen2.5-coder-7b` profile, runs repair
and validation on failures, and returns clean, agent-friendly output.

**No parameter tuning needed.** Do not set temperature, `top_p`, `max_tokens`, or
`thinking` in OpenCode. Gumi applies the correct values from the validated
model profile automatically. If you omit them, Gumi uses profile defaults. If
you include them, Gumi respects your values but still runs validation and
repair.

## How Gumi modes work for OpenCode

Gumi offers several runtime modes. The default (`stabilized`) is the most
reliable. For OpenCode, lightweight mode is the best balance of speed and quality.

| Mode | Label | Use with OpenCode |
|------|-------|-------------------|
| Lightweight | `C-GumiLightweight` | **Recommended for OpenCode.** Minimal prompt, fastest response. Works well for coding agents that do not need strict JSON output. |
| Direct | `B-GumiDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |
| Stabilized | `D-GumiStabilized` | Full quality gate. Slower but catches more edge cases. Use if you see failures in lightweight mode. |
| Structured | `E-GumiStructured` | Strict JSON/schema output. Use when OpenCode must receive valid structured data from the model. |

You do not need to configure the mode in OpenCode. The benchmark and Profile
Doctor tools determine which mode a model can safely use.

## Quick curl test by mode

```bash
# Lightweight (recommended for OpenCode)
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"lmstudio:qwen2.5-coder-7b-instruct","messages":[{"role":"user","content":"hello"}],"gumi_mode":"lightweight"}'

# Stabilised (fallback if lightweight has issues)
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"lmstudio:qwen2.5-coder-7b-instruct","messages":[{"role":"user","content":"hello"}],"gumi_mode":"stabilized"}'
```

## Troubleshooting

### Port 8787 already in use

```bash
# Find the process using port 8787
lsof -i :8787
# Kill it
kill -9 <PID>
# Or start Gumi on a different port
GUMI_PROVIDER_DEFAULT=lmstudio GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
  GUMI_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
  ./gumi start --port 8790
```

### LM Studio not reachable

```bash
# Test LM Studio directly
curl http://192.168.0.164:1234/v1/models
```

If this fails, check:
- LM Studio is running and the local API server is enabled (Settings → Local
  API Server → Enable).
- The URL (host, port, `/v1` path) is correct.
- No firewall is blocking the connection.

### Model not found

```bash
# List loaded models in LM Studio
curl http://192.168.0.164:1234/v1/models

# If qwen2.5-coder-7b-instruct is not listed, load it in LM Studio's GUI,
# then restart Gumi.
```

### Empty or reasoning-only responses

The model may be outputting a chain-of-thought reasoning block without a visible
answer. Try:

- Restart Gumi with `GUMI_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Use stabilised mode instead of lightweight by omitting `gumi_mode` or setting
  it to `"stabilized"`.
- Run `./gumi doctor` to check provider health and model availability.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct`

### Slow responses

| Cause | Fix |
|-------|-----|
| LM Studio is running on a CPU-only machine | Use a quantised model (e.g., `q4_k_m`) or upgrade to a GPU-backed setup. |
| `max_tokens` is too high | Gumi profiles cap this automatically. If overridden in OpenCode, remove `max_tokens` from your config. |
| Repair loop | Check Gumi dashboard at http://127.0.0.1:8788 for repair events. Switch to stabilised mode if repeated repairs are expected. |
| LM Studio is using a large context window | Reduce `context_length` in LM Studio's model settings. |

## Recommended model choices

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

Each model has a validated Gumi profile in `profiles/`. Profiles set the
correct `max_tokens`, `thinking` policy, prompt instructions, and repair
strategy automatically.
