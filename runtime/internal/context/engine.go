// Package contextengine prepares a compact, provider-ready message context.
package contextengine

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/profiles"
)

const (
	defaultModelContextLimit = 32000
	defaultReservedOutput    = 2048
	defaultReservedSystem    = 1200
	defaultReservedMemory    = 1200
	defaultRecentMessages    = 12
)

// Strategy controls how the Context Engine prepares messages.
type Strategy string

const (
	StrategyNone   Strategy = "none"
	StrategyTrim   Strategy = "trim"
	StrategyHybrid Strategy = "hybrid"
)

// Input is the Context Engine request.
type Input struct {
	RequestID              string
	RuntimeMode            string
	Messages               []api.Message
	Strategy               string
	MaxInputTokens         int
	PreserveRecentMessages int
	ModelProfile           *profiles.Profile
}

// Output is the Context Engine result.
type Output struct {
	NormalizedMessages []api.Message
	FinalMessages      []api.Message
	Package            Package
	Report             Report
	Warnings           []string
}

// Package is the inspectable context package produced for the pipeline.
type Package struct {
	ActiveRequest         string   `json:"active_request,omitempty"`
	SystemContext         []string `json:"system_context,omitempty"`
	RecentMessages        []string `json:"recent_messages,omitempty"`
	PreservedFacts        []string `json:"preserved_facts,omitempty"`
	OmittedContentSummary string   `json:"omitted_content_summary,omitempty"`
	TokenBudget           Budget   `json:"token_budget_report"`
	Warnings              []string `json:"warnings,omitempty"`
}

// Budget is the simple V1 token budget report.
type Budget struct {
	ModelContextLimit    int `json:"model_context_limit"`
	ReservedOutputTokens int `json:"reserved_output_tokens"`
	ReservedSystemTokens int `json:"reserved_system_tokens"`
	ReservedMemoryTokens int `json:"reserved_memory_tokens"`
	AvailableInputTokens int `json:"available_input_tokens"`
	EstimatedBefore      int `json:"estimated_before"`
	EstimatedAfter       int `json:"estimated_after"`
	OverflowTokens       int `json:"overflow_tokens"`
}

// Report describes what the Context Engine changed.
type Report struct {
	StrategyUsed          string   `json:"strategy_used"`
	EstimatedTokensBefore int      `json:"estimated_tokens_before"`
	EstimatedTokensAfter  int      `json:"estimated_tokens_after"`
	CompressionRatio      float64  `json:"compression_ratio"`
	ItemsRemoved          int      `json:"items_removed"`
	DuplicatesRemoved     int      `json:"duplicates_removed"`
	ToolResultsSummarized int      `json:"tool_results_summarized"`
	FactsPreserved        int      `json:"facts_preserved"`
	Warnings              []string `json:"warnings,omitempty"`
	FallbackUsed          bool     `json:"fallback_used"`
}

// Engine prepares message context for stabilized and structured modes.
type Engine struct{}

// New creates a Context Engine.
func New() *Engine {
	return &Engine{}
}

// Prepare normalizes, deduplicates, and trims messages to fit a token budget.
func (e *Engine) Prepare(in Input) Output {
	strategy := resolveStrategy(in.Strategy, in.RuntimeMode)
	maxInput := in.MaxInputTokens
	preserveRecent := in.PreserveRecentMessages
	if in.ModelProfile != nil {
		if in.ModelProfile.Context.Strategy != "" && in.Strategy == "" {
			strategy = resolveStrategy(in.ModelProfile.Context.Strategy, in.RuntimeMode)
		}
		if maxInput <= 0 && in.ModelProfile.Context.MaxInputTokens > 0 {
			maxInput = in.ModelProfile.Context.MaxInputTokens
		}
		if in.ModelProfile.ContextLimit > 0 && maxInput <= 0 {
			maxInput = in.ModelProfile.ContextLimit - defaultReservedOutput - defaultReservedSystem - defaultReservedMemory
		}
		if preserveRecent <= 0 && in.ModelProfile.Context.PreserveRecentMessages > 0 {
			preserveRecent = in.ModelProfile.Context.PreserveRecentMessages
		}
	}

	normalized, emptyRemoved := normalizeMessages(in.Messages)
	normalized, toolSummarized := summarizeOldToolResults(normalized, preserveRecent)
	before := EstimateMessages(normalized)

	limit := maxInput
	if limit <= 0 {
		limit = defaultModelContextLimit - defaultReservedOutput - defaultReservedSystem - defaultReservedMemory
	}

	final := append([]api.Message(nil), normalized...)
	duplicatesRemoved := 0
	itemsRemoved := emptyRemoved
	fallbackUsed := false

	if strategy != StrategyNone {
		final, duplicatesRemoved = removeExactDuplicates(final)
		itemsRemoved += duplicatesRemoved

		if strategy == StrategyHybrid || strategy == StrategyTrim {
			var trimmed int
			final, trimmed = trimToBudget(final, limit, preserveRecent)
			itemsRemoved += trimmed
			fallbackUsed = trimmed > 0
		}
	}

	after := EstimateMessages(final)
	warnings := []string{}
	overflow := after - limit
	if overflow > 0 {
		warnings = append(warnings, fmt.Sprintf("context still exceeds token budget by approximately %d tokens", overflow))
	} else {
		overflow = 0
	}

	active := latestUserText(final)
	systemContext := roleTexts(final, "system")
	recent := summarizeRecentMessages(final, 6)
	facts := extractFacts(final)
	omitted := ""
	if itemsRemoved > 0 {
		omitted = fmt.Sprintf("%d low-priority or duplicate message(s) were omitted.", itemsRemoved)
	}

	ratio := 0.0
	if before > 0 {
		ratio = float64(before-after) / float64(before)
		if ratio < 0 {
			ratio = 0
		}
	}

	budget := Budget{
		ModelContextLimit:    defaultModelContextLimit,
		ReservedOutputTokens: defaultReservedOutput,
		ReservedSystemTokens: defaultReservedSystem,
		ReservedMemoryTokens: defaultReservedMemory,
		AvailableInputTokens: limit,
		EstimatedBefore:      before,
		EstimatedAfter:       after,
		OverflowTokens:       overflow,
	}

	report := Report{
		StrategyUsed:          string(strategy),
		EstimatedTokensBefore: before,
		EstimatedTokensAfter:  after,
		CompressionRatio:      ratio,
		ItemsRemoved:          itemsRemoved,
		DuplicatesRemoved:     duplicatesRemoved,
		ToolResultsSummarized: toolSummarized,
		FactsPreserved:        len(facts),
		Warnings:              warnings,
		FallbackUsed:          fallbackUsed,
	}

	return Output{
		NormalizedMessages: normalized,
		FinalMessages:      final,
		Package: Package{
			ActiveRequest:         active,
			SystemContext:         systemContext,
			RecentMessages:        recent,
			PreservedFacts:        facts,
			OmittedContentSummary: omitted,
			TokenBudget:           budget,
			Warnings:              warnings,
		},
		Report:   report,
		Warnings: warnings,
	}
}

// EstimateMessages estimates total tokens for messages.
func EstimateMessages(messages []api.Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateText(msg.Role) + EstimateText(messageText(msg)) + 4
	}
	return total
}

// EstimateText approximates token count without model-specific tokenizers.
func EstimateText(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := utf8.RuneCountInString(text)
	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	if tokens < 1 {
		return 1
	}
	return tokens
}

// summarizeOldToolResults replaces the content of tool messages that fall
// outside the recent-message preservation window with a compact summary. This
// keeps token budgets under control during long agent loops without discarding
// the fact that a tool was called.
func summarizeOldToolResults(messages []api.Message, preserveRecent int) ([]api.Message, int) {
	if preserveRecent <= 0 {
		preserveRecent = defaultRecentMessages
	}
	preserveFrom := len(messages) - preserveRecent
	if preserveFrom < 0 {
		preserveFrom = 0
	}
	result := make([]api.Message, 0, len(messages))
	summarized := 0
	for i, msg := range messages {
		if msg.Role == "tool" && i < preserveFrom {
			summary := summarizeToolResult(msg)
			if summary != "" {
				msg.Content = summary
				summarized++
			}
		}
		result = append(result, msg)
	}
	return result, summarized
}

func summarizeToolResult(msg api.Message) string {
	if msg.ToolCallID == "" {
		return ""
	}
	text := messageText(msg)
	length := EstimateText(text)
	return fmt.Sprintf("[Tool result %s: %d tokens omitted]", msg.ToolCallID, length)
}

func resolveStrategy(raw string, mode string) Strategy {
	switch Strategy(strings.ToLower(strings.TrimSpace(raw))) {
	case StrategyNone:
		return StrategyNone
	case StrategyTrim:
		return StrategyTrim
	case StrategyHybrid:
		return StrategyHybrid
	}
	if mode == "direct" {
		return StrategyNone
	}
	return StrategyHybrid
}

func normalizeMessages(messages []api.Message) ([]api.Message, int) {
	result := make([]api.Message, 0, len(messages))
	removed := 0
	for _, msg := range messages {
		msg.Role = strings.ToLower(strings.TrimSpace(msg.Role))
		if msg.Role == "" {
			removed++
			continue
		}
		if s, ok := msg.Content.(string); ok {
			msg.Content = strings.TrimSpace(s)
			if msg.Content == "" && len(msg.ToolCalls) == 0 && msg.ToolCallID == "" {
				removed++
				continue
			}
		}
		result = append(result, msg)
	}
	return result, removed
}

func removeExactDuplicates(messages []api.Message) ([]api.Message, int) {
	seen := map[string]bool{}
	result := make([]api.Message, 0, len(messages))
	removed := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		key := msg.Role + "\x00" + strings.TrimSpace(messageText(msg))
		if seen[key] && !isCritical(msg, i, len(messages)) {
			removed++
			continue
		}
		seen[key] = true
		result = append(result, msg)
	}
	reverseMessages(result)
	return result, removed
}

func trimToBudget(messages []api.Message, limit int, preserveRecent int) ([]api.Message, int) {
	if limit <= 0 || EstimateMessages(messages) <= limit {
		return messages, 0
	}
	if preserveRecent <= 0 {
		preserveRecent = defaultRecentMessages
	}

	result := append([]api.Message(nil), messages...)
	removed := 0
	for EstimateMessages(result) > limit {
		idx := removableIndex(result, preserveRecent)
		if idx < 0 {
			break
		}
		result = append(result[:idx], result[idx+1:]...)
		removed++
	}
	return result, removed
}

func removableIndex(messages []api.Message, preserveRecent int) int {
	preserveFrom := len(messages) - preserveRecent
	if preserveFrom < 0 {
		preserveFrom = 0
	}
	for i, msg := range messages {
		if i >= preserveFrom {
			continue
		}
		if msg.Role == "assistant" && !isCritical(msg, i, len(messages)) {
			return i
		}
	}
	for i, msg := range messages {
		if i >= preserveFrom {
			continue
		}
		if !isCritical(msg, i, len(messages)) {
			return i
		}
	}
	return -1
}

func isCritical(msg api.Message, index int, total int) bool {
	if msg.Role == "system" || msg.Role == "developer" {
		return true
	}
	return index == total-1 && msg.Role == "user"
}

func messageText(msg api.Message) string {
	if s, ok := msg.Content.(string); ok {
		return s
	}
	if msg.Content == nil {
		return ""
	}
	data, err := json.Marshal(msg.Content)
	if err != nil {
		return fmt.Sprint(msg.Content)
	}
	return string(data)
}

func latestUserText(messages []api.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messageText(messages[i])
		}
	}
	return ""
}

func roleTexts(messages []api.Message, role string) []string {
	var result []string
	for _, msg := range messages {
		if msg.Role == role {
			result = append(result, messageText(msg))
		}
	}
	return result
}

func summarizeRecentMessages(messages []api.Message, max int) []string {
	if max <= 0 || len(messages) == 0 {
		return nil
	}
	start := len(messages) - max
	if start < 0 {
		start = 0
	}
	result := make([]string, 0, len(messages)-start)
	for _, msg := range messages[start:] {
		text := messageText(msg)
		if len(text) > 160 {
			text = text[:160]
		}
		result = append(result, msg.Role+": "+text)
	}
	return result
}

func extractFacts(messages []api.Message) []string {
	keywords := []string{"must ", "must not", "use ", "default ", "project ", "v1 ", "local-first", "openai-compatible"}
	seen := map[string]bool{}
	var facts []string
	for _, msg := range messages {
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "developer" {
			continue
		}
		for _, line := range strings.Split(messageText(msg), "\n") {
			line = strings.TrimSpace(strings.TrimLeft(line, "-*0123456789. "))
			lower := strings.ToLower(line)
			if line == "" || len(line) > 180 {
				continue
			}
			for _, kw := range keywords {
				if strings.Contains(lower, kw) && !seen[line] {
					seen[line] = true
					facts = append(facts, line)
				}
			}
		}
	}
	if len(facts) > 8 {
		return facts[:8]
	}
	return facts
}

func reverseMessages(messages []api.Message) {
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
}
