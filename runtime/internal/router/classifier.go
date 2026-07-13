// Package router implements the Agentic Coding Router — a per-step,
// difficulty-based model router for coding agents.
package router

import (
	"regexp"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/api"
)

// ---------------------------------------------------------------------------
// CodingTaskProfile
// ---------------------------------------------------------------------------

// Difficulty levels for coding tasks.
const (
	DifficultyTrivial  = 1
	DifficultySimple   = 2
	DifficultyModerate = 3
	DifficultyComplex  = 4
	DifficultyNovel    = 5
)

// difficultyLabel returns a human-readable label for a difficulty level.
func difficultyLabel(d int) string {
	switch d {
	case DifficultyTrivial:
		return "trivial"
	case DifficultySimple:
		return "simple"
	case DifficultyModerate:
		return "moderate"
	case DifficultyComplex:
		return "complex"
	case DifficultyNovel:
		return "novel"
	}
	return "unknown"
}

// TaskType enumerates the coding task types the classifier can detect.
type TaskType string

const (
	TaskFix     TaskType = "fix"
	TaskRefactor TaskType = "refactor"
	TaskFeature TaskType = "feature"
	TaskTest    TaskType = "test"
	TaskReview  TaskType = "review"
	TaskDocs    TaskType = "docs"
	TaskSearch  TaskType = "search"
	TaskPlan    TaskType = "plan"
)

// CodingTaskProfile is the output of structural classification.
type CodingTaskProfile struct {
	Difficulty           int      `json:"difficulty"`
	DifficultyLabel      string   `json:"difficulty_label"`
	TaskType             TaskType `json:"task_type"`
	FileCount            int      `json:"file_count"`
	HasTraceback         bool     `json:"has_traceback"`
	Step                 int      `json:"step"`
	Retries              int      `json:"retries"`
	ClassificationSource string   `json:"classification_source"` // diagnostic
	LatencyMs            int64    `json:"classification_latency_ms"`
}

// ---------------------------------------------------------------------------
// Classifier
// ---------------------------------------------------------------------------

// CodingTaskClassifier analyses agent-step messages using only structural
// heuristics and returns a difficulty profile in < 20 ms.
type CodingTaskClassifier struct {
	// Escalation thresholds for agent-state-aware re-routing.
	EscalationRetries    int
	EscalationSteps      int
	EscalationRepetitions int
}

// NewCodingTaskClassifier returns a classifier with sensible defaults.
func NewCodingTaskClassifier() *CodingTaskClassifier {
	return &CodingTaskClassifier{
		EscalationRetries:    3,
		EscalationSteps:      6,
		EscalationRepetitions: 3,
	}
}

// Classify analyses the current agent step and returns a coding profile.
// Both `messages` and `agentState` are optional — the classifier works with
// whatever is available.
func (c *CodingTaskClassifier) Classify(
	messages []api.Message,
	stepCount int,
	retryAttempt int,
) *CodingTaskProfile {
	start := time.Now()

	// Extract the latest user message for analysis.
	text := latestUserText(messages)
	if text == "" {
		// Try the latest assistant message if no user message found.
		text = latestAssistantText(messages)
	}

	// Structural signals.
	hasTraceback := containsStackTrace(text)
	fileCount := countFilePaths(text)
	codeBlockSize := maxCodeBlockSize(text)
	keywords := extractCodingKeywords(text)

	// ---- Determine task type ----
	taskType := classifyTaskType(text, keywords, hasTraceback)

	// ---- Determine difficulty ----
	difficulty := classifyDifficulty(
		text, fileCount, hasTraceback, codeBlockSize,
		taskType, stepCount, retryAttempt, keywords,
	)

	// ---- Apply agent-state escalation ----
	difficulty = c.applyEscalation(difficulty, stepCount, retryAttempt)

	source := "structural"
	latency := time.Since(start).Milliseconds()

	return &CodingTaskProfile{
		Difficulty:           difficulty,
		DifficultyLabel:      difficultyLabel(difficulty),
		TaskType:             taskType,
		FileCount:            fileCount,
		HasTraceback:         hasTraceback,
		Step:                 stepCount,
		Retries:              retryAttempt,
		ClassificationSource: source,
		LatencyMs:            latency,
	}
}

// ---------------------------------------------------------------------------
// Signal extraction helpers
// ---------------------------------------------------------------------------

var (
	filePathRe   = regexp.MustCompile(`[a-zA-Z0-9_\-./]+\.(go|py|js|ts|rs|rb|java|kt|swift|c|cpp|h|hpp|cs|php|ex|exs|scala|clj|cljs|coffee|sh|bash|zsh|fish|yaml|yml|json|xml|toml|md|txt|css|scss|less|html|tsx|jsx|vue|svelte|sql|graphql|proto|lock|mod|sum)`)

	// Common file path patterns used in agent prompts.
	pathPatternRe = regexp.MustCompile(`\b(src|lib|app|cmd|pkg|internal|test|spec|fixtures?|config|scripts?|docs?)\/`)

	// Stack-trace-like patterns.
	tracebackRe = regexp.MustCompile(`(?m)^\s*(at |in |File "|Error:|Traceback|panic:|fatal error|exit status|\.go:\d+)`)

	// Code block markers.
	codeFenceRe = regexp.MustCompile("```[a-zA-Z]*\n")
)

func latestUserText(messages []api.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.ToLower(messages[i].Role) == "user" {
			if s, ok := messages[i].Content.(string); ok {
				return s
			}
		}
	}
	return ""
}

func latestAssistantText(messages []api.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.ToLower(messages[i].Role) == "assistant" {
			if s, ok := messages[i].Content.(string); ok {
				return s
			}
		}
	}
	return ""
}

func containsStackTrace(text string) bool {
	return tracebackRe.MatchString(text)
}

func countFilePaths(text string) int {
	seen := make(map[string]bool)
	for _, m := range filePathRe.FindAllString(text, -1) {
		seen[strings.ToLower(m)] = true
	}
	for _, m := range pathPatternRe.FindAllString(text, -1) {
		seen[strings.ToLower(m)] = true
	}
	return len(seen)
}

func maxCodeBlockSize(text string) int {
	max := 0
	matches := codeFenceRe.FindAllStringIndex(text, -1)
	for _, m := range matches {
		start := m[1] // after ```
		end := strings.Index(text[start:], "```")
		if end < 0 {
			end = len(text) - start
		}
		if end > max {
			max = end
		}
	}
	return max
}

// extractCodingKeywords returns a set of lowercase keywords found in text.
func extractCodingKeywords(text string) map[string]bool {
	words := strings.Fields(strings.ToLower(text))
	keywords := map[string]bool{}
	for _, w := range words {
		w = strings.Trim(w, ",.;:!?\"'()`[]{}")
		if len(w) > 0 {
			keywords[w] = true
		}
	}
	return keywords
}

// ---------------------------------------------------------------------------
// Classification logic
// ---------------------------------------------------------------------------

func classifyTaskType(text string, keywords map[string]bool, hasTraceback bool) TaskType {
	lower := strings.ToLower(text)

	if hasTraceback || keywords["error"] || keywords["bug"] || keywords["fix"] ||
		keywords["bugfix"] || keywords["hotfix"] || keywords["patch"] {
		return TaskFix
	}
	if keywords["refactor"] || keywords["restructure"] || keywords["rename"] ||
		keywords["extract"] || keywords["inline"] || keywords["move"] {
		return TaskRefactor
	}
	if keywords["implement"] || keywords["create"] || keywords["add"] ||
		keywords["write"] || keywords["build"] || keywords["new"] ||
		keywords["feature"] {
		return TaskFeature
	}
	if keywords["test"] || keywords["assert"] || keywords["mock"] ||
		keywords["spec"] || strings.Contains(lower, "unit test") ||
		strings.Contains(lower, "integration test") {
		return TaskTest
	}
	if keywords["review"] || keywords["explain"] || strings.Contains(lower, "what does") ||
		keywords["understand"] || keywords["summarize"] {
		return TaskReview
	}
	if keywords["search"] || keywords["find"] || strings.Contains(lower, "where is") ||
		keywords["grep"] || keywords["locate"] {
		return TaskSearch
	}
	if keywords["design"] || keywords["architecture"] || keywords["plan"] ||
		keywords["approach"] || keywords["strategy"] || keywords["proposal"] {
		return TaskPlan
	}
	if keywords["docs"] || keywords["documentation"] || keywords["comment"] ||
		keywords["readme"] {
		return TaskDocs
	}

	return TaskFeature // default for coding tasks
}

func classifyDifficulty(
	text string,
	fileCount int,
	hasTraceback bool,
	codeBlockSize int,
	taskType TaskType,
	stepCount int,
	retryAttempt int,
	keywords map[string]bool,
) int {
	textLen := len(text)

	// ---- Heuristic rules in priority order ----

	// Trivial: very short and no traceback
	if textLen < 50 && fileCount <= 1 && !hasTraceback {
		return DifficultyTrivial
	}

	// Search tasks are always trivial/simple
	if taskType == TaskSearch {
		return DifficultyTrivial
	}

	// Simple fix: short with traceback, one file
	if taskType == TaskFix && fileCount == 1 && textLen < 200 {
		return DifficultySimple
	}

	// Simple: short text, few files, early in session
	if textLen < 200 && fileCount <= 1 && stepCount <= 1 && retryAttempt <= 1 {
		return DifficultySimple
	}

	// Novel: planning, architecture, design with multiple files
	if taskType == TaskPlan && fileCount >= 2 {
		return DifficultyNovel
	}

	// Complex: many files or late in session or high retries
	if fileCount >= 3 || stepCount > 5 || retryAttempt > 2 {
		return DifficultyComplex
	}

	// Large code blocks → complex
	if codeBlockSize > 100 {
		return DifficultyComplex
	}

	// Complex fix with traceback and multiple files
	if hasTraceback && fileCount >= 2 {
		return DifficultyComplex
	}

	// Review with multiple files → complex
	if taskType == TaskReview && fileCount >= 2 {
		return DifficultyComplex
	}

	// Moderate: up to 2 files, reasonable step count
	if fileCount <= 2 && stepCount <= 3 && retryAttempt <= 1 {
		return DifficultyModerate
	}

	// Default: moderate
	return DifficultyModerate
}

// applyEscalation bumps difficulty when the agent is stuck.
func (c *CodingTaskClassifier) applyEscalation(difficulty int, stepCount int, retryAttempt int) int {
	escalated := difficulty

	// Many retries → stuck, escalate
	if retryAttempt >= c.EscalationRetries {
		escalated = max(escalated, DifficultyComplex)
	}

	// Many steps without progress → escalate
	if stepCount >= c.EscalationSteps {
		escalated = max(escalated, DifficultyComplex)
	}

	return max(difficulty, escalated)
}
