// Package api provides OpenAI-compatible and Gumi-specific request and
// response types used across the runtime.
package api

import (
	"encoding/json"
	"fmt"
)

// ErrorResponse is the standard Gumi error envelope.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// APIError represents a single runtime error.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Type       string `json:"type"`
	Engine     string `json:"engine,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface for APIError.
func (e APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewRequestError builds a generic request error.
func NewRequestError(code, message, requestID string) ErrorResponse {
	return ErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			Type:      "request_error",
			Engine:    "gateway",
			Retryable: false,
			RequestID: requestID,
		},
	}
}

// NewAuthError builds an authentication error.
func NewAuthError(message, requestID string) ErrorResponse {
	return ErrorResponse{
		Error: APIError{
			Code:       "INVALID_API_KEY",
			Message:    message,
			Type:       "auth_error",
			Engine:     "gateway",
			Retryable:  false,
			RequestID:  requestID,
			Suggestion: "Use Authorization: Bearer <local-key> or set auth.mode to disabled in config.",
		},
	}
}

// NewRuntimeError builds a runtime error.
func NewRuntimeError(code, message, requestID string) ErrorResponse {
	return ErrorResponse{
		Error: APIError{
			Code:      code,
			Message:   message,
			Type:      "runtime_error",
			Engine:    "gateway",
			Retryable: true,
			RequestID: requestID,
		},
	}
}

// Marshal returns the JSON encoding of the error response.
func (er ErrorResponse) Marshal() []byte {
	b, _ := json.Marshal(er)
	return b
}
