package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
	"github.com/EffNine/gumi/runtime/internal/provider"
	"github.com/EffNine/gumi/runtime/internal/storage"
)

func newTestWriter(t *testing.T) (*Writer, *storage.Storage) {
	t.Helper()
	store, err := storage.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Telemetry.Local = true
	log := logger.New("error")
	w := NewWithStorage(store, cfg, log)
	return w, store
}

func TestRedactJSONMasksSecrets(t *testing.T) {
	input := []byte(`{"api_key":"sk-12345","authorization":"Bearer token","model":"llama3","nested":{"secret":"hidden"}}`)
	out := RedactJSON(input)

	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("redacted JSON invalid: %v", err)
	}

	if m["api_key"] != redactedPlaceholder {
		t.Errorf("expected api_key redacted, got %v", m["api_key"])
	}
	if m["authorization"] != redactedPlaceholder {
		t.Errorf("expected authorization redacted, got %v", m["authorization"])
	}
	if m["model"] != "llama3" {
		t.Errorf("expected model unchanged, got %v", m["model"])
	}
	nested := m["nested"].(map[string]interface{})
	if nested["secret"] != redactedPlaceholder {
		t.Errorf("expected nested secret redacted, got %v", nested["secret"])
	}
}

func TestRedactString(t *testing.T) {
	if got := RedactString("api_key", "secret"); got != redactedPlaceholder {
		t.Errorf("expected redacted, got %s", got)
	}
	if got := RedactString("model", "llama3"); got != "llama3" {
		t.Errorf("expected unchanged, got %s", got)
	}
}

func TestRecordRequest(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordRequest(context.Background(), RequestRecord{
		RequestID:   "req_abc",
		CreatedAt:   time.Now(),
		WorkspaceID: "default",
		RuntimeMode: "stabilized",
		Provider:    "ollama",
		Model:       "llama3",
		Status:      "success",
		LatencyMs:   123,
	})

	var status string
	err := store.DB().QueryRow("SELECT status FROM requests WHERE id=?", "req_abc").Scan(&status)
	if err != nil {
		t.Fatalf("failed to read request: %v", err)
	}
	if status != "success" {
		t.Errorf("expected success, got %s", status)
	}
}

func TestRecordRequestDoesNotStorePromptsByDefault(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	req := api.ChatCompletionRequest{
		Model: "ollama:llama3",
		Messages: []api.Message{
			{Role: "user", Content: "this is a secret prompt"},
		},
	}

	w.RecordRequest(context.Background(), RequestRecord{
		RequestID:       "req_privacy",
		CreatedAt:       time.Now(),
		WorkspaceID:     "default",
		RuntimeMode:     "stabilized",
		Status:          "success",
		PromptLogged:    false,
		ResponseLogged:  false,
		PromptPreview:   ExtractContentPreview(req, false),
		ResponsePreview: ExtractResponsePreview(&api.ChatCompletionResponse{Choices: []api.Choice{{Message: api.Message{Role: "assistant", Content: "secret response"}}}}, false),
	})

	var promptLogged, responseLogged int
	var promptPreview, responsePreview sql.NullString
	err := store.DB().QueryRow("SELECT prompt_logged, response_logged, prompt_preview, response_preview FROM requests WHERE id=?", "req_privacy").
		Scan(&promptLogged, &responseLogged, &promptPreview, &responsePreview)
	if err != nil {
		t.Fatalf("failed to read request: %v", err)
	}
	if promptLogged != 0 {
		t.Errorf("expected prompt_logged 0, got %d", promptLogged)
	}
	if responseLogged != 0 {
		t.Errorf("expected response_logged 0, got %d", responseLogged)
	}
	if promptPreview.Valid {
		t.Errorf("expected prompt_preview null, got %v", promptPreview.String)
	}
	if responsePreview.Valid {
		t.Errorf("expected response_preview null, got %v", responsePreview.String)
	}
}

func TestRecordRequestStoresPreviewsWhenEnabled(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	w.cfg.Telemetry.LogPrompts = true
	w.cfg.Telemetry.LogResponses = true

	req := api.ChatCompletionRequest{
		Model: "ollama:llama3",
		Messages: []api.Message{
			{Role: "user", Content: "this is a prompt"},
		},
	}
	resp := &api.ChatCompletionResponse{
		Choices: []api.Choice{{Message: api.Message{Role: "assistant", Content: "this is a response"}}},
	}

	w.RecordRequest(context.Background(), RequestRecord{
		RequestID:       "req_preview",
		CreatedAt:       time.Now(),
		WorkspaceID:     "default",
		RuntimeMode:     "stabilized",
		Status:          "success",
		PromptLogged:    true,
		ResponseLogged:  true,
		PromptPreview:   ExtractContentPreview(req, true),
		ResponsePreview: ExtractResponsePreview(resp, true),
	})

	var promptPreview, responsePreview sql.NullString
	err := store.DB().QueryRow("SELECT prompt_preview, response_preview FROM requests WHERE id=?", "req_preview").
		Scan(&promptPreview, &responsePreview)
	if err != nil {
		t.Fatalf("failed to read request: %v", err)
	}
	if !promptPreview.Valid || promptPreview.String == "" {
		t.Errorf("expected prompt preview, got %v", promptPreview)
	}
	if !responsePreview.Valid || responsePreview.String == "" {
		t.Errorf("expected response preview, got %v", responsePreview)
	}
}

func TestRecordPipelineEvents(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordPipelineEvents(context.Background(), []PipelineEventRecord{
		{
			RequestID: "req_evt",
			Timestamp: time.Now(),
			Engine:    "pipeline",
			Event:     "request_received",
			Severity:  "info",
			Message:   "request received",
		},
	})

	var count int
	err := store.DB().QueryRow("SELECT count(*) FROM pipeline_events WHERE request_id=?", "req_evt").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count pipeline events: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 pipeline event, got %d", count)
	}
}

func TestRecordError(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordError(context.Background(), "req_err", "provider", provider.ProviderError{
		Code:       provider.ProviderUnavailable,
		Message:    "ollama offline",
		Suggestion: "start ollama",
	})

	var code string
	err := store.DB().QueryRow("SELECT code FROM errors WHERE request_id=?", "req_err").Scan(&code)
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}
	if code != "PROVIDER_UNAVAILABLE" {
		t.Errorf("expected PROVIDER_UNAVAILABLE, got %s", code)
	}
}

func TestRecordProviderHealth(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordProviderHealth(context.Background(), "ollama", provider.StatusOK, 15*time.Millisecond, provider.ProviderError{})

	var status string
	err := store.DB().QueryRow("SELECT status FROM provider_health WHERE provider=?", "ollama").Scan(&status)
	if err != nil {
		t.Fatalf("failed to read provider health: %v", err)
	}
	if status != "ok" {
		t.Errorf("expected ok, got %s", status)
	}
}

func TestRecentRequests(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordRequest(context.Background(), RequestRecord{
		RequestID: "req_recent",
		CreatedAt: time.Now(),
		Status:    "success",
		Provider:  "ollama",
		Model:     "llama3",
	})

	recent, err := w.RecentRequests(context.Background(), 10)
	if err != nil {
		t.Fatalf("RecentRequests failed: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent request, got %d", len(recent))
	}
	if recent[0].ID != "req_recent" {
		t.Errorf("expected req_recent, got %s", recent[0].ID)
	}
	if recent[0].Provider != "ollama" {
		t.Errorf("expected provider ollama, got %s", recent[0].Provider)
	}
}

func TestWriterDoesNotPanicWhenStorageUnavailable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Telemetry.Local = true
	w := NewNoop(cfg, nil)

	ctx := context.Background()
	w.RecordRequest(ctx, RequestRecord{RequestID: "req_x"})
	w.RecordPipelineEvents(ctx, []PipelineEventRecord{{RequestID: "req_x", Engine: "pipeline", Event: "test", Severity: "info"}})
	w.RecordError(ctx, "req_x", "provider", provider.ProviderError{Code: provider.ProviderUnavailable})
	w.RecordProviderHealth(ctx, "ollama", provider.StatusOffline, 0, provider.ProviderError{Code: provider.ProviderUnavailable})

	recent, err := w.RecentRequests(ctx, 10)
	if err != nil {
		t.Fatalf("RecentRequests on noop writer returned error: %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("expected empty recent requests from noop writer, got %d", len(recent))
	}
}

func TestEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Telemetry.Local = false
	w := NewNoop(cfg, nil)
	if w.Enabled() {
		t.Error("expected telemetry disabled")
	}

	cfg.Telemetry.Local = true
	w2 := NewNoop(cfg, nil)
	if !w2.Enabled() {
		t.Error("expected telemetry enabled")
	}
}

func TestStorageStatusUnavailableForNoop(t *testing.T) {
	w := NewNoop(config.DefaultConfig(), nil)
	if w.StorageStatus() != "unavailable" {
		t.Errorf("expected unavailable, got %s", w.StorageStatus())
	}
}
