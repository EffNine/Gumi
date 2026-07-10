# Novexa Runtime

This directory contains the Novexa Runtime, a local-first intelligence layer
that sits between AI applications and local inference engines.

## Sprint 10: Packaging and Release

The runtime now ships as an installable 0.1.0 alpha release. It connects to
local inference providers through thin adapters and includes:

- `GET /health` — runtime health (independent of provider state).
- `GET /v1/models` — merged list of `local:auto` and discovered provider models.
- `POST /v1/chat/completions` — delegates to Ollama, LM Studio, or an
  OpenAI-compatible local server.
- Supported local providers: `ollama`, `lmstudio`, `openai-compatible-local`.
- Provider health checks, model discovery, and normalized provider errors.
- Provider timeout handling with a 60-second default.
- Bearer-token auth with the local key `novexa-local`.
- Request-ID tracking via `X-Request-ID`.
- Standard Novexa JSON error format.
- Graceful shutdown on `Ctrl+C` (SIGINT/SIGTERM).

The runtime also includes the intelligence pipeline, local telemetry, model
profiles, diagnostics CLI, and a privacy-first local dashboard.

### Building a release binary

From the repository root:

```bash
make build
```

This rebuilds `dashboard/dist` and produces a `novexa` binary with release
metadata injected via ldflags.

### Cross-platform release archives

From the repository root:

```bash
make release
make check-release
```

The archives are written to `dist/releases/` and include the binary, dashboard,
starter profiles, README, LICENSE, CHANGELOG, and example config.

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
go run ./cmd/novexa status
go run ./cmd/novexa doctor
go run ./cmd/novexa providers
go run ./cmd/novexa models
go run ./cmd/novexa config show
go run ./cmd/novexa benchmark
```

`go run ./cmd/novexa start` starts the HTTP gateway and runs until it receives
`Ctrl+C`, at which point it shuts down gracefully.

The API is served at `http://127.0.0.1:8787/v1`. When `dashboard/dist` has
been built, the dashboard is served at `http://127.0.0.1:8788`.

## Testing

```bash
go test ./...
```

## Configuration

The 0.1.0 alpha release continues to use hard-coded safe defaults. YAML config
loading will be added in a release after the alpha. `novexa.example.yaml` at the
repository root documents the planned configuration format.

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
