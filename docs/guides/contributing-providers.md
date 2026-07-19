# Contributing Provider Adapters

Gumi connects to local inference providers through thin adapter implementations.
This guide explains how to add support for a new provider.

> **Rule of thumb:** Provider adapters are translators, not brains. They map
> Gumi's canonical `api.ChatCompletionRequest` → provider wire format and back.
> Prompt optimisation, validation, retry logic, and routing belong in the
> Pipeline Engine, not in adapters.

## Overview

Every provider adapter is a Go struct that implements the
[`ProviderAdapter`](#the-provideradapter-interface) interface defined in
`runtime/internal/provider/adapter.go`. Adapters are stateless HTTP clients
that talk to a local server (Ollama, LM Studio, vLLM, llama.cpp, etc.).

Why thin adapters matter:

- **Swappable backends.** The rest of the runtime talks only to the interface,
  never to a concrete provider.
- **Testable.** Each adapter can be unit-tested with `httptest.Server` without
  requiring the real provider to be installed.
- **Future-proof.** The Pipeline Engine will consume adapters without knowing
  which concrete type is underneath.

## The ProviderAdapter Interface

```go
type ProviderAdapter interface {
    // Name returns the provider key used in config and model IDs.
    Name() string

    // Type returns a human-readable provider type.
    Type() string

    // HealthCheck probes the provider and returns its status.
    HealthCheck(ctx context.Context) (ProviderStatus, error)

    // ListModels returns the models currently available from the provider.
    ListModels(ctx context.Context) ([]ModelInfo, error)

    // Generate performs a non-streaming chat completion.
    Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error)

    // GenerateStream performs a streaming chat completion. It returns:
    //   - chunkCh: receives ChatCompletionChunk values as they arrive
    //   - errCh: receives exactly one terminal error or nil when the stream closes
    //   - setupErr: a synchronous error if the stream could not be started
    // Adapters that cannot stream return provider.StreamingUnsupported from setupErr.
    GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (chunkCh <-chan api.ChatCompletionChunk, errCh <-chan error, setupErr error)

    // Capabilities reports adapter capabilities.
    Capabilities() Capabilities

    // NormalizeError maps a raw error to a normalized ProviderError.
    NormalizeError(err error) ProviderError
}
```

### Supporting types

```go
type ProviderStatus string // "ok", "offline", "degraded", "misconfigured", "unknown"

type ModelInfo struct {
    ID        string
    Name      string
    Provider  string
    CreatedAt time.Time
}

type Capabilities struct {
    Streaming        bool
    ToolUse          bool
    StructuredOutput bool
}

type ProviderError struct {
    Code       ProviderErrorCode // "PROVIDER_UNAVAILABLE", "PROVIDER_TIMEOUT", etc.
    Message    string
    Suggestion string
    Cause      error
}
```

Normalized error codes are defined in `runtime/internal/provider/errors.go`.
Use `NormalizeHTTPError()` (also in that file) for HTTP-level error mapping.

## Step-by-Step Guide

### 1. Understand the interface

Read `runtime/internal/provider/adapter.go` end-to-end. Study at least one
existing adapter — `ollama.go` is the simplest, `lmstudio.go` shows model
management extras, and `openai_local.go` demonstrates a generic OpenAI-compatible
client.

Key principles:

- **Stateless HTTP client.** Store only `baseURL`, `timeout`, `*http.Client`,
  and `*logger.Logger`. Do not cache model weights or maintain connection pools
  beyond what `http.Client` gives you.
- **Context propagation.** Every HTTP request must use `http.NewRequestWithContext`
  so that timeouts and cancellations propagate correctly.
- **Canonical types in, canonical types out.** Accept `api.ChatCompletionRequest`,
  return `*api.ChatCompletionResponse`. Translate to/from the provider's wire
  format in between.

### 2. Create the adapter file

Add a new file at `runtime/internal/provider/<providername>.go`.

```
runtime/internal/provider/
    ollama.go
    lmstudio.go
    openai_local.go
    myprovider.go          ← your new adapter
    myprovider_test.go     ← your tests
```

### 3. Implement required methods

#### Struct and constructor

```go
package provider

import (
    "context"
    "time"

    "github.com/EffNine/gumi/runtime/internal/api"
    "github.com/EffNine/gumi/runtime/internal/config"
    "github.com/EffNine/gumi/runtime/internal/logger"
)

const myProviderDefaultURL = "http://localhost:9999"

type MyProviderAdapter struct {
    name    string
    baseURL string
    timeout time.Duration
    client  *http.Client
    log     *logger.Logger
}

func NewMyProviderAdapter(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
    baseURL := settings.URL
    if baseURL == "" {
        baseURL = myProviderDefaultURL
    }
    baseURL = strings.TrimSuffix(baseURL, "/")

    timeout := time.Duration(settings.TimeoutSeconds) * time.Second
    if timeout <= 0 {
        timeout = 60 * time.Second
    }

    return &MyProviderAdapter{
        name:    name,
        baseURL: baseURL,
        timeout: timeout,
        client: &http.Client{
            Timeout: timeout,
        },
        log: log,
    }, nil
}
```

#### Name and Type

```go
func (m *MyProviderAdapter) Name() string { return m.name }
func (m *MyProviderAdapter) Type() string { return "myprovider" }
```

#### Capabilities

Report honestly what your provider supports. Default to `false` for
`ToolUse` and `StructuredOutput` unless you've verified the provider handles
them.

```go
func (m *MyProviderAdapter) Capabilities() Capabilities {
    return Capabilities{
        Streaming:        true,
        ToolUse:          false,
        StructuredOutput: false,
    }
}
```

#### HealthCheck

Probe whatever endpoint tells you the provider is alive. For OpenAI-compatible
servers use `/v1/models`; for Ollama use `/api/tags`; for custom servers pick
the lightest endpoint.

```go
func (m *MyProviderAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
    url := m.baseURL + "/health" // or /api/tags, /v1/models, etc.
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return StatusMisconfigured, err
    }

    resp, err := m.client.Do(req)
    if err != nil {
        return StatusOffline, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return StatusDegraded, fmt.Errorf("myprovider health check returned status %d", resp.StatusCode)
    }

    return StatusOK, nil
}
```

#### ListModels

Fetch available models and return `[]ModelInfo`. Map the provider's response
shape to the canonical `ModelInfo` struct.

```go
func (m *MyProviderAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
    url := m.baseURL + "/models"
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := m.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("myprovider model list returned status %d", resp.StatusCode)
    }

    var list myProviderModelsResponse
    if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
        return nil, err
    }

    models := make([]ModelInfo, 0, len(list.Models))
    for _, md := range list.Models {
        models = append(models, ModelInfo{
            Name:     md.ID,
            Provider: m.name,
        })
    }
    return models, nil
}
```

#### Generate

Translate `api.ChatCompletionRequest` → provider wire format, POST, then
translate the response back to `*api.ChatCompletionResponse`.

```go
func (m *MyProviderAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
    url := m.baseURL + "/chat"

    payload := myProviderChatRequest{
        Model:    req.Model,
        Messages: translateMessages(req.Messages),
        Stream:   false,
        // ... map Temperature, TopP, MaxTokens, etc.
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return nil, ProviderError{
            Code:    ProviderBadResponse,
            Message: "failed to marshal myprovider request",
            Cause:   err,
        }
    }

    httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := m.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, NormalizeHTTPError(resp.StatusCode, nil, "myprovider")
    }

    var raw myProviderChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
        return nil, ProviderError{
            Code:    ProviderBadResponse,
            Message: "failed to decode myprovider response",
            Cause:   err,
        }
    }

    return &api.ChatCompletionResponse{
        // ... translate raw into canonical response
    }, nil
}
```

#### GenerateStream

Return SSE chunks via channels. If your provider doesn't support streaming,
return `provider.StreamingUnsupported` as `setupErr`.

```go
func (m *MyProviderAdapter) GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (
    <-chan api.ChatCompletionChunk, <-chan error, error,
) {
    // Set Stream=true on the request
    // Open SSE connection
    // Read lines, decode, send to chunkCh
    // Close chunkCh when done, send nil or error to errCh
    // Return setupErr synchronously if connection cannot be opened
}
```

See `ollama.go` or `openai_local.go` for reference SSE streaming patterns using
`bufio.Scanner` over the response body.

#### NormalizeError

Delegate to the shared helper:

```go
func (m *MyProviderAdapter) NormalizeError(err error) ProviderError {
    // If err is already a ProviderError, return it.
    // Otherwise use NormalizeHTTPError or classifyNetworkError.
    var perr ProviderError
    if errors.As(err, &perr) {
        return perr
    }
    return classifyNetworkError(err, m.name)
}
```

### 4. Register the provider

Edit `runtime/internal/provider/default.go` and add your factory to
`DefaultRegistry()`:

```go
func DefaultRegistry() *Registry {
    r := NewRegistry()
    r.Register("ollama", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
        return NewOllamaAdapter(name, settings, log)
    })
    r.Register("lmstudio", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
        return NewLMStudioAdapter(name, settings, log)
    })
    r.Register("openai_compatible_local", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
        return NewOpenAICompatibleAdapter(name, settings, log)
    })
    // ── Add your provider below ───────────────────────────────
    r.Register("myprovider", func(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
        return NewMyProviderAdapter(name, settings, log)
    })
    return r
}
```

The **provider key** (`"myprovider"`) becomes part of model IDs:
`myprovider:llama3.2` and is used in `gumi.yaml` under `providers:`.

If your provider key contains underscores (e.g. `my_provider`), the registry
handles it as-is. The `modelIDPrefix()` function in `registry.go` maps special
keys to display-friendly prefixes.

### 5. Write tests

See [Testing Requirements](#testing-requirements) below. At minimum, create
`runtime/internal/provider/myprovider_test.go`.

## Testing Requirements

Every new provider adapter must include:

### Unit tests (required)

Use `net/http/httptest` to mock the provider server. See
`ollama_test.go` for the pattern:

| Test | What it validates |
|------|-------------------|
| `Test<Provider>HealthCheck` | Returns `StatusOK` when server is up, `StatusOffline` when unreachable |
| `Test<Provider>ListModels` | Parses provider model list into `[]ModelInfo` correctly |
| `Test<Provider>Generate` | Translates request/response through the canonical types |
| `Test<Provider>GenerateStream` | Streams chunks correctly (or verifies `StreamingUnsupported`) |
| `Test<Provider>NormalizeError*` | Maps various error conditions to correct `ProviderErrorCode` values |
| `Test<Provider>Capabilities` | Reports accurate capability flags |

### Integration / health tests (required)

If the provider has a known testable binary or Docker image, add a
`*_integration_test.go` file guarded by a `//go:build integration` tag that
starts the real provider, runs a full request cycle, and tears it down.

### Provider-specific tests

Write at least one test that exercises a quirk of your provider's wire format:

- Non-standard field names
- Extra JSON fields in responses
- Edge cases (empty model list, 4xx errors, timeouts)

### Running tests

```bash
cd runtime && go test ./internal/provider/ -v -run "TestMyProvider"
```

## Common Pitfalls

### 1. Statefulness

**Do:** Store only configuration (`baseURL`, `timeout`, `*http.Client`, logger).

**Don't:** Cache model weights, maintain per-request state, or store mutable
fields that differ between calls. The same adapter instance may serve concurrent
requests.

### 2. Connection pooling

Use the `http.Client` with a reasonable `Timeout`. Go's default `Transport`
handles connection pooling automatically. Do not create a new `http.Client`
per request.

### 3. Context propagation

Always use `http.NewRequestWithContext(ctx, ...)`. If you forget, the request
will ignore timeouts and cancellations, causing goroutine leaks.

### 4. Error handling

Use `NormalizeHTTPError()` from `errors.go` for HTTP status codes. Use
`classifyNetworkError()` for transport-level errors. Never return raw errors
from the HTTP layer — always wrap in `ProviderError`.

### 5. Streaming channel leaks

Both `chunkCh` and `errCh` must be drained or closed. If the stream fails
mid-way, send the error to `errCh` and close `chunkCh`. If the stream succeeds,
close both channels.

### 6. Cloud providers

**V1 does not support cloud providers.** Adapters must target local inference
servers only. OpenAI, Anthropic, and other cloud APIs are out of scope for V1.

### 7. Business logic in adapters

Adapters must not implement:

- Prompt optimisation
- Retry logic
- Response validation
- Routing decisions
- Caching

These belong in the Pipeline Engine. The adapter translates; the engine decides.

## Example: Minimal Provider Adapter

Here's a minimal skeleton for a hypothetical provider called "simplellm" that
exposes a simple JSON API:

```go
package provider

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/EffNine/gumi/runtime/internal/api"
    "github.com/EffNine/gumi/runtime/internal/config"
    "github.com/EffNine/gumi/runtime/internal/logger"
)

const simpleLLMDefaultURL = "http://localhost:8080"

type simpleLLMAdapter struct {
    name    string
    baseURL string
    timeout time.Duration
    client  *http.Client
    log     *logger.Logger
}

func NewSimpleLLMAdapter(name string, settings config.ProviderSettings, log *logger.Logger) (ProviderAdapter, error) {
    baseURL := settings.URL
    if baseURL == "" {
        baseURL = simpleLLMDefaultURL
    }
    baseURL = strings.TrimSuffix(baseURL, "/")

    timeout := time.Duration(settings.TimeoutSeconds) * time.Second
    if timeout <= 0 {
        timeout = 60 * time.Second
    }

    return &simpleLLMAdapter{
        name:    name,
        baseURL: baseURL,
        timeout: timeout,
        client:  &http.Client{Timeout: timeout},
        log:     log,
    }, nil
}

func (s *simpleLLMAdapter) Name() string    { return s.name }
func (s *simpleLLMAdapter) Type() string    { return "simplellm" }
func (s *simpleLLMAdapter) Capabilities() Capabilities {
    return Capabilities{Streaming: true, ToolUse: false, StructuredOutput: false}
}

func (s *simpleLLMAdapter) HealthCheck(ctx context.Context) (ProviderStatus, error) {
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/ping", nil)
    resp, err := s.client.Do(req)
    if err != nil {
        return StatusOffline, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return StatusDegraded, fmt.Errorf("status %d", resp.StatusCode)
    }
    return StatusOK, nil
}

func (s *simpleLLMAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
    // Fetch /models, decode, return []ModelInfo
    // ... (see ollama.go or openai_local.go for the full pattern)
    return nil, nil
}

func (s *simpleLLMAdapter) Generate(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
    body, _ := json.Marshal(map[string]any{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   false,
    })
    httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/generate", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    resp, err := s.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, NormalizeHTTPError(resp.StatusCode, nil, "simplellm")
    }
    var raw struct {
        Content string `json:"content"`
    }
    json.NewDecoder(resp.Body).Decode(&raw)
    return &api.ChatCompletionResponse{
        Choices: []api.Choice{{Message: api.Message{Role: "assistant", Content: raw.Content}}}}, nil
}

func (s *simpleLLMAdapter) GenerateStream(ctx context.Context, req api.ChatCompletionRequest) (<-chan api.ChatCompletionChunk, <-chan error, error) {
    // Open SSE connection, read lines, send chunks.
    // See openai_local.go for the full streaming pattern.
    return nil, nil, nil
}

func (s *simpleLLMAdapter) NormalizeError(err error) ProviderError {
    var perr ProviderError
    if errors.As(err, &perr) {
        return perr
    }
    return classifyNetworkError(err, s.name)
}
```

## Submission

### Before you submit

1. **Tests pass.** `cd runtime && go test ./internal/provider/ -v -run "Test<YourProvider>"`
2. **Builds clean.** `cd runtime && go build ./...`
3. **Formatted.** `cd runtime && gofmt -w internal/provider/<yourprovider>.go`
4. **Thin.** Double-check: no business logic, no retries, no prompt handling.
5. **Local only.** No cloud API keys, no cloud endpoints.

### Pull request process

1. Fork the repo and create a branch: `git checkout -b provider/<your-provider>`
2. Commit your adapter + tests + registry registration.
3. Open a PR targeting `main`.
4. In the PR description, include:
   - Provider name and URL pattern
   - Link to the provider's API documentation
   - Screenshots or logs of health check + generate passing
   - List of capabilities (`Streaming`, `ToolUse`, `StructuredOutput`)
5. The maintainer will review for:
   - Interface compliance
   - Test coverage
   - Thin adapter principle adherence
   - No cloud provider leakage

### Review criteria

| Criterion | Required? |
|-----------|-----------|
| Implements `ProviderAdapter` fully | Yes |
| Health check returns correct `ProviderStatus` values | Yes |
| Unit tests with `httptest.Server` | Yes |
| `NormalizeError` delegates to shared helpers | Yes |
| No business logic in adapter | Yes |
| Local-only (no cloud providers) | Yes |
| Streaming works or returns `StreamingUnsupported` | Yes |
| Registered in `DefaultRegistry()` | Yes |
