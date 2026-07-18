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
