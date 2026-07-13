# Novexa Release Standard

**Version:** 1.0  
**Status:** Active  
**Scope:** Versioning, release process, and artifact conventions for Novexa Runtime

---

## 1. Versioning

Novexa follows **Semantic Versioning 2.0** (`MAJOR.MINOR.PATCH`).

### Pre-1.0 Convention

While Novexa is in alpha/beta (v0.x), the convention is:

| Component | Meaning |
|-----------|---------|
| `MAJOR` | Always `0` until stable V1 |
| `MINOR` | Incremented when a sprint adds significant new features |
| `PATCH` | Bug fixes, documentation, minor improvements |
| `-alpha` | Unstable, APIs may change |
| `-beta` | Feature-complete, stabilizing for release |

### Current Version

`0.1.0-alpha`

### When to Increment

| Change | Increment |
|--------|-----------|
| New feature sprint (Memory Engine, Router, Agent Mode) | `MINOR` +1 |
| Bug fix, test, docs, refactor | `PATCH` +1 |
| Breaking API change | `MAJOR` +1 (or `MINOR` +1 pre-1.0 with clear notice) |

---

## 2. Release Cadence

Novexa does not follow a fixed calendar schedule. Releases are made when:

- A sprint completes with all planned features merged and tested
- A critical bug fix is needed
- The maintainer decides the current state is worth publishing

Recommended: tag a release at the end of each sprint.

---

## 3. Release Process

### 3.1 Pre-Release Checklist

Before tagging a release, verify:

- [ ] `CHANGELOG.md` is up to date with all changes since last release
- [ ] `README.md` reflects current features and usage
- [ ] `novexa.example.yaml` matches the current config schema
- [ ] `go test ./runtime/...` passes
- [ ] `go vet ./runtime/...` passes
- [ ] `make dashboard` builds successfully
- [ ] `make build` produces a working binary
- [ ] No real credentials, API keys, or private paths in any release artifact
- [ ] `docs/specs/` files are NOT included in the release (they are gitignored)
- [ ] Version string in `Makefile` is correct
- [ ] Version string in `runtime/internal/version/version.go` is correct

### 3.2 Tagging

```bash
# For a new version:
git tag -a v0.2.0-alpha -m "v0.2.0-alpha: Agent mode, Memory Engine, LM Studio management"
git push origin v0.2.0-alpha
```

The GitHub Actions workflow will:
1. Build the dashboard
2. Cross-compile for 5 platforms
3. Create a draft pre-release on GitHub
4. Upload archives + SHA256SUMS

### 3.3 Post-Release

- [ ] Verify the draft release on GitHub, add release notes, publish
- [ ] Smoke-test the binary on at least one platform
- [ ] Update any integration docs that reference the old version

---

## 4. Artifact Contents

Each release archive contains:

```
novexa                    # Go binary (or novexa.exe on Windows)
dashboard/                # Production dashboard build
profiles/                 # Model profile YAML files
README.md                 # Project readme
LICENSE                   # License file
CHANGELOG.md              # Change log
novexa.example.yaml       # Example configuration
```

**Explicitly excluded:**
- `docs/` directory (contains internal specs)
- `.claude/`, `.remember/` (agent artifacts)
- `.env` files (environment secrets)
- `*.db` files (local databases)
- `benchmarks/` (raw benchmark outputs)

---

## 5. Release Notes Format

Each GitHub release should include:

```markdown
## Novexa v0.X.0-alpha

### Added
- Feature 1
- Feature 2

### Changed
- Change 1

### Fixed
- Fix 1

### Benchmarks
Key benchmark results for this release.

### Upgrading
Any migration steps needed.
```

The release notes should be copy-edited from the CHANGELOG entries.

---

## 6. Branch Strategy

- `main` — development branch, always in a releasable state
- Tags (`v*`) — point releases, immutable once published
- No long-lived feature branches (commit directly to main or use short-lived branches)

---

## 7. Security

- All release artifacts are scanned for secrets before publishing
- The release checklist includes a security review step
- No real credentials, API keys, or private paths are ever included in release artifacts
- Internal spec documents (`docs/specs/`) are gitignored and excluded from archives