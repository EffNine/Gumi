// Package provider implements thin adapters that connect the Gumi gateway
// to local inference providers. Adapters only translate requests and responses;
// they do not implement prompt optimization, validation, repair, or retry logic.
package provider

import (
	"context"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
)

// ProviderStatus represents the current health state of a provider.
type ProviderStatus string

const (
	// StatusOK means the provider is reachable and serving requests.
	StatusOK ProviderStatus = "ok"
	// StatusOffline means the provider is not reachable.
	StatusOffline ProviderStatus = "offline"
	// StatusDegraded means the provider is reachable but reporting problems.
	StatusDegraded ProviderStatus = "degraded"
	// StatusMisconfigured means the adapter configuration is invalid.
	StatusMisconfigured ProviderStatus = "misconfigured"
	// StatusUnknown means the provider state has not been checked yet.
	StatusUnknown ProviderStatus = "unknown"
)

// ModelInfo describes a model returned by a provider's discovery endpoint.
type ModelInfo struct {
	ID        string
	Name      string
	Provider  string
	CreatedAt time.Time
}

// Capabilities describes what a provider adapter supports.
type Capabilities struct {
	Streaming        bool
	ToolUse          bool
	StructuredOutput bool
}

// ProviderAdapter is the common interface implemented by every local provider
// adapter. It is intentionally small so that future Pipeline Engine integration
// can reuse adapters without rework.
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
