# Model Inventory

**Date:** 2026-08-07  
**Provider:** Ollama  
**Hardware:** NVIDIA RTX 5070 (12GB VRAM), 30GB RAM  
**Ollama Version:** 0.32.6

---

## Discovered Models

| Model | Provider | Quantization | Parameters | Context Length | File Size |
|-------|----------|-------------|------------|---------------|-----------|
| qwen3:8b | Ollama | Q4_K_M | 8.2B | 40,960 | 5.2 GB |
| gemma3:4b | Ollama | Q4_K_M | 4.3B | N/A | 3.3 GB |
| llama3.1:8b | Ollama | Q4_K_M | 8.0B | 131,072 | 4.9 GB |

---

## Model Details

### Qwen3 8B
- **Full name:** qwen3:8b
- **Family:** qwen3
- **Format:** GGUF
- **Quantization:** Q4_K_M
- **Parameters:** 8.2B
- **Context length:** 40,960 tokens
- **Embedding length:** 4,096
- **Capabilities:** completion, tools, thinking
- **File size:** 5.2 GB
- **Digest:** 500a1f067a9f782620b40bee6f7b0c89e17ae61f686b92c24933e4ca4b2b8b41
- **VRAM estimate:** ~6.5 GB (Q4 quantized 8B model)

### Gemma 3 4B
- **Full name:** gemma3:4b
- **Family:** gemma3
- **Format:** GGUF
- **Quantization:** Q4_K_M
- **Parameters:** 4.3B
- **Capabilities:** completion
- **File size:** 3.3 GB
- **Digest:** a2af6cc3eb7fa8be8504abaf9b04e88f17a119ec3f04a3addf55f92841195f5a
- **VRAM estimate:** ~3 GB (Q4 quantized 4B model)

### Llama 3.1 8B
- **Full name:** llama3.1:8b
- **Family:** llama
- **Format:** GGUF
- **Quantization:** Q4_K_M
- **Parameters:** 8.0B
- **Context length:** 131,072 tokens
- **Embedding length:** 4,096
- **Capabilities:** completion, tools
- **File size:** 4.9 GB
- **Digest:** 46e0c10c039e019119339687c3c1757cc81b9da49709a3b3924863ba87ca666e
- **VRAM estimate:** ~6 GB (Q4 quantized 8B model)

---

## GPU Memory Analysis

| Model | Est. VRAM | Available | Status |
|-------|-----------|-----------|--------|
| qwen3:8b | ~6.5 GB | 12 GB | OK |
| gemma3:4b | ~3 GB | 12 GB | OK |
| llama3.1:8b | ~6 GB | 12 GB | OK |

All models fit within the 12 GB VRAM limit. The Gemma 3 4B model is the most memory-efficient option if multiple models need to coexist.

---

## Provider Connectivity

- **Ollama:** Running on `http://localhost:11434`
- **LM Studio:** Not installed/running
