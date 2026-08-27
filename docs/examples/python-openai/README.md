> **Historical — Pre-Pivot Runtime Example (Frozen)**
> This example calls the pre-pivot Gumi runtime's OpenAI-compatible endpoint (`127.0.0.1:8787`). It is **not** the current V1 product. The V1 product is the **local inference auto-tuner** (`gumi tune` / `gumi export`; see `26-gumi-v1-auto-tuner.md`). Retained for provenance — the V1 export equivalent is `gumi export --target lmstudio|ollama|llama.cpp`.

---

# Python + OpenAI Example

Use the official OpenAI Python client with Gumi's local OpenAI-compatible endpoint.

## Setup

Install the client and start Gumi:

```bash
pip install openai
make build
./gumi start
```

## Request

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/v1",
    api_key="gumi-local",
)

response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[
        {"role": "user", "content": "Write a tiny TypeScript add function."}
    ],
)

print(response.choices[0].message.content)
```

## Expected output

```text
function add(a: number, b: number): number {
    return a + b;
}
```

## Streaming

```python
stream = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[{"role": "user", "content": "Count to 5"}],
    stream=True,
)
for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="")
```

## Troubleshooting

- `ConnectionError` → Gumi is not running or bound to a different host/port.
- `AuthenticationError` → The API key does not match `auth.local_key`.
- `NotFoundError` → Check the model ID and provider prefix.
