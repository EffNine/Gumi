# Model artifact provenance — Experiment 01

| Field | Value |
|---|---|
| Source | Hugging Face `unsloth/Qwen3-30B-A3B-GGUF` (main) |
| URL | https://huggingface.co/unsloth/Qwen3-30B-A3B-GGUF/resolve/main/Qwen3-30B-A3B-Q4_K_M.gguf |
| Filename | Qwen3-30B-A3B-Q4_K_M.gguf |
| Local path | ~/models/local/Qwen3-30B-A3B-Q4_K_M.gguf |
| Size | 18,556,686,912 bytes (17.28 GiB) — matches source-declared LFS size exactly |
| SHA256 | 9f1a24700a339b09c06009b729b5c809e0b64c213b8af5b711b3dbdfd0c5ba48 (matches source LFS oid; verified locally via `sha256sum -c`) |

## Backend provenance

| Field | Value |
|---|---|
| Binary | llama-cli, version 10360 (90e6a9131), Unsloth build |
| Origin bundle | github.com/unslothai/llama.cpp release `b10360-mix-87da1a2`, asset `app-b10360-mix-87da1a2-linux-x64-cuda13-newer.tar.gz` |
| Bundle SHA256 | fab818cf61711f1a0cb84406a0eaa49b209994d8a526313c14526e8bee77ef5f (verified before extraction) |
| Installed to | ~/.unsloth/llama.cpp/build/bin/ alongside the pre-existing same-version runtime libs (RUNPATH $ORIGIN); symlinked to ~/.local/bin/llama-cli |
| GPU detection | `llama-cli --list-devices` → CUDA0: NVIDIA GeForce RTX 5070 (11774 MiB, 11523 MiB free) |

Same version as the pre-existing `llama-server` v10360 install — one consistent backend across the experiment.

## Verified GGUF metadata (`gumi inspect`, not filename-derived)

```
Architecture: qwen3moe          ✓ required
Name:         Qwen3-30B-A3B     ✓ required
Quantization: Q4_K_M            ✓ required (from general.file_type)
Layers:       48
Heads:        32 (KV: 4, head_dim 128)
Train ctx:    40960 tokens
MoE experts:  128 total, 8 active, expert ffn 768
Expert bytes: 16.35 GB of weights
KV/token:     98304 bytes at f16 (= 96 KiB — matches Gumi's unit-tested geometry)
File size:    17.28 GB
Parameters:   ~30.5B (exact tensor-element sum 30532122624)
```
