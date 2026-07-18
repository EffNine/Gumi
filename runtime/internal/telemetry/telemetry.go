package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/logger"
	"github.com/EffNine/gumi/runtime/internal/provider"
	"github.com/EffNine/gumi/runtime/internal/storage"
)

// schemaVersion is stored in runtime_info for future migration tracking.
const schemaVersion = "1"

// previewMaxChars limits prompt/response previews when logging is enabled.
const previewMaxChars = 200

// Writer records local telemetry. It is safe to call with a nil store: every
// public method returns without panicking and without blocking request handling.
type Writer struct {
	store *storage.Storage
	cfg   *config.Config
	log   *logger.Logger
}

// Open opens the configured SQLite database and returns a telemetry Writer.
// If dbPath is empty, the default ~/.gumi/gumi.db path is used.
func Open(cfg *config.Config, log *logger.Logger) (*Writer, error) {
	path := cfg.Storage.DBPath
	if path == "" {
		path = storage.DefaultPath()
	}

	store, err := storage.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open telemetry storage: %w", err)
	}

	w := &Writer{store: store, cfg: cfg, log: log}
	if err := w.writeRuntimeInfo("database_schema_version", schemaVersion); err != nil && log != nil {
		log.Error("failed to write schema version", err)
	}
	return w, nil
}

// NewWithStorage creates a Writer from an existing storage handle. It is useful
// for tests that want an in-memory database.
func NewWithStorage(store *storage.Storage, cfg *config.Config, log *logger.Logger) *Writer {
	return &Writer{store: store, cfg: cfg, log: log}
}

// NewNoop returns a Writer that does not persist anything. It is used when
// storage fails to initialize so the runtime can start in degraded mode.
func NewNoop(cfg *config.Config, log *logger.Logger) *Writer {
	return &Writer{store: nil, cfg: cfg, log: log}
}

// Enabled reports whether local telemetry is configured.
func (w *Writer) Enabled() bool {
	if w == nil || w.cfg == nil {
		return false
	}
	return w.cfg.Telemetry.Local
}

// StorageStatus returns a short status string for diagnostics.
func (w *Writer) StorageStatus() string {
	if w == nil || w.store == nil {
		return "unavailable"
	}
	if err := w.store.DB().PingContext(context.Background()); err != nil {
		return "degraded"
	}
	return "ok"
}

// Close closes the underlying storage connection.
func (w *Writer) Close() error {
	if w == nil || w.store == nil {
		return nil
	}
	return w.store.Close()
}

// RequestRecord captures the metadata stored for one chat completion request.
type RequestRecord struct {
	RequestID               string
	CreatedAt               time.Time
	WorkspaceID             string
	SessionID               string
	RuntimeMode             string
	Provider                string
	Model                   string
	Status                  string
	Stream                  bool
	LatencyMs               int64
	ProviderLatencyMs       int64
	PromptTokens            int
	CompletionTokens        int
	TotalTokens             int
	ContextCompressed       bool
	ValidationPassed        bool
	RepairApplied           bool
	RetryCount              int
	ErrorCode               string
	PromptLogged            bool
	ResponseLogged          bool
	PromptPreview           string
	ResponsePreview         string
	ThinkingEnabled         string
	ReasoningContentPresent bool
	AgentStepCount          int  `json:"agent_step_count,omitempty"`
	AgentLoopDetected       bool `json:"agent_loop_detected,omitempty"`
}

// PipelineEventRecord captures one pipeline event for storage.
type PipelineEventRecord struct {
	RequestID string
	Timestamp time.Time
	Engine    string
	Event     string
	Severity  string
	Message   string
	Metadata  map[string]string
}

// RecordRequest inserts or replaces the requests row for a chat completion.
func (w *Writer) RecordRequest(ctx context.Context, r RequestRecord) {
	if w == nil || w.store == nil || !w.Enabled() {
		return
	}

	_, err := w.store.DB().ExecContext(ctx, `
		INSERT INTO requests (
			id, created_at, workspace_id, session_id, runtime_mode, provider, model,
			status, stream, latency_ms, provider_latency_ms, prompt_tokens,
			completion_tokens, total_tokens, context_compressed, validation_passed,
			repair_applied, retry_count, error_code, prompt_logged, response_logged,
			prompt_preview, response_preview, thinking_enabled, reasoning_content_present,
			agent_step_count, agent_loop_detected
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			workspace_id = excluded.workspace_id,
			session_id = excluded.session_id,
			runtime_mode = excluded.runtime_mode,
			provider = excluded.provider,
			model = excluded.model,
			status = excluded.status,
			stream = excluded.stream,
			latency_ms = excluded.latency_ms,
			provider_latency_ms = excluded.provider_latency_ms,
			prompt_tokens = excluded.prompt_tokens,
			completion_tokens = excluded.completion_tokens,
			total_tokens = excluded.total_tokens,
			context_compressed = excluded.context_compressed,
			validation_passed = excluded.validation_passed,
			repair_applied = excluded.repair_applied,
			retry_count = excluded.retry_count,
			error_code = excluded.error_code,
			prompt_logged = excluded.prompt_logged,
			response_logged = excluded.response_logged,
			prompt_preview = excluded.prompt_preview,
			response_preview = excluded.response_preview,
			thinking_enabled = excluded.thinking_enabled,
			reasoning_content_present = excluded.reasoning_content_present,
			agent_step_count = excluded.agent_step_count,
			agent_loop_detected = excluded.agent_loop_detected
	`,
		r.RequestID,
		r.CreatedAt.UTC().Format(time.RFC3339),
		r.WorkspaceID,
		r.SessionID,
		r.RuntimeMode,
		r.Provider,
		r.Model,
		r.Status,
		boolToInt(r.Stream),
		r.LatencyMs,
		r.ProviderLatencyMs,
		r.PromptTokens,
		r.CompletionTokens,
		r.TotalTokens,
		boolToInt(r.ContextCompressed),
		boolToInt(r.ValidationPassed),
		boolToInt(r.RepairApplied),
		r.RetryCount,
		r.ErrorCode,
		boolToInt(r.PromptLogged),
		boolToInt(r.ResponseLogged),
		nullableString(r.PromptPreview),
		nullableString(r.ResponsePreview),
		nullableString(r.ThinkingEnabled),
		boolToInt(r.ReasoningContentPresent),
		r.AgentStepCount,
		boolToInt(r.AgentLoopDetected),
	)
	if err != nil && w.log != nil {
		w.log.Error("telemetry: failed to record request", err, "request_id", r.RequestID)
	}
}

// RecordPipelineEvents persists a batch of pipeline events.
func (w *Writer) RecordPipelineEvents(ctx context.Context, events []PipelineEventRecord) {
	if w == nil || w.store == nil || !w.Enabled() || len(events) == 0 {
		return
	}

	for _, e := range events {
		metadataJSON, err := json.Marshal(redactMap(e.Metadata))
		if err != nil {
			metadataJSON = []byte("{}")
		}

		_, err = w.store.DB().ExecContext(ctx, `
			INSERT INTO pipeline_events (request_id, timestamp, engine, event, severity, message, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			e.RequestID,
			e.Timestamp.UTC().Format(time.RFC3339),
			e.Engine,
			e.Event,
			e.Severity,
			e.Message,
			string(metadataJSON),
		)
		if err != nil && w.log != nil {
			w.log.Error("telemetry: failed to record pipeline event", err,
				"request_id", e.RequestID, "event", e.Event)
		}
	}
}

// ValidationReportRecord is the telemetry projection of a validation report.
// It avoids importing the validation package directly to prevent cycles.
type ValidationReportRecord struct {
	RequestID               string
	Passed                  bool
	Severity                string
	Repairable              bool
	SuggestedRepairStrategy string
	Issues                  []ValidationIssueRecord
	Metadata                map[string]string
}

// ValidationIssueRecord is the telemetry projection of a validation issue.
type ValidationIssueRecord struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Location string `json:"location,omitempty"`
}

// RecordValidationReport persists a validation report to the validation_reports
// table so that validation failures can be diagnosed after the fact.
func (w *Writer) RecordValidationReport(ctx context.Context, r ValidationReportRecord) {
	if w == nil || w.store == nil || !w.Enabled() {
		return
	}

	issuesJSON, err := json.Marshal(r.Issues)
	if err != nil {
		issuesJSON = []byte("[]")
	}

	metadataJSON, err := json.Marshal(redactMap(r.Metadata))
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = w.store.DB().ExecContext(ctx, `
		INSERT INTO validation_reports (request_id, created_at, passed, severity, repairable, suggested_repair_strategy, issues_json, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.RequestID,
		time.Now().UTC().Format(time.RFC3339),
		boolToInt(r.Passed),
		r.Severity,
		boolToInt(r.Repairable),
		r.SuggestedRepairStrategy,
		string(issuesJSON),
		string(metadataJSON),
	)
	if err != nil && w.log != nil {
		w.log.Error("telemetry: failed to record validation report", err, "request_id", r.RequestID)
	}
}

// RepairReportRecord is the telemetry projection of a repair report.
type RepairReportRecord struct {
	RequestID       string
	Attempted       bool
	Success         bool
	Strategy        string
	RetryRequested  bool
	ChangesApplied  int
	RemainingIssues int
}

// RecordRepairReport persists a repair report to the repair_reports table.
func (w *Writer) RecordRepairReport(ctx context.Context, r RepairReportRecord) {
	if w == nil || w.store == nil || !w.Enabled() {
		return
	}

	changesJSON := fmt.Sprintf(`{"count":%d}`, r.ChangesApplied)
	remainingJSON := fmt.Sprintf(`{"count":%d}`, r.RemainingIssues)

	_, err := w.store.DB().ExecContext(ctx, `
		INSERT INTO repair_reports (request_id, created_at, attempted, strategy, success, changes_json, remaining_issues_json, retry_requested)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.RequestID,
		time.Now().UTC().Format(time.RFC3339),
		boolToInt(r.Attempted),
		r.Strategy,
		boolToInt(r.Success),
		changesJSON,
		remainingJSON,
		boolToInt(r.RetryRequested),
	)
	if err != nil && w.log != nil {
		w.log.Error("telemetry: failed to record repair report", err, "request_id", r.RequestID)
	}
}

// RecordError persists a normalized runtime or provider error.
func (w *Writer) RecordError(ctx context.Context, requestID string, engine string, perr provider.ProviderError) {
	if w == nil || w.store == nil || !w.Enabled() {
		return
	}

	retryable := perr.Code == provider.ProviderUnavailable ||
		perr.Code == provider.ProviderTimeout ||
		perr.Code == provider.ProviderBadResponse

	details := map[string]string{}
	if perr.Cause != nil {
		details["cause"] = perr.Cause.Error()
	}
	detailsJSON, _ := json.Marshal(details)
	detailsJSON = RedactJSON(detailsJSON)

	_, err := w.store.DB().ExecContext(ctx, `
		INSERT INTO errors (request_id, created_at, code, type, engine, message, retryable, suggestion, details_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		requestID,
		time.Now().UTC().Format(time.RFC3339),
		string(perr.Code),
		providerErrorType(perr.Code),
		engine,
		perr.Message,
		boolToInt(retryable),
		perr.Suggestion,
		string(detailsJSON),
	)
	if err != nil && w.log != nil {
		w.log.Error("telemetry: failed to record error", err, "request_id", requestID, "code", string(perr.Code))
	}
}

// RecordProviderHealth persists the result of a provider health check.
func (w *Writer) RecordProviderHealth(ctx context.Context, name string, status provider.ProviderStatus, latency time.Duration, perr provider.ProviderError) {
	if w == nil || w.store == nil || !w.Enabled() {
		return
	}

	var message, errorCode string
	if perr.Code != "" {
		message = perr.Message
		errorCode = string(perr.Code)
	}

	metadata := map[string]string{
		"status": string(status),
	}
	metadataJSON, _ := json.Marshal(redactMap(metadata))

	_, err := w.store.DB().ExecContext(ctx, `
		INSERT INTO provider_health (provider, checked_at, status, latency_ms, message, error_code, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		name,
		time.Now().UTC().Format(time.RFC3339),
		string(status),
		int64(latency.Milliseconds()),
		message,
		errorCode,
		string(metadataJSON),
	)
	if err != nil && w.log != nil {
		w.log.Error("telemetry: failed to record provider health", err, "provider", name)
	}
}

// TelemetryFilter holds optional filter fields for querying telemetry data.
type TelemetryFilter struct {
	Start       *time.Time
	End         *time.Time
	Provider    string
	Model       string
	Status      string
	RuntimeMode string
	ErrorCode   string
	RequestID   string
}

// TimeBucket holds aggregated telemetry data for a single time bucket.
type TimeBucket struct {
	Bucket      string  `json:"bucket"`
	Count       int     `json:"count"`
	AvgLatency  float64 `json:"avg_latency"`
	MinLatency  float64 `json:"min_latency"`
	MaxLatency  float64 `json:"max_latency"`
	P50Latency  float64 `json:"p50_latency"`
	P95Latency  float64 `json:"p95_latency"`
	P99Latency  float64 `json:"p99_latency"`
	ErrorCount  int     `json:"error_count"`
	RepairCount int     `json:"repair_count"`
}

// QueryRequests returns recent request metadata filtered by the given filter.
// Results are ordered by created_at DESC, id DESC with limit/offset pagination.
func (w *Writer) QueryRequests(ctx context.Context, filter TelemetryFilter, limit, offset int) ([]RecentRequest, error) {
	if w == nil || w.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	var conditions []string
	var args []interface{}

	if filter.Start != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.Start.UTC().Format(time.RFC3339))
	}
	if filter.End != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, filter.End.UTC().Format(time.RFC3339))
	}
	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.RuntimeMode != "" {
		conditions = append(conditions, "runtime_mode = ?")
		args = append(args, filter.RuntimeMode)
	}
	if filter.ErrorCode != "" {
		conditions = append(conditions, "error_code = ?")
		args = append(args, filter.ErrorCode)
	}
	if filter.RequestID != "" {
		conditions = append(conditions, "id = ?")
		args = append(args, filter.RequestID)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := `SELECT id, created_at, runtime_mode, provider, model, status, latency_ms, error_code, repair_applied, retry_count, thinking_enabled, reasoning_content_present
		FROM requests` + where + ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := w.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RecentRequest
	for rows.Next() {
		var r RecentRequest
		var errorCode sql.NullString
		var latencyMs sql.NullInt64
		var repairApplied int
		var provider, model sql.NullString
		var thinkingEnabled sql.NullString
		var reasoningContentPresent int

		if err := rows.Scan(
			&r.ID, &r.CreatedAt, &r.RuntimeMode, &provider, &model,
			&r.Status, &latencyMs, &errorCode, &repairApplied,
			&r.RetryCount, &thinkingEnabled, &reasoningContentPresent,
		); err != nil {
			if w.log != nil {
				w.log.Error("telemetry: failed to scan query request", err)
			}
			continue
		}
		r.Provider = provider.String
		r.Model = model.String
		r.ErrorCode = errorCode.String
		r.LatencyMs = latencyMs.Int64
		r.RepairApplied = repairApplied != 0
		r.ThinkingEnabled = thinkingEnabled.String
		r.ReasoningContentPresent = reasoningContentPresent != 0
		result = append(result, r)
	}
	return result, rows.Err()
}

// CountRequests returns the number of requests matching the filter.
func (w *Writer) CountRequests(ctx context.Context, filter TelemetryFilter) (int, error) {
	if w == nil || w.store == nil {
		return 0, nil
	}

	var conditions []string
	var args []interface{}

	if filter.Start != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.Start.UTC().Format(time.RFC3339))
	}
	if filter.End != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, filter.End.UTC().Format(time.RFC3339))
	}
	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.RuntimeMode != "" {
		conditions = append(conditions, "runtime_mode = ?")
		args = append(args, filter.RuntimeMode)
	}
	if filter.ErrorCode != "" {
		conditions = append(conditions, "error_code = ?")
		args = append(args, filter.ErrorCode)
	}
	if filter.RequestID != "" {
		conditions = append(conditions, "id = ?")
		args = append(args, filter.RequestID)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	err := w.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM requests`+where, args...).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// AggregateRequests returns time-bucketed aggregations of request data.
// bucket can be "hour" or "day"; defaults to "hour".
func (w *Writer) AggregateRequests(ctx context.Context, filter TelemetryFilter, bucket string) ([]TimeBucket, error) {
	if w == nil || w.store == nil {
		return nil, nil
	}

	if bucket != "day" {
		bucket = "hour"
	}

	var timeFmt string
	if bucket == "day" {
		timeFmt = "%Y-%m-%d"
	} else {
		timeFmt = "%Y-%m-%d %H:00"
	}

	var conditions []string
	var args []interface{}

	if filter.Start != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.Start.UTC().Format(time.RFC3339))
	}
	if filter.End != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, filter.End.UTC().Format(time.RFC3339))
	}
	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.RuntimeMode != "" {
		conditions = append(conditions, "runtime_mode = ?")
		args = append(args, filter.RuntimeMode)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Fetch all matching latencies per bucket for percentile computation in Go
	query := `SELECT ` + "strftime('" + timeFmt + "', created_at)" + ` AS bucket,
		COUNT(*) AS count,
		AVG(COALESCE(latency_ms, 0)) AS avg_latency,
		MIN(COALESCE(latency_ms, 0)) AS min_latency,
		MAX(COALESCE(latency_ms, 0)) AS max_latency,
		SUM(CASE WHEN status = 'error' OR error_code != '' THEN 1 ELSE 0 END) AS error_count,
		SUM(repair_applied) AS repair_count
		FROM requests` + where + ` GROUP BY bucket ORDER BY bucket ASC`

	rows, err := w.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimeBucket
	for rows.Next() {
		var tb TimeBucket
		if err := rows.Scan(
			&tb.Bucket, &tb.Count, &tb.AvgLatency, &tb.MinLatency, &tb.MaxLatency,
			&tb.ErrorCount, &tb.RepairCount,
		); err != nil {
			if w.log != nil {
				w.log.Error("telemetry: failed to scan aggregate", err)
			}
			continue
		}
		result = append(result, tb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Compute percentiles per bucket by fetching individual latencies.
	// Apply the same filters as the aggregate query so percentiles match counts.
	for i := range result {
		percConditions := append([]string{"strftime('" + timeFmt + "', created_at) = ?"}, conditions...)
		percArgs := append([]interface{}{result[i].Bucket}, args...)
		percWhere := " WHERE " + strings.Join(percConditions, " AND ")
		latRows, err := w.store.DB().QueryContext(ctx,
			`SELECT COALESCE(latency_ms, 0) FROM requests`+percWhere+` ORDER BY latency_ms ASC`, percArgs...)
		if err != nil {
			continue
		}
		var lats []float64
		for latRows.Next() {
			var lat float64
			if err := latRows.Scan(&lat); err == nil {
				lats = append(lats, lat)
			}
		}
		_ = latRows.Err()
		latRows.Close()
		if len(lats) > 0 {
			result[i].P50Latency = percentile(lats, 50)
			result[i].P95Latency = percentile(lats, 95)
			result[i].P99Latency = percentile(lats, 99)
		}
	}

	return result, nil
}

// percentile returns the p-th percentile from a sorted slice of float64s.
func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	n := float64(p) / 100.0 * float64(len(sorted)-1)
	idx := int(n)
	frac := n - float64(idx)
	if idx >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	return sorted[idx] + frac*(sorted[idx+1]-sorted[idx])
}

// ListPipelineEvents returns pipeline events for a given request ID.
func (w *Writer) ListPipelineEvents(ctx context.Context, requestID string, limit int) ([]PipelineEventRecord, error) {
	if w == nil || w.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := w.store.DB().QueryContext(ctx, `
		SELECT request_id, timestamp, engine, event, severity, message, COALESCE(metadata_json, '{}')
		FROM pipeline_events
		WHERE request_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, requestID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PipelineEventRecord
	for rows.Next() {
		var r PipelineEventRecord
		var tsStr, metadataJSON string
		if err := rows.Scan(&r.RequestID, &tsStr, &r.Engine, &r.Event, &r.Severity, &r.Message, &metadataJSON); err != nil {
			if w.log != nil {
				w.log.Error("telemetry: failed to scan pipeline event", err)
			}
			continue
		}
		if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
			r.Timestamp = t
		}
		_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
		result = append(result, r)
	}
	return result, rows.Err()
}

// ListProviderHealth returns provider health check records, optionally filtered.
func (w *Writer) ListProviderHealth(ctx context.Context, filter TelemetryFilter, limit int) ([]ProviderHealthRecord, error) {
	if w == nil || w.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	var conditions []string
	var args []interface{}

	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Start != nil {
		conditions = append(conditions, "checked_at >= ?")
		args = append(args, filter.Start.UTC().Format(time.RFC3339))
	}
	if filter.End != nil {
		conditions = append(conditions, "checked_at <= ?")
		args = append(args, filter.End.UTC().Format(time.RFC3339))
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := `SELECT provider, checked_at, status, COALESCE(latency_ms, 0), COALESCE(message, ''), COALESCE(error_code, '')
		FROM provider_health` + where + ` ORDER BY checked_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := w.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProviderHealthRecord
	for rows.Next() {
		var r ProviderHealthRecord
		if err := rows.Scan(&r.Provider, &r.CheckedAt, &r.Status, &r.LatencyMs, &r.Message, &r.ErrorCode); err != nil {
			if w.log != nil {
				w.log.Error("telemetry: failed to scan provider health", err)
			}
			continue
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ProviderHealthRecord is a single provider health check record.
type ProviderHealthRecord struct {
	Provider  string `json:"provider"`
	CheckedAt string `json:"checked_at"`
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

// RecentRequest is the public metadata returned by the recent telemetry API.
type RecentRequest struct {
	ID                      string `json:"id"`
	CreatedAt               string `json:"created_at"`
	RuntimeMode             string `json:"runtime_mode"`
	Provider                string `json:"provider"`
	Model                   string `json:"model"`
	Status                  string `json:"status"`
	LatencyMs               int64  `json:"latency_ms"`
	ErrorCode               string `json:"error_code,omitempty"`
	RepairApplied           bool   `json:"repair_applied"`
	RetryCount              int    `json:"retry_count"`
	ThinkingEnabled         string `json:"thinking_enabled,omitempty"`
	ReasoningContentPresent bool   `json:"reasoning_content_present"`
}

// RecentRequests returns the most recent request metadata, newest first.
func (w *Writer) RecentRequests(ctx context.Context, limit int) ([]RecentRequest, error) {
	if w == nil || w.store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := w.store.DB().QueryContext(ctx, `
		SELECT id, created_at, runtime_mode, provider, model, status, latency_ms, error_code, repair_applied, retry_count, thinking_enabled, reasoning_content_present
		FROM requests
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RecentRequest
	for rows.Next() {
		var r RecentRequest
		var errorCode sql.NullString
		var latencyMs sql.NullInt64
		var repairApplied int
		var provider, model sql.NullString
		var thinkingEnabled sql.NullString
		var reasoningContentPresent int

		if err := rows.Scan(
			&r.ID,
			&r.CreatedAt,
			&r.RuntimeMode,
			&provider,
			&model,
			&r.Status,
			&latencyMs,
			&errorCode,
			&repairApplied,
			&r.RetryCount,
			&thinkingEnabled,
			&reasoningContentPresent,
		); err != nil {
			if w.log != nil {
				w.log.Error("telemetry: failed to scan recent request", err)
			}
			continue
		}
		r.Provider = provider.String
		r.Model = model.String
		r.ErrorCode = errorCode.String
		r.LatencyMs = latencyMs.Int64
		r.RepairApplied = repairApplied != 0
		r.ThinkingEnabled = thinkingEnabled.String
		r.ReasoningContentPresent = reasoningContentPresent != 0
		result = append(result, r)
	}
	return result, rows.Err()
}

// writeRuntimeInfo writes a key/value pair to the runtime_info table.
func (w *Writer) writeRuntimeInfo(key, value string) error {
	if w == nil || w.store == nil {
		return nil
	}
	_, err := w.store.DB().ExecContext(context.Background(), `
		INSERT INTO runtime_info (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().UTC().Format(time.RFC3339))
	return err
}

// boolToInt converts a bool to a SQLite integer.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableString returns a SQL NULL for empty strings so preview columns stay
// null unless a preview is explicitly produced.
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// providerErrorType maps provider error codes to API error categories.
func providerErrorType(code provider.ProviderErrorCode) string {
	switch code {
	case provider.ProviderUnavailable, provider.ProviderTimeout,
		provider.ProviderBadResponse, provider.ModelNotFound,
		provider.StreamingUnsupported:
		return "provider_error"
	case provider.ProviderAuthError:
		return "auth_error"
	case provider.EmptyPrompt:
		return "request_error"
	case provider.ContextLimitExceeded:
		return "context_error"
	case provider.ValidationFailed:
		return "validation_error"
	case provider.ProviderMisconfigured:
		return "config_error"
	default:
		return "provider_error"
	}
}

// redactMap redacts sensitive keys in a string map. The returned map is a copy.
func redactMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			out[k] = redactedPlaceholder
			continue
		}
		// Also redact values that look like Authorization headers.
		if strings.EqualFold(k, "authorization") || strings.HasPrefix(strings.ToLower(v), "bearer ") {
			out[k] = redactedPlaceholder
			continue
		}
		out[k] = v
	}
	return out
}

// Preview returns a short preview of text if logging is enabled, otherwise an
// empty string. The returned value is safe to store when the caller has already
// checked the logging flag.
func Preview(text string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = previewMaxChars
	}
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen]
}

// ExtractContentPreview returns a short preview of the first text message in a
// chat completion request, or an empty string if previews are disabled.
func ExtractContentPreview(req api.ChatCompletionRequest, enabled bool) string {
	if !enabled {
		return ""
	}
	for _, m := range req.Messages {
		if m.Role == "user" || m.Role == "system" || m.Role == "assistant" {
			if s, ok := m.Content.(string); ok && s != "" {
				return Preview(s, previewMaxChars)
			}
		}
	}
	return ""
}

// ExtractResponsePreview returns a short preview of the first assistant message
// in a chat completion response, or an empty string if previews are disabled.
func ExtractResponsePreview(resp *api.ChatCompletionResponse, enabled bool) string {
	if !enabled || resp == nil {
		return ""
	}
	for _, c := range resp.Choices {
		if s, ok := c.Message.Content.(string); ok && s != "" {
			return Preview(s, previewMaxChars)
		}
	}
	return ""
}
