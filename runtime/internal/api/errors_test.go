package api

import (
	"encoding/json"
	"testing"
)

func TestNewRequestError(t *testing.T) {
	err := NewRequestError("INVALID_REQUEST", "bad request", "req_abc")
	if err.Error.Code != "INVALID_REQUEST" {
		t.Errorf("expected code INVALID_REQUEST, got %s", err.Error.Code)
	}
	if err.Error.Type != "request_error" {
		t.Errorf("expected type request_error, got %s", err.Error.Type)
	}
	if err.Error.RequestID != "req_abc" {
		t.Errorf("expected request_id req_abc, got %s", err.Error.RequestID)
	}
}

func TestNewAuthError(t *testing.T) {
	err := NewAuthError("invalid key", "req_xyz")
	if err.Error.Code != "INVALID_API_KEY" {
		t.Errorf("expected code INVALID_API_KEY, got %s", err.Error.Code)
	}
	if err.Error.Type != "auth_error" {
		t.Errorf("expected type auth_error, got %s", err.Error.Type)
	}
	if err.Error.Suggestion == "" {
		t.Error("expected non-empty suggestion")
	}
}

func TestErrorMarshal(t *testing.T) {
	err := NewRuntimeError("RUNTIME_ERROR", "boom", "req_123")
	b := err.Marshal()

	var decoded ErrorResponse
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}
	if decoded.Error.Code != "RUNTIME_ERROR" {
		t.Errorf("expected code RUNTIME_ERROR after marshal, got %s", decoded.Error.Code)
	}
}
