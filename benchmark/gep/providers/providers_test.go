package providers

import (
	"testing"
)

func TestNewProviderLMStudio(t *testing.T) {
	p, err := NewProvider("lmstudio", "http://localhost:1234/v1", "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type() != "lmstudio" {
		t.Errorf("expected type lmstudio, got %s", p.Type())
	}
	if p.Name() != "LM Studio" {
		t.Errorf("expected name LM Studio, got %s", p.Name())
	}
}

func TestNewProviderOllama(t *testing.T) {
	p, err := NewProvider("ollama", "http://localhost:11434", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Type() != "ollama" {
		t.Errorf("expected type ollama, got %s", p.Type())
	}
	if p.Name() != "Ollama" {
		t.Errorf("expected name Ollama, got %s", p.Name())
	}
}

func TestNewProviderUnsupported(t *testing.T) {
	_, err := NewProvider("unknown", "http://localhost:1234", "")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestLMStudioProviderURLTrimming(t *testing.T) {
	p := NewLMStudioProvider("http://localhost:1234/v1/", "")
	if p.baseURL != "http://localhost:1234/v1" {
		t.Errorf("expected URL to be trimmed, got %s", p.baseURL)
	}
}

func TestOllamaProviderURLTrimming(t *testing.T) {
	p := NewOllamaProvider("http://localhost:11434/", "")
	if p.baseURL != "http://localhost:11434" {
		t.Errorf("expected URL to be trimmed, got %s", p.baseURL)
	}
}
