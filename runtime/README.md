# Novexa Runtime

This directory contains the Novexa Runtime, a local-first intelligence layer
that sits between AI applications and local inference engines.

## Sprint 2: Gateway API

The runtime now exposes an OpenAI-compatible local HTTP gateway:

- `GET /health` — runtime health.
- `GET /v1/models` — static model list.
- `POST /v1/chat/completions` — placeholder chat response.
- Bearer-token auth with the local key `novexa-local`.
- Request-ID tracking via `X-Request-ID`.
- Standard Novexa JSON error format.
- Graceful shutdown on `Ctrl+C` (SIGINT/SIGTERM).

Real provider calls, streaming, the Pipeline Engine, telemetry, and the
dashboard will be added in subsequent sprints.

## Project Layout

```text
runtime/
├── cmd/novexa/main.go        # CLI entrypoint
├── internal/
│   ├── api/                  # OpenAI-compatible request/response types
│   ├── cli/                  # Command parsing and dispatch
│   ├── config/               # Configuration defaults and loader
│   ├── gateway/              # HTTP server, routes, middleware, handlers
│   └── logger/               # Leveled logger
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
curl http://localhost:8787/health

curl http://localhost:8787/v1/models

curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}'
```
