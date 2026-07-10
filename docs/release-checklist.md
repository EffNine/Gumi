# Release Checklist

Use this checklist before publishing a new Novexa alpha release. The goal is to
ensure every release artifact is complete, tested, and free of secrets.

## Before cutting the release

- [ ] `CHANGELOG.md` is updated with the version, date, and summary of changes.
- [ ] `README.md` and the runtime/dashboard README files reflect the current
      release status and limitations.
- [ ] `docs/installation.md`, `docs/quickstart.md`, and `docs/troubleshooting.md`
      are accurate for the new release.
- [ ] `novexa.example.yaml` is still labeled as documentation-only if YAML
      loading has not been enabled.
- [ ] `runtime/internal/version/version.go` still defaults to the previous
      development version. Release version is injected by build scripts only.

## Automated checks

- [ ] `gofmt -w .` ran in `runtime/`.
- [ ] `go test ./...` passes in `runtime/`.
- [ ] `go vet ./...` passes in `runtime/`.
- [ ] `npm ci` and `npm run build` pass in `dashboard/`.
- [ ] `git diff --check` passes (no conflict markers or whitespace errors).

## Build and package

- [ ] `make dashboard` produced `dashboard/dist/index.html`.
- [ ] `make build` produced a working `novexa` binary for the current platform.
- [ ] `make release` produced all five archives under `dist/releases/`:
  - `novexa-<version>-darwin-arm64.tar.gz`
  - `novexa-<version>-darwin-amd64.tar.gz`
  - `novexa-<version>-linux-amd64.tar.gz`
  - `novexa-<version>-linux-arm64.tar.gz`
  - `novexa-<version>-windows-amd64.zip`
- [ ] `make check-release` verified that every archive contains:
  - the correct binary (`novexa` or `novexa.exe`)
  - `dashboard/dist/`
  - `profiles/`
  - `README.md`, `LICENSE`, `CHANGELOG.md`, `novexa.example.yaml`
- [ ] `dist/releases/SHA256SUMS.txt` matches every archive.

## Docker

- [ ] `docker build -t novexa:<version> .` succeeds.
- [ ] A container started from the image responds to `GET /health` on port 8787.
- [ ] The SQLite database is persisted through the `/data` volume.
- [ ] The container runs as a non-root user.

## Security and cleanliness

- [ ] No real API keys, provider tokens, or local secrets are hard-coded in any
      release artifact.
- [ ] `novexa.example.yaml` keeps `telemetry.external: false` and does not log
      full prompts or responses.
- [ ] `.claude/` is not staged or included in release archives.
- [ ] `node_modules/`, `dashboard/dist/`, and `dist/releases/` are excluded from
      git.

## Publish

- [ ] Create a signed Git tag: `git tag -s v0.1.0-alpha -m "Release v0.1.0-alpha"`
- [ ] Push the tag: `git push origin v0.1.0-alpha`
- [ ] The GitHub release workflow builds and uploads the archives.
- [ ] Mark the GitHub release as a pre-release.
- [ ] Attach release notes from `CHANGELOG.md`.

## After publishing

- [ ] Download and smoke-test at least one archive on the primary development
      platform.
- [ ] Update any outstanding documentation links to point to the new release.
