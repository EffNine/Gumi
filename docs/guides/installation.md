# Installation Guide

This guide covers installing Gumi from source, from a release archive, or as a
Docker container. Gumi is a local-first runtime, so every installation method
keeps the API and dashboard bound to `127.0.0.1` by default.

## Requirements

- A local inference provider such as [Ollama](https://ollama.com),
  [LM Studio](https://lmstudio.ai), or any OpenAI-compatible local server.
- For building from source: Go 1.25+ and Node.js 22+ with npm.

## Build from source

```bash
git clone https://github.com/EffNine/Gumi.git
cd gumi
make build
```

`make build` runs:

1. `npm ci` and `npm run build` in `dashboard/`
2. `go build -ldflags ...` in `runtime/`

The resulting `gumi` binary is placed in the repository root and serves the
dashboard from `dashboard/dist`.

## Download a release archive

1. Visit the [GitHub releases](https://github.com/EffNine/Gumi/releases) page.
2. Download the archive for your OS and architecture:
   - macOS Apple Silicon: `gumi-<version>-darwin-arm64.tar.gz`
   - macOS Intel: `gumi-<version>-darwin-amd64.tar.gz`
   - Linux AMD64: `gumi-<version>-linux-amd64.tar.gz`
   - Linux ARM64: `gumi-<version>-linux-arm64.tar.gz`
   - Windows AMD64: `gumi-<version>-windows-amd64.zip`
3. Verify the SHA256 checksum from `SHA256SUMS.txt`.
4. Extract the archive and run the binary from the extracted directory.

Example (macOS Apple Silicon):

```bash
tar -xzf gumi-v1.0.0-rc1-darwin-arm64.tar.gz
cd gumi-v1.0.0-rc1-darwin-arm64
./gumi version
./gumi start
```

Each archive contains:

```text
gumi-<version>-<os>-<arch>/
├── gumi                 # gumi.exe on Windows
├── dashboard/dist/
├── profiles/
├── gumi.example.yaml
├── README.md
├── LICENSE
└── CHANGELOG.md
```

### Install to PATH with install.sh

From an extracted release archive or the repository root after `make build`:

```bash
./scripts/install.sh
```

This installs the binary and assets to `/usr/local/lib/gumi` and creates a
symlink at `/usr/local/bin/gumi`.

## Docker

Build the image:

```bash
docker build -t gumi:v1.0.0-rc1 .
```

Run with the API and dashboard published only to localhost, and persist the
SQLite database in a Docker volume:

```bash
docker run -d \
  --name gumi \
  -p 127.0.0.1:8787:8787 \
  -p 127.0.0.1:8788:8788 \
  -v gumi-data:/data \
  gumi:v1.0.0-rc1
```

The runtime stores telemetry at `/data/.gumi/gumi.db` because the container
runs as a non-root user whose home directory is `/data`.

Stop and remove:

```bash
docker stop gumi
docker rm gumi
```

## macOS

### Apple Silicon (M1/M2/M3/M4)

Download the `darwin-arm64` archive. The binary is native arm64 and does not
need Rosetta.

If macOS shows a security warning the first time you run `./gumi`, see
[macOS executable quarantine warning](./troubleshooting.md#macos-executable-quarantine-warning)
in the troubleshooting guide.

### Intel (amd64)

Download the `darwin-amd64` archive. The binary runs on Intel Macs without any
additional translation layer.

## Linux

Download the `linux-amd64` or `linux-arm64` archive, extract it, and run the
binary. No additional dependencies are required.

```bash
tar -xzf gumi-v1.0.0-rc1-linux-amd64.tar.gz
cd gumi-v1.0.0-rc1-linux-amd64
./gumi start
```

## Windows

### Prerequisites

- **PowerShell 5.1+** (included by default on Windows 10/11) or **Command Prompt**.
- If you plan to build from source: [Go 1.25+](https://golang.org/dl/) and [Node.js 22+](https://nodejs.org/).
- A local inference provider (Ollama, LM Studio, or any OpenAI-compatible server).

### Execution policy

Windows may block unsigned executables. If you see an error like

```
.\gumi.exe cannot be loaded because running scripts is disabled on this system
```

enable script execution for your user account:

```powershell
Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned
```

This allows locally-written scripts to run while still requiring downloaded scripts to be signed.

### Manual install from release ZIP

1. Download `gumi-<version>-windows-amd64.zip` from the [GitHub releases](https://github.com/EffNine/Gumi/releases) page.
2. Verify the SHA256 checksum from `SHA256SUMS.txt`.
3. Extract the archive (double-click in File Explorer, or via PowerShell):

```powershell
Expand-Archive -Path gumi-v1.0.0-rc1-windows-amd64.zip -DestinationPath gumi
cd gumi
```

4. Unblock the binary (bypasses SmartScreen for local use):

```powershell
Unblock-File .\gumi.exe
```

5. Run:

```powershell
.\gumi.exe version
.\gumi.exe start
```

### Using the install script via WSL2 or Git Bash

The `scripts/gumi-install.sh` script (which installs to `/usr/local/bin`) works inside **WSL2** or **Git Bash**:

```bash
# Inside WSL2 or Git Bash
chmod +x scripts/gumi-install.sh
./scripts/gumi-install.sh
```

After installation, `gumi` will be available on your `$PATH`.

### SmartScreen and antivirus

On first run, Windows SmartScreen may show a warning:

> **SmartScreen prevented an unrecognized app from starting.**
> Running this app may put your PC at risk.

Since Gumi is a local development tool and not yet code-signed:

1. Click **More info**.
2. Click **Run anyway**.

This is safe because you downloaded the binary from the official GitHub releases page and verified the checksum.

Some antivirus programs may also flag the binary as a false positive. To mitigate:

- Add the Gumi installation directory to your antivirus exclusions.
- In Windows Security: **Virus & threat protection → Manage settings → Exclusions → Add an exclusion → Folder**, then select the Gumi directory.

### Firewall and port binding

When you run `gumi.exe start`, Windows Defender Firewall may prompt you to allow Gumi through the firewall:

> **Allow Gumi to communicate on private/public networks?**

- Select **Private** for home or office networks.
- Select **Public** only if you explicitly need remote access (not recommended for local development).

If you accidentally blocked Gumi and it won't start, allow it manually:

```powershell
New-NetFirewallRule -DisplayName "Gumi API" -Direction Inbound -Protocol TCP -LocalPort 8787 -Action Allow
New-NetFirewallRule -DisplayName "Gumi Dashboard" -Direction Inbound -Protocol TCP -LocalPort 8788 -Action Allow
```

By default Gumi binds to `127.0.0.1` only, so it is not exposed to the network even if the firewall rule exists.

### CMD vs PowerShell

You can run Gumi from either **Command Prompt (CMD)** or **PowerShell**:

```cmd
:: CMD
gumi.exe version
gumi.exe start
```

```powershell
# PowerShell
.\gumi.exe version
.\gumi.exe start
```

The `.\` prefix is required in PowerShell to invoke a local executable. In CMD, it is optional.

## Start Gumi

From the extracted archive or after `make build`:

```bash
./gumi start
```

You should see output similar to:

```text
Gumi Runtime 0.1.0

API        http://127.0.0.1:8787/v1
Dashboard  http://127.0.0.1:8788
Mode       stabilized
Provider   ollama
Model      local:auto

Status     ready
```

Useful flags:

```bash
./gumi start --port 8790
./gumi start --dashboard-port 8791
./gumi start --provider ollama --model qwen3:8b
```

## Open the dashboard

Once the runtime is running, open:

```text
http://127.0.0.1:8788
```

The dashboard is served from the bundled `dashboard/dist` directory. It shows
runtime status, provider health, recent telemetry, and diagnostic checks.

## Connect an OpenAI-compatible client

Set the base URL and API key to point at the local Gumi runtime:

```bash
export OPENAI_BASE_URL=http://127.0.0.1:8787/v1
export OPENAI_API_KEY=gumi-local
```

Example with cURL:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}'
```

Example with the Python OpenAI SDK:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/v1",
    api_key="gumi-local",
)

response = client.chat.completions.create(
    model="local:auto",
    messages=[{"role": "user", "content": "Hello"}],
)
print(response.choices[0].message.content)
```

## Uninstall Gumi

If you used `scripts/install.sh`:

```bash
rm -f /usr/local/bin/gumi
rm -rf /usr/local/lib/gumi
```

If you used a release archive, simply delete the extracted directory.

To remove local telemetry and logs:

```bash
rm -rf ~/.gumi
```

For Docker:

```bash
docker stop gumi
docker rm gumi
docker volume rm gumi-data
```
