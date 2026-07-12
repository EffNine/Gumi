package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"syscall"
	"time"
)

// ProviderErrorCode is a stable normalized code for provider failures.
type ProviderErrorCode string

const (
	// ProviderUnavailable means the provider is not reachable.
	ProviderUnavailable ProviderErrorCode = "PROVIDER_UNAVAILABLE"
	// ProviderTimeout means the request timed out.
	ProviderTimeout ProviderErrorCode = "PROVIDER_TIMEOUT"
	// ModelNotFound means the requested model is not available on the provider.
	ModelNotFound ProviderErrorCode = "MODEL_NOT_FOUND"
	// ProviderBadResponse means the provider returned an unexpected response.
	ProviderBadResponse ProviderErrorCode = "PROVIDER_BAD_RESPONSE"
	// ProviderAuthError means the provider rejected authentication.
	ProviderAuthError ProviderErrorCode = "PROVIDER_AUTH_ERROR"
	// StreamingUnsupported means the selected runtime/provider path cannot stream.
	StreamingUnsupported ProviderErrorCode = "STREAMING_UNSUPPORTED"
	// EmptyPrompt means the prompt is empty after runtime normalization.
	EmptyPrompt ProviderErrorCode = "EMPTY_PROMPT"
	// ContextLimitExceeded means the prepared context still exceeds its budget.
	ContextLimitExceeded ProviderErrorCode = "CONTEXT_LIMIT_EXCEEDED"
	// ValidationFailed means model output failed validation and could not be repaired.
	ValidationFailed ProviderErrorCode = "VALIDATION_FAILED"
	// ProviderMisconfigured means the adapter configuration is invalid.
	ProviderMisconfigured ProviderErrorCode = "PROVIDER_MISCONFIGURED"
	// AGENT_STEP_LIMIT_EXCEEDED means the agent has exceeded its step budget.
	AGENT_STEP_LIMIT_EXCEEDED ProviderErrorCode = "AGENT_STEP_LIMIT_EXCEEDED"
	// AGENT_TOOL_CALL_LOOP means the agent is repeating the same tool call.
	AGENT_TOOL_CALL_LOOP ProviderErrorCode = "AGENT_TOOL_CALL_LOOP"
	// AGENT_INVALID_TOOL_CALL means the agent returned an invalid tool call.
	AGENT_INVALID_TOOL_CALL ProviderErrorCode = "AGENT_INVALID_TOOL_CALL"
	// ProviderUnknownError is a catch-all for unrecognized failures.
	ProviderUnknownError ProviderErrorCode = "PROVIDER_UNKNOWN_ERROR"
)

// ProviderError is a normalized provider failure.
type ProviderError struct {
	Code       ProviderErrorCode
	Message    string
	Suggestion string
	Cause      error
}

// Error implements the error interface.
func (e ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause.
func (e ProviderError) Unwrap() error {
	return e.Cause
}

// NormalizeHTTPError maps an HTTP response status and raw error to a normalized
// provider error. It is used by adapters when a provider call fails.
func NormalizeHTTPError(status int, err error, providerName string) ProviderError {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return ProviderError{
			Code:       ProviderAuthError,
			Message:    fmt.Sprintf("%s rejected the request due to authentication", providerName),
			Suggestion: "Verify the provider API key or local server settings.",
			Cause:      err,
		}
	}
	if status == http.StatusNotFound {
		return ProviderError{
			Code:       ModelNotFound,
			Message:    fmt.Sprintf("%s reported the requested model was not found", providerName),
			Suggestion: "Check the model ID and ensure the model is loaded or available.",
			Cause:      err,
		}
	}
	if status >= 500 {
		return ProviderError{
			Code:       ProviderUnavailable,
			Message:    fmt.Sprintf("%s returned a server error", providerName),
			Suggestion: "Check that the local provider is running and healthy.",
			Cause:      err,
		}
	}
	if status >= 400 {
		return ProviderError{
			Code:       ProviderBadResponse,
			Message:    fmt.Sprintf("%s rejected the request", providerName),
			Suggestion: "Review the request payload and provider logs.",
			Cause:      err,
		}
	}
	return classifyNetworkError(err, providerName)
}

// classifyNetworkError maps transport-level errors to normalized codes.
func classifyNetworkError(err error, providerName string) ProviderError {
	if err == nil {
		return ProviderError{
			Code:    ProviderUnknownError,
			Message: fmt.Sprintf("%s returned an unexpected error", providerName),
		}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ProviderError{
			Code:       ProviderTimeout,
			Message:    fmt.Sprintf("%s request timed out", providerName),
			Suggestion: "Increase the provider timeout or check provider responsiveness.",
			Cause:      err,
		}
	}

	// Detect common connection failures without importing OS-specific packages.
	var netErr interface {
		Timeout() bool
	}
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ProviderError{
			Code:       ProviderTimeout,
			Message:    fmt.Sprintf("%s connection timed out", providerName),
			Suggestion: "Check network connectivity to the provider.",
			Cause:      err,
		}
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return ProviderError{
			Code:       ProviderUnavailable,
			Message:    fmt.Sprintf("%s is not reachable (connection refused)", providerName),
			Suggestion: "Start the local provider and verify the configured URL.",
			Cause:      err,
		}
	}

	return ProviderError{
		Code:       ProviderUnavailable,
		Message:    fmt.Sprintf("%s request failed: %v", providerName, err),
		Suggestion: "Check that the provider is running and reachable.",
		Cause:      err,
	}
}

// NewTimeoutError returns a provider timeout error.
func NewTimeoutError(providerName string, timeout time.Duration) ProviderError {
	return ProviderError{
		Code:       ProviderTimeout,
		Message:    fmt.Sprintf("%s request exceeded %v timeout", providerName, timeout),
		Suggestion: "Increase the provider timeout or check provider responsiveness.",
	}
}
