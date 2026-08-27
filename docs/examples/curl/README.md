> **Historical — Pre-Pivot Runtime Example (Frozen)**
> This example calls the pre-pivot Gumi runtime's OpenAI-compatible endpoint (`127.0.0.1:8787`). It is **not** the current V1 product. The V1 product is the **local inference auto-tuner** (`gumi tune` / `gumi export`; see `26-gumi-v1-auto-tuner.md`). Retained for provenance — the V1 export equivalent is `gumi export --target lmstudio|ollama|llama.cpp`.

---

# cURL Example

Call Gumi's OpenAI-compatible endpoint with plain HTTP.

## Setup

Start Gumi:

```bash
make build
./gumi start
```

Default URL: `http://127.0.0.1:8787/v1`  
Default auth token: `gumi-local`

## Request

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [
      {"role": "user", "content": "Write a Go function that adds two ints. Code only."}
    ]
  }'
```

## Expected output

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "model": "lmstudio:qwen2.5-coder-7b-instruct",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "func add(a, b int) int { return a + b }"
      }
    }
  ]
}
```

## Troubleshooting

- `connection refused` → Gumi is not running or is bound to a different port. Check `gumi.yaml`.
- `401 Unauthorized` → The bearer token does not match `auth.local_key` in your config.
- `model not found` → Verify the provider prefix (`lmstudio:`, `ollama:`) and that the model is loaded.
