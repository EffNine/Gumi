# Contributing to Gumi

Thank you for your interest in contributing to Gumi.

Gumi is a local-first AI runtime designed to make local models more stable, observable, and production-ready.

---

## Core Rules

Before contributing, please understand these rules:

1. Keep Gumi local-first.
2. Do not add cloud providers to V1.
3. Do not add billing or hosted accounts to V1.
4. Do not bypass the Pipeline Engine.
5. Keep provider adapters thin.
6. Do not store full prompts by default.
7. Do not store full responses by default.
8. Do not send external telemetry by default.
9. Keep the runtime modular.
10. Update documentation when behaviour changes.

---

## Architecture Source of Truth

Read the files in `docs/` before making major changes.

Important files:

```text
docs/specs/00-vision-and-positioning.md
docs/specs/02-runtime-architecture.md
docs/specs/04-api-specification.md
docs/specs/07-pipeline-specification.md
docs/specs/14-implementation-roadmap.md
```

---

## Testing

Gumi uses a layered test strategy. Every PR runs the full suite in CI.

### Test Matrix

| Layer | Command | CI Job | Platforms |
|-------|---------|--------|-----------|
| Lint & format | `gofmt -w .` | `runtime` | ubuntu-latest |
| Unit tests | `go test ./...` | `runtime` | ubuntu-latest |
| Static analysis | `go vet ./...` | `runtime` | ubuntu-latest |
| Build | `go build ./cmd/gumi` | `runtime` | ubuntu-latest |
| Integration | `go test -run "TestIntegration|TestSelfTuningIntegration|TestTune_" ./...` | `integration` | ubuntu + macOS |
| Dashboard | `npm run build` | `dashboard` | ubuntu-latest |

The `integration` job runs **after** the `runtime` job succeeds, ensuring unit tests pass first. It exercises the full request chain (gateway → pipeline → provider → telemetry) using mock servers — no external services required.

### Running Tests Locally

```bash
# All unit tests
cd runtime && go test ./...

# Integration tests only
cd runtime && go test -run "TestIntegration|TestSelfTuningIntegration|TestTune_" ./...

# Single integration test
cd runtime && go test -run TestIntegrationFullRequestChain ./internal/gateway/
```

The dashboard has no tests yet — only a build check in CI.