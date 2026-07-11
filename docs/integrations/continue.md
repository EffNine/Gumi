# Continue + Novexa

Connect [Continue](https://continue.dev) to a local LLM through
[Novexa](https://novexa.dev) in under 30 seconds. Novexa handles profile tuning,
thinking policy, provider quirks, JSON validation, and prompt optimization so you
do not have to.

**What you get:**

- A coding assistant backed by `qwen2.5-coder-7b-instruct` running on your own
  machine via LM Studio.
- Tab autocomplete, inline edits, and chat — all local, no API keys, no rate
  limits.
- Novexa lightweight mode is optimised for agentic coding workloads — minimal
  prompt overhead, 24 % faster than Novexa stabilised mode, 70 % fewer prompt
  tokens.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed and running.
- The model loaded: `qwen2.5-coder-7b-instruct`
- [Novexa](https://novexa.dev) built or downloaded
- [Continue](https://continue.dev) VS Code or JetBrains extension installed

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

## Step 2 — Configure Continue

Open Continue's config file:

- **VS Code:** `~/.continue/config.json`
- **JetBrains:** `~/.continue/config.json`

Add a model entry pointing at Novexa:

```jsonc
{
  "models": [
    {
      "title": "Novexa (qwen2.5-coder-7b)",
      "provider": "openai",
      "apiBase": "http://127.0.0.1:8787/v1",
      "apiKey": "novexa-local",
      "model": "lmstudio:qwen2.5-coder-7b-instruct"
    }
  ]
}
```

That is it. Continue will now send every request through Novexa, which routes
them to LM Studio, applies the validated `qwen2.5-coder-7b` profile, runs repair
and validation on failures, and returns clean, agent-friendly output.

**No parameter tuning needed.** Do not set `temperature`, `topP`, `maxTokens`,
or `stop` in Continue. Novexa applies the correct values from the validated
model profile automatically.

## Step 3 — Quick verification

Open Continue in VS Code or JetBrains and send a chat message:

```
Write a Go function that reverses a string.
```

Expect a clean code response with no preamble or reasoning blocks.

## How Novexa modes work for Continue

| Mode | Label | Use with Continue |
|------|-------|-------------------|
| Lightweight | `C-NovexaLightweight` | **Recommended for Continue.** Minimal prompt, fastest response. Works well for tab autocomplete and inline edits. |
| Direct | `B-NovexaDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |
| Stabilized | `D-NovexaStabilized` | Full quality gate. Slower but catches more edge cases. Use if you see failures in lightweight mode. |
| Structured | `E-NovexaStructured` | Strict JSON/schema output. Use when Continue must receive valid structured data from the model. |

You do not need to configure the mode in Continue. The benchmark and Profile
Doctor tools determine which mode a model can safely use.

## Troubleshooting

### Continue cannot connect

- Verify Novexa is running: `curl http://127.0.0.1:8787/v1/models`
- Check `apiBase` in `config.json` — must end with `/v1`.
- Check `apiKey` matches `novexa-local`.

### Empty or slow responses

- Restart Novexa with `NOVEXA_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Run `./novexa doctor` to check provider health.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct`

### Tab autocomplete not working

Continue's tab autocomplete requires a separate model endpoint. Novexa does not
yet provide a dedicated autocomplete endpoint. For autocomplete, configure
Continue to use LM Studio directly:

```jsonc
{
  "tabAutocompleteModel": {
    "title": "LM Studio (qwen2.5-coder-7b)",
    "provider": "openai",
    "apiBase": "http://192.168.0.164:1234/v1",
    "apiKey": "not-needed",
    "model": "qwen2.5-coder-7b-instruct"
  }
}
```

Chat and inline edits still route through Novexa for quality and repair.

## Recommended model choices

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |
