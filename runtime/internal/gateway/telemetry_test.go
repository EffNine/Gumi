package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
	"github.com/EffNine/gumi/runtime/internal/provider"
	"github.com/EffNine/gumi/runtime/internal/telemetry"
)

// telemetryTestHelper creates a test server with telemetry enabled and seeds
// some request data so the endpoints return non-empty results.
func telemetryTestHelper(t *testing.T) *Server {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "disabled"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Telemetry.Local = true
	// Use a temp directory for the database
	cfg.Storage.DBPath = fmt.Sprintf("%s/gumi_telemetry_test.db", t.TempDir())

	log := logger.New("error")
	srv := New(cfg, log)

	// Seed some telemetry data
	if srv.telemetry != nil {
		ctx := context.Background()
		now := time.Now()
		for i := 0; i < 5; i++ {
			status := "success"
			errorCode := ""
			if i%2 == 0 {
				status = "error"
				errorCode = "PROVIDER_UNAVAILABLE"
			}
			srv.telemetry.RecordRequest(ctx, telemetry.RequestRecord{
				RequestID:     fmt.Sprintf("test_req_%d", i),
				CreatedAt:     now.Add(-time.Duration(i) * 30 * time.Minute),
				WorkspaceID:   "default",
				RuntimeMode:   "stabilized",
				Provider:      "ollama",
				Model:         "llama3",
				Status:        status,
				LatencyMs:     int64(100 + i*100),
				ErrorCode:     errorCode,
				RepairApplied: i%3 == 0,
				RetryCount:    i % 2,
			})
		}
		// Seed pipeline events
		srv.telemetry.RecordPipelineEvents(ctx, []telemetry.PipelineEventRecord{
			{RequestID: "test_req_0", Timestamp: now, Engine: "pipeline", Event: "start", Severity: "info", Message: "request started"},
			{RequestID: "test_req_0", Timestamp: now, Engine: "provider", Event: "inference", Severity: "info", Message: "inference completed"},
		})
		// Seed provider health
		srv.telemetry.RecordProviderHealth(ctx, "ollama", provider.StatusOK, 15*time.Millisecond, provider.ProviderError{})
	}

	return srv
}

func TestTelemetryRequestsEndpoint(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/requests", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Object string                    `json:"object"`
		Data   []telemetry.RecentRequest `json:"data"`
		Total  int                       `json:"total"`
		Limit  int                       `json:"limit"`
		Offset int                       `json:"offset"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Object != "gumi.telemetry.requests" {
		t.Errorf("expected object gumi.telemetry.requests, got %s", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Error("expected at least 1 request in response")
	}
	if resp.Limit <= 0 {
		t.Errorf("expected positive limit, got %d", resp.Limit)
	}
}

func TestTelemetryRequestsWithProviderFilter(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/requests?provider=ollama", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Data []telemetry.RecentRequest `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Data) == 0 {
		t.Error("expected results for ollama filter")
	}
	for _, r := range resp.Data {
		if r.Provider != "ollama" {
			t.Errorf("expected provider ollama, got %s", r.Provider)
		}
	}
}

func TestTelemetryRequestsWithStatusFilter(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/requests?status=error", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Data []telemetry.RecentRequest `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Data) == 0 {
		t.Error("expected results for error status filter")
	}
	for _, r := range resp.Data {
		if r.Status != "error" {
			t.Errorf("expected status error, got %s", r.Status)
		}
	}
}

func TestTelemetryRequestsWithLimit(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/requests?limit=2", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Data  []telemetry.RecentRequest `json:"data"`
		Limit int                       `json:"limit"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Data) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(resp.Data))
	}
	if resp.Limit != 2 {
		t.Errorf("expected limit 2, got %d", resp.Limit)
	}
}

func TestTelemetryAggregateEndpoint(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/aggregate?bucket=hour", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Object string                 `json:"object"`
		Data   []telemetry.TimeBucket `json:"data"`
		Bucket string                 `json:"bucket"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Object != "gumi.telemetry.aggregate" {
		t.Errorf("expected object gumi.telemetry.aggregate, got %s", resp.Object)
	}
	if resp.Bucket != "hour" {
		t.Errorf("expected bucket hour, got %s", resp.Bucket)
	}
	if len(resp.Data) == 0 {
		t.Error("expected at least 1 bucket")
	}
}

func TestTelemetryAggregateDayBucket(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/aggregate?bucket=day", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Bucket string `json:"bucket"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Bucket != "day" {
		t.Errorf("expected bucket day, got %s", resp.Bucket)
	}
}

func TestTelemetryEventsEndpoint(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/events/test_req_0", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Object string                          `json:"object"`
		Data   []telemetry.PipelineEventRecord `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Object != "gumi.telemetry.events" {
		t.Errorf("expected object gumi.telemetry.events, got %s", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Error("expected at least 1 pipeline event")
	}
}

func TestTelemetryEventsMissingRequestID(t *testing.T) {
	srv := telemetryTestHelper(t)

	// Use a path without request_id
	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/events/", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	// This should 404 because the route won't match an empty request_id
	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for missing request_id")
	}
}

func TestTelemetryHealthEndpoint(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/health", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Object string                           `json:"object"`
		Data   []telemetry.ProviderHealthRecord `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Object != "gumi.telemetry.health" {
		t.Errorf("expected object gumi.telemetry.health, got %s", resp.Object)
	}
}

func TestTelemetryHealthWithProviderFilter(t *testing.T) {
	srv := telemetryTestHelper(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/health?provider=ollama", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Data []telemetry.ProviderHealthRecord `json:"data"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	for _, r := range resp.Data {
		if r.Provider != "ollama" {
			t.Errorf("expected provider ollama, got %s", r.Provider)
		}
	}
}

func TestTelemetryEndpointsRequireAuth(t *testing.T) {
	// Test with auth mode = local
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Telemetry.Local = true
	cfg.Storage.DBPath = fmt.Sprintf("%s/gumi_auth_test.db", t.TempDir())

	srv := New(cfg, logger.New("error"))

	endpoints := []string{
		"/v1/gumi/telemetry/requests",
		"/v1/gumi/telemetry/aggregate",
		"/v1/gumi/telemetry/events/test_req",
		"/v1/gumi/telemetry/health",
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		rr := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", ep, rr.Code)
		}
	}
}

func TestTelemetryEndpointsAuthorized(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Mode = "local"
	cfg.Runtime.Host = "127.0.0.1"
	cfg.Runtime.Port = 0
	cfg.Telemetry.Local = true
	cfg.Storage.DBPath = fmt.Sprintf("%s/gumi_auth_ok_test.db", t.TempDir())

	srv := New(cfg, logger.New("error"))

	endpoints := []string{
		"/v1/gumi/telemetry/requests",
		"/v1/gumi/telemetry/aggregate",
		"/v1/gumi/telemetry/events/test_req",
		"/v1/gumi/telemetry/health",
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		req.Header.Set("Authorization", "Bearer gumi-local")
		rr := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rr, req)

		// These may return 200 or 500 depending on whether telemetry is available,
		// but should not return 401
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("%s: expected non-401 with valid auth, got %d", ep, rr.Code)
		}
	}
}

func TestParseTelemetryFilter(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/gumi/telemetry/requests?provider=ollama&model=llama3&status=error&mode=stabilized&start=2026-01-01T00:00:00Z&end=2026-12-31T23:59:59Z&error_code=PROVIDER_UNAVAILABLE&request_id=req_abc", nil)
	f := parseTelemetryFilter(req)

	if f.Provider != "ollama" {
		t.Errorf("expected provider ollama, got %s", f.Provider)
	}
	if f.Model != "llama3" {
		t.Errorf("expected model llama3, got %s", f.Model)
	}
	if f.Status != "error" {
		t.Errorf("expected status error, got %s", f.Status)
	}
	if f.RuntimeMode != "stabilized" {
		t.Errorf("expected mode stabilized, got %s", f.RuntimeMode)
	}
	if f.ErrorCode != "PROVIDER_UNAVAILABLE" {
		t.Errorf("expected error_code PROVIDER_UNAVAILABLE, got %s", f.ErrorCode)
	}
	if f.RequestID != "req_abc" {
		t.Errorf("expected request_id req_abc, got %s", f.RequestID)
	}
	if f.Start == nil {
		t.Error("expected start time to be parsed")
	}
	if f.End == nil {
		t.Error("expected end time to be parsed")
	}
}

func TestParseIntParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=50&offset=10", nil)
	if v := parseIntParam(req, "limit", 100); v != 50 {
		t.Errorf("expected 50, got %d", v)
	}
	if v := parseIntParam(req, "offset", 0); v != 10 {
		t.Errorf("expected 10, got %d", v)
	}
	if v := parseIntParam(req, "nonexistent", 42); v != 42 {
		t.Errorf("expected default 42, got %d", v)
	}
	if v := parseIntParam(req, "limit", 100); v != 50 {
		t.Errorf("expected 50, got %d", v)
	}
}

// TestTelemetryHealthRecordJSON checks that ProviderHealthRecord serializes correctly.
func TestTelemetryHealthRecordJSON(t *testing.T) {
	rec := telemetry.ProviderHealthRecord{
		Provider:  "ollama",
		CheckedAt: "2026-07-17T12:00:00Z",
		Status:    "ok",
		LatencyMs: 15,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"provider":"ollama"`) {
		t.Errorf("expected provider in JSON, got %s", string(data))
	}
	if !strings.Contains(string(data), `"status":"ok"`) {
		t.Errorf("expected status in JSON, got %s", string(data))
	}
}
