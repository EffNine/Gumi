# Examples — Local Inference Auto-Tuner

Validated, copy-pasteable examples for the **V1 local inference auto-tuner**.

| Example | What it shows | Location |
|---|---|---|
| Optimization report | Annotated `report.md` from a real tuning run | [optimization-report.example.md](./optimization-report.example.md) |
| cURL (historical) | Pre-pivot runtime endpoint — frozen | [curl/](./curl/) |
| Python OpenAI (historical) | Pre-pivot runtime endpoint — frozen | [python-openai/](./python-openai/) |
| LAN GPU setup (historical) | Pre-pivot two-box LAN — frozen | [lan-gpu-setup.md](./lan-gpu-setup.md) |

## V1 quick copy-paste

```bash
# Tune (default: agentic_coding)
./gumi tune ./model.gguf --workload agentic_coding

# With an explicit decode floor
./gumi tune ./model.gguf --workload agentic_coding --min-decode 25

# Dry-run planning (no llama.cpp needed)
./gumi tune ./model.gguf --workload agentic_coding --dry-run

# Compare your current config through the same pipeline
./gumi tune ./model.gguf --baseline 'ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128'

# Inspect and probe (no backend needed)
./gumi inspect ./model.gguf --json
./gumi probe --json
./gumi profiles

# Export the winner to another tool
./gumi export --config reports/<run>/candidates.json --id balanced \
    --target llama.cpp --model ./model.gguf
# also: --target llama-server | lmstudio | ollama
```

Historical examples are bannered and retained for provenance; current product docs are `README.md` + `docs/specs/26-gumi-v1-auto-tuner.md`.
