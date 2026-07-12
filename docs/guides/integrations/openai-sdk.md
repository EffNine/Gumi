# OpenAI SDK + Novexa

Connect any OpenAI-compatible client to a local LLM through
[Novexa](https://novexa.dev). Novexa exposes the standard OpenAI chat completions
API, so you can use the official Python SDK, the official JavaScript/TypeScript
SDK, cURL, or any library that accepts a `base_url`/`baseURL`, `api_key`/`apiKey`,
and `model`.

**What you get:**

- Drop-in replacement for the OpenAI API running entirely on your local machine
  via LM Studio.
- No API keys, no cloud credits, no rate limits.
- Novexa applies validated model profiles, repair, validation, and thinking
  policy automatically.

## Recommended default connection

```text
base_url: http://127.0.0.1:8787/v1
api_key:  novexa-local
model:    lmstudio:qwen2.5-coder-7b-instruct
```

No temperature, `top_p`, `max_tokens`, or `thinking` tuning needed. Novexa
applies the correct values from the validated model profile automatically.

## Prerequisites

- [LM Studio](https://lmstudio.ai) installed and running.
- The model loaded: `qwen2.5-coder-7b-instruct`
- [Novexa](https://novexa.dev) built or downloaded
- An OpenAI-compatible client installed:
  - `curl` (built-in on most systems)
  - Python: `pip install openai`
  - JavaScript/TypeScript: `npm install openai`

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

## Step 2 — Verify with cURL

List available models:

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

## Step 3 — Python OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/v1",
    api_key="novexa-local",
)

response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[
        {"role": "user", "content": "Write a Python function that reverses a string."}
    ],
)

print(response.choices[0].message.content)
```

## Step 4 — JavaScript/TypeScript OpenAI SDK

```typescript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://127.0.0.1:8787/v1",
  apiKey: "novexa-local",
});

const response = await client.chat.completions.create({
  model: "lmstudio:qwen2.5-coder-7b-instruct",
  messages: [
    { role: "user", content: "Write a TypeScript function that reverses a string." },
  ],
});

console.log(response.choices[0].message.content);
```

## Using Novexa modes

Pass a `novexa` extra body object to select a runtime mode. If you omit it,
Novexa chooses the safest validated mode for the model.

### Lightweight mode

Best for coding-agent style calls. Minimal prompt overhead and fastest
response.

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "hello"}],
    "novexa": { "mode": "lightweight" }
  }'
```

### Stabilized mode

Best for normal chat quality. Full quality gate, repair, and validation.

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "hello"}],
    "novexa": { "mode": "stabilized" }
  }'
```

### Structured mode

Best for JSON/schema output. Request valid JSON with `response_format`.

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "Return a JSON object with keys name and age."}],
    "response_format": { "type": "json_object" },
    "novexa": { "mode": "structured" }
  }'
```

Python and JavaScript clients can pass the same extra fields in the request
body. For example, in Python:

```python
response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[{"role": "user", "content": "hello"}],
    extra_body={"novexa": {"mode": "lightweight"}},
)
```

## How Novexa modes work for OpenAI SDK clients

| Mode | Label | Use with OpenAI SDK clients |
|------|-------|-----------------------------|
| Lightweight | `C-NovexaLightweight` | **Recommended for coding-agent style calls.** Minimal prompt, fastest response. Best when the client does not need strict JSON output. |
| Stabilized | `D-NovexaStabilized` | **Recommended for normal chat quality.** Full quality gate. Slower but catches more edge cases. |
| Structured | `E-NovexaStructured` | Strict JSON/schema output. Use when the client must receive valid structured data from the model. |
| Direct | `B-NovexaDirect` | Diagnostic only. Thin proxy — no repair, no validation, no profile. Use to test whether LM Studio is reachable. |

You do not need to select the mode manually unless you want to override the
validated default. The benchmark and Profile Doctor tools determine which mode
each model can safely use.

## Troubleshooting

### 401 Unauthorized

- Verify the `Authorization` header uses `Bearer novexa-local`.
- Ensure the API key matches the value Novexa expects. Local Novexa defaults to
  `novexa-local`.

### Model not found

- Confirm the model is loaded in LM Studio:
  `curl http://192.168.0.164:1234/v1/models`
- The model ID in your client must match the Novexa model identifier:
  `lmstudio:qwen2.5-coder-7b-instruct`
- Restart Novexa after loading a new model in LM Studio.

### Provider unavailable

- Verify Novexa is running:
  `curl http://127.0.0.1:8787/v1/models -H "Authorization: Bearer novexa-local"`
- Check Novexa logs for the configured provider.
- Run `./novexa doctor` to check provider health.

### LM Studio unreachable

- Test LM Studio directly:
  `curl http://192.168.0.164:1234/v1/models`
- Ensure LM Studio's local API server is enabled (Settings → Local API Server → Enable).
- Check the host and port in `NOVEXA_LMSTUDIO_URL`.
- Verify no firewall is blocking the connection.

### Empty or reasoning-only response

- Restart Novexa with `NOVEXA_PROVIDER_TIMEOUT_SECONDS=180` for longer timeouts.
- Use stabilized mode for normal chat.
- Run `./novexa doctor` to check provider health and model availability.
- Benchmark the model: `./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct`

### Streaming unsupported

Novexa currently returns complete chat completions. If your client enables
streaming (`stream: true`) and the connection hangs or errors, disable
streaming in the client request and consume the full response instead.

```python
response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[{"role": "user", "content": "hello"}],
    stream=False,  # Required while Novexa does not support streaming
)
```

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
