# Continue + Gumi

Connect [Continue](https://continue.dev) to a local LLM through
[Gumi](https://gumi.dev) in under 30 seconds. Gumi handles profile tuning,
thinking policy, provider quirks, JSON validation, and prompt optimization so you
do not have to.

**What you get:**

- A coding assistant backed by `qwen2.5-coder-7b-instruct` running on your own
  machine via LM Studio.
- Chat and inline edits routed through Gumi — all local, no cloud API keys, no
  rate limits. Tab autocomplete can use LM Studio directly until Gumi adds a
  dedicated autocomplete endpoint.
- Gumi lightweight mode is optimised for agentic coding workloads — minimal
  prompt overhead, 24 % faster than Gumi stabilised mode, 70 % fewer prompt
  tokens.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed and running.
- The model loaded: `qwen2.5-coder-7b-instruct`
- [Gumi](https://gumi.dev) built or downloaded
- [Continue](https://continue.dev) VS Code or JetBrains extension installed

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

## Step 2 — Configure Continue

Open Continue's config file:

- **VS Code:** `~/.continue/config.json`
- **JetBrains:** `~/.continue/config.json`

Add a model entry pointing at Gumi:

```jsonc
{
  "models": [
    {
      "title": "Gumi (qwen2.5-coder-7b)",
      "provider": "openai",
      "apiBase": "http://127.0.0.1:8787/v1",
      "apiKey": "gumi-local",
      "model": "lmstudio:qwen2.5-coder-7b-instruct"
    }
  ]
}
```

That is it. Continue will now send chat and inline-edit requests through
Gumi, which routes them to LM Studio, applies the validated
`qwen2.5-coder-7b` profile, runs repair and validation on failures, and returns
clean, agent-friendly output.

**No parameter tuning needed.** Do not set `temperature`, `topP`, `maxTokens`,
or `stop` in Continue. Gumi applies the correct values from the validated
model profile automatically.

## Step 3 — Quick verification

Open Continue in VS Code or JetBrains and send a chat message:

```
Write a Go function that reverses a string.
```

Expect a clean code response with no preamble or reasoning blocks.

## How Gumi modes work for Continue

| Mode | Label | Use with Continue |
|------|-------|-------------------|
| Lightweight | `C-GumiLightweight` | **Recommended for Continue.** Minimal prompt, fastest response. Works well for tab autocomplete and inline edits. |
| Direct | `B-GumiDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |
| Stabilized | `D-GumiStabilized` | Full quality gate. Slower but catches more edge cases. Use if you see failures in lightweight mode. |
| Structured | `E-GumiStructured` | Strict JSON/schema output. Use when Continue must receive valid structured data from the model. |

You do not need to configure the mode in Continue. The benchmark and Profile
Doctor tools determine which mode a model can safely use.

## Troubleshooting

### Continue cannot connect

- Verify Gumi is running:
  `curl http://127.0.0.1:8787/v1/models -H "Authorization: Bearer gumi-local"`
- Check `apiBase` in `config.json` — must end with `/v1`.
- Check `apiKey` matches `gumi-local`.

### Empty or slow responses

- Restart Gumi with `GUMI_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Run `./gumi doctor` to check provider health.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct`

### Tab autocomplete not working

Continue's tab autocomplete requires a separate model endpoint. Gumi does not
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

Chat and inline edits still route through Gumi for quality and repair.

## Recommended model choices

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |
