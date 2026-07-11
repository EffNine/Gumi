# Novexa

**Intelligence Runtime for Local AI**

Run any local model like it's a premium AI.

---

## What is Novexa?

Novexa is a local-first AI runtime that sits between AI applications and local
inference engines. It exposes an OpenAI-compatible API and improves local AI
reliability through validation, repair, anti-loop detection, model profiles, and
telemetry — without changing your model or workflow.

Novexa is not a model, a chatbot, or a cloud gateway.

---

## Quick start

1. Download the latest release archive for your platform from the
   [GitHub Releases](https://github.com/EffNine/Novexa/releases) page.
2. Extract the archive:
   ```bash
   tar -xzf novexa-<version>-<os>-<arch>.tar.gz
   ```
3. Run Novexa:
   ```bash
   cd novexa-<version>-<os>-<arch>
   ./novexa start
   ```
4. Point an OpenAI-compatible client at `http://127.0.0.1:8787/v1` with API key
   `novexa-local`.

See [docs/installation.md](./docs/installation.md) for platform-specific
instructions and [docs/quickstart.md](./docs/quickstart.md) for the full
walkthrough.

---

## Integration guides

| Guide | Description |
|-------|-------------|
| [Open WebUI](./docs/integrations/open-webui.md) | Connect Open WebUI through Novexa for local chat. |
| [OpenCode](./docs/integrations/opencode.md) | Connect OpenCode through Novexa lightweight mode. |
| [Continue](./docs/integrations/continue.md) | Connect Continue for chat and inline edits. |
| [Cline](./docs/integrations/cline.md) | Connect Cline for codebase-aware coding. |
| [OpenAI SDK](./docs/integrations/openai-sdk.md) | Use the Python or JavaScript OpenAI SDK with Novexa. |
| [LM Studio](./docs/integrations/lmstudio.md) | Set up LM Studio as the inference backend. |

---

## Supported providers

- Ollama
- LM Studio
- OpenAI-compatible local servers

---

## Alpha limitations

- YAML configuration loading is not implemented yet.
- Streaming responses are not implemented yet.
- CLI `stop` and `restart` commands are not implemented yet.

---

## License

Proprietary. See [LICENSE](./LICENSE).

For commercial licensing inquiries, contact **legal@effnine.com**.
