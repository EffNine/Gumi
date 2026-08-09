# Fine-Tuning Plan: qwen3:8b → gumi-finetuned

## Objective
Improve `ollama:qwen3:8b` from 7.77/10 to **8.5+/10** on the regression harness
while keeping it general-purpose across factual, analytical, coding, and security.

## Dataset Strategy

### Source A — Best-of-Breed responses (70%)
- Ornith 9B top-performing outputs from benchmark sessions
- Llama 3.1 8B strong coding/analytical answers
- Yi-Coder 9B clean code generations
- Filter criteria:
  - Structure score >= 8
  - Contains step-by-step reasoning markers
  - Bullet/numbered list present
  - No repetition loops
  - Finish_reason = stop

### Source B — Curated human-written answers (30%)
- 200 pairs per category = 800 total examples
- Categories: factual, analytical, coding, security
- Format: Qwen chat template with system prompt + user + assistant

### Target Distribution
| Category | Raw count | Target % |
|---|---|---|
| Factual | 140 | 17.5% |
| Analytical | 280 | 35% |
| Coding | 240 | 30% |
| Security | 140 | 17.5% |
| **Total** | **800** | **100%** |

## Training Setup

### Hardware
- 12GB VRAM (Apple Silicon / RTX 3060 equivalent)
- Q8_K_M quantization target (~5.7GB)
- LoRA rank 32, alpha 64

### Hyperparameters
| Param | Value |
|---|---|
| Epochs | 3 |
| Batch size | 4 |
| Gradient accumulation | 4 |
| Learning rate | 2e-4 |
| Warmup steps | 50 |
| Max seq length | 2048 |
| Optimizer | AdamW 8-bit |
| Scheduler | cosine |
| LoRA target modules | q_proj, k_proj, v_proj, o_proj, gate_proj |
| Dropout | 0.05 |

### Base Model
- `Qwen/Qwen3-8B` from Hugging Face
- Apache 2.0 license
- Chat template: Qwen2.5 instruction format

## Fine-Tuning Steps

### 1. Environment Setup
```bash
pip install torch transformers peft trl datasets accelerate bitsandbytes
```

### 2. Dataset Preparation
```python
# scripts/prepare_finetune_dataset.py
# - Load benchmark outputs from ~/.gumi/test-results/
# - Filter by score >= 7
# - Add 200 curated human-written pairs
# - Format with Qwen chat template
# - Split: 90% train, 10% eval
```

### 3. Training
```bash
# scripts/train_qwen3_8b_lora.py
CUDA_VISIBLE_DEVICES=0 python train_qwen3_8b_lora.py \
  --model_name Qwen/Qwen3-8B \
  --dataset_path /tmp/gumi_sft_dataset \
  --output_dir ./models/qwen3-8b-gumi-lora \
  --lora_r 32 --lora_alpha 64 \
  --epochs 3 --batch_size 4 --grad_accum 4 \
  --lr 2e-4 --max_seq_len 2048
```

### 4. Merge & Export
```bash
python merge_lora.py \
  --base_model Qwen/Qwen3-8B \
  --lora_model ./models/qwen3-8b-gumi-lora \
  --output_dir ./models/qwen3-8b-gumi-merged
```

### 5. GGUF Conversion
```bash
python convert_to_gguf.py ./models/qwen3-8b-gumi-merged \
  --outfile ./models/qwen3-8b-gumi-Q8_K_M.gguf \
  --quant Q8_K_M
```

### 6. Ollama Import
```bash
ollama create gumi-qwen3-8b -f ./Modelfile
```

## Validation Pipeline

### Golden Set Test
Run after each epoch:
```bash
python3 scripts/regression_harness.py \
  --model ollama:gumi-qwen3-8b \
  --mode benchmark --questions 3 --save
```

### Stop Conditions
- **PASS:** Overall >= 8.5/10 AND no category < 7.0
- **STOP early:** Overall < 7.5/10 after any epoch (regression)
- **MAX:** 3 epochs

### A/B Test
After fine-tuning:
- 50 coding questions split between base and fine-tuned
- Measure: structure score, correctness, latency
- Accept if > 0.5 point improvement with < 5% latency penalty

## Expected Outcomes

### Baseline vs Target
| Metric | Baseline | Target |
|---|---|---|
| Overall | 7.77/10 | 8.5+/10 |
| Factual | 8.5 | 8.5 |
| Analytical | 7.2 | 8.5+ |
| Coding | 7.2 | 8.5+ |
| Security | 8.2 | 8.5+ |
| Latency delta | — | < +5% |

### Risk Mitigation
- Keep base model untouched
- LoRA adapter only ~300MB, easy to swap
- If regression: discard adapter, fallback to base

## Timeline

| Week | Task |
|---|---|
| Week 1 | Dataset preparation from benchmark outputs + human curation |
| Week 2 | Training + first checkpoint validation |
| Week 3 | Merge, GGUF export, Ollama integration |
| Week 4 | A/B testing + README/docs update |

## Next Actions

1. [ ] Export best responses from `~/.gumi/test-results/` JSON files
2. [ ] Curate 800 SFT examples with Qwen chat template
3. [ ] Set up training environment
4. [ ] Run first LoRA training run
5. [ ] Validate with regression harness

---
Ready to proceed to **Week 1: Dataset preparation**?
