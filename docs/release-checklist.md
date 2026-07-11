# Release Checklist

Use this checklist before publishing a new Novexa alpha release. The goal is to
ensure every release artifact is complete, tested, and free of secrets.

## Before cutting the release

- [ ] `CHANGELOG.md` is updated with the version, date, and summary of changes.
- [ ] `README.md` reflects the current release status and limitations.
- [ ] `docs/installation.md`, `docs/quickstart.md`, and `docs/troubleshooting.md`
      are accurate for the new release.

## Build and package (from private source branch)

- [ ] `make test` passes.
- [ ] `make release` produced all five archives under `dist/releases/`:
  - `novexa-<version>-darwin-arm64.tar.gz`
  - `novexa-<version>-darwin-amd64.tar.gz`
  - `novexa-<version>-linux-amd64.tar.gz`
  - `novexa-<version>-linux-arm64.tar.gz`
  - `novexa-<version>-windows-amd64.zip`
- [ ] `make check-release` verified every archive.
- [ ] `dist/releases/SHA256SUMS.txt` matches every archive.

## Publish

- [ ] Create a signed Git tag on the public release branch:
      `git tag -s v0.1.0-alpha -m "Release v0.1.0-alpha"`
- [ ] Push the tag: `git push origin v0.1.0-alpha`
- [ ] Upload release artifacts from `dist/releases/` to the GitHub Release.
- [ ] Mark the GitHub release as a pre-release.
- [ ] Attach release notes from `CHANGELOG.md`.

## After publishing

- [ ] Download and smoke-test at least one archive.
- [ ] Update any outstanding documentation links to point to the new release.
