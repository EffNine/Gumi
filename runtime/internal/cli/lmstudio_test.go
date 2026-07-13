package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newLMStudioCLITestServer creates an httptest.Server that simulates the
// LM Studio v1 REST API for CLI commands.
func newLMStudioCLITestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/models":
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"model": "qwen3-8b", "type": "local", "path": "/models/qwen3-8b.gguf", "size": "4.7 GB"},
					{"model": "qwen3-1.7b", "type": "local", "path": "/models/qwen3-1.7b.gguf", "size": "1.1 GB"},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "/api/v1/models/load":
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
				return
			}
			modelID, _ := req["model"].(string)
			resp := map[string]interface{}{
				"type":              "model_loaded",
				"instance_id":       "inst_" + modelID,
				"load_time_seconds": 2.5,
				"status":            "loaded",
				"load_config": map[string]interface{}{
					"context_length": 32768,
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "/api/v1/models/unload":
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
				return
			}
			instanceID, _ := req["instance_id"].(string)
			resp := map[string]interface{}{
				"instance_id": instanceID,
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		}
	}))
}

func TestResolveLMStudioURL(t *testing.T) {
	// Explicit URL should be returned as-is.
	got := resolveLMStudioURL("http://custom:8080/v1")
	if got != "http://custom:8080/v1" {
		t.Fatalf("expected 'http://custom:8080/v1', got %q", got)
	}
}

func TestResolveLMStudioURLDefaults(t *testing.T) {
	// Empty URL should return a non-empty string (default or config-based).
	got := resolveLMStudioURL("")
	if got == "" {
		t.Fatal("expected non-empty URL")
	}
}

func TestMgmtURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://localhost:1234/v1", "http://localhost:1234/api/v1"},
		{"http://localhost:1234", "http://localhost:1234/api/v1"},
		{"http://192.168.0.164:1234/v1", "http://192.168.0.164:1234/api/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mgmtURL(tt.input)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestLMStudioAPIGet(t *testing.T) {
	server := newLMStudioCLITestServer(t)
	defer server.Close()

	body, err := lmstudioAPIGet(server.URL + "/api/v1/models")
	if err != nil {
		t.Fatalf("lmstudioAPIGet failed: %v", err)
	}

	var result struct {
		Data []struct {
			Model string `json:"model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result.Data))
	}
	if result.Data[0].Model != "qwen3-8b" {
		t.Fatalf("expected first model 'qwen3-8b', got %q", result.Data[0].Model)
	}
}

func TestLMStudioAPIGetServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, `{"error":"internal error"}`)
	}))
	defer server.Close()

	_, err := lmstudioAPIGet(server.URL + "/api/v1/models")
	if err == nil {
		t.Fatal("expected error from server error response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error to contain status code, got: %v", err)
	}
}

func TestLMStudioAPIGetUnreachable(t *testing.T) {
	_, err := lmstudioAPIGet("http://127.0.0.1:1/api/v1/models")
	if err == nil {
		t.Fatal("expected error from unreachable server")
	}
}

func TestLMStudioAPIPost(t *testing.T) {
	server := newLMStudioCLITestServer(t)
	defer server.Close()

	payload := map[string]string{"model": "qwen3-8b"}
	body, err := lmstudioAPIPost(server.URL+"/api/v1/models/load", payload)
	if err != nil {
		t.Fatalf("lmstudioAPIPost failed: %v", err)
	}

	var resp struct {
		InstanceID string `json:"instance_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.InstanceID != "inst_qwen3-8b" {
		t.Fatalf("expected instance_id 'inst_qwen3-8b', got %q", resp.InstanceID)
	}
	if resp.Status != "loaded" {
		t.Fatalf("expected status 'loaded', got %q", resp.Status)
	}
}

func TestLMStudioAPIPostServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, `{"error":"bad request"}`)
	}))
	defer server.Close()

	_, err := lmstudioAPIPost(server.URL+"/api/v1/models/load", map[string]string{"model": "test"})
	if err == nil {
		t.Fatal("expected error from server error response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected error to contain status code, got: %v", err)
	}
}

func TestLMStudioAPIPostUnreachable(t *testing.T) {
	_, err := lmstudioAPIPost("http://127.0.0.1:1/api/v1/models/load", map[string]string{"model": "test"})
	if err == nil {
		t.Fatal("expected error from unreachable server")
	}
}

func TestLMStudioStatusJSONOutput(t *testing.T) {
	server := newLMStudioCLITestServer(t)
	defer server.Close()

	// Test that the status command can parse the /api/v1/models response.
	body, err := lmstudioAPIGet(server.URL + "/api/v1/models")
	if err != nil {
		t.Fatalf("lmstudioAPIGet failed: %v", err)
	}

	var result struct {
		Data []struct {
			Model string `json:"model"`
			Type  string `json:"type"`
			Path  string `json:"path,omitempty"`
			Size  string `json:"size,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse models response: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result.Data))
	}
	if result.Data[0].Size != "4.7 GB" {
		t.Fatalf("expected size '4.7 GB', got %q", result.Data[0].Size)
	}
}

func TestLMStudioLoadJSONOutput(t *testing.T) {
	server := newLMStudioCLITestServer(t)
	defer server.Close()

	payload := map[string]interface{}{
		"model":            "qwen3-8b",
		"echo_load_config": true,
	}
	body, err := lmstudioAPIPost(server.URL+"/api/v1/models/load", payload)
	if err != nil {
		t.Fatalf("lmstudioAPIPost failed: %v", err)
	}

	var resp struct {
		Type            string                 `json:"type"`
		InstanceID      string                 `json:"instance_id"`
		LoadTimeSeconds float64                `json:"load_time_seconds"`
		Status          string                 `json:"status"`
		LoadConfig      map[string]interface{} `json:"load_config,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse load response: %v", err)
	}
	if resp.Type != "model_loaded" {
		t.Fatalf("expected type 'model_loaded', got %q", resp.Type)
	}
	if resp.LoadTimeSeconds <= 0 {
		t.Fatal("expected positive load_time_seconds")
	}
	if resp.LoadConfig == nil {
		t.Fatal("expected load_config in response")
	}
}

func TestLMStudioUnloadJSONOutput(t *testing.T) {
	server := newLMStudioCLITestServer(t)
	defer server.Close()

	payload := map[string]string{"instance_id": "inst_qwen3-8b"}
	body, err := lmstudioAPIPost(server.URL+"/api/v1/models/unload", payload)
	if err != nil {
		t.Fatalf("lmstudioAPIPost failed: %v", err)
	}

	var resp struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to parse unload response: %v", err)
	}
	if resp.InstanceID != "inst_qwen3-8b" {
		t.Fatalf("expected instance_id 'inst_qwen3-8b', got %q", resp.InstanceID)
	}
}

func TestLMStudioListModelsJSONOutput(t *testing.T) {
	server := newLMStudioCLITestServer(t)
	defer server.Close()

	body, err := lmstudioAPIGet(server.URL + "/api/v1/models")
	if err != nil {
		t.Fatalf("lmstudioAPIGet failed: %v", err)
	}

	var result struct {
		Data []struct {
			Model string `json:"model"`
			Type  string `json:"type"`
			Path  string `json:"path,omitempty"`
			Size  string `json:"size,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse models response: %v", err)
	}
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result.Data))
	}
}

func TestLMStudioStatusEmptyModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	body, err := lmstudioAPIGet(server.URL + "/api/v1/models")
	if err != nil {
		t.Fatalf("lmstudioAPIGet failed: %v", err)
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result.Data) != 0 {
		t.Fatalf("expected 0 models, got %d", len(result.Data))
	}
}
