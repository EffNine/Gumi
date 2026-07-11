# Open WebUI + Novexa

Connect [Open WebUI](https://openwebui.com) to a local LLM through
[Novexa](https://novexa.dev). Novexa handles profile tuning, thinking policy,
provider quirks, JSON validation, and prompt optimization so you do not have to.

**What you get:**

- A local chat interface backed by models running on your own machine via LM Studio.
- No API keys, no cloud credits, no rate limits.
- Novexa stabilized mode is the best default for normal chat quality — full
  quality gating, repair, and validation.

## When to use Novexa instead of connecting Open WebUI directly to LM Studio

Open WebUI can connect directly to LM Studio's OpenAI-compatible endpoint.
Routing through Novexa is worthwhile when you want:

- **Validated profiles** — correct `max_tokens`, `thinking` policy, prompt
  instructions, and repair strategy applied automatically for each model.
- **Repair and validation** — Novexa catches malformed or reasoning-only output
  and re-runs the request instead of showing a broken response.
- **Structured output** — strict JSON/schema mode for tools or pipelines that
  consume model responses programmatically.
- **Consistent behavior** across different models and providers without
  per-model tuning inside Open WebUI.

For normal casual chat, Novexa stabilized mode improves answer quality and
reliability. For fast, lightweight conversations, keep the default stabilized
mode or use a smaller model such as `qwen/qwen3-1.7b`.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed and running.
- [Novexa](https://novexa.dev) built or downloaded.
- [Open WebUI](https://openwebui.com) installed and running.
- At least one validated model loaded in LM Studio.

## Step 1 — Start Novexa

Run Novexa with LM Studio as the provider and a general-chat model as the default:

```bash
NOVEXA_PROVIDER_DEFAULT=lmstudio \
NOVEXA_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
NOVEXA_DEFAULT_MODEL=qwen/qwen3-1.7b \
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
Model      qwen/qwen3-1.7b

Status     ready
```

Leave this terminal open. Novexa runs until you press `Ctrl+C`.

**Custom LM Studio URL.** If your LM Studio is on a different host or port,
change `NOVEXA_LMSTUDIO_URL`. Default is `http://localhost:1234/v1`.

## Step 2 — Configure Open WebUI

Open the Open WebUI **Admin Panel** → **Settings** → **Connections** → **OpenAI API**.

| Setting | Value |
|---------|-------|
| API Base URL | `http://127.0.0.1:8787/v1` |
| API Key | `novexa-local` |
| Model | `lmstudio:qwen/qwen3-1.7b` |

Save the connection. Open WebUI only needs the base URL, API key, and model ID.
Novexa applies profile tuning, repair, and validation automatically.

## Step 3 — Quick verification

In another terminal, confirm Novexa is reachable and the model is listed:

```bash
curl http://127.0.0.1:8787/v1/models \
  -H "Authorization: Bearer novexa-local"
```

Then send a chat completion:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen/qwen3-1.7b",
    "messages": [{"role": "user", "content": "What is the capital of France?"}]
  }'
```

Expect an OpenAI-compatible response with a clean answer and no visible
reasoning blocks.

## How Novexa modes work for Open WebUI

| Mode | Label | Use with Open WebUI |
|------|-------|---------------------|
| Stabilized | `D-NovexaStabilized` | **Recommended default for normal chat.** Full quality gate, repair, and validation. Best balance of quality and reliability in a chat UI. |
| Structured | `E-NovexaStructured` | Strict JSON/schema output. Use when Open WebUI pipelines, tools, or functions require valid structured data from the model. |
| Direct | `B-NovexaDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |
| Lightweight | `C-NovexaLightweight` | Minimal prompt, fastest response. Better suited to coding-agent clients than a normal chat UI; use stabilized mode for general chat instead. |

You do not need to configure the mode in Open WebUI. The benchmark and Profile
Doctor tools determine which mode a model can safely use.

## Troubleshooting

### Open WebUI cannot connect

- Verify Novexa is running:
  `curl http://127.0.0.1:8787/v1/models -H "Authorization: Bearer novexa-local"`
- Check **API Base URL** in Open WebUI — must end with `/v1`.
- Check **API Key** matches `novexa-local`.
- If Open WebUI runs inside Docker, `127.0.0.1` from the container is not the host.
  Use the host's reachable IP address or `host.docker.internal`, e.g.
  `http://host.docker.internal:8787/v1`.

### Model not listed

- Confirm the model is loaded in LM Studio:
  `curl http://192.168.0.164:1234/v1/models`
- The model ID in Open WebUI must match the Novexa model identifier, including the
  `lmstudio:` prefix:
  `lmstudio:qwen/qwen3-1.7b`
- Restart Novexa after loading a new model in LM Studio.

### LM Studio unreachable

- Test LM Studio directly:
  `curl http://192.168.0.164:1234/v1/models`
- Ensure LM Studio's local API server is enabled (Settings → Local API Server → Enable).
- Check the host and port in `NOVEXA_LMSTUDIO_URL`.
- Verify no firewall is blocking the connection.

### Novexa port already in use

```bash
# Find the process using port 8787
lsof -i :8787
# Kill it
kill -9 <PID>
# Or start Novexa on a different port
NOVEXA_PROVIDER_DEFAULT=lmstudio NOVEXA_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
  NOVEXA_DEFAULT_MODEL=qwen/qwen3-1.7b \
  ./novexa start --port 8790
```

### Slow response

| Cause | Fix |
|-------|-----|
| LM Studio is running on a CPU-only machine | Use a quantised model (e.g., `q4_k_m`) or upgrade to a GPU-backed setup. |
| Large context window | Reduce `context_length` in LM Studio's model settings. |
| Repair loop | Check Novexa dashboard at http://127.0.0.1:8788 for repair events. Switch to stabilized mode if repeated repairs are expected. |
| Timeout too short | Restart Novexa with `NOVEXA_PROVIDER_TIMEOUT_SECONDS=180`. |

### Empty or reasoning-only response

- Restart Novexa with `NOVEXA_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Run `./novexa doctor` to check provider health and model availability.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen/qwen3-1.7b`
- Use stabilized mode for normal chat; lightweight mode is designed for coding
  agents and may leave reasoning blocks visible in a chat UI.

## Recommended model choices

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

Each model has a validated Novexa profile in `profiles/`. Profiles set the
correct `max_tokens`, `thinking` policy, prompt instructions, and repair
strategy automatically.
