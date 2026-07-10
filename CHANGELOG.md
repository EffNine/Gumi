# Changelog

All notable changes to Novexa are documented in this file.

## 0.1.0-alpha — Sprint 10: Packaging and Release

### Added

- Cross-platform release build scripts (`scripts/build-release.sh`,
  `scripts/check-release.sh`, `scripts/install.sh`).
- Top-level `Makefile` with targets: `test`, `vet`, `dashboard`, `build`, `run`,
  `release`, `clean`, `check-release`.
- Multi-stage `Dockerfile` that builds the dashboard in Node, the runtime in
  Go, and ships a small Alpine image with a non-root user and a `/data` volume.
- `.dockerignore` to keep the image small and avoid shipping secrets or build
  artifacts.
- GitHub Actions CI workflow (`.github/workflows/ci.yml`) for gofmt, go test,
  go vet, npm ci/build, and `git diff --check`.
- GitHub Actions release workflow (`.github/workflows/release.yml`) triggered
  on `v*` tags, building all supported targets, packaging archives, generating
  SHA256 checksums, and uploading draft pre-release artifacts.
- Release archive layout including binary, `dashboard/dist`, starter profiles,
  `novexa.example.yaml`, `README.md`, `LICENSE`, and `CHANGELOG.md`.
- `novexa.example.yaml` documenting the planned local-first configuration
  format for the alpha. YAML config parsing is not implemented yet.
- Installation guide (`docs/installation.md`) covering source builds, release
  archives, Docker, macOS, Linux, Windows, starting, dashboard, client setup,
  and uninstalling.
- Quickstart guide (`docs/quickstart.md`) with Ollama setup, model pull,
  startup, dashboard, chat completions, and `novexa doctor`.
- Troubleshooting guide (`docs/troubleshooting.md`) for common issues such as
  port conflicts, Ollama availability, missing models, dashboard build errors,
  SQLite permissions, invalid API keys, provider timeouts, missing profiles,
  streaming, and macOS quarantine.
- Release checklist (`docs/release-checklist.md`) for verifying builds,
  archives, Docker, and security before publishing.
- Build-time version metadata package (`runtime/internal/version`) with
  `Version`, `Commit`, and `BuildDate` injected via ldflags. `novexa version`
  shows metadata in release builds while preserving the one-line default output
  for development builds.

### Release targets

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- Windows amd64

### Known limitations

- YAML configuration parsing is not implemented in the alpha.
- CLI `stop` and `restart` commands are not implemented.
- Streaming chat completions are not supported.
- Cross-platform release artifacts are cross-compiled but not necessarily
  manually run on every target.
- A Dockerfile is included for convenience but the Docker image has not been
  manually built or tested in this environment.

## Previous sprints

- Sprint 9: CLI and dashboard (status, doctor, config show, providers, models,
  benchmark, logs; local dashboard with overview, providers, requests, config,
  doctor, telemetry).
- Sprint 8: Model profiles with starter presets.
- Sprint 7: Validation, repair, and guard engines.
- Sprint 6: Context and prompt engines.
- Sprint 5: SQLite storage and local telemetry.
- Sprint 4: Pipeline engine.
- Sprint 3: Provider adapters for Ollama, LM Studio, and OpenAI-compatible
  local servers.
- Sprint 2: OpenAI-compatible gateway API.
- Sprint 1: Runtime skeleton.
- Sprint 0: Repository and documentation setup.
