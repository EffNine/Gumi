# LM Studio + Novexa

Configure [LM Studio](https://lmstudio.ai) as the local inference backend for
[Novexa](https://novexa.dev). LM Studio runs the model; Novexa adds profiles,
validation, repair, and an OpenAI-compatible API that any client can use.

## What LM Studio provides

- A desktop app for downloading, loading, and running local LLMs.
- An OpenAI-compatible **Local API Server** at `http://localhost:1234/v1`.
- GPU/CPU inference on your own machine — no cloud API keys, no rate limits.

## What Novexa adds on top

- **Validated profiles** — correct `max_tokens`, `temperature`, `top_p`,
  `thinking` policy, prompt instructions, and repair strategy for each model.
- **Repair and validation** — catches malformed or reasoning-only output and
  re-runs the request.
- **Structured output** — maps OpenAI `response_format` to LM Studio-compatible
  JSON/schema handling.
- **Model aliases** — normalizes model names behind the `lmstudio:` prefix so
  clients use a single identifier regardless of how LM Studio exposes the
  underlying model.
- **Quirk handling** — for example, sends `reasoning_effort: "none"` for
  thinking-disabled profiles and fixes provider-specific parameter mapping.

## Recommended architecture

```text
App / SDK / Tool
    ↓
Novexa  http://127.0.0.1:8787/v1
    ↓
LM Studio  http://localhost:1234/v1  (or your LAN URL)
    ↓
Local model on GPU/CPU
```

Clients connect only to Novexa. Novexa connects to LM Studio. You can run LM
Studio on the same machine as Novexa or on another machine on your LAN.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed.
- A model downloaded and loaded in LM Studio.
- LM Studio's **Local API Server** enabled.
- [Novexa](https://novexa.dev) built or downloaded.

## Recommended models

| Use case | Model identifier | Profile |
|----------|-----------------|---------|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

Each model has a validated Novexa profile in `profiles/`. Profiles set the
correct `max_tokens`, `thinking` policy, prompt instructions, and repair
strategy automatically.

## Step 1 — Set up LM Studio

1. Open LM Studio.
2. Download and load one of the recommended models. For example, search for
   `qwen2.5-coder-7b-instruct` in the model search pane, download it, then load
   it in the chat/inference panel.
3. Start the **Local API Server** from the left sidebar.
   - The default URL is `http://localhost:1234/v1`.
   - Leave CORS enabled unless you have a specific reason to disable it.
4. Confirm the server is responding:

```bash
curl http://localhost:1234/v1/models
```

You should see a JSON list containing the loaded model.

## Step 2 — LAN setup (optional)

If Novexa runs on a different machine than LM Studio, enable the API server to
listen on your LAN IP:

1. In LM Studio, start the Local API Server and note the IP shown in the UI, or
   check your machine's local IP address.
2. Use that IP in the Novexa configuration.

Example LAN URL (replace `192.168.0.164` with your actual LM Studio host IP):

```text
http://192.168.0.164:1234/v1
```

Verify reachability from the Novexa machine:

```bash
curl http://192.168.0.164:1234/v1/models
```

If this fails, check firewalls and that LM Studio is allowed to accept
connections from the local network.

## Step 3 — Start Novexa (localhost)

Run Novexa with LM Studio on the same machine:

```bash
NOVEXA_PROVIDER_DEFAULT=lmstudio \
NOVEXA_LMSTUDIO_URL=http://localhost:1234/v1 \
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

## Step 4 — Start Novexa (LAN example)

If LM Studio is on another machine at `192.168.0.164`:

```bash
NOVEXA_PROVIDER_DEFAULT=lmstudio \
NOVEXA_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
NOVEXA_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
NOVEXA_PROVIDER_TIMEOUT_SECONDS=120 \
./novexa start
```

Replace `192.168.0.164` with your actual LM Studio host IP address.

## Step 5 — Verify Novexa

List models through Novexa:

```bash
curl http://127.0.0.1:8787/v1/models \
  -H "Authorization: Bearer novexa-local"
```

Send a chat completion:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "Write a Python function that reverses a string."}]
  }'
```

Expect an OpenAI-compatible response with generated code.

## LM Studio quirks Novexa handles

| Quirk | What Novexa does |
|-------|------------------|
| `reasoning_effort` | Sends `reasoning_effort: "none"` for profiles that disable chain-of-thought. |
| `response_format` | Maps OpenAI-style JSON/schema requests to LM Studio-compatible parameters. |
| Model aliases | Accepts `lmstudio:qwen2.5-coder-7b-instruct` and resolves it to the loaded LM Studio model. |
| Parameter defaults | Applies profile defaults for `max_tokens`, `temperature`, `top_p`, and other parameters automatically. |

You do not need to tune these parameters in your client. Novexa applies the
validated model profile automatically.

## Troubleshooting

### LM Studio not reachable

- Verify LM Studio is running.
- Verify the Local API Server is enabled.
- Test directly:
  `curl http://localhost:1234/v1/models`
- Check the host and port. The path must include `/v1`.

### Model not loaded

- In LM Studio, load the model in the chat/inference panel.
- Confirm with:
  `curl http://localhost:1234/v1/models`
- Restart Novexa after loading or changing models.

### Wrong URL or missing `/v1`

LM Studio's OpenAI-compatible endpoint requires the `/v1` path:

```text
http://localhost:1234/v1   # correct
http://localhost:1234      # wrong
```

### LAN firewall issue

If `curl http://192.168.0.164:1234/v1/models` from the Novexa machine fails:

- Ensure LM Studio's API server is bound to the LAN interface, not only
  `127.0.0.1`.
- Check operating-system firewalls allow port `1234` on the LM Studio machine.
- Try temporarily disabling the firewall for testing, then re-enable with the
  correct rule.

### Empty or reasoning-only output

- Restart Novexa with `NOVEXA_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Use stabilized mode for normal chat.
- Run `./novexa doctor` to check provider health and model availability.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct`

### Slow response

| Cause | Fix |
|-------|-----|
| LM Studio is running on a CPU-only machine | Use a quantised model (e.g., `q4_k_m`) or upgrade to a GPU-backed setup. |
| Large context window | Reduce `context_length` in LM Studio's model settings. |
| Model too large for available VRAM | Load a smaller model or a lower quantisation. |

### Model name mismatch

- The identifier you pass to clients must use the `lmstudio:` prefix and match
  the validated profile, e.g. `lmstudio:qwen2.5-coder-7b-instruct`.
- The underlying LM Studio model name may differ; Novexa resolves the alias.
