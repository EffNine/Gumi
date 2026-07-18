package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/telemetry"
)

// ─── Request helpers ───

// parseTelemetryFilter extracts a TelemetryFilter from query parameters.
func parseTelemetryFilter(r *http.Request) telemetry.TelemetryFilter {
	var f telemetry.TelemetryFilter

	if s := r.URL.Query().Get("start"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.Start = &t
		}
	}
	if s := r.URL.Query().Get("end"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.End = &t
		}
	}
	f.Provider = r.URL.Query().Get("provider")
	f.Model = r.URL.Query().Get("model")
	f.Status = r.URL.Query().Get("status")
	f.RuntimeMode = r.URL.Query().Get("mode")
	f.ErrorCode = r.URL.Query().Get("error_code")
	f.RequestID = r.URL.Query().Get("request_id")

	return f
}

// parseIntParam parses an integer query parameter with a default value.
func parseIntParam(r *http.Request, name string, defaultVal int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}

// ─── Response types ───

type telemetryListResponse struct {
	Object string      `json:"object"`
	Data   interface{} `json:"data"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type telemetryAggregateResponse struct {
	Object string                 `json:"object"`
	Data   []telemetry.TimeBucket `json:"data"`
	Bucket string                 `json:"bucket"`
}

// ─── Handlers ───

// GET /v1/gumi/telemetry/requests
func (s *Server) handleTelemetryRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := parseTelemetryFilter(r)
	limit := parseIntParam(r, "limit", 100)
	offset := parseIntParam(r, "offset", 0)

	results, err := s.telemetry.QueryRequests(ctx, filter, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("TELEMETRY_ERROR", fmt.Sprintf("failed to query requests: %v", err), requestIDFromContext(ctx)))
		return
	}
	if results == nil {
		results = []telemetry.RecentRequest{}
	}

	total, err := s.telemetry.CountRequests(ctx, filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("TELEMETRY_ERROR", fmt.Sprintf("failed to count requests: %v", err), requestIDFromContext(ctx)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(telemetryListResponse{
		Object: "gumi.telemetry.requests",
		Data:   results,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GET /v1/gumi/telemetry/aggregate
func (s *Server) handleTelemetryAggregate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := parseTelemetryFilter(r)
	bucket := r.URL.Query().Get("bucket")
	if bucket == "" {
		bucket = "hour"
	}

	results, err := s.telemetry.AggregateRequests(ctx, filter, bucket)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("TELEMETRY_ERROR", fmt.Sprintf("failed to aggregate requests: %v", err), requestIDFromContext(ctx)))
		return
	}
	if results == nil {
		results = []telemetry.TimeBucket{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(telemetryAggregateResponse{
		Object: "gumi.telemetry.aggregate",
		Data:   results,
		Bucket: bucket,
	})
}

// GET /v1/gumi/telemetry/events/{request_id}
func (s *Server) handleTelemetryEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := r.PathValue("request_id")
	if requestID == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_REQUEST_ID", "request_id path parameter is required", requestIDFromContext(ctx)))
		return
	}

	limit := parseIntParam(r, "limit", 100)
	events, err := s.telemetry.ListPipelineEvents(ctx, requestID, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("TELEMETRY_ERROR", fmt.Sprintf("failed to list pipeline events: %v", err), requestIDFromContext(ctx)))
		return
	}
	if events == nil {
		events = []telemetry.PipelineEventRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "gumi.telemetry.events",
		"data":   events,
	})
}

// GET /v1/gumi/telemetry/health
func (s *Server) handleTelemetryHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := parseTelemetryFilter(r)
	limit := parseIntParam(r, "limit", 100)

	records, err := s.telemetry.ListProviderHealth(ctx, filter, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("TELEMETRY_ERROR", fmt.Sprintf("failed to list provider health: %v", err), requestIDFromContext(ctx)))
		return
	}
	if records == nil {
		records = []telemetry.ProviderHealthRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"object": "gumi.telemetry.health",
		"data":   records,
	})
}
