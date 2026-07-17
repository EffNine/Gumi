# AGENTS.md

## Cursor Cloud specific instructions

Gumi is a single product: a local-first, OpenAI-compatible AI reliability runtime
(Go) with an embedded React/Vite observability dashboard. Components live in one
repo: `runtime/` (Go service + CLI), `dashboard/` (React UI), `benchmark/` (Go
bench lib/CLI), `profiles/` (YAML tuning presets), plus docs.

### Build / lint / test / run

Standard commands are in the root `Makefile`, `README.md`, and CI
(`.github/workflows/ci.yml`). In short:

- Build everything: `make build` (builds `dashboard/dist`, then the `gumi`
  binary at repo root). Runtime-only build: `cd runtime && go build ./cmd/gumi`.
- Run: `./gumi start` (or `make run`). API on `127.0.0.1:8787`, dashboard on
  `127.0.0.1:8788`. Stop with `./gumi stop`; check with `./gumi status` /
  `./gumi doctor`.
- Go tests / vet: `cd runtime && go test ./...` / `go vet ./...`
  (CI also enforces `gofmt`).
- Dashboard dev server: `cd dashboard && npm run dev` (proxies `/api` →
  runtime on 8787, so the runtime must already be running).

### Non-obvious caveats

- The runtime entrypoint is `runtime/cmd/gumi/main.go` (delegates to
  `internal/cli`). A previously unanchored `.gitignore` rule (`gumi`) matched
  this source dir, so it must stay anchored as `/gumi`; otherwise `main.go`
  silently disappears and `go build ./cmd/gumi` (Makefile/Dockerfile/CI) breaks.
- Model naming uses a `provider:model` scheme. Requests must use `local:auto`
  or an explicit prefix like `ollama:llama3.2:1b`. A bare `llama3.2:1b` is
  parsed as provider `llama3.2` and fails with `PROVIDER_UNAVAILABLE`.
- API auth is a bearer token; the local default is `gumi-local`
  (`Authorization: Bearer gumi-local`).
- Real chat completions require a local inference provider (Ollama / LM Studio /
  any OpenAI-compatible server). None is bundled. SQLite telemetry is embedded
  (`~/.gumi/gumi.db`) and needs no separate service.
- Ollama (optional, for E2E): install is not in the update script. On this
  CPU-only VM the latest Ollama (0.32.x) segfaults during model warmup; use a
  known-good older release, e.g. `curl -fsSL https://ollama.com/install.sh |
  OLLAMA_VERSION=0.5.13 sh` (the installer needs the `zstd` apt package). There
  is no systemd, so start it manually: `ollama serve`. Then a small model like
  `ollama pull llama3.2:1b` works on CPU.
- The dashboard Playground's streamed response can render scrambled/partial with
  very fast or tiny models; this is a UI display quirk, not a backend failure —
  verify via the API directly or the dashboard "Requests" page (which shows the
  correctly-recorded telemetry).
- `cd dashboard && npm run lint` currently fails: the repo ships no
  `eslint.config.js` (ESLint v9+ flat-config). Dashboard lint is not part of CI
  (CI only runs `npm ci && npm run build`).
