// Package instruction implements the Instruction-Following Assist Engine.
//
// It helps small local models follow complex formatting and content constraints
// by extracting constraints from user prompts, injecting adaptive reminders into
// the system prompt, and validating responses post-generation with automatic
// retry on failure.
//
// Sprint 17R2 redesign: minimal intervention strategy.
// Sprint 17R3 adaptive strategy: deterministic hint profiles based on constraint
// complexity, not model identity. Profiles: NONE, MINIMAL, STANDARD, EXPLICIT.
// - No hard constraint → inject nothing.
// - Simple hard constraint → inject only the minimum necessary reminder.
// - Multiple hard constraints → adaptive hint block based on complexity.
// - Soft hints are NOT injected by default.
// - Conflict metadata is diagnostic-only, never model-facing.
// - Preserve exact user intent; never introduce model-specific wording.
package instruction

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Constraint is a single rule extracted from the user's prompt.
type Constraint struct {
	Type  string `json:"type"`  // sentences, words, lines, bullets, json, no_word, end_with, start_with, min_chars, digit_answer, one_word
	Label string `json:"label"` // human-readable description
	Value string `json:"value"` // param (e.g. "3", "health", "learning")
	Hint  string `json:"hint"`  // reminder text for the model
	Check string `json:"check"` // validation check name
}

// ValidationResult is the outcome of checking a response against constraints.
type ValidationResult struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations"`
	Satisfied  []string `json:"satisfied"`
}

// Telemetry records instrumentation for the instruction engine.
type Telemetry struct {
	OriginalPromptTokens   int    `json:"original_prompt_tokens"`
	NormalizedPromptTokens int    `json:"normalized_prompt_tokens"`
	InstructionTokens      int    `json:"instruction_tokens"`
	TotalProviderTokens    int    `json:"total_provider_tokens"`
	ConstraintCount        int    `json:"constraint_count"`
	HardConstraintCount    int    `json:"hard_constraint_count"`
	SoftHintCount          int    `json:"soft_hint_count"`
	ConflictCount          int    `json:"conflict_count"`
	RetryCount             int    `json:"retry_count"`
	RetryReason            string `json:"retry_reason,omitempty"`
	ProviderRequestCount   int    `json:"provider_request_count"`
	ProviderLatencyMs      int64  `json:"provider_latency_ms"`
	TotalLatencyMs         int64  `json:"total_latency_ms"`
}

// HintProfile represents the selected hint verbosity level.
type HintProfile string

const (
	// ProfileNone means no hint is injected. Used when there are no hard constraints.
	ProfileNone HintProfile = "none"
	// ProfileMinimal means one concise line per constraint. Used for single/simple constraints.
	ProfileMinimal HintProfile = "minimal"
	// ProfileStandard means compact grouped summary. Used for multiple non-conflicting constraints.
	ProfileStandard HintProfile = "standard"
	// ProfileExplicit means ordered requirements with verification reminder.
	// Used for complex or high-risk constraint combinations.
	ProfileExplicit HintProfile = "explicit"
)

// Result holds extracted constraints and hint text.
type Result struct {
	Constraints       []Constraint `json:"constraints"`
	HintBlock         string       `json:"hint_block"`
	HasConstraints    bool         `json:"has_constraints"`
	Conflicts         []string     `json:"conflicts,omitempty"`
	DeduplicatedCount int          `json:"deduplicated_count,omitempty"`
	// SelectedProfile is the hint profile chosen for this request.
	SelectedProfile HintProfile `json:"selected_profile,omitempty"`
	// ComplexityScore is the deterministic complexity score used to select the profile.
	ComplexityScore int `json:"complexity_score,omitempty"`
}

var (
	// ── Constraint detectors ──────────────────────────────────────────
	reSentences   = regexp.MustCompile(`(?i)(?:exactly|just|only)\s+(\d+)\s+sentences?`)
	reWords       = regexp.MustCompile(`(?i)(?:exactly|just|only)\s+(\d+)\s+words?`)
	reLines       = regexp.MustCompile(`(?i)(\d+)[- ]lines?\s+(?:poem|answer|response|output)`)
	reLinesSimple = regexp.MustCompile(`(?i)(?:write|create|give|return)\s+(?:a|an)\s+(\d+)[- ]lines?`)
	reBullets     = regexp.MustCompile(`(?i)(?:bullet\s*points?|use\s+bullet|dashes?|with\s+dashes?)`)
	reNoWord      = regexp.MustCompile(`(?i)(?:do\s+not\s+(?:use|say|include|write|mention)\s+(?:the\s+)?(?:word|term|phrase)?\s*['"]?(\w+)['"]?)|(?:avoid\s+(?:the\s+)?(?:word|term|phrase)?\s*['"]?(\w+)['"]?)`)
	reEndWith     = regexp.MustCompile(`(?i)(?:end|finish|conclude)\s+(?:with|in)\s+(?:the\s+)?(?:word|phrase)?\s*['"]?([a-zA-Z0-9.]+)['"]?`)
	reStartWith   = regexp.MustCompile(`(?i)(?:start|begin)\s+(?:each\s+line\s+)?(?:with|in)\s+(?:a\s+)?(capital\s*letter|uppercase)`)
	reJSON        = regexp.MustCompile(`(?i)(?:return|output|respond\s+with|format\s+your\s+response\s+as|respond\s+in)\s+(?:only\s+)?(?:valid\s+)?json`)
	reMinChars    = regexp.MustCompile(`(?i)(?:at\s+least|minimum\s+of|more\s+than|over)\s+(\d+)\s+(?:characters?|chars?|letters?)`)
	reMinWords    = regexp.MustCompile(`(?i)(?:at\s+least|minimum\s+of|more\s+than|over)\s+(\d+)\s+words?`)
	reNoCommas    = regexp.MustCompile(`(?i)(?:do\s+not\s+use\s+(?:any\s+)?commas?|no\s+commas?|without\s+commas?)`)
	reNoMarkdown  = regexp.MustCompile(`(?i)(?:no\s+markdown|without\s+markdown|do\s+not\s+use\s+markdown)`)
	reSections    = regexp.MustCompile(`(?i)(?:highlight\s+at\s+least\s+)?(\d+)\s+sections?`)
	reEachCap     = regexp.MustCompile(`(?i)(?:each\s+line\s+must\s+start\s+with\s+a\s+capital)`)
	reNoRhyme     = regexp.MustCompile(`(?i)(?:do\s+not\s+rhyme|no\s+rhym(?:e|ing))`)
	reOneWord     = regexp.MustCompile(`(?i)(?:answer|respond|reply|output|return)\s+(?:with\s+|in\s+)?one\s+word|(?:in|with)\s+one\s+word\.?`)
	reDigitAsk    = regexp.MustCompile(`(?i)(?:answer|respond|reply|output|return)\s+(?:with\s+)?(?:a\s+)?(?:numeric\s+)?(?:digit|number)|numeric\s+digit\s+only|(?:as|with)\s+(?:a\s+)?digit`)
	reMathAsk     = regexp.MustCompile(`(?i)(?:\d+\s*[\+\-\*\/×÷]\s*\d+|what\s+is\s+\d+|how\s+many|\d+\s*\+\s*\d+)`)

	// Soft hint detectors — these never inject into the model prompt by default.
	reComplexQ   = regexp.MustCompile(`(?i)(?:explain|analyze|compare|contrast|why|how\s+(?:does|do|can|would|should)|what\s+is\s+the\s+difference|solve|calculate|evaluate|interpret|summarize|break\s+down|walk\s+me\s+through|step\s+by\s+step)`)
	reFactualQ   = regexp.MustCompile(`(?i)(?:who\s+is|what\s+is|where\s+is|when\s+(?:was|did|will)|define|describe|list|what\s+are|tell\s+me\s+about)`)
	reMultiStepQ = regexp.MustCompile(`(?i)(?:first\s+.*then\s+.*finally|step\s+\d|multi[\s-]step|multiple\s+parts|several\s+steps)`)

	spelledNumbers = map[string]struct{}{
		"zero": {}, "one": {}, "two": {}, "three": {}, "four": {}, "five": {},
		"six": {}, "seven": {}, "eight": {}, "nine": {}, "ten": {},
		"eleven": {}, "twelve": {}, "thirteen": {}, "fourteen": {}, "fifteen": {},
		"sixteen": {}, "seventeen": {}, "eighteen": {}, "nineteen": {}, "twenty": {},
		"thirty": {}, "forty": {}, "fifty": {}, "sixty": {}, "seventy": {},
		"eighty": {}, "ninety": {}, "hundred": {}, "thousand": {},
	}
)

// Engine extracts and validates instruction constraints.
type Engine struct{}

// New creates an Instruction Engine.
func New() *Engine {
	return &Engine{}
}

// SelectProfile deterministically selects a hint profile based on constraint
// complexity. Adaptation is based on request characteristics, NOT model identity.
//
// Scoring rules:
//   - Base score = number of hard constraints.
//   - Each conflict adds +1 (indicates complexity requiring explicit guidance).
//   - JSON with other constraints adds +1 (format-heavy requests benefit from clarity).
//
// Profile thresholds:
//   - Score 1 → MINIMAL (single simple constraint)
//   - Score 2-3 → STANDARD (multiple non-conflicting constraints)
//   - Score 4+ → EXPLICIT (complex or high-risk combinations)
func (e *Engine) SelectProfile(constraints []Constraint, conflicts []string) (HintProfile, int) {
	if len(constraints) == 0 {
		return ProfileNone, 0
	}

	score := len(constraints)
	score += len(conflicts)

	hasJSON := false
	for _, c := range constraints {
		if c.Type == "json" && len(constraints) > 1 {
			hasJSON = true
			break
		}
	}
	if hasJSON {
		score++
	}

	var profile HintProfile
	switch {
	case score <= 1:
		profile = ProfileMinimal
	case score <= 3:
		profile = ProfileStandard
	default:
		profile = ProfileExplicit
	}

	return profile, score
}

// buildHintBlock constructs the hint block using the selected profile.
func (e *Engine) buildHintBlock(constraints []Constraint, profile HintProfile) string {
	switch profile {
	case ProfileNone:
		return ""
	case ProfileMinimal:
		return buildMinimalHintBlock(constraints)
	case ProfileStandard:
		return buildStandardHintBlock(constraints)
	case ProfileExplicit:
		return buildExplicitHintBlock(constraints)
	default:
		return buildMinimalHintBlock(constraints)
	}
}

// extractConstraints scans the user prompt for known constraint patterns.
// Returns hard constraints (with Check != "none") and soft hints separately.
func (e *Engine) extractConstraints(userMessage string) (hardConstraints []Constraint, softHints []Constraint) {
	if strings.TrimSpace(userMessage) == "" {
		return
	}

	// Sentence count
	if m := reSentences.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "sentences", Label: fmt.Sprintf("exactly %d sentences", n),
			Value: m[1], Hint: fmt.Sprintf("exactly %d sentences", n),
			Check: "sentence_count",
		})
	}

	// Word count
	if m := reWords.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "words", Label: fmt.Sprintf("exactly %d words", n),
			Value: m[1], Hint: fmt.Sprintf("exactly %d words", n),
			Check: "word_count",
		})
	}

	// Line count
	if m := reLines.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "lines", Label: fmt.Sprintf("%d lines", n),
			Value: m[1], Hint: fmt.Sprintf("%d lines", n),
			Check: "line_count",
		})
	} else if m := reLinesSimple.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "lines", Label: fmt.Sprintf("%d lines", n),
			Value: m[1], Hint: fmt.Sprintf("%d lines", n),
			Check: "line_count",
		})
	}

	// Bullet points / dashes
	if reBullets.MatchString(userMessage) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "bullets", Label: "bullet points",
			Value: "", Hint: "use dash bullets",
			Check: "dash_bullets",
		})
	}

	// Forbidden words
	for _, m := range reNoWord.FindAllStringSubmatch(userMessage, -1) {
		word := ""
		if m[1] != "" {
			word = m[1]
		} else if m[2] != "" {
			word = m[2]
		}
		if word != "" {
			hardConstraints = append(hardConstraints, Constraint{
				Type: "no_word", Label: fmt.Sprintf("do not use '%s'", word),
				Value: word, Hint: fmt.Sprintf("no '%s'", word),
				Check: "forbidden_word",
			})
		}
	}

	// End with specific word/phrase
	if m := reEndWith.FindStringSubmatch(userMessage); m != nil {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "end_with", Label: fmt.Sprintf("end with '%s'", m[1]),
			Value: m[1], Hint: fmt.Sprintf("end with '%s'", m[1]),
			Check: "end_with",
		})
	}

	// Start with capital letter
	if reStartWith.MatchString(userMessage) || reEachCap.MatchString(userMessage) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "capital_start", Label: "start with capital",
			Value: "", Hint: "each line starts with capital",
			Check: "capital_start",
		})
	}

	// JSON output
	if reJSON.MatchString(userMessage) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "json", Label: "JSON only",
			Value: "", Hint: "return valid JSON only",
			Check: "json",
		})
	}

	// Minimum character count
	if m := reMinChars.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "min_chars", Label: fmt.Sprintf("at least %d chars", n),
			Value: m[1], Hint: fmt.Sprintf("at least %d characters", n),
			Check: "min_chars",
		})
	}

	// Minimum word count
	if m := reMinWords.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "min_words", Label: fmt.Sprintf("at least %d words", n),
			Value: m[1], Hint: fmt.Sprintf("at least %d words", n),
			Check: "min_words",
		})
	}

	// No commas
	if reNoCommas.MatchString(userMessage) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "no_commas", Label: "no commas",
			Value: "", Hint: "no commas",
			Check: "no_commas",
		})
	}

	// No markdown
	if reNoMarkdown.MatchString(userMessage) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "no_markdown", Label: "no markdown",
			Value: "", Hint: "no markdown",
			Check: "no_markdown",
		})
	}

	// Highlight sections
	if m := reSections.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		hardConstraints = append(hardConstraints, Constraint{
			Type: "sections", Label: fmt.Sprintf("%d sections", n),
			Value: m[1], Hint: fmt.Sprintf("%d sections", n),
			Check: "sections",
		})
	}

	// No rhyme
	if reNoRhyme.MatchString(userMessage) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "no_rhyme", Label: "do not rhyme",
			Value: "", Hint: "do not rhyme",
			Check: "no_rhyme",
		})
	}

	wantsOneWord := reOneWord.MatchString(userMessage)
	wantsDigit := reDigitAsk.MatchString(userMessage)
	isMath := reMathAsk.MatchString(userMessage)

	// Math + one-word / digit format → require a numeric digit (not "Four").
	if isMath && (wantsOneWord || wantsDigit) {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "digit_answer", Label: "numeric digit only",
			Value: "", Hint: "numeric digit only",
			Check: "digit_answer",
		})
	} else if wantsDigit {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "digit_answer", Label: "numeric digit only",
			Value: "", Hint: "numeric digit only",
			Check: "digit_answer",
		})
	}

	// One-word answers
	if wantsOneWord {
		hardConstraints = append(hardConstraints, Constraint{
			Type: "one_word", Label: "exactly one word",
			Value: "1", Hint: "one word",
			Check: "one_word",
		})
	}

	// Soft hints — detected but NOT injected into model prompt by default.
	if reComplexQ.MatchString(userMessage) || reMultiStepQ.MatchString(userMessage) {
		softHints = append(softHints, Constraint{
			Type: "complex_reasoning", Label: "step-by-step reasoning",
			Value: "", Hint: "Break this question down step-by-step. Think through each part before answering.",
			Check: "none",
		})
	}

	if reFactualQ.MatchString(userMessage) {
		softHints = append(softHints, Constraint{
			Type: "factual_confidence", Label: "confidence indication",
			Value: "", Hint: "If you are uncertain about any factual claim, state your confidence level (high/medium/low).",
			Check: "none",
		})
	}

	return hardConstraints, softHints
}

// Extract scans the user prompt for known constraint patterns and returns
// structured constraints plus a minimal hint block to inject into the system prompt.
func (e *Engine) Extract(userMessage string) Result {
	if strings.TrimSpace(userMessage) == "" {
		return Result{}
	}

	hardConstraints, _ := e.extractConstraints(userMessage)

	// Deduplicate hard constraints
	constraints, dedupCount := deduplicateConstraints(hardConstraints)

	// Detect conflicts (diagnostic only — not injected into model prompt)
	conflicts := detectConflicts(constraints)

	if len(constraints) == 0 {
		return Result{
			Constraints:       nil,
			HintBlock:         "",
			HasConstraints:    false,
			Conflicts:         conflicts,
			DeduplicatedCount: dedupCount,
			SelectedProfile:   ProfileNone,
		}
	}

	// Select adaptive hint profile based on constraint complexity
	profile, complexityScore := e.SelectProfile(constraints, conflicts)

	// Build hint block using selected profile
	hintBlock := e.buildHintBlock(constraints, profile)

	return Result{
		Constraints:       constraints,
		HintBlock:         hintBlock,
		HasConstraints:    true,
		Conflicts:         conflicts,
		DeduplicatedCount: dedupCount,
		SelectedProfile:   profile,
		ComplexityScore:   complexityScore,
	}
}

// deduplicateConstraints removes duplicate constraints of the same type,
// keeping the first occurrence. Returns the deduplicated list and the count removed.
func deduplicateConstraints(constraints []Constraint) ([]Constraint, int) {
	seen := make(map[string]bool)
	var result []Constraint
	dropped := 0
	for _, c := range constraints {
		if seen[c.Type] {
			dropped++
			continue
		}
		seen[c.Type] = true
		result = append(result, c)
	}
	return result, dropped
}

// detectConflicts identifies contradictory constraints in the list.
// Returns a list of human-readable conflict descriptions for diagnostics only.
func detectConflicts(constraints []Constraint) []string {
	var conflicts []string

	oneWord := false
	oneWordCount := 0
	for _, c := range constraints {
		if c.Type == "one_word" {
			oneWord = true
		}
		if c.Type == "words" {
			n, _ := strconv.Atoi(c.Value)
			oneWordCount = n
		}
	}
	if oneWord && oneWordCount > 0 && oneWordCount != 1 {
		conflicts = append(conflicts, fmt.Sprintf("CONFLICT: 'one word' but also 'exactly %d words'", oneWordCount))
	}

	oneSentence := false
	for _, c := range constraints {
		if c.Type == "sentences" {
			n, _ := strconv.Atoi(c.Value)
			oneSentence = n == 1
		}
	}
	if oneWord && oneSentence {
		conflicts = append(conflicts, "NOTE: 'one word' and 'one sentence' are compatible only if the sentence is a single word")
	}

	hasJSON := false
	for _, c := range constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if hasJSON && oneWord {
		conflicts = append(conflicts, "CONFLICT: 'JSON output' and 'one word' are incompatible — JSON requires braces")
	}

	hasDigit := false
	for _, c := range constraints {
		if c.Type == "digit_answer" {
			hasDigit = true
			break
		}
	}
	if hasDigit && oneWord {
		conflicts = append(conflicts, "NOTE: 'digit answer' and 'one word' are compatible")
	}

	return conflicts
}

// buildMinimalHintBlock constructs a concise hint block from hard constraints.
// Principles:
//   - No header fluff ("CRITICAL", "Follow ALL rules")
//   - No numbered lists for single constraints
//   - No soft hints injected
//   - No conflict warnings injected
//   - No verification footer
//   - One line per constraint, minimum words
func buildMinimalHintBlock(constraints []Constraint) string {
	if len(constraints) == 0 {
		return ""
	}

	var parts []string
	for _, c := range constraints {
		if c.Check == "none" {
			continue
		}
		parts = append(parts, c.Hint)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}

// buildStandardHintBlock constructs a compact grouped summary for multiple
// non-conflicting constraints. Each constraint gets a brief descriptive label.
// Target: <=40 tokens typical, <=80 tokens hard upper bound.
func buildStandardHintBlock(constraints []Constraint) string {
	if len(constraints) == 0 {
		return ""
	}

	var parts []string
	for _, c := range constraints {
		if c.Check == "none" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", c.Label, c.Hint))
	}

	return strings.Join(parts, "\n")
}

// buildExplicitHintBlock constructs an ordered requirements list with a final
// verification reminder for complex or high-risk constraint combinations.
// Target: <=60 tokens typical, <=100 tokens hard upper bound.
func buildExplicitHintBlock(constraints []Constraint) string {
	if len(constraints) == 0 {
		return ""
	}

	var parts []string
	for i, c := range constraints {
		if c.Check == "none" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d. %s: %s", i+1, c.Label, c.Hint))
	}

	parts = append(parts, "verify all requirements before responding")

	return strings.Join(parts, "\n")
}

// Validate checks a response against extracted constraints and returns
// violations found. An empty list means all constraints pass.
func (e *Engine) Validate(response string, constraints []Constraint) ValidationResult {
	result := ValidationResult{Passed: true}

	for _, c := range constraints {
		passed, detail := checkConstraint(response, c)
		if passed {
			result.Satisfied = append(result.Satisfied, c.Label)
		} else {
			result.Passed = false
			result.Violations = append(result.Violations, detail)
		}
	}

	return result
}

// IsFormatRestrictive returns true if the constraints include format-restrictive
// rules that conflict with verbose or step-by-step guidance.
func (e *Engine) IsFormatRestrictive(constraints []Constraint) bool {
	for _, c := range constraints {
		switch c.Type {
		case "one_word", "digit_answer", "sentences", "words", "lines":
			return true
		}
	}
	return false
}

// BuildRetryHint creates a stronger reminder for constraints that failed.
func (e *Engine) BuildRetryHint(violations []string, constraints []Constraint) string {
	if len(violations) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "Fix these issues:")
	for i, v := range violations {
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, v))
	}

	return strings.Join(parts, "\n")
}

// ── Constraint checkers ──────────────────────────────────────────

func checkConstraint(response string, c Constraint) (bool, string) {
	switch c.Check {
	case "sentence_count":
		return checkSentenceCount(response, c)
	case "word_count":
		return checkWordCount(response, c)
	case "line_count":
		return checkLineCount(response, c)
	case "dash_bullets":
		return checkDashBullets(response)
	case "forbidden_word":
		return checkForbiddenWord(response, c)
	case "end_with":
		return checkEndWith(response, c)
	case "capital_start":
		return checkCapitalStart(response)
	case "json":
		return checkJSON(response)
	case "min_chars":
		return checkMinChars(response, c)
	case "min_words":
		return checkMinWords(response, c)
	case "no_commas":
		return checkNoCommas(response)
	case "no_markdown":
		return checkNoMarkdown(response)
	case "sections":
		return checkSections(response, c)
	case "no_rhyme":
		return checkNoRhyme(response)
	case "digit_answer":
		return checkDigitAnswer(response)
	case "one_word":
		return checkOneWord(response)
	case "none":
		return true, ""
	}
	return true, ""
}

func checkSentenceCount(response string, c Constraint) (bool, string) {
	n, _ := strconv.Atoi(c.Value)
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return false, fmt.Sprintf("empty response (expected %d sentences)", n)
	}
	count := 0
	for _, ch := range trimmed {
		if ch == '.' || ch == '!' || ch == '?' {
			count++
		}
	}
	if count == 0 {
		count = 1
	} else if trimmed[len(trimmed)-1] != '.' && trimmed[len(trimmed)-1] != '!' && trimmed[len(trimmed)-1] != '?' {
		count++
	}
	if count != n {
		return false, fmt.Sprintf("expected %d sentences, got %d", n, count)
	}
	return true, ""
}

func checkWordCount(response string, c Constraint) (bool, string) {
	n, _ := strconv.Atoi(c.Value)
	words := strings.Fields(response)
	if len(words) != n {
		return false, fmt.Sprintf("expected %d words, got %d", n, len(words))
	}
	return true, ""
}

func checkLineCount(response string, c Constraint) (bool, string) {
	n, _ := strconv.Atoi(c.Value)
	lines := nonEmptyLines(response)
	if len(lines) != n {
		return false, fmt.Sprintf("expected %d lines, got %d", n, len(lines))
	}
	return true, ""
}

func checkDashBullets(response string) (bool, string) {
	lines := nonEmptyLines(response)
	if len(lines) == 0 {
		return false, "empty response (expected bullet points)"
	}
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "-") {
			return false, "not all items use dash (-) bullets"
		}
	}
	return true, ""
}

func checkForbiddenWord(response string, c Constraint) (bool, string) {
	word := strings.ToLower(c.Value)
	if strings.Contains(strings.ToLower(response), word) {
		return false, fmt.Sprintf("contains forbidden word '%s'", c.Value)
	}
	return true, ""
}

func checkEndWith(response string, c Constraint) (bool, string) {
	trimmed := strings.TrimSpace(response)
	expected := strings.ToLower(strings.TrimRight(c.Value, "."))
	actual := strings.ToLower(strings.TrimRight(trimmed, "."))

	actualWords := strings.Fields(actual)
	if len(actualWords) == 0 {
		return false, fmt.Sprintf("empty response (expected to end with '%s')", c.Value)
	}
	lastWord := strings.TrimRight(actualWords[len(actualWords)-1], ".!?,;: ")
	if lastWord != expected {
		return false, fmt.Sprintf("expected to end with '%s', ends with '%s'", c.Value, lastWord)
	}
	return true, ""
}

func checkCapitalStart(response string) (bool, string) {
	lines := nonEmptyLines(response)
	for i, line := range lines {
		first := strings.TrimSpace(line)
		if len(first) > 0 && first[0] >= 'a' && first[0] <= 'z' {
			return false, fmt.Sprintf("line %d does not start with capital letter: '%s'", i+1, line)
		}
	}
	return true, ""
}

func checkJSON(response string) (bool, string) {
	trimmed := strings.TrimSpace(response)
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			fenceLine := trimmed[:idx]
			rest := strings.TrimSpace(trimmed[idx+1:])
			if fenceLine == "```" || (strings.HasPrefix(fenceLine, "```") && !strings.ContainsAny(fenceLine[3:], " \t")) {
				trimmed = rest
			}
		}
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "```"))
	}
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false, "response is not JSON (must start with { or [)"
	}
	return true, ""
}

func checkMinChars(response string, c Constraint) (bool, string) {
	n, _ := strconv.Atoi(c.Value)
	if len([]rune(response)) < n {
		return false, fmt.Sprintf("expected at least %d characters, got %d", n, len([]rune(response)))
	}
	return true, ""
}

func checkMinWords(response string, c Constraint) (bool, string) {
	n, _ := strconv.Atoi(c.Value)
	words := strings.Fields(response)
	if len(words) < n {
		return false, fmt.Sprintf("expected at least %d words, got %d", n, len(words))
	}
	return true, ""
}

func checkNoCommas(response string) (bool, string) {
	if strings.Contains(response, ",") {
		return false, "response contains commas"
	}
	return true, ""
}

func checkNoMarkdown(response string) (bool, string) {
	markdownPatterns := []string{"```", "**", "__", "~~", "# ", "## ", "### "}
	for _, p := range markdownPatterns {
		if strings.Contains(response, p) {
			return false, fmt.Sprintf("response contains markdown: %s", p)
		}
	}
	return true, ""
}

func checkSections(response string, c Constraint) (bool, string) {
	n, _ := strconv.Atoi(c.Value)
	re := regexp.MustCompile(`\*[^*]+\*`)
	matches := re.FindAllString(response, -1)
	if len(matches) < n {
		return false, fmt.Sprintf("expected at least %d highlighted sections, found %d", n, len(matches))
	}
	return true, ""
}

func checkNoRhyme(response string) (bool, string) {
	lines := nonEmptyLines(response)
	if len(lines) < 2 {
		return true, ""
	}
	for i := 0; i < len(lines)-1; i++ {
		for j := i + 1; j < len(lines); j++ {
			w1 := lastWord(lines[i])
			w2 := lastWord(lines[j])
			if w1 != "" && w2 != "" && len(w1) > 2 && len(w2) > 2 {
				if strings.HasSuffix(strings.ToLower(w1), strings.ToLower(w2[len(w2)-3:])) ||
					strings.HasSuffix(strings.ToLower(w2), strings.ToLower(w1[len(w1)-3:])) {
					return false, fmt.Sprintf("lines may rhyme: '%s' and '%s'", w1, w2)
				}
			}
		}
	}
	return true, ""
}

func checkDigitAnswer(response string) (bool, string) {
	cleaned := normalizeAnswerToken(response)
	if cleaned == "" {
		return false, "empty response (expected a numeric digit)"
	}
	if _, spelled := spelledNumbers[cleaned]; spelled {
		return false, fmt.Sprintf("spelled number '%s' is not allowed; use a numeric digit instead", cleaned)
	}
	start := 0
	if cleaned[0] == '-' {
		if len(cleaned) == 1 {
			return false, fmt.Sprintf("expected a numeric digit, got '%s'", cleaned)
		}
		start = 1
	}
	for i := start; i < len(cleaned); i++ {
		ch := cleaned[i]
		if ch < '0' || ch > '9' {
			return false, fmt.Sprintf("expected a numeric digit, got '%s'", cleaned)
		}
	}
	return true, ""
}

func checkOneWord(response string) (bool, string) {
	cleaned := strings.TrimSpace(response)
	if cleaned == "" {
		return false, "empty response (expected exactly one word)"
	}
	fields := strings.Fields(cleaned)
	if len(fields) != 1 {
		return false, fmt.Sprintf("expected exactly one word, got %d", len(fields))
	}
	return true, ""
}

func normalizeAnswerToken(response string) string {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
	token := fields[0]
	var b strings.Builder
	for _, ch := range strings.ToLower(token) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// ── Helpers ────────────────────────────────────────────────────────

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func lastWord(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	return words[len(words)-1]
}

// EstimateTokens is a rough token estimator (4 chars per token).
func EstimateTokens(s string) int {
	return (len(s) + 3) / 4
}
