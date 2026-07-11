# Quickstart

This quickstart assumes you are using Ollama. It takes about five minutes and
covers installing Novexa, pulling a local model, and sending your first request.

## 1. Install or start Ollama

Install Ollama for your platform:

```bash
# macOS / Linux
curl -fsSL https://ollama.com/install.sh | sh
```

Or download Ollama from [ollama.com](https://ollama.com).

Verify Ollama is running:

```bash
ollama --version
```

If it is not running, start it:

```bash
ollama serve
```

## 2. Pull a local model

Pull a small general-purpose model. This example uses `qwen3:8b`, but any
Ollama model works.

```bash
ollama pull qwen3:8b
```

Verify the model is installed:

```bash
ollama list
```

## 3. Start Novexa

If you extracted a release archive, run:

```bash
./novexa start
```

You should see:

```text
Novexa Runtime 0.1.0

API        http://127.0.0.1:8787/v1
Dashboard  http://127.0.0.1:8788
Mode       stabilized
Provider   ollama
Model      local:auto

Status     ready
```

Leave this terminal open. Novexa runs until you press `Ctrl+C`.

## 4. Open the dashboard

Open http://127.0.0.1:8788 in your browser. The dashboard shows:

- runtime status
- provider health
- recent request metadata (no full prompts or responses by default)
- a `doctor` view with diagnostic checks

## 5. Call the chat completions endpoint

In another terminal, run:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local:auto",
    "messages": [{"role": "user", "content": "Explain local AI runtimes in one sentence."}]
  }'
```

Novexa selects an available local provider and model automatically, then returns
an OpenAI-compatible response.

To request a specific model, use the `provider:model` form:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama:qwen3:8b",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 6. Run `novexa doctor` if something fails

If a request does not work, run diagnostics in another terminal:

```bash
./novexa doctor
```

Example output when Ollama is reachable but the default model is missing:

```text
Novexa Doctor

Runtime Config        ok
Telemetry Storage     ok
Provider ollama       ok
Provider lmstudio     offline
Provider openai_compatible_local offline

Warnings:
- Default model local:auto may resolve to a model that is not installed.

Suggestion:
- Run: ollama pull qwen3:8b
- Or start Novexa with: ./novexa start --model qwen3:8b
```

Common fixes are in the [troubleshooting guide](./troubleshooting.md).

## Next steps

- Read the [installation guide](./installation.md) for Docker and release-archive options.
- Try the CLI commands: `status`, `providers`, `models`, `config show`.
