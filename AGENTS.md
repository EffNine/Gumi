# AGENTS.md

Gumi is a single product: a **Local Inference Auto-Tuner** — a CLI-first,
local-first Go tool that experiments with inference configurations on the
user's CUDA machine, measures real performance, verifies capability, and
returns the best verified configurations it can prove.

Core philosophy: *Don't guess the best local inference settings. Measure them
on the user's actual machine.* Gumi is NOT a runtime, model server, dashboard,
web UI, coding agent, quantizer, or cluster scheduler.

The optimizer lives at the repo root: `cmd/gumi/` + `internal/*` (single Go
module, zero external dependencies). The pre-pivot runtime/dashboard/
benchmark components are frozen in place under their own modules; do not
extend them. V1 spec: `docs/specs/26-gumi-v1-auto-tuner.md`. Historical specs
`00`–`22` + `GEP_v1` describe the frozen pre-pivot architecture and are
marked as such; current docs are `23`–`27`.

Preferred language: *local inference auto-tuner*, *hardware-aware inference
tuning*, *measured/verified configuration*, *practical context frontier*,
*capability gate*, *performance objective*. Avoid: *AI-powered optimization*,
*guaranteed optimal*, *same intelligence*, *lossless* unless qualified by
measured evidence.

### Build / lint / test / run

- Build everything: `make build` (produces `./gumi`).
- Tests: `make test` (= `go test ./internal/... ./cmd/...`). No llama.cpp or
  model files needed — backend/hardware are injected fakes in tests.
- Vet: `make vet`. Formatting: `make fmt` (CI enforces gofmt cleanliness).
- Run: `./gumi tune <model.gguf> [--workload agentic_coding|chat]
  [--min-decode N]` is the V1 product command (`optimize` is an alias;
  `--dry-run` plans without a backend; `--baseline 'ngl=..,c=..,kv=..'`
  admits a human config as a gated CURRENT-BASELINE candidate). Stop/check
  commands from the old runtime no longer exist at root; see `legacy` note
  in README.md.

### Non-obvious caveats

- The tuner stages live in `internal/optimize` (orchestration) and
  `internal/search` (pure strategy: ladder, bisection midpoint, objective
  evaluation, dominance, profile selection). Search math must stay pure and
  hardware-blind; measurements belong to the pipeline.
- Dominance has two hard guards: only capability-CLEARED points may dominate
  anything (a fast-but-dumb config prunes nothing), and strictness must
  exceed combined measurement noise (jitter never prunes). Context counts as
  a RESOURCE axis (KV memory scales with window), not a benefit.
- Backend capabilities are discovered from `llama-cli --help`
  (`internal/backend/capabilities.go`) BEFORE planning; the candidate
  generator is capability-aware so unsupported KV types / `-ot` placement
  are suppressed upstream with recorded reasons, never measured silently.
- There is NO universal tok/s floor. Floors come from `--min-decode` or the
  workload's relative retention rule (`workload.Profile.DecodeRetention`),
  anchored on the best decode measured in THIS run.
- MAX PRACTICAL CONTEXT is capability-gated like any recommendation; on gate
  regression the frontier steps down through measured passing levels.
- `go.work` includes `.`, `./runtime`, `./benchmark`. Root make/CI targets
  use `./internal/... ./cmd/...`, not `./...`, because
  `dashboard/node_modules` contains stray vendored Go packages that would
  otherwise be picked up by the root module.
- Real optimization runs need `llama-cli` (llama.cpp) on PATH or via
  `--backend-bin`; none is bundled. Flag drift across llama.cpp versions is
  handled by a `--help` probe plus retry chain (`internal/backend`).
- `gumi inspect` reads GGUF metadata directly — there is no manually
  maintained model catalog.
- KV-cache arithmetic is exact from GGUF geometry (`internal/gguf`):
  2 × layers × kv_heads × head_dim × bytes-per-elem (with ggml block sizes
  for quantized KV). Qwen3-30B-A3B geometry = 96 KiB/token at f16 — covered
  by tests; do not "approximate" it.
- The capability gate is the core differentiator: a faster config that
  regresses vs. the REFERENCE candidate on paired prompts/seeds must be
  rejected (`internal/verify.Gate`, exercised end-to-end in
  `internal/optimize/pipeline_test.go` and `tuner_test.go` with fake
  backends).
- Evidence semantics are three separate things (`docs/specs/25-evidence-hardening.md`):
  capability confidence (per candidate), performance stability (± half-range),
  and ranking confidence (`internal/confidence.RankConfidence`, top-two
  passers only). Zero observed variance means UNKNOWN noise floor — never a
  separation claim; ties fall back to the safer operating margin.
- Safe-tuning boundary: never auto-change weights, active expert count, RoPE
  scaling, system prompt, or reasoning/sampling behavior. Quantization
  changes are recommend-only.
- Hardware probers never fabricate values: unknown stays unknown
  (`internal/hardware`). Parsers are pure functions tested against fixtures.
  No GPU model, VRAM size, generation, or throughput assumptions exist in
  `internal/*`; the same engine must serve a laptop RTX and an H100.
