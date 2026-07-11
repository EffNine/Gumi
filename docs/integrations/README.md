# Integrations

Novexa exposes an OpenAI-compatible API so any client that speaks the OpenAI
chat completions format can connect with three settings:

```
base_url: http://127.0.0.1:8787/v1
api_key: novexa-local
model: lmstudio:qwen2.5-coder-7b-instruct
```

No temperature, `top_p`, `max_tokens`, or `thinking` tuning needed. Novexa
applies the validated model profile, handles repair and validation, and
selects the correct provider automatically.

## Current guides

| Guide | Description |
|-------|-------------|
| [OpenCode](./opencode.md) | Connect OpenCode through Novexa lightweight mode. Recommended for coding agents. |
| [Continue](./continue.md) | Connect Continue for chat, inline edits, and tab autocomplete through Novexa. |
| [Cline](./cline.md) | Connect Cline for file editing, terminal commands, and codebase search through Novexa. |

## Integration types

- **OpenAI-compatible clients** — any tool or library that accepts an OpenAI
  base URL and API key can point at Novexa. Works with the OpenAI Python SDK,
  JS SDK, cURL, and similar.
- **LM Studio through Novexa** — LM Studio provides the local inference engine;
  Novexa adds prompt profiles, validation, repair, and structured output on top.
- **Coding agents through Novexa lightweight mode** — optimised for agentic
  coding workloads with minimal prompt overhead and 24 % faster responses.

## Planned guides

- [ ] Open WebUI
- [ ] Generic OpenAI SDK clients (Python, JavaScript)
- [ ] LM Studio setup guide

## Recommended default setup

```jsonc
{
  "base_url": "http://127.0.0.1:8787/v1",
  "api_key": "novexa-local",
  "model": "lmstudio:qwen2.5-coder-7b-instruct"
}
```

Novexa selects lightweight mode by default for agentic clients and stabilized
mode for general chat. The benchmark tools determine which mode each model can
safely use — you only configure the connection.
