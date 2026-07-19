# System Requirements

Will Gumi run on your machine? Here's the quick answer:

- **OS:** macOS (Apple Silicon or Intel), Linux (AMD64 or ARM64), or Windows 10 19041+ / Windows 11 (AMD64)
- **RAM:** 4 GB minimum for Gumi alone; **8 GB+ recommended** if running a local inference provider
- **Disk:** ~100 MB for Gumi binary + dashboard; model files vary (500 MB–40 GB+)
- **Inference provider:** Ollama, LM Studio, or any OpenAI-compatible local server (not bundled)
- **Browser:** Any modern browser for the dashboard (Chrome, Firefox, Safari, Edge)

If you meet those, you're good. Read on for details.

---

## Minimum Requirements

| Resource | Minimum | Notes |
|---|---|---|
| CPU | x86\_64 or ARM64, 2 cores | Apple Silicon (M1+) performs best |
| RAM | 4 GB | Gumi runtime + dashboard; **does not include** inference provider |
| Disk | ~100 MB | Binary (~50 MB) + dashboard assets (~50 MB) |
| OS | macOS 13+, Ubuntu 20.04+, Windows 10+ | See platform notes below |
| Inference provider | Any local OpenAI-compatible server | Not bundled — install separately |

## Recommended Requirements

| Resource | Recommended | Notes |
|---|---|---|
| CPU | x86\_64 or ARM64, 4+ cores | More cores = faster inference |
| RAM | 8–16 GB | Accommodates Gumi + a 7B-parameter model comfortably |
| Disk | 20 GB free | Gumi + SQLite DB growth + one or more models |
| OS | Latest stable for your platform | Ensures compatibility with inference providers |

---

## Platform-Specific Notes

### macOS

- **Apple Silicon (M1/M2/M3/M4):** Best performance. Gumi binaries are optimized for `darwin-arm64`.
- **Intel Macs:** Fully supported via `darwin-amd64` binaries. Expect slower inference with larger models due to limited RAM bandwidth.
- **Gatekeeper:** The binary may show "unidentified developer" on first run. Run `xattr -d com.apple.quarantine ./gumi` or allow in System Settings → Privacy & Security.

### Linux

- **Ubuntu / Debian:** Pre-built `amd64` and `arm64` tarballs available on [GitHub Releases](https://github.com/EffNine/Gumi/releases).
- **Fedora / Arch:** Same binaries work; ensure `libstdc++` and `ca-certificates` are installed.
- **Container:** Docker images built on Alpine with CGO disabled for a fully static binary. See [installation.md](./installation.md) for Docker setup.
- **ARM SBCs:** Raspberry Pi 4/5 (ARM64) is supported. Use the `linux-arm64` release.

### Windows

- **Supported:** Windows 10 version 19041+ (October 2019 Update) and Windows 11.
- **Architecture:** AMD64 only. No ARM64 Windows release is available yet.
- **PowerShell:** May block the binary with SmartScreen on first run. Run
  `Unblock-File .\gumi.exe` after extraction, or click **More info → Run
  anyway** on the SmartScreen dialog.
- **WSL2:** Gumi can run inside WSL2, but the dashboard and inference provider
  must both be accessible from within the WSL2 network namespace. Native Windows
  installation is preferred for GPU access (DirectML / CUDA via WSL2 requires
  additional configuration).
- **Firewall:** Windows Defender Firewall may prompt to allow Gumi on first run.
  Allow on **Private** networks for local development.
- **Database path:** Telemetry is stored at `%APPDATA%\gumi\gumi.db` on Windows
  (equivalent to `~/.gumi/gumi.db` on macOS/Linux).

---

## Inference Provider Requirements

Gumi does **not** bundle a model or inference engine. You must run a local inference provider alongside Gumi.

### Ollama

| Model Size | Params | Approx. RAM | Disk |
|---|---|---|---|
| Tiny | 0.5B–1B | ~0.5 GB | ~1 GB |
| Small | 3B | ~2 GB | ~2 GB |
| Medium | 7B | ~4 GB | ~5 GB |
| Large | 13B | ~8 GB | ~10 GB |
| XL | 30B–34B | ~18 GB | ~20 GB |
| XXL | 70B+ | ~40 GB | ~40 GB |

- **Install:** `curl -fsSL https://ollama.com/install.sh | sh` (macOS/Linux) or download from [ollama.com](https://ollama.com).
- **Default URL:** `http://localhost:11434`
- **Start:** `ollama serve` (background daemon on macOS/Linux; service on Windows).
- **Pull a model:** `ollama pull qwen2.5-coder:1.5b` (small models load faster on constrained hardware).

### LM Studio

- Download from [lmstudio.ai](https://lmstudio.ai).
- Start the local server from the UI (port 1234 by default).
- Gumi connects via the `provider:model` naming scheme, e.g. `lmstudio:qwen2.5-coder-7b-instruct`.
- RAM needs match the table above; LM Studio loads the full model into memory.

### Other OpenAI-Compatible Servers

Any server that implements the OpenAI `/v1/chat/completions` endpoint works. Common options:

| Server | RAM overhead | Notes |
|---|---|---|
| vLLM | Model weights only | Best throughput; Linux-focused |
| text-generation-webui | Model weights only | Flexible, Python-based |
| llama.cpp server | Model weights only | Lightweight, C++ |
| Hugging Face TGI | Model weights only | Production-grade, heavier |

**RAM rule of thumb:** The inference provider needs roughly **0.5 GB per 1B parameters** in FP16 quantization. Gumi itself adds ~50 MB. Plan accordingly.

---

## Storage Requirements

| Component | Size | Growth |
|---|---|---|
| Gumi binary | ~50 MB | Static per release |
| Dashboard assets | ~50 MB | Static (bundled with binary) |
| SQLite telemetry DB | ~5–50 MB | Grows with request volume; ~1 KB per recorded request |
| Profile YAMLs | Negligible | Static |
| Model files | Varies widely | See inference provider table above |

The SQLite database lives at `~/.gumi/gumi.db` (or `%APPDATA%\gumi\gumi.db` on Windows). It stores request/response telemetry, health checks, and benchmark results. No separate database service is needed.

---

## Network Requirements

- **Localhost only by default.** Gumi binds to `127.0.0.1` on ports **8787** (API) and **8788** (dashboard). No external network access is required.
- **No internet needed** after installation. Gumi, its telemetry, and its tuning profiles are fully self-contained.
- **Inference provider** must be reachable on localhost (or a custom URL configured in `gumi.example.yaml`).
- **Firewall:** If you bind to `0.0.0.0` (not recommended for local use), ensure your firewall rules restrict access appropriately.

Ports can be customized:

```bash
./gumi start --port 8790 --dashboard-port 8791
```

See [troubleshooting.md](./troubleshooting.md) if ports are already in use.

---

## Browser Compatibility

The dashboard is a React 19 + Vite application. It works in:

| Browser | Version |
|---|---|
| Chrome | 120+ |
| Firefox | 121+ |
| Safari | 17+ |
| Edge | 120+ |

Features used:

- **WebSockets** — for real-time request streaming display
- **LocalStorage** — for dashboard preferences (theme, pagination)
- **Fetch API** — for all dashboard ↔ API communication

No special browser extensions or configurations are required.

---

## Quick Checklist

Before starting Gumi, verify:

- [ ] OS is supported (macOS 13+, Linux x86_64/ARM64, Windows 10 19041+/11 AMD64)
- [ ] At least 4 GB RAM available (8 GB+ if running inference locally)
- [ ] ~100 MB disk free for Gumi; additional space for models
- [ ] Inference provider installed and running (Ollama, LM Studio, etc.)
- [ ] Ports 8787 and 8788 are available (or configured with `--port` / `--dashboard-port`)
- [ ] Modern browser for dashboard access

Need help? Run `./gumi doctor` for an automated diagnostic, or see [troubleshooting.md](./troubleshooting.md).
