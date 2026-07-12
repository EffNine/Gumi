# Installation Guide

This guide covers installing Novexa from a release archive or as a Docker
container. Novexa is a local-first runtime, so every installation method keeps
the API and dashboard bound to `127.0.0.1` by default.

Novexa is distributed as pre-built binary releases. The public repository
contains reference source code and documentation; building from source
requires the private development branch which includes additional internal
packages not present in the public tree.

## Requirements

- A local inference provider such as [Ollama](https://ollama.com),
  [LM Studio](https://lmstudio.ai), or any OpenAI-compatible local server.

## Download a release archive

1. Visit the [GitHub releases](https://github.com/EffNine/Novexa/releases) page.
2. Download the archive for your OS and architecture:
   - macOS Apple Silicon: `novexa-<version>-darwin-arm64.tar.gz`
   - macOS Intel: `novexa-<version>-darwin-amd64.tar.gz`
   - Linux AMD64: `novexa-<version>-linux-amd64.tar.gz`
   - Linux ARM64: `novexa-<version>-linux-arm64.tar.gz`
   - Windows AMD64: `novexa-<version>-windows-amd64.zip`
3. Verify the SHA256 checksum from `SHA256SUMS.txt`.
4. Extract the archive and run the binary from the extracted directory.

Example (macOS Apple Silicon):

```bash
tar -xzf novexa-0.1.0-alpha-darwin-arm64.tar.gz
cd novexa-0.1.0-alpha-darwin-arm64
./novexa version
./novexa start
```

Each archive contains:

```text
novexa-<version>-<os>-<arch>/
├── novexa                 # novexa.exe on Windows
├── dashboard/dist/
├── profiles/
├── novexa.example.yaml
├── README.md
├── LICENSE
└── CHANGELOG.md
```

## Docker

A `Dockerfile` is included in the private development branch. If you have
access, build the image:

```bash
docker build -t novexa:0.1.0-alpha .
```

Alternatively, use the pre-built binary inside a minimal image:

```dockerfile
FROM alpine:latest
COPY novexa /usr/local/bin/
COPY dashboard/dist/ /opt/novexa/dashboard/dist/
COPY profiles/ /opt/novexa/profiles/
EXPOSE 8787 8788
CMD ["novexa", "start"]
```

Run with the API and dashboard published only to localhost, and persist the
SQLite database in a Docker volume:

```bash
docker run -d \
  --name novexa \
  -p 127.0.0.1:8787:8787 \
  -p 127.0.0.1:8788:8788 \
  -v novexa-data:/data \
  novexa:0.1.0-alpha
```

The runtime stores telemetry at `/data/.novexa/novexa.db` because the container
runs as a non-root user whose home directory is `/data`.

Stop and remove:

```bash
docker stop novexa
docker rm novexa
```

## macOS

### Apple Silicon (M1/M2/M3/M4)

Download the `darwin-arm64` archive. The binary is native arm64 and does not
need Rosetta.

If macOS shows a security warning the first time you run `./novexa`, see
[macOS executable quarantine warning](./troubleshooting.md#macos-executable-quarantine-warning)
in the troubleshooting guide.

### Intel (amd64)

Download the `darwin-amd64` archive. The binary runs on Intel Macs without any
additional translation layer.

## Linux

Download the `linux-amd64` or `linux-arm64` archive, extract it, and run the
binary. No additional dependencies are required.

```bash
tar -xzf novexa-0.1.0-alpha-linux-amd64.tar.gz
cd novexa-0.1.0-alpha-linux-amd64
./novexa start
```

## Windows

1. Download the `windows-amd64.zip` archive.
2. Extract it with File Explorer or PowerShell:

```powershell
Expand-Archive -Path novexa-0.1.0-alpha-windows-amd64.zip -DestinationPath novexa
```

3. Run the binary in PowerShell:

```powershell
cd novexa
.\novexa.exe version
.\novexa.exe start
```

Windows Defender may warn about an unrecognized binary. You can click
"More info" and "Run anyway" for a local development tool.

## Start Novexa

From the extracted archive:

```bash
./novexa start
```

You should see output similar to:

```text
Novexa Runtime 0.1.0

API        http://127.0.0.1:8787/v1
Dashboard  http://127.0.0.1:8788
Mode       stabilized
Provider   ollama
Model      local:auto

Status     ready
```

Useful flags:

```bash
./novexa start --port 8790
./novexa start --dashboard-port 8791
./novexa start --provider ollama --model qwen3:8b
```

## Open the dashboard

Once the runtime is running, open:

```text
http://127.0.0.1:8788
```

The dashboard shows runtime status, provider health, recent telemetry, and
diagnostic checks.

## Connect an OpenAI-compatible client

Set the base URL and API key to point at the local Novexa runtime:

```bash
export OPENAI_BASE_URL=http://127.0.0.1:8787/v1
export OPENAI_API_KEY=novexa-local
```

Example with cURL:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{"model":"local:auto","messages":[{"role":"user","content":"Hello"}]}'
```

Example with the Python OpenAI SDK:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/v1",
    api_key="novexa-local",
)

response = client.chat.completions.create(
    model="local:auto",
    messages=[{"role": "user", "content": "Hello"}],
)
print(response.choices[0].message.content)
```

## Uninstall Novexa

Simply delete the extracted directory.

To remove local telemetry and logs:

```bash
rm -rf ~/.novexa
```

For Docker:

```bash
docker stop novexa
docker rm novexa
docker volume rm novexa-data
```
