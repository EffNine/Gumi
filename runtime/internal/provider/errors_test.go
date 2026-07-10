package provider

import (
	"context"
	"errors"
	"net/http"
	"syscall"
	"testing"
	"time"
)

func TestNormalizeHTTPErrorAuth(t *testing.T) {
	err := NormalizeHTTPError(http.StatusUnauthorized, errors.New("unauthorized"), "ollama")
	if err.Code != ProviderAuthError {
		t.Errorf("expected %s, got %s", ProviderAuthError, err.Code)
	}
	if err.Suggestion == "" {
		t.Error("expected suggestion for auth error")
	}
}

func TestNormalizeHTTPErrorModelNotFound(t *testing.T) {
	err := NormalizeHTTPError(http.StatusNotFound, errors.New("not found"), "ollama")
	if err.Code != ModelNotFound {
		t.Errorf("expected %s, got %s", ModelNotFound, err.Code)
	}
}

func TestNormalizeHTTPErrorServerError(t *testing.T) {
	err := NormalizeHTTPError(http.StatusInternalServerError, errors.New("boom"), "lmstudio")
	if err.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, err.Code)
	}
}

func TestNormalizeHTTPErrorBadRequest(t *testing.T) {
	err := NormalizeHTTPError(http.StatusBadRequest, errors.New("bad request"), "openai-compatible")
	if err.Code != ProviderBadResponse {
		t.Errorf("expected %s, got %s", ProviderBadResponse, err.Code)
	}
}

func TestClassifyNetworkErrorConnectionRefused(t *testing.T) {
	err := classifyNetworkError(syscall.ECONNREFUSED, "ollama")
	if err.Code != ProviderUnavailable {
		t.Errorf("expected %s, got %s", ProviderUnavailable, err.Code)
	}
}

func TestClassifyNetworkErrorTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := classifyNetworkError(ctx.Err(), "lmstudio")
	if err.Code != ProviderTimeout {
		t.Errorf("expected %s, got %s", ProviderTimeout, err.Code)
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError("ollama", 60*time.Second)
	if err.Code != ProviderTimeout {
		t.Errorf("expected %s, got %s", ProviderTimeout, err.Code)
	}
	if err.Message == "" {
		t.Error("expected timeout message")
	}
}

func TestProviderErrorImplementsError(t *testing.T) {
	pe := ProviderError{Code: ProviderUnknownError, Message: "something failed", Cause: errors.New("cause")}
	if pe.Error() == "" {
		t.Error("expected ProviderError to implement error interface")
	}
	if pe.Unwrap() == nil {
		t.Error("expected Unwrap to return cause")
	}
}
