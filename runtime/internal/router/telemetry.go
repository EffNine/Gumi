package router

import (
	"fmt"
	"strconv"
)

// ---------------------------------------------------------------------------
// RoutingTelemetry records every routing decision for observability.
// ---------------------------------------------------------------------------

// RoutingTelemetry is the recorded telemetry for one routing decision.
// It is emitted as a pipeline event and consumed by the dashboard.
type RoutingTelemetry struct {
	RequestID               string            `json:"request_id"`
	StepCount               int               `json:"step_count"`
	Request                 TelemetryRequest  `json:"request"`
	Profile                 TelemetryProfile  `json:"profile"`
	Decision                TelemetryDecision `json:"decision"`
	ClassificationLatencyMs int64             `json:"classification_latency_ms"`
	FallbackUsed            bool              `json:"fallback_used"`
}

// TelemetryRequest describes the incoming request signals.
type TelemetryRequest struct {
	TextLength   int  `json:"text_length"`
	FileCount    int  `json:"file_count"`
	HasTraceback bool `json:"has_traceback"`
	HasTools     bool `json:"has_tools"`
}

// TelemetryProfile describes the classification result.
type TelemetryProfile struct {
	Difficulty int    `json:"difficulty"`
	TaskType   string `json:"task_type"`
	Step       int    `json:"step"`
	Retries    int    `json:"retries"`
}

// TelemetryDecision describes the routing outcome.
type TelemetryDecision struct {
	MatchedRule  string                  `json:"matched_rule"`
	Provider     string                  `json:"provider"`
	Model        string                  `json:"model"`
	Strategy     PreferenceStrategy      `json:"strategy"`
	Alternatives []AlternativeConsidered `json:"alternatives,omitempty"`
}

// NewRoutingTelemetry builds a telemetry record from a routing result and context.
func NewRoutingTelemetry(
	requestID string,
	stepCount int,
	codingProfile *CodingTaskProfile,
	result *RouteResult,
	hasTools bool,
) *RoutingTelemetry {
	t := &RoutingTelemetry{
		RequestID: requestID,
		StepCount: stepCount,
		Request: TelemetryRequest{
			TextLength:   0, // populated by caller if needed
			FileCount:    codingProfile.FileCount,
			HasTraceback: codingProfile.HasTraceback,
			HasTools:     hasTools,
		},
		Profile: TelemetryProfile{
			Difficulty: codingProfile.Difficulty,
			TaskType:   string(codingProfile.TaskType),
			Step:       codingProfile.Step,
			Retries:    codingProfile.Retries,
		},
		ClassificationLatencyMs: codingProfile.LatencyMs,
		FallbackUsed:            result != nil && result.FallbackUsed,
	}

	if result != nil {
		t.Decision = TelemetryDecision{
			MatchedRule:  result.MatchedRule,
			Provider:     result.Provider,
			Model:        result.Model,
			Strategy:     result.Strategy,
			Alternatives: result.Alternatives,
		}
	}

	return t
}

// ToMetadata converts the telemetry to a flat map[string]string suitable for
// pipeline event metadata.
func (t *RoutingTelemetry) ToMetadata() map[string]string {
	m := map[string]string{
		"difficulty":                strconv.Itoa(t.Profile.Difficulty),
		"task_type":                 t.Profile.TaskType,
		"step":                      strconv.Itoa(t.Profile.Step),
		"retries":                   strconv.Itoa(t.Profile.Retries),
		"file_count":                strconv.Itoa(t.Request.FileCount),
		"has_traceback":             fmt.Sprintf("%t", t.Request.HasTraceback),
		"classification_latency_ms": fmt.Sprintf("%d", t.ClassificationLatencyMs),
		"fallback_used":             fmt.Sprintf("%t", t.FallbackUsed),
	}
	if t.Decision.Provider != "" {
		m["selected_provider"] = t.Decision.Provider
	}
	if t.Decision.Model != "" {
		m["selected_model"] = t.Decision.Model
	}
	if t.Decision.MatchedRule != "" {
		m["matched_rule"] = t.Decision.MatchedRule
	}
	if t.Decision.Strategy != "" {
		m["strategy"] = string(t.Decision.Strategy)
	}
	return m
}
