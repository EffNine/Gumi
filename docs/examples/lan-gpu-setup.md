# Two-box LAN GPU setup (Mac + Windows)

Use a Mac (or cloud CI) for building and unit tests, and a Windows PC with a GPU
for live Ollama / LM Studio inference. Cloud runners do **not** need a GPU —
pipeline/router/gateway tests use `fakeAdapter` and `httptest`.

## Verified layout

| Role | Machine | Notes |
|------|---------|--------|
| Dev / gumi runtime / benchmarks | Mac | Talks to Windows over LAN |
| Inference | Windows RTX 5070 (~12GB) | Ollama `:11434`, LM Studio `:1234` |

Example Windows LAN IP used in this project: `192.168.0.164`.

## Windows box

1. Ollama listening on all interfaces (not only localhost):

```powershell
$env:OLLAMA_HOST = "0.0.0.0:11434"
# restart Ollama after setting
```

2. LM Studio → Local Server → enable **Serve on Local Network** (port `1234`).

3. Allow inbound TCP `11434` and `1234` in Windows Firewall.

4. Pull complementary Ollama models (avoid duplicating LM Studio catalog):

```bash
ollama pull llama3.2:3b
ollama pull qwen3.5:2b
ollama pull qwen3:1.7b
ollama pull gemma3:1b
ollama pull gemma3:4b
ollama pull qwen2.5-coder:7b
ollama pull llama3.1:8b
ollama pull qwen3:8b
ollama pull deepseek-r1:8b
ollama pull mistral-small
ollama pull gemma3:12b
```

Skip on Ollama if already on LM Studio: `qwen3.5:9b`, gemma-4*, liquid/lfm*, glm-4.6v.

Cap context to ~8k–16k when loading 12B-class models on 12GB VRAM.

## Mac / gumi runtime

```bash
export GUMI_OLLAMA_URL=http://192.168.0.164:11434
export GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1

# start gumi (example)
cd runtime && go run ./cmd/gumi
```

### Smoke

```bash
curl -s http://192.168.0.164:11434/api/tags | head
curl -s http://192.168.0.164:1234/v1/models | head

curl -s http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama:llama3.2:1b","messages":[{"role":"user","content":"ping"}],"max_tokens":32}'
```

### Quality benchmarks

Benchmark CLI talks to **local gumi** (`--base-url`); gumi talks to the Windows providers.

```bash
cd benchmark
go run ./cmd/run.go \
  --provider lmstudio \
  --model qwen/qwen3.5-9b \
  --base-url http://127.0.0.1:8787 \
  --attempts 1 \
  --conditions gumi-stabilized \
  --output ../benchmarks/reports/
```

Ollama quality defaults: `qwen2.5-coder:7b` or `llama3.1:8b`.

## Model roles (12GB)

| Role | Model |
|------|--------|
| Fast smoke (Ollama) | `ollama:llama3.2:1b` / `:3b` |
| Fast smoke (LM Studio) | `lmstudio:google/gemma-4-e4b` |
| Quality (LM Studio) | `lmstudio:qwen/qwen3.5-9b` |
| Quality (Ollama) | `ollama:qwen2.5-coder:7b` |
| Reasoning | `ollama:deepseek-r1:8b` |
| Stretch | `gemma3:12b` / `gemma-4-12b-qat` (cap context) |
