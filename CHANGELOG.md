# Changelog

All notable changes to Novexa are documented in this file.

## 0.1.0-alpha — Sprint 10: Packaging and Release

### Added

- Cross-platform release build scripts.
- Top-level `Makefile` with targets for test, build, release, and verification.
- Multi-stage `Dockerfile` that builds a small Alpine image with a non-root user.
- GitHub Actions CI and release workflows.
- Release archive layout including binary, dashboard, starter profiles, config
  example, README, LICENSE, and CHANGELOG.
- Installation guide covering release archives, Docker, macOS, Linux, and
  Windows.
- Quickstart guide with Ollama setup and first request.
- Troubleshooting guide for common issues.
- Release checklist for verifying builds before publishing.
- Build-time version metadata shown in `novexa version`.

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
- A Dockerfile is included for convenience but the Docker image has not been
  manually tested on every platform.

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
