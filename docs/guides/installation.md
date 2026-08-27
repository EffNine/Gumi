# Installation — Local Inference Auto-Tuner

## Requirements

- **Go 1.25+** to build.
- **CUDA / NVIDIA** GPU (single GPU, any class: consumer, workstation, datacenter). CPU-only and non-CUDA accelerators are out of scope for V1.
- **GGUF model file** — Gumi parses GGUF directly; no model catalog.
- **`llama-cli` (llama.cpp, CUDA build recommended)** on `PATH` or via `--backend-bin` for real tuning. Not bundled. Flag drift across llama.cpp versions is handled by a `--help` probe plus retry chain (`internal/backend`).
- Only the root Go module has runtime dependencies — **zero external dependencies** (`go test ./internal/... ./cmd/...`).

The pre-pivot runtime/dashboard/benchmark components (`runtime/`, `dashboard/`, `benchmark/`) are frozen and have their own Go modules; they are not required to build the V1 auto-tuner.

## Build from source

```bash
git clone https://github.com/EffNine/Gumi.git
cd gumi
make build        # produces ./gumi at repo root
./gumi --help
./gumi tune --help
```

`make` targets (root module):

```bash
make build        # go build -ldflags ... -o gumi ./cmd/gumi
make test         # go test ./internal/... ./cmd/...
make vet          # go vet  ./internal/... ./cmd/...
make fmt          # gofmt -l -w cmd internal  (CI enforces cleanliness)
```

Note: `go.work` includes `.`, `./runtime`, `./benchmark`. Root `make` targets use `./internal/... ./cmd/...` (not `./...`) because `dashboard/node_modules` contains stray vendored Go packages.

## llama.cpp backend

Real optimization runs need `llama-cli`:

```bash
# Check
which llama-cli
llama-cli --help | head -n 20

# Or pass explicitly
./gumi tune ./model.gguf --backend-bin /path/to/llama-cli
```

Gumi probes `llama-cli --help` once at startup to learn what this build supports (KV cache types `-ctk`, flash attention syntax `-fa on|off|auto` vs bare flag, `-ot` override-tensor, `-ngl`/`-b`/`-ub`, `mmap`/`mlock`, `--single-turn`). Unsupported dimensions are suppressed upstream with recorded reasons; one missing optional feature never fails the session. A legacy unknown-argument retry chain remains the final arbiter when discovery produced nothing usable.

## Verify the installation

```bash
# No backend needed — these three run anywhere
./gumi version
./gumi inspect ./model.gguf
./gumi inspect ./model.gguf --json
./gumi probe
./gumi probe --json
./gumi profiles

# Dry-run planning — no backend needed
./gumi tune ./model.gguf --workload agentic_coding --dry-run

# Real tuning — requires llama-cli
./gumi tune ./model.gguf --workload agentic_coding
```

Expected dry-run output includes the candidate plan and report skeleton without verified numbers.

## Platform notes

### Linux (primary target)

- GPU detection via `nvidia-smi` (preferred) → `rocm-smi` → `lspci` vendor-only fallback. `lspci` never fabricates VRAM.
- RAM via `/proc/meminfo` or `sysinfo`; threads from CPU topology; filesystem via `statfs` + mmap capability. Parsers are pure functions tested against fixtures; unknown stays unknown.
- Process-group + `SIGKILL` on per-run timeout; VRAM sampled via `nvidia-smi` polling (delta over baseline), RAM via child RSS polling.

### macOS / Windows

- Process management and RSS sampling have platform files but are Linux-primary. CUDA probing assumes `nvidia-smi`. Untested platforms stay untested (see `docs/specs/27-gumi-v1-release-audit.md` §14).
- The same engine must serve a laptop RTX and an H100 — no GPU model, VRAM size, generation, or throughput assumptions exist in `internal/*`.

## Uninstall

Delete the built binary and reports:

```bash
rm -f ./gumi
rm -rf ./reports
```

No global state is created. Each tuning run writes only to the `--out` directory (default `reports/<model>-<workload>-<timestamp>/`).

## Pre-pivot artifacts

`runtime/`, `dashboard/`, `benchmark/`, and specs `00`–`22` + `GEP_v1` are frozen
pre-pivot components. Their installation instructions (Docker, `gumi start`,
`8787`/`8788`, etc.) no longer describe the current product. See `README.md`
for the migration note.
