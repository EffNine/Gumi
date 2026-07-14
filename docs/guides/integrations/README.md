# Integrations

Gumi exposes an OpenAI-compatible API so any client that speaks the OpenAI
chat completions format can connect with three settings:

```
base_url: http://127.0.0.1:8787/v1
api_key: gumi-local
model: lmstudio:qwen2.5-coder-7b-instruct
```

No temperature, `top_p`, `max_tokens`, or `thinking` tuning needed. Gumi
applies the validated model profile, handles repair and validation, and
selects the correct provider automatically.

## Current guides

| Guide | Description |
|-------|-------------|
| [OpenCode](./opencode.md) | Connect OpenCode through Gumi lightweight mode. Recommended for coding agents. |
| [Continue](./continue.md) | Connect Continue for chat, inline edits, and tab autocomplete through Gumi. |
| [Cline](./cline.md) | Connect Cline for file editing, terminal commands, and codebase search through Gumi. |
| [Open WebUI](./open-webui.md) | Connect Open WebUI through Gumi stabilized mode for local chat. |
| [OpenAI SDK](./openai-sdk.md) | Connect any OpenAI-compatible Python, JavaScript, or cURL client through Gumi. |
| [LM Studio](./lmstudio.md) | Set up LM Studio as the local inference backend for Gumi. |

## Integration types

- **OpenAI-compatible clients** — any tool or library that accepts an OpenAI
  base URL and API key can point at Gumi. Works with the OpenAI Python SDK,
  JS SDK, cURL, and similar.
- **LM Studio through Gumi** — LM Studio provides the local inference engine;
  Gumi adds prompt profiles, validation, repair, and structured output on top.
- **Coding agents through Gumi lightweight mode** — optimised for agentic
  coding workloads with minimal prompt overhead and 24 % faster responses.



## Recommended default setup

```jsonc
{
  "base_url": "http://127.0.0.1:8787/v1",
  "api_key": "gumi-local",
  "model": "lmstudio:qwen2.5-coder-7b-instruct"
}
```

Gumi selects lightweight mode by default for agentic clients and stabilized
mode for general chat. The benchmark tools determine which mode each model can
safely use — you only configure the connection.
