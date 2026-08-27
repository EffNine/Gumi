# Gumi — Local LLM Optimization Engine (top-level task runner).
#
# The primary product is the `gumi` optimizer CLI at the repo root
# (cmd/gumi + internal/*). The pre-pivot runtime/dashboard/benchmark code is
# frozen in place under their own Go modules; see README.md.

VERSION_GIT ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "")
VERSION     ?= $(if $(filter v%,$(VERSION_GIT)),$(VERSION_GIT),v1.0.0)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
		-X github.com/EffNine/gumi/internal/version.Version=$(VERSION) \
		-X github.com/EffNine/gumi/internal/version.Commit=$(COMMIT) \
		-X github.com/EffNine/gumi/internal/version.BuildDate=$(BUILD_DATE)

GO ?= go

.PHONY: all build test vet fmt clean optimize

all: fmt vet test build

# Build the optimizer CLI into ./gumi.
build:
	$(GO) build -ldflags '$(LDFLAGS)' -o gumi ./cmd/gumi

test:
	$(GO) test ./internal/... ./cmd/...

vet:
	$(GO) vet ./internal/... ./cmd/...

fmt:
	gofmt -l -w cmd internal

clean:
	rm -f gumi gumi.exe
	rm -rf reports

# Quick demo: plan candidates for a model without running a backend.
optimize: build
	./gumi optimize $(MODEL) --workload $(WORKLOAD) --dry-run
