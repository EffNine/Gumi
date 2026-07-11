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

2. Increase the timeout. The default is 60 seconds.
3. For very large context windows or slow hardware, use a smaller model or
   fewer messages.

## Profiles directory missing

If the `profiles/` directory is not next to the `novexa` binary, Novexa falls
back to the built-in `generic-local` profile and prints a warning.

Fix: keep the `profiles/` directory next to the binary. The directory is
included in release archives.

## Unsupported streaming

Streaming responses are not implemented in this alpha. Requests with
`"stream": true` return:

```json
{
  "error": {
    "code": "STREAMING_UNSUPPORTED",
    "message": "streaming chat completions are not supported in this release",
    "type": "runtime_error",
    "engine": "pipeline",
    "suggestion": "Set stream=false until streaming support is implemented.",
    "request_id": "req_..."
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

This warning appears because Novexa is not signed or notarized yet.

## Qwen 3.5 thinking exhausts max tokens

Symptom: requests to Qwen 3.5 models return `VALIDATION_FAILED` with an empty
final answer, or the model is very slow in stabilized mode.

Cause: Qwen 3.5 models (especially the 2B variant) may consume the entire
`max_tokens` budget on internal reasoning/thinking, leaving no tokens for the
final answer. Novexa detects this and returns a clear error.

Fix:

1. Disable thinking through the Novexa extension:

```json
{
  "novexa": {
    "thinking": {
      "enabled": false
    }
  }
}
```

2. The built-in `qwen3.5-2b` profile disables thinking by default. Use the
   profile by requesting `ollama:qwen3.5:2b`.

3. If you need thinking enabled, increase `max_tokens` significantly to give
   the model room for both reasoning and the final answer.

## Why raw reasoning is not returned or logged

Novexa never appends thinking/reasoning text into the assistant final content.
Reasoning text is considered private model internals. Novexa also never stores
actual reasoning text in telemetry by default. Only safe metadata is recorded:

- `thinking_enabled`: true/false/unspecified
- `reasoning_content_present`: true/false

This protects user privacy and prevents accidental exposure of model reasoning
in application output.
