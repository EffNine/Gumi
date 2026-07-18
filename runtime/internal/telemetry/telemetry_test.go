package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

func seedTestData(t *testing.T, w *Writer) {
	t.Helper()
	now := time.Now()
	for i := 0; i < 10; i++ {
		status := "success"
		errorCode := ""
		if i%3 == 0 {
			status = "error"
			errorCode = "PROVIDER_UNAVAILABLE"
		}
		repair := i%5 == 0
		w.RecordRequest(context.Background(), RequestRecord{
			RequestID:     fmt.Sprintf("req_query_%d", i),
			CreatedAt:     now.Add(-time.Duration(i) * time.Hour),
			WorkspaceID:   "default",
			RuntimeMode:   "stabilized",
			Provider:      "ollama",
			Model:         "llama3",
			Status:        status,
			LatencyMs:     int64(100 + i*50),
			ErrorCode:     errorCode,
			RepairApplied: repair,
			RetryCount:    i % 3,
		})
	}
	// Add a few with different provider/model
	w.RecordRequest(context.Background(), RequestRecord{
		RequestID:   "req_lmstudio",
		CreatedAt:   now.Add(-30 * time.Minute),
		WorkspaceID: "default",
		RuntimeMode: "direct",
		Provider:    "lmstudio",
		Model:       "qwen3-8b",
		Status:      "success",
		LatencyMs:   200,
	})
	w.RecordRequest(context.Background(), RequestRecord{
		RequestID:   "req_failed",
		CreatedAt:   now.Add(-2 * time.Hour),
		WorkspaceID: "default",
		RuntimeMode: "stabilized",
		Provider:    "ollama",
		Model:       "llama3",
		Status:      "failed",
		LatencyMs:   5000,
		ErrorCode:   "CONTEXT_LIMIT_EXCEEDED",
	})
}

func TestQueryRequestsWithProviderFilter(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	results, err := w.QueryRequests(context.Background(), TelemetryFilter{Provider: "lmstudio"}, 10, 0)
	if err != nil {
		t.Fatalf("QueryRequests failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for lmstudio, got %d", len(results))
	}
	if results[0].ID != "req_lmstudio" {
		t.Errorf("expected req_lmstudio, got %s", results[0].ID)
	}
}

func TestQueryRequestsWithModelFilter(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	results, err := w.QueryRequests(context.Background(), TelemetryFilter{Model: "qwen3-8b"}, 10, 0)
	if err != nil {
		t.Fatalf("QueryRequests failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for qwen3-8b, got %d", len(results))
	}
	if results[0].Provider != "lmstudio" {
		t.Errorf("expected lmstudio provider, got %s", results[0].Provider)
	}
}

func TestQueryRequestsWithStatusFilter(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	results, err := w.QueryRequests(context.Background(), TelemetryFilter{Status: "error"}, 10, 0)
	if err != nil {
		t.Fatalf("QueryRequests failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 error result")
	}
	for _, r := range results {
		if r.Status != "error" {
			t.Errorf("expected status error, got %s", r.Status)
		}
	}
}

func TestQueryRequestsWithTimeRange(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)
	results, err := w.QueryRequests(context.Background(), TelemetryFilter{Start: &start, End: &end}, 10, 0)
	if err != nil {
		t.Fatalf("QueryRequests failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result in time range")
	}
}

func TestQueryRequestsWithLimitOffset(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	results, err := w.QueryRequests(context.Background(), TelemetryFilter{}, 3, 0)
	if err != nil {
		t.Fatalf("QueryRequests failed: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestAggregateRequestsHourBucket(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	buckets, err := w.AggregateRequests(context.Background(), TelemetryFilter{}, "hour")
	if err != nil {
		t.Fatalf("AggregateRequests failed: %v", err)
	}
	if len(buckets) == 0 {
		t.Fatal("expected at least 1 bucket")
	}
	// Check that buckets have valid data
	for _, b := range buckets {
		if b.Count <= 0 {
			t.Errorf("expected positive count, got %d", b.Count)
		}
	}
}

func TestAggregateRequestsDayBucket(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	buckets, err := w.AggregateRequests(context.Background(), TelemetryFilter{}, "day")
	if err != nil {
		t.Fatalf("AggregateRequests failed: %v", err)
	}
	if len(buckets) == 0 {
		t.Fatal("expected at least 1 bucket")
	}
}

func TestAggregateRequestsFilteredPercentiles(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	now := time.Now().UTC().Truncate(time.Hour)
	// Same hour bucket, different providers/latencies.
	w.RecordRequest(context.Background(), RequestRecord{
		RequestID: "agg_ollama_fast", CreatedAt: now, Provider: "ollama", Model: "llama3",
		Status: "success", LatencyMs: 100, RuntimeMode: "stabilized",
	})
	w.RecordRequest(context.Background(), RequestRecord{
		RequestID: "agg_ollama_slow", CreatedAt: now.Add(time.Minute), Provider: "ollama", Model: "llama3",
		Status: "success", LatencyMs: 300, RuntimeMode: "stabilized",
	})
	w.RecordRequest(context.Background(), RequestRecord{
		RequestID: "agg_lms_outlier", CreatedAt: now.Add(2 * time.Minute), Provider: "lmstudio", Model: "qwen",
		Status: "success", LatencyMs: 9000, RuntimeMode: "direct",
	})

	buckets, err := w.AggregateRequests(context.Background(), TelemetryFilter{Provider: "ollama"}, "hour")
	if err != nil {
		t.Fatalf("AggregateRequests failed: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	b := buckets[0]
	if b.Count != 2 {
		t.Fatalf("expected count 2 for ollama filter, got %d", b.Count)
	}
	if b.P50Latency < 100 || b.P50Latency > 300 {
		t.Fatalf("expected p50 within ollama latencies, got %v", b.P50Latency)
	}
	if b.P95Latency > 300.1 {
		t.Fatalf("filtered p95 should ignore lmstudio outlier, got %v", b.P95Latency)
	}
}

func TestCountRequestsMatchesFilter(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()
	seedTestData(t, w)

	total, err := w.CountRequests(context.Background(), TelemetryFilter{Provider: "lmstudio"})
	if err != nil {
		t.Fatalf("CountRequests failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1 for lmstudio, got %d", total)
	}
}

func TestListPipelineEventsByRequestID(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordPipelineEvents(context.Background(), []PipelineEventRecord{
		{RequestID: "req_pipe_1", Timestamp: time.Now(), Engine: "pipeline", Event: "start", Severity: "info", Message: "started"},
		{RequestID: "req_pipe_1", Timestamp: time.Now(), Engine: "pipeline", Event: "end", Severity: "info", Message: "ended"},
		{RequestID: "req_pipe_2", Timestamp: time.Now(), Engine: "pipeline", Event: "start", Severity: "info", Message: "other"},
	})

	events, err := w.ListPipelineEvents(context.Background(), "req_pipe_1", 10)
	if err != nil {
		t.Fatalf("ListPipelineEvents failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for req_pipe_1, got %d", len(events))
	}
}

func TestListProviderHealth(t *testing.T) {
	w, store := newTestWriter(t)
	defer store.Close()

	w.RecordProviderHealth(context.Background(), "ollama", provider.StatusOK, 15*time.Millisecond, provider.ProviderError{})
	w.RecordProviderHealth(context.Background(), "lmstudio", provider.StatusOffline, 0, provider.ProviderError{Code: provider.ProviderUnavailable})

	records, err := w.ListProviderHealth(context.Background(), TelemetryFilter{}, 10)
	if err != nil {
		t.Fatalf("ListProviderHealth failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 health records, got %d", len(records))
	}

	// Filter by provider
	ollamaRecords, err := w.ListProviderHealth(context.Background(), TelemetryFilter{Provider: "ollama"}, 10)
	if err != nil {
		t.Fatalf("ListProviderHealth with filter failed: %v", err)
	}
	if len(ollamaRecords) != 1 {
		t.Fatalf("expected 1 ollama record, got %d", len(ollamaRecords))
	}
	if ollamaRecords[0].Status != "ok" {
		t.Errorf("expected status ok, got %s", ollamaRecords[0].Status)
	}
}
