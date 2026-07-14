# Gumi top-level task runner.
#
# This Makefile orchestrates the Go runtime and React dashboard builds. It uses
# build-time ldflags to inject release metadata so `gumi version` can report
# the exact release it was built from without editing source files.

VERSION_GIT ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "")
VERSION     ?= $(if $(filter v%,$(VERSION_GIT)),$(VERSION_GIT),0.2.0-alpha)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     = -s -w \
		-X github.com/EffNine/gumi/runtime/internal/version.Version=$(VERSION) \
		-X github.com/EffNine/gumi/runtime/internal/version.Commit=$(COMMIT) \
		-X github.com/EffNine/gumi/runtime/internal/version.BuildDate=$(BUILD_DATE)

GO     ?= go
NPM    ?= npm

.PHONY: all test vet dashboard build run release clean check-release

all: dashboard build

test:
	cd runtime && $(GO) test ./...

vet:
	cd runtime && $(GO) vet ./...

# Build the dashboard production bundle using the lockfile. The resulting
# dashboard/dist directory is embedded into release archives by build-release.sh.
dashboard:
	cd dashboard && $(NPM) ci && $(NPM) run build

# Build a local development binary for the current platform. The dashboard is
# rebuilt first so the local server can serve it on port 8788.
build: dashboard
	cd runtime && $(GO) build -ldflags '$(LDFLAGS)' -o ../gumi ./cmd/gumi

# Run the locally built binary. Use Ctrl+C to stop.
run: build
	./gumi start

# Create cross-platform release archives under dist/releases/.
release: clean dashboard
	./scripts/build-release.sh "$(VERSION)" "$(COMMIT)" "$(BUILD_DATE)"

# Verify that every release archive contains the expected files and that its
# checksum is valid. This target depends on release so the artifacts exist.
check-release: release
	./scripts/check-release.sh "$(VERSION)"

# Remove generated build artifacts. Preserve the source lockfiles and profiles.
clean:
	rm -rf dist/releases dashboard/dist gumi runtime/gumi

# ── Benchmark ──────────────────────────────────────────────
.PHONY: benchmark benchmark-quick benchmark-thorough

benchmark: build
	./gumi benchmark --model "$(MODEL)" --mode auto

benchmark-quick: build
	./gumi benchmark --model "$(MODEL)" --mode quick

benchmark-thorough: build
	./gumi benchmark --model "$(MODEL)" --mode thorough --attempts 10
