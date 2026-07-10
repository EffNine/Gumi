# Novexa Runtime

This directory contains the Novexa Runtime, a local-first intelligence layer
that sits between AI applications and local inference engines.

## Sprint 3: Provider Adapters

The runtime now connects to local inference providers through thin adapters:

- `GET /health` — runtime health (independent of provider state).
- `GET /v1/models` — merged list of `local:auto` and discovered provider models.
- `POST /v1/chat/completions` — delegates to Ollama, LM Studio, or an
  OpenAI-compatible local server; falls back to a placeholder only for
  `local:auto` when no provider is available.
- Supported local providers: `ollama`, `lmstudio`, `openai-compatible-local`.
- Provider health checks, model discovery, and normalized provider errors.
- Provider timeout handling with a 60-second default.
- Bearer-token auth with the local key `novexa-local`.
- Request-ID tracking via `X-Request-ID`.
- Standard Novexa JSON error format.
- Graceful shutdown on `Ctrl+C` (SIGINT/SIGTERM).

Streaming, the Pipeline Engine, Context/Prompt/Validation engines, telemetry,
and the dashboard will be added in subsequent sprints.

## Project Layout

```text
runtime/
├── cmd/novexa/main.go        # CLI entrypoint
├── internal/
│   ├── api/                  # OpenAI-compatible request/response types
│   ├── cli/                  # Command parsing and dispatch
│   ├── config/               # Configuration defaults and loader
│   ├── gateway/              # HTTP server, routes, middleware, handlers
│   ├── logger/               # Leveled logger
│   └── provider/             # Provider adapters and manager
├── go.mod
└── README.md
```

## Running

From inside `runtime/`:

```bash
go run ./cmd/novexa version
go run ./cmd/novexa start
go run ./cmd/novexa start --port 8787
```

`go run ./cmd/novexa start` starts the HTTP gateway and runs until it receives
`Ctrl+C`, at which point it shuts down gracefully.

## Testing

```bash
go test ./...
```

## Configuration

Sprint 2 continues to use hard-coded safe defaults. YAML config loading will be
added in a future sprint as described in `docs/05-configuration-specification.md`.

Default local API:

```text
http://127.0.0.1:8787/v1
```

Example usage:

```bash
# Runtime health (no auth required)
curl http://localhost:8787/health

# Discovered models (auth required)
curl http://localhost:8787/v1/models \
  -H "Authorization: Bearer novexa-local"

# Auto-select an available local provider/model
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}'

# Request a specific provider/model when available
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama:llama3","messages":[{"role":"user","content":"Hello"}]}'

# Provider unavailable error for an explicit model request
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama:not-a-real-model","messages":[{"role":"user","content":"Hello"}]}'
```
