# Novexa Runtime

This directory contains the Novexa Runtime, a local-first intelligence layer
that sits between AI applications and local inference engines.

## Sprint 1: Runtime Skeleton

The runtime currently provides:

- A runnable `novexa` CLI entrypoint.
- `novexa version` — prints the runtime version.
- `novexa start` — loads safe defaults and runs a placeholder loop.
- Graceful shutdown on `Ctrl+C` (SIGINT/SIGTERM).
- A config loader that returns safe local defaults when no config file exists.
- A simple, readable logger.

No providers, dashboard, HTTP API, pipeline engine, or storage are implemented
yet. Those will be added in subsequent sprints.

## Project Layout

```text
runtime/
├── cmd/novexa/main.go        # CLI entrypoint
├── internal/
│   ├── cli/                  # Command parsing and dispatch
│   ├── config/               # Configuration defaults and loader
│   └── logger/               # Leveled logger
├── go.mod
└── README.md
```

## Running

From inside `runtime/`:

```bash
go run ./cmd/novexa version
go run ./cmd/novexa start
```

`go run ./cmd/novexa start` prints a startup banner and runs until it receives
`Ctrl+C`, at which point it shuts down gracefully.

## Testing

```bash
go test ./...
```

## Configuration

Sprint 1 uses hard-coded safe defaults. A future sprint will add YAML config
loading and the search order described in `docs/05-configuration-specification.md`.
