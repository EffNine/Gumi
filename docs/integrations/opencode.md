# OpenCode + Novexa

Connect [OpenCode](https://opencode.ai) to a local LLM through
[Novexa](https://novexa.dev) in under 30 seconds. Novexa handles profile tuning,
thinking policy, provider quirks, JSON validation, and prompt optimization so you
do not have to.

**What you get:**

- A coding agent backed by `qwen2.5-coder-7b-instruct` running on your own
  machine via LM Studio.
- No API keys, no cloud credits, no rate limits.
- Novexa lightweight mode is optimised for agentic coding workloads — minimal
  prompt overhead, 24 % faster than Novexa stabilised mode, 70 % fewer prompt
  tokens.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed and running.
- The model loaded: `qwen2.5-coder-7b-instruct`
- [Novexa](https://novexa.dev) built or downloaded
- A terminal with `curl`

## Step 1 — Start Novexa

Run Novexa with LM Studio as the provider:

```bash
NOVEXA_PROVIDER_DEFAULT=lmstudio \
NOVEXA_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
NOVEXA_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
NOVEXA_PROVIDER_TIMEOUT_SECONDS=120 \
./novexa start
```

You should see:

```text
Novexa Runtime 0.1.0

API        http://127.0.0.1:8787/v1
Dashboard  http://127.0.0.1:8788
Mode       stabilized
Provider   lmstudio
Model      qwen2.5-coder-7b-instruct

Status     ready
```

Leave this terminal open. Novexa runs until you press `Ctrl+C`.

**Custom LM Studio URL.** If your LM Studio is on a different host or port,
change `NOVEXA_LMSTUDIO_URL`. Default is `http://localhost:1234/v1`.

## Step 2 — Quick verification

In another terminal, confirm Novexa is reachable:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
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
    "api_key": "novexa-local",
    "model": "lmstudio:qwen2.5-coder-7b-instruct"
  }
}
```

That is it. OpenCode will now send every request through Novexa, which routes
them to LM Studio, applies the validated `qwen2.5-coder-7b` profile, runs repair
and validation on failures, and returns clean, agent-friendly output.

**No parameter tuning needed.** Do not set temperature, `top_p`, `max_tokens`, or
`thinking` in OpenCode. Novexa applies the correct values from the validated
model profile automatically. If you omit them, Novexa uses profile defaults. If
you include them, Novexa respects your values but still runs validation and
repair.

## How Novexa modes work for OpenCode

Novexa offers several runtime modes. The default (`stabilized`) is the most
reliable. For OpenCode, lightweight mode is the best balance of speed and quality.

| Mode | Label | Use with OpenCode |
|------|-------|-------------------|
| Lightweight | `C-NovexaLightweight` | **Recommended for OpenCode.** Minimal prompt, fastest response. Works well for coding agents that do not need strict JSON output. |
| Direct | `B-NovexaDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |
| Stabilized | `D-NovexaStabilized` | Full quality gate. Slower but catches more edge cases. Use if you see failures in lightweight mode. |
| Structured | `E-NovexaStructured` | Strict JSON/schema output. Use when OpenCode must receive valid structured data from the model. |

You do not need to configure the mode in OpenCode. The benchmark and Profile
Doctor tools determine which mode a model can safely use.

## Quick curl test by mode

```bash
# Lightweight (recommended for OpenCode)
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"lmstudio:qwen2.5-coder-7b-instruct","messages":[{"role":"user","content":"hello"}],"novexa_mode":"lightweight"}'

# Stabilised (fallback if lightweight has issues)
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"lmstudio:qwen2.5-coder-7b-instruct","messages":[{"role":"user","content":"hello"}],"novexa_mode":"stabilized"}'
```

## Troubleshooting

### Port 8787 already in use

```bash
# Find the process using port 8787
lsof -i :8787
# Kill it
kill -9 <PID>
# Or start Novexa on a different port
NOVEXA_PROVIDER_DEFAULT=lmstudio NOVEXA_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
  NOVEXA_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
  ./novexa start --port 8790
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
# then restart Novexa.
```

### Empty or reasoning-only responses

The model may be outputting a chain-of-thought reasoning block without a visible
answer. Try:

- Restart Novexa with `NOVEXA_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Use stabilised mode instead of lightweight by omitting `novexa_mode` or setting
  it to `"stabilized"`.
- Run `./novexa doctor` to check provider health and model availability.

### Slow responses

| Cause | Fix |
|-------|-----|
| LM Studio is running on a CPU-only machine | Use a quantised model (e.g., `q4_k_m`) or upgrade to a GPU-backed setup. |
| `max_tokens` is too high | Novexa profiles cap this automatically. If overridden in OpenCode, remove `max_tokens` from your config. |
| Repair loop | Check Novexa dashboard at http://127.0.0.1:8788 for repair events. Switch to stabilised mode if repeated repairs are expected. |
| LM Studio is using a large context window | Reduce `context_length` in LM Studio's model settings. |

## Recommended model choices

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

Each model has a validated Novexa profile in `profiles/`. Profiles set the
correct `max_tokens`, `thinking` policy, prompt instructions, and repair
strategy automatically.
