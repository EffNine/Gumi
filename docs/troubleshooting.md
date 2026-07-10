# Troubleshooting

This guide explains common first-run problems and how to fix them. If you do not
see your issue here, run `novexa doctor` for a quick diagnosis.

## Port 8787 already in use

Symptom:

```text
Novexa could not start.
Reason: API port 8787 is already in use.
```

Fix:

1. Find the process using the port:

```bash
# macOS / Linux
lsof -i :8787

# Windows PowerShell
Get-NetTCPConnection -LocalPort 8787
```

2. Stop that process, or start Novexa on a different port:

```bash
./novexa start --port 8790
```

## Port 8788 already in use

Start the dashboard on a different port:

```bash
./novexa start --dashboard-port 8791
```

## Ollama unavailable

Symptom:

```text
Ollama is not reachable at http://localhost:11434.
```

Fix:

1. Make sure Ollama is running:

```bash
ollama serve
```

2. Check that the URL in the default config matches your Ollama server. The
   default is `http://localhost:11434`.
3. If Ollama runs on a different host or port, update `providers.ollama.url`
   in `novexa.example.yaml` for reference; YAML config loading will be enabled
   after the alpha release.

## Model not installed

Symptom:

```text
Default model qwen3:8b is not installed in Ollama.
```

Fix:

```bash
ollama pull qwen3:8b
```

Or start Novexa with an installed model:

```bash
./novexa start --model llama3
```

## Dashboard build missing

Symptom: opening http://127.0.0.1:8788 shows a page saying the dashboard build
was not found.

Fix: build the dashboard and restart Novexa:

```bash
cd dashboard
npm install
npm run build
cd ..
./novexa start
```

Release archives already include `dashboard/dist`, so this only happens when
building from source without running `make dashboard` first.

## SQLite permission error

Symptom:

```text
open telemetry storage: create storage directory: mkdir ~/.novexa: permission denied
```

Fix:

1. Make sure your home directory is writable:

```bash
mkdir -p ~/.novexa
```

2. If you run Novexa in Docker, make sure the `/data` volume is writable by the
   container user:

```bash
docker run -v novexa-data:/data novexa:0.1.0-alpha
```

The runtime uses `~/.novexa/novexa.db` by default. In the official Docker
image, the database is stored at `/data/.novexa/novexa.db`.

## Invalid API key

Symptom: requests return `401 Unauthorized` with error code `INVALID_API_KEY`.

Fix: use the local key shown at startup (`novexa-local` by default):

```bash
curl http://127.0.0.1:8787/v1/models \
  -H "Authorization: Bearer novexa-local"
```

If you changed the key, pass the new one.

## Provider timeout

Symptom:

```text
provider timeout
```

Fix:

1. Check that the provider is healthy:

```bash
./novexa doctor
```

2. Increase the timeout in the example configuration. The default is 60 seconds.
3. For very large context windows or slow hardware, use a smaller model or
   fewer messages.

## Profiles directory missing

If the `profiles/` directory is not next to the `novexa` binary, Novexa falls
back to the built-in `generic-local` profile and prints a warning.

Fix: keep the `profiles/` directory next to the binary, or when running from
source make sure you start Novexa from the repository root:

```bash
cd /path/to/novexa
./novexa start
```

## Unsupported streaming

Streaming responses are not implemented in this alpha. Requests with
`"stream": true` return:

```json
{
  "error": {
    "code": "STREAMING_UNSUPPORTED",
    "message": "Streaming is not supported in this release."
  }
}
```

Fix: send non-streaming requests (`"stream": false`, or omit the field).

## macOS executable quarantine warning

Symptom: macOS shows a dialog saying `novexa` cannot be opened because the
developer cannot be verified.

Fix (choose one):

1. Remove the quarantine attribute:

```bash
xattr -d com.apple.quarantine ./novexa
```

2. If that does not work, allow the binary in **System Settings > Privacy &
   Security > Security** and click **Allow Anyway** after attempting to run it.

This warning appears because Novexa is not signed or notarized yet. Building from
source avoids the warning entirely.
