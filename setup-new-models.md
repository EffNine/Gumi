# New 9B Model Setup Instructions

## Quick Install for New 9B Models

### Model Names to Use in Gumi
```bash
# Replace with exact model names when verified
export GUMI_DEFAULT_MODEL=yi-coder:9b
export GUMI_DEFAULT_PROVIDER=lmstudio
export GUMI_LMSTUDIO_URL=http://dev-pc-tailscale-ip:1234/v1
```

### Step 1 - Verify Model Installation

**If using LM Studio (most likely):**
```bash
# Check if LM Studio is running
ps aux | grep lmstudio

# List available models
curl -s http://localhost:1234/v1/models \
  | jq -r '.data[] | select(.name | contains("yi-coder") or contains("omnicoder")) | .name' 2>/dev/null
```

**If using Ollama directly:**
```bash
# Install via Ollama (if available)
ollama pull yi-coder:9b
ollama pull carstenuhlig/omnicoder:9b
ollama pull <12b-model-name>  # e.g., gemma-4-e4b
```

### Step 2 - Update Gumi Configuration

Edit `~/.gumi/gumi.yaml`:

```yaml
# Provider section - replace with your URL
runtime:
  host: 0.0.0.0
  port: 8787

providers:
  ollama:
    enabled: true
    url: http://localhost:11434  # Only if using Ollama

  lmstudio:
    enabled: true
    url: http://dev-pc-tailscale-ip:1234/v1  # Update this

  # Add any additional providers if needed
```

### Step 3 - Restart Gumi
```bash
./gumi stop && ./gumi start
```

### Step 4 - Verify and Test

**Quick test:**
```bash
# Test Yi-Coder
./gumi exec --model yi-coder:9b \
  "Explain security vulnerabilities in Flask API in 3 bullets."

# Test OmniCoder-9B
./gumi exec --model carstenuhlig/omnicoder:9b \
  "Write a Python error handler for database connections."
```

### Step 5 - Run Full Benchmark Suite
```bash
# Test on new models
python3 scripts/regression_harness.py \
  --model yi-coder:9b \
  --mode full \
  --questions 2 \
  --save

python3 scripts/regression_harness.py \
  --model carstenuhlig/omnicoder:9b \
  --mode full \
  --questions 2 \
  --save
```

## Expected Model Files

### LM Studio Format
If downloaded via LM Studio:
```
Library/
  Yi Coder/                     # Or OmniCoder
    models/
      gguf/
        yi-coder-9b.Q4_K_M.gguf
        omnicoder-9b.Q4_K_M.gguf
        <12b-model>.gguf
```

### HuggingFace Direct Load
```bash
# Example HuggingFace paths (replace with actual model IDs)
huggingface_hub download 01-ai/Yi-Coder-9B \
  --resume-strategy=any 
  --cache-dir ~/.cache/huggingface

# Then configure Gumi with local path or HF API
```

## Next Steps for Current System

Given we have:
- **Yi-Coder-9B** (01-ai)
- **OmniCoder-9B** (carstenuhlig) 
- **12B model** (unspecified)

### 1. Update Gumi Config
Open `~/.gumi/gumi.yaml` and set:

```yaml
# Add/update LM Studio provider
lmstudio:
  enabled: true
  url: http://dev-pc-tailscale-ip:1234/v1  # Update to your dev-pc IP

# Or add Ollama provider if models are installed via Ollama
ollama:
  enabled: true
  url: http://dev-pc-tailscale-ip:11434  # Update if using Ollama
```

### 2. Test Availability
```bash
# Quick availability check
curl -s http://dev-pc-tailscale-ip:1234/v1/models | jq -r '.data[].name' 2>/dev/null
```

### 3. Configure Profiles
Update model profiles in `~/.gumi/gumi.yaml`:

```yaml
profiles:
  yi-coder-9b:
    description: Yi-Coder 9B for coding
    temperature: 0.1
    provider: lmstudio
    model: yi-coder-9b

  omnicoder-9b:
    description: OmniCoder-9B for agentic coding  
    temperature: 0.7
    provider: lmstudio
    model: carstenuhlig/omnicoder:9b
```

## Quick Setup Script

Save as `setup-new-models.sh`:
```bash
#!/bin/bash

# Setup new 9B models for Gumi

echo "Setting up new 9B models..."

# Update Gumi config
cat > ~/.gumi/gumi.yaml << 'EOF'
# Gumi Runtime Configuration
# Providers pointing to dev-pc via Tailscale
runtime:
  host: 0.0.0.0
  port: 8787
  mode: stabilized

dashboard:
  host: 127.0.0.1
  port: 8788

providers:
  lmstudio:
    enabled: true
    url: http://dev-pc-tailscale-ip:1234/v1
    default_model: ""

  conductor:
    enabled: true
    url: https://conductor-yknfkg.fly.dev/v1
    api_key: "ac39a9fcb590931851f7f08a57297d6361b79dd821b1d693c4169381db73cb3e"
    default_model: ""
EOF

# Restart Gumi
./gumi stop && ./gumi start

# Wait for startup
sleep 5

# Test new model availability
if curl -s http://127.0.0.1:8787/v1/gumi/status | grep -q \"providers\"; then
    echo "✅ Gumi running with new LM Studio models!"
else
    echo "❌ Gumi status check failed"
    exit 1
fi

echo "✅ New models configured successfully!"
```

## Testing the New Models

### Basic Functionality
```bash
# Test Yi-Coder coding capabilities
./gumi exec --model yi-coder-9b \
  "Write a Python function to validate email addresses."

# Test OmniCoder agentic capabilities  
./gumi exec --model omnicoder-9b \
  "Given this problematic code, fix the security vulnerabilities and improve the error handling."
```

### Benchmark New Models
```bash
# Run comparison benchmarks
python3 scripts/regression_harness.py \
  --model yi-coder-9b \
  --mode full \
  --questions 5 \
  --save

python3 scripts/regression_harness.py \
  --model omnicoder-9b \
  --mode full \
  --questions 5 \
  --save
```

### Performance Comparison
Compile benchmark results into a summary report:

```python
# scripts/compare-models.py
import json, glob

def load_results(pattern):
    files = glob.glob(pattern)
    return [json.load(open(f)) for f in files]

# Load all model results
results = []
for model in ["yi-coder-9b", "omnicoder-9b", "qwen3-8b", "llama3-1-8b"]:
    try:
        files = glob.glob(f"~/.gumi/test-results/benchmark_*.json")
        for f in files:
            data = json.load(open(f))
            if data.get("model") == model:
                results.append(data)
    except: pass

# Generate comparison
print("Model Performance Comparison")
print("=" * 60)
for r in results:
    model = r["model"]
    score = r["overall_score"]
    latency = r.get("avg_latency", "N/A")
    print(f"{model:20} {score:5.2f}/10.0  Latency: {latency}s")
```

## Next Steps Checklist

1. [ ] **Update Gumi config** with correct dev-pc IP
2. [ ] **Restart Gumi** to apply changes
3. [ ] **Test model availability** via `/v1/gumi/status`
4. [ ] **Run benchmarks** on new models
5. [ ] **Compare performance** with existing qwen3:8b baseline
6. [ ] **Update documentation** with new profiles
7. [ ] **Document fine-tuning strategy** for optimal performance

## Expected Performance

Based on existing benchmarks:
- **Top baseline:** qwen3:8b (7.77/10 overall)
- **Yi-Coder-9B:** Should beat qwen2.5-coder-7b for coding tasks
- **OmniCoder-9B:** Agentic capabilities may exceed qwen3.5:9b but with higher latency
- **12B model:** Largest – might have trade-offs with VRAM limits

Proceed with testing once the setup script is run and models are available in Gumi! 🚀
