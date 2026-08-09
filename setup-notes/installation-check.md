⚠️ **Installation Check**

The user reports they have downloaded both model on dev PC + other model 12B. However, the current `
ollama list` shows only cloud-based models, not the local models they mentioned.

## Possible Scenarios

### 1. LM Studio Installation (Likely)
If they are running the standard Gumi setup with LM Studio,
```bash
# LM Studio model loading command
# Models: yi-coder-9b, omnicoder-9b, [12B-model]
# Located at: /Users/afnanrudy/Library/Application\ Support/LM\ Studio/models/
```

LM Studio models are loaded via its built-in GGUF handler, not through `ollama pull`.

### 2. Direct HuggingFace Local Loading
Direct HF local loading:
```bash
# Using HF API or hfc CLI
curl -X POST https://huggingface.co/api/models/01-ai/Yi-Coder-9B/pull \
  -H "Authorization: Bearer HUGGINGFACE_TOKEN"
```

### 3. Other Inference Frameworks
If using llama.cpp directly:
```bash
# llama.cpp model path
cat models/yi-coder/gguf/model.gguf | ollama create yi-coder:9b -i
```

## Gumi Setup Strategy

Based on current system state:

### Current Configured
- **Runtime:** Gumi on `0.0.0.0:8787` (root cause of timeout)
- **Ollama:** Various cloud models available
- **LM Studio:** Likely configured in GUMI_LMSTUDIO_URL

### Recommendation

1. **Verify LM Studio installation** (presumed - typical Gumi setup)
2. **Update Gumi config** to point to the appropriate LM Studio model path
3. **Check available 9B/12B models** in LM Studio:

```bash
# List LM Studio models
# via LM Studio GUI or API
curl [localhost:1234/v1/models] 2>/dev/null || echo "LM Studio not running"
```

4. **Update profiles** in `~/.gumi/gumi.yaml`:
```yaml
providers:
  lmstudio:
    enabled: true
    url: http://localhost:1234/v1  # or the dev-pc Tailscale IP
    # Specify the exact 9B/12B model name
    model: yi-coder-9b  # OR omnicoder-9b
```

### Next Steps

1. **Confirm installation method** and update config files accordingly
2. **Test the harness** with the properly configured model
3. **Update the documentation** with the correct installation procedure

## Current Known Models (in our previous testing)

We successfully tested with:
- `ollama:ornith:9b` (Ornith 9B Q4_K_M)
- `ollama:qwen3:8b` 
- `ollama:llama3.1:8b`
- `ollama:qwen2.5-coder:7b`

If Yi-Coder-9B/OmniCoder-9B are installed via LM Studio, we need to configure Gumi to point to them.

## Immediate Action

```bash
# Verify LM Studio model loading
ps aux | grep lmstudio

# Check Gumi config file
nano ~/.gumi/gumi.yaml

# Update model name in profiles
# Then restart Gumi
./gumi stop && ./gumi start
```

## Environment Variables (for convenience)

For quick model testing:
```bash
export GUMI_DEFAULT_MODEL=yi-coder-9b
export GUMI_PROVIDER=lmstudio
export GUMI_LMSTUDIO_URL=http://dev-pc-tailscale-ip:1234/v1
```

This will help the user configure their system correctly with the new 9B/12B models.
