# Troubleshooting

This guide explains common first-run problems and how to fix them. If you do not
see your issue here, run `gumi doctor` for a quick diagnosis.

## Port 8787 already in use

Symptom:

```text
Gumi could not start.
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

2. Stop that process, or start Gumi on a different port:

```bash
./gumi start --port 8790
```

## Port 8788 already in use

Start the dashboard on a different port:

```bash
./gumi start --dashboard-port 8791
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
   in `gumi.example.yaml` for reference; YAML config loading will be enabled
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

Or start Gumi with an installed model:

```bash
./gumi start --model llama3
```

## Dashboard build missing

Symptom: opening http://127.0.0.1:8788 shows a page saying the dashboard build
was not found.

Fix: build the dashboard and restart Gumi:

```bash
cd dashboard
npm install
npm run build
cd ..
./gumi start
```

Release archives already include `dashboard/dist`, so this only happens when
building from source without running `make dashboard` first.

## SQLite permission error

Symptom:

```text
open telemetry storage: create storage directory: mkdir ~/.gumi: permission denied
```

Fix:

1. Make sure your home directory is writable:

```bash
mkdir -p ~/.gumi
```

2. If you run Gumi in Docker, make sure the `/data` volume is writable by the
   container user:

```bash
docker run -v gumi-data:/data gumi:0.1.0-alpha
```

The runtime uses `~/.gumi/gumi.db` by default. In the official Docker
image, the database is stored at `/data/.gumi/gumi.db`.

## Invalid API key

Symptom: requests return `401 Unauthorized` with error code `INVALID_API_KEY`.

Fix: use the local key shown at startup (`gumi-local` by default):

```bash
curl http://127.0.0.1:8787/v1/models \
  -H "Authorization: Bearer gumi-local"
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
./gumi doctor
```

2. Increase the timeout in the example configuration. The default is 60 seconds.
3. For very large context windows or slow hardware, use a smaller model or
   fewer messages.

## Intermittent 502 errors (LM Studio model loading)

Symptom: approximately 50% of requests through Gumi fail with:

```json
{
  "error": {
    "code": "PROVIDER_BAD_RESPONSE",
    "message": "lmstudio rejected the request",
    "suggestion": "Review the request payload and provider logs."
  }
}
```

And the Gumi error database shows:

```text
"Failed to load model ... Engine protocol startup was aborted"
```

Cause: LM Studio unloads and reloads models from memory between requests. When
a model is being loaded, the server returns a 400 error for 3-10 seconds. This
is a transient condition, not a request payload problem.

Fix: Gumi Sprint 12+ automatically retries with exponential backoff
(3s → 6s for model-loading errors). If you still see frequent 502s:

1. In LM Studio, enable **Keep model in memory** to prevent model swapping.
2. Reduce the number of models loaded simultaneously in LM Studio.
3. Ensure sufficient RAM for the model size (9B Q4 models need ~6GB).
4. Check the error database for details:

```bash
sqlite3 ~/.gumi/gumi.db \
  "SELECT code, COUNT(*) FROM errors GROUP BY code ORDER BY COUNT(*) DESC;"
```

## LM Studio provider not reachable

Symptom:

```text
PROVIDER_UNAVAILABLE: lmstudio is not reachable (connection refused)
```

Cause: Gumi defaults to the Ollama provider. To use LM Studio, set the
provider and URL via environment variables:

```bash
GUMI_PROVIDER_DEFAULT=lmstudio \
GUMI_LMSTUDIO_URL=http://localhost:1234/v1 \
GUMI_DEFAULT_MODEL=ornith-1.0-9b@q4_k_m \
./gumi start --verbose
```

If LM Studio runs on a different machine (e.g. LAN), use that IP:

```bash
GUMI_LMSTUDIO_URL=http://192.168.0.164:1234/v1
```

Verify LM Studio is running:

```bash
curl http://localhost:1234/v1/models
```

## Validation failures on JSON output

Symptom: requests in stabilized or structured mode return `VALIDATION_FAILED`
even though the response looks like valid JSON.

Cause (pre-Sprint 12): the repetition detector flagged JSON structural elements
(`}`, repeated keys across array objects) as repetition. This is fixed in
Sprint 12 — repetition detection is skipped for JSON output.

To diagnose validation failures, query the validation reports table:

```bash
sqlite3 ~/.gumi/gumi.db \
  "SELECT request_id, passed, severity, issues_json FROM validation_reports WHERE passed=0 ORDER BY id DESC LIMIT 5;"
```

And the error details:

```bash
sqlite3 ~/.gumi/gumi.db \
  "SELECT request_id, details_json FROM errors WHERE code='VALIDATION_FAILED' ORDER BY created_at DESC LIMIT 5;"
```

## Profiles directory missing

If the `profiles/` directory is not next to the `gumi` binary, Gumi falls
back to the built-in `generic-local` profile and prints a warning.

Fix: keep the `profiles/` directory next to the binary, or when running from
source make sure you start Gumi from the repository root:

```bash
cd /path/to/gumi
./gumi start
```

## Streaming

Streaming chat completions (`stream: true`) are now supported through SSE.

However, structured mode + streaming is rejected (post-hoc repair is impossible
mid-stream). Use `stream: false` with `response_format` for structured output.

If you get a `STREAMING_UNSUPPORTED` error with structured mode:
set `"stream": false` (or omit `stream`) and retry with `response_format` set.

## macOS executable quarantine warning

Symptom: macOS shows a dialog saying `gumi` cannot be opened because the
developer cannot be verified.

Fix (choose one):

1. Remove the quarantine attribute:

```bash
xattr -d com.apple.quarantine ./gumi
```

2. If that does not work, allow the binary in **System Settings > Privacy &
   Security > Security** and click **Allow Anyway** after attempting to run it.

This warning appears because Gumi is not signed or notarized yet. Building from
source avoids the warning entirely.

## Qwen 3.5 thinking exhausts max tokens

Symptom: requests to Qwen 3.5 models return `VALIDATION_FAILED` with an empty
final answer, or the model is very slow in stabilized mode.

Cause: Qwen 3.5 models (especially the 2B variant) may consume the entire
`max_tokens` budget on internal reasoning/thinking, leaving no tokens for the
final answer. Gumi detects this and returns a clear error.

Fix:

1. Disable thinking through the Gumi extension:

```json
{
  "gumi": {
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

Gumi never appends thinking/reasoning text into the assistant final content.
Reasoning text is considered private model internals. Gumi also never stores
actual reasoning text in telemetry by default. Only safe metadata is recorded:

- `thinking_enabled`: true/false/unspecified
- `reasoning_content_present`: true/false

This protects user privacy and prevents accidental exposure of model reasoning
in application output.
