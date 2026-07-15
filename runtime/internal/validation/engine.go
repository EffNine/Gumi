// Package validation checks provider output before it reaches the client.
package validation

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/EffNine/gumi/runtime/internal/api"
)

// IssueCode identifies a validation issue.
type IssueCode string

const (
	IssueEmptyResponse      IssueCode = "EMPTY_RESPONSE"
	IssueIncompleteResponse IssueCode = "INCOMPLETE_RESPONSE"
	IssueRepetition         IssueCode = "REPETITION"
	IssueInvalidJSON        IssueCode = "INVALID_JSON"
)

// Strategy names the suggested repair strategy.
type Strategy string

const (
	StrategyNone             Strategy = "none"
	StrategyRetryGeneration  Strategy = "retry_generation"
	StrategyLocalParseRepair Strategy = "local_parse_repair"
	StrategyRegexCleanup     Strategy = "regex_cleanup"
)

// Issue describes one validation problem.
type Issue struct {
	Code     IssueCode `json:"code"`
	Message  string    `json:"message"`
	Location string    `json:"location,omitempty"`
}

// Report describes validation outcome.
type Report struct {
	Passed                  bool              `json:"passed"`
	Severity                string            `json:"severity"`
	Issues                  []Issue           `json:"issues,omitempty"`
	Repairable              bool              `json:"repairable"`
	SuggestedRepairStrategy Strategy          `json:"suggested_repair_strategy"`
	Confidence              float64           `json:"confidence"`
	Metadata                map[string]string `json:"metadata,omitempty"`
}

// Input is the Validation Engine request.
type Input struct {
	Response       *api.ChatCompletionResponse
	ResponseFormat *api.ResponseFormat
	RuntimeMode    string
}

// Engine validates normalized provider responses.
type Engine struct{}

// New creates a Validation Engine.
func New() *Engine {
	return &Engine{}
}

// Validate checks empty, incomplete, repetition, and structured JSON output.
func (e *Engine) Validate(in Input) Report {
	content, finish := AssistantContent(in.Response)
	hasToolCalls := hasToolCalls(in.Response)
	report := Report{
		Passed:     true,
		Severity:   "info",
		Confidence: 1,
		Metadata: map[string]string{
			"response_length": strconv.Itoa(len(content)),
		},
	}

	if strings.TrimSpace(content) == "" && !hasToolCalls {
		report.add(IssueEmptyResponse, "assistant response is empty", "choices[0].message.content", "error", true, StrategyRetryGeneration)
		return report
	}

	if !hasToolCalls && (finish == "length" || hasUnclosedCodeFence(content)) {
		report.add(IssueIncompleteResponse, "assistant response appears incomplete", "choices[0]", "warning", true, StrategyRetryGeneration)
	}

	if !hasToolCalls && hasRepetition(content, in.ResponseFormat, in.RuntimeMode) {
		report.add(IssueRepetition, "assistant response contains repeated lines or sentences", "choices[0].message.content", "error", true, StrategyRegexCleanup)
	}

	if !hasToolCalls && requiresJSON(in.ResponseFormat, in.RuntimeMode, content) {
		trimmed := strings.TrimSpace(content)
		candidate := ExtractJSONCandidate(content)
		if candidate == "" || !json.Valid([]byte(candidate)) {
			report.add(IssueInvalidJSON, "assistant response is not valid JSON", "choices[0].message.content", "error", true, StrategyLocalParseRepair)
		} else if strings.TrimSpace(candidate) != trimmed {
			report.add(IssueInvalidJSON, "assistant response contains JSON with markdown fences or surrounding prose", "choices[0].message.content", "error", true, StrategyLocalParseRepair)
		} else if in.ResponseFormat != nil && in.ResponseFormat.Type == "json_object" {
			var decoded interface{}
			_ = json.Unmarshal([]byte(candidate), &decoded)
			if _, ok := decoded.(map[string]interface{}); !ok {
				report.add(IssueInvalidJSON, "assistant response JSON root is not an object", "choices[0].message.content", "error", true, StrategyLocalParseRepair)
			}
		}

		if in.ResponseFormat != nil && in.ResponseFormat.Type == "json_schema" && in.ResponseFormat.JSONSchema != nil {
			var decoded map[string]interface{}
			_ = json.Unmarshal([]byte(candidate), &decoded)
			if missing := missingRequiredKeys(in.ResponseFormat.JSONSchema.Schema, decoded); len(missing) > 0 {
				report.add(IssueInvalidJSON, "assistant response JSON is missing required keys: "+strings.Join(missing, ", "), "choices[0].message.content", "error", true, StrategyLocalParseRepair)
			}
		}
	}

	return report
}

// missingRequiredKeys returns the required top-level schema keys that are absent
// from the decoded response object.
func missingRequiredKeys(schema map[string]interface{}, decoded map[string]interface{}) []string {
	if len(schema) == 0 {
		return nil
	}
	req, ok := schema["required"].([]interface{})
	if !ok {
		return nil
	}
	var missing []string
	for _, r := range req {
		key, ok := r.(string)
		if !ok {
			continue
		}
		if _, present := decoded[key]; !present {
			missing = append(missing, key)
		}
	}
	return missing
}

func (r *Report) add(code IssueCode, message string, location string, severity string, repairable bool, strategy Strategy) {
	r.Passed = false
	if severity == "error" || r.Severity == "info" {
		r.Severity = severity
	}
	r.Issues = append(r.Issues, Issue{Code: code, Message: message, Location: location})
	r.Repairable = r.Repairable || repairable
	if r.SuggestedRepairStrategy == "" || r.SuggestedRepairStrategy == StrategyNone || severity == "error" {
		r.SuggestedRepairStrategy = strategy
	}
	if r.Confidence == 0 {
		r.Confidence = 0.9
	}
}

func hasToolCalls(resp *api.ChatCompletionResponse) bool {
	if resp == nil || len(resp.Choices) == 0 {
		return false
	}
	return len(resp.Choices[0].Message.ToolCalls) > 0
}

// AssistantContent returns the first assistant content and finish reason.
func AssistantContent(resp *api.ChatCompletionResponse) (string, string) {
	if resp == nil || len(resp.Choices) == 0 {
		return "", ""
	}
	choice := resp.Choices[0]
	content, _ := choice.Message.Content.(string)
	return content, choice.FinishReason
}

// SetAssistantContent updates the first assistant content.
func SetAssistantContent(resp *api.ChatCompletionResponse, content string) {
	if resp == nil || len(resp.Choices) == 0 {
		return
	}
	resp.Choices[0].Message.Content = content
}

func requiresJSON(format *api.ResponseFormat, mode string, content string) bool {
	if format != nil && (format.Type == "json_object" || format.Type == "json_schema") {
		return true
	}
	if mode == "structured" {
		return true
	}
	trimmed := strings.TrimSpace(content)
	// Detect bare JSON or any language-tagged fence (```json, ```python, etc.).
	// Models like RNJ-1 wrap JSON in ```python blocks, so we need to catch
	// those and let ExtractJSONCandidate + repair extract the inner JSON.
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "```") {
		return true
	}
	return false
}

// ExtractJSONCandidate extracts a JSON object from markdown fences or prose.
// Handles any language-tagged fence (```json, ```python, ```javascript, etc.)
// since some models (e.g. Essential AI RNJ-1) wrap JSON inside ```python blocks.
func ExtractJSONCandidate(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "```") {
		// Strip the opening fence along with any language tag (json, python, etc.).
		// The fence is ``` optionally followed by a language identifier and a newline.
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			fenceLine := trimmed[:idx]
			rest := strings.TrimSpace(trimmed[idx+1:])
			// Only strip if the fence line is ``` or ```<lang> (no spaces in the tag).
			if fenceLine == "```" || (strings.HasPrefix(fenceLine, "```") && !strings.ContainsAny(fenceLine[3:], " \t")) {
				trimmed = rest
			}
		}
		// Strip the closing fence if present.
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}
	return trimmed
}

func hasUnclosedCodeFence(content string) bool {
	return strings.Count(content, "```")%2 != 0
}

func hasRepetition(content string, format *api.ResponseFormat, mode string) bool {
	// Skip repetition detection for JSON output. JSON structural elements
	// (braces, brackets, repeated keys across objects in an array) are
	// legitimate and should not be flagged as repetition. JSON validity is
	// checked separately by the JSON validation block.
	if format != nil && (format.Type == "json_object" || format.Type == "json_schema") {
		return false
	}
	if mode == "structured" {
		return false
	}
	// Also skip if the content looks like JSON even without an explicit format.
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		if json.Valid([]byte(trimmed)) {
			return false
		}
	}

	lines := strings.Split(content, "\n")
	counts := map[string]int{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		counts[line]++
		if counts[line] > 2 {
			return true
		}
	}
	sentences := strings.FieldsFunc(content, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	counts = map[string]int{}
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) < 12 {
			continue
		}
		counts[sentence]++
		if counts[sentence] > 2 {
			return true
		}
	}
	return false
}
