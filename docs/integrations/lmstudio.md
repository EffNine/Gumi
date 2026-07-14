# LM Studio Integration

LM Studio is the recommended local inference provider for Gumi. It can run on
a secondary machine (Mac Mini, PC with GPU) and expose an OpenAI-compatible API
over the local network — zero RAM impact on your development machine.

---

## Architecture

```
Your Mac (dev machine)              Secondary machine (e.g. Mac Mini)
┌─────────────────────┐            ┌──────────────────────────┐
│ Gumi Runtime      │  HTTP      │ LM Studio                │
│ localhost:8787      │───────────▶│ 192.168.0.164:1234       │
│                     │            │                          │
│ Agentic Coding      │            │ Model loaded in VRAM     │
│ Router              │            │ (not touching your Mac)  │
│ Pipeline Engine     │            │                          │
└─────────────────────┘            └──────────────────────────┘
```

This is the **recommended setup** for Gumi — all inference happens on the
remote machine, your dev machine stays free.

---

## What Gumi Controls Today

Gumi's `LMStudioAdapter` uses LM Studio's **OpenAI-compatible** endpoint
(`/v1/chat/completions`) and controls these inference parameters:

| Parameter | Set via profile | Per-request override |
|-----------|:---------------:|:--------------------:|
| `temperature` | ✅ `defaults.temperature` | ✅ |
| `top_p` | ✅ `defaults.top_p` | ✅ |
| `max_tokens` | ✅ `defaults.max_tokens` | ✅ |
| `stop` sequences | ✅ `defaults.stop` | ✅ |
| `frequency_penalty` | ❌ (not in profile defaults) | ✅ (passthrough) |
| `presence_penalty` | ❌ (not in profile defaults) | ✅ (passthrough) |
| Tool calling | ✅ (via shim for weak models) | ✅ |
| Response format | ✅ (JSON schema) | ✅ |
| Reasoning effort | ✅ (via `thinking_policy`) | ✅ |

### Profile Example

```yaml
# profiles/essentialai-rnj-1.yaml
defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096
  thinking: false
```

If the incoming request doesn't set a parameter, Gumi fills it from the
profile. Explicit request values are never overwritten.

---

## What Gumi Controls via v1 REST API (Model Management)

Gumi now uses LM Studio's **v1 REST API** (`/api/v1/*`) for model lifecycle management:

| Endpoint | Purpose | Gumi status |
|----------|---------|:-------------:|
| `POST /api/v1/models/load` | Load model with `context_length`, `flash_attention`, `offload_kv_cache_to_gpu`, `eval_batch_size`, `num_experts` | ✅ Implemented |
| `POST /api/v1/models/unload` | Unload model by `instance_id` | ✅ Implemented |
| `GET /api/v1/models` | List available models on disk | ✅ Implemented |
| `POST /api/v1/models/download` | Download models on-demand | ❌ Not planned |

The `ModelManager` interface (`runtime/internal/provider/lmstudio_mgmt.go`) defines:

```go
type ModelManager interface {
    LoadModel(ctx context.Context, modelID string, config *LMStudioModelCfg) (*LMStudioLoadResponse, error)
    UnloadModel(ctx context.Context, instanceID string) error
    ListAvailableModels(ctx context.Context) ([]LMStudioModelEntry, error)
    LoadedModelID() string
    BuildPerModelConfig(modelID string) *LMStudioModelCfg
}
```

Gumi + Agentic Coding Router can now:

1. Classify the coding task (trivial → novel)
2. Decide the best model and config via the router
3. Load that model on LM Studio with optimal settings (`applyModelManagement()`)
4. Run inference
5. (optionally) Auto-unload when done to free VRAM

### Config Example

```yaml
# gumi.yaml
providers:
  lmstudio:
    url: http://192.168.0.164:1234/v1
    management:
      enabled: true
      auto_unload: true
      default_context_length: 8192
      default_flash_attention: true
      default_offload_kv_cache: true
      default_eval_batch_size: 512
      model_config:
        "qwen3-1.7b":
          context_length: 4096
          flash_attention: false
        "qwen2.5-coder-7b":
          context_length: 16384
          flash_attention: true
          offload_kv_cache: true
        "ornith-1.0-9b-q4-km":
          context_length: 32768
          flash_attention: true
          offload_kv_cache: true
          eval_batch_size: 512
```

### CLI Commands

```bash
# Show available models on disk and currently loaded model
gumi lmstudio status

# Load a model with optional overrides
gumi lmstudio load qwen2.5-coder-7b-instruct \
  --context-length 16384 \
  --flash-attention \
  --offload-kv-cache

# Unload a model by instance ID
gumi lmstudio unload <instance-id>

# List models on disk
gumi lmstudio models
```

All commands support `--url` to point at a specific LM Studio instance, or
`--json` for machine-readable output.

---

## Quick Setup

### 1. Set up LM Studio on a remote machine

Install LM Studio on a secondary machine (e.g. Mac Mini, PC with NVIDIA GPU).
Enable the local inference server in Settings → Server → "Start Server".

Default URL: `http://localhost:1234/v1`

### 2. Point Gumi at it

```bash
GUMI_PROVIDER_DEFAULT=lmstudio \
GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1 \
GUMI_DEFAULT_MODEL=lmstudio:qwen2.5-coder-7b-instruct \
GUMI_PROVIDER_TIMEOUT_SECONDS=120 \
./gumi start
```

Or in `gumi.yaml`:

```yaml
providers:
  lmstudio:
    url: http://192.168.0.164:1234/v1
    timeout_seconds: 120

defaults:
  provider: lmstudio
  model: lmstudio:qwen2.5-coder-7b-instruct
```

### 3. Connect your app

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/v1",
    api_key="gumi-local",
)

response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[{"role": "user", "content": "Write a Go HTTP server."}],
)
```

---

## Recommended Models

| Use case | Model | Profile |
|----------|-------|---------|
| Fast coding | `qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Agentic coding | `ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |
| Reasoning | `essentialai/rnj-1` | `essentialai-rnj-1` |
| Fast chat | `qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size chat | `google/gemma-4-e4b` | `gemma-4-e4b` |

---

## Troubleshooting

**Model loading takes 3-10 seconds on first request.**
This is normal — LM Studio loads the model into VRAM on the first inference
call. Gumi uses exponential backoff (3s → 6s) for model-loading errors.

**Connection refused.**
Ensure LM Studio's server is running on the remote machine. Check the URL and
port. LM Studio defaults to port 1234.

**Slow inference.**
LM Studio runs on the secondary machine's hardware. For better performance:
- Use a machine with a discrete GPU (NVIDIA recommended)
- Enable flash attention in LM Studio model settings
- Use smaller quantized models (q4_k_m, q4_0)
- Set a reasonable context length (not max)

**"Engine protocol startup was aborted" error.**
This happens when LM Studio is swapping models. Gumi retries with
exponential backoff automatically. If it persists, check:
- The model isn't too large for available VRAM
- Another model isn't being loaded simultaneously
- LM Studio is up to date

---

## References

- [LM Studio REST API](https://lmstudio.ai/docs/developer/rest)
- [LM Studio Load Model](https://lmstudio.ai/docs/developer/rest/load)
- [LM Studio Unload Model](https://lmstudio.ai/docs/developer/rest/unload)
- [Gumi Agentic Coding Router spec](../specs/19-agentic-coding-router-specification.md)
- [Gumi Implementation Roadmap](../specs/14-implementation-roadmap.md)
