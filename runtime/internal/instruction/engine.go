// Package instruction implements the Instruction-Following Assist Engine.
//
// It helps small local models follow complex formatting and content constraints
// by extracting constraints from user prompts, injecting explicit reminders into
// the system prompt, and validating responses post-generation with automatic
// retry on failure.
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

// Result holds extracted constraints and hint text.
type Result struct {
	Constraints       []Constraint `json:"constraints"`
	HintBlock         string       `json:"hint_block"`
	HasConstraints    bool         `json:"has_constraints"`
	Conflicts         []string     `json:"conflicts,omitempty"`
	DeduplicatedCount int          `json:"deduplicated_count,omitempty"`
	PriorityOrder     string       `json:"priority_order,omitempty"`
}

// ValidationResult is the outcome of checking a response against constraints.
type ValidationResult struct {
	Passed     bool     `json:"passed"`
	Violations []string `json:"violations"`
	Satisfied  []string `json:"satisfied"`
}

// Engine extracts and validates instruction constraints.
type Engine struct{}

// New creates an Instruction Engine.
func New() *Engine {
	return &Engine{}
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

	// Complex / multi-step reasoning questions
	reComplexQ = regexp.MustCompile(`(?i)(?:explain|analyze|compare|contrast|why|how\s+(?:does|do|can|would|should)|what\s+is\s+the\s+difference|solve|calculate|evaluate|interpret|summarize|break\s+down|walk\s+me\s+through|step\s+by\s+step)`)

	// Factual / definition questions — benefit from confidence indication
	reFactualQ = regexp.MustCompile(`(?i)(?:who\s+is|what\s+is|where\s+is|when\s+(?:was|did|will)|define|describe|list|what\s+are|tell\s+me\s+about)`)

	// Multi-step task indicators
	reMultiStepQ = regexp.MustCompile(`(?i)(?:first\s+.*then\s+.*finally|step\s+\d|multi[\s-]step|multiple\s+parts|several\s+steps)`)

	// Spelled cardinals that local models often emit instead of digits.
	spelledNumbers = map[string]struct{}{
		"zero": {}, "one": {}, "two": {}, "three": {}, "four": {}, "five": {},
		"six": {}, "seven": {}, "eight": {}, "nine": {}, "ten": {},
		"eleven": {}, "twelve": {}, "thirteen": {}, "fourteen": {}, "fifteen": {},
		"sixteen": {}, "seventeen": {}, "eighteen": {}, "nineteen": {}, "twenty": {},
		"thirty": {}, "forty": {}, "fifty": {}, "sixty": {}, "seventy": {},
		"eighty": {}, "ninety": {}, "hundred": {}, "thousand": {},
	}
)

// Constraint priority order for hint block construction.
// Higher priority constraints appear first in the hint block.
var constraintPriority = map[string]int{
	"json":               1,
	"one_word":           2,
	"digit_answer":       3,
	"sentences":          4,
	"words":              5,
	"lines":              6,
	"min_chars":          7,
	"min_words":          8,
	"end_with":           9,
	"start_with":         10,
	"capital_start":      11,
	"no_word":            12,
	"no_commas":          13,
	"no_markdown":        14,
	"no_rhyme":           15,
	"bullets":            16,
	"sections":           17,
	"complex_reasoning":  99,
	"factual_confidence": 99,
}

// Extract scans the user prompt for known constraint patterns and returns
// structured constraints plus a hint block to inject into the system prompt.
func (e *Engine) Extract(userMessage string) Result {
	if strings.TrimSpace(userMessage) == "" {
		return Result{}
	}

	var rawConstraints []Constraint

	// Sentence count
	if m := reSentences.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "sentences", Label: fmt.Sprintf("exactly %d sentences", n),
			Value: m[1], Hint: fmt.Sprintf("Your response must contain exactly %d sentence(s). No more, no less.", n),
			Check: "sentence_count",
		})
	}

	// Word count
	if m := reWords.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "words", Label: fmt.Sprintf("exactly %d words", n),
			Value: m[1], Hint: fmt.Sprintf("Your response must contain exactly %d word(s).", n),
			Check: "word_count",
		})
	}

	// Line count
	if m := reLines.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "lines", Label: fmt.Sprintf("%d lines", n),
			Value: m[1], Hint: fmt.Sprintf("Your response must have exactly %d line(s).", n),
			Check: "line_count",
		})
	} else if m := reLinesSimple.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "lines", Label: fmt.Sprintf("%d lines", n),
			Value: m[1], Hint: fmt.Sprintf("Produce exactly %d line(s).", n),
			Check: "line_count",
		})
	}

	// Bullet points / dashes
	if reBullets.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "bullets", Label: "bullet points",
			Value: "", Hint: "Use dash (-) bullet points for each item. Start each item on a new line with a dash.",
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
			rawConstraints = append(rawConstraints, Constraint{
				Type: "no_word", Label: fmt.Sprintf("do not use '%s'", word),
				Value: word, Hint: fmt.Sprintf("Do NOT use the word '%s' anywhere in your response.", word),
				Check: "forbidden_word",
			})
		}
	}

	// End with specific word/phrase
	if m := reEndWith.FindStringSubmatch(userMessage); m != nil {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "end_with", Label: fmt.Sprintf("end with '%s'", m[1]),
			Value: m[1], Hint: fmt.Sprintf("Your response must END with the word '%s'. Make sure the last word is '%s'.", m[1], m[1]),
			Check: "end_with",
		})
	}

	// Start with capital letter
	if reStartWith.MatchString(userMessage) || reEachCap.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "capital_start", Label: "start with capital",
			Value: "", Hint: "Each line of your response MUST start with a capital (uppercase) letter.",
			Check: "capital_start",
		})
	}

	// JSON output
	if reJSON.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "json", Label: "JSON only",
			Value: "", Hint: "Return ONLY a valid JSON object. No markdown fences, no explanation, no text outside the JSON.",
			Check: "json",
		})
	}

	// Minimum character count
	if m := reMinChars.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "min_chars", Label: fmt.Sprintf("at least %d chars", n),
			Value: m[1], Hint: fmt.Sprintf("Your response must be at least %d characters long.", n),
			Check: "min_chars",
		})
	}

	// Minimum word count
	if m := reMinWords.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "min_words", Label: fmt.Sprintf("at least %d words", n),
			Value: m[1], Hint: fmt.Sprintf("Your response must contain at least %d words.", n),
			Check: "min_words",
		})
	}

	// No commas
	if reNoCommas.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "no_commas", Label: "no commas",
			Value: "", Hint: "Do NOT use any commas (,) in your response.",
			Check: "no_commas",
		})
	}

	// No markdown
	if reNoMarkdown.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "no_markdown", Label: "no markdown",
			Value: "", Hint: "Do NOT use any markdown formatting in your response.",
			Check: "no_markdown",
		})
	}

	// Highlight sections
	if m := reSections.FindStringSubmatch(userMessage); m != nil {
		n, _ := strconv.Atoi(m[1])
		rawConstraints = append(rawConstraints, Constraint{
			Type: "sections", Label: fmt.Sprintf("%d sections", n),
			Value: m[1], Hint: fmt.Sprintf("Highlight at least %d sections using markdown titles like *Section Title*.", n),
			Check: "sections",
		})
	}

	// No rhyme
	if reNoRhyme.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "no_rhyme", Label: "do not rhyme",
			Value: "", Hint: "Lines should NOT rhyme with each other.",
			Check: "no_rhyme",
		})
	}

	wantsOneWord := reOneWord.MatchString(userMessage)
	wantsDigit := reDigitAsk.MatchString(userMessage)
	isMath := reMathAsk.MatchString(userMessage)

	// Math + one-word / digit format → require a numeric digit (not "Four").
	if isMath && (wantsOneWord || wantsDigit) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "digit_answer", Label: "numeric digit only",
			Value: "", Hint: "For this math question, answer with the numeric digit only (example: 4). Do NOT spell the number (example: not Four). Output a single digit or number with no punctuation or explanation.",
			Check: "digit_answer",
		})
	} else if wantsDigit {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "digit_answer", Label: "numeric digit only",
			Value: "", Hint: "Answer with the numeric digit only (example: 4). Do NOT spell the number (example: not Four).",
			Check: "digit_answer",
		})
	}

	// One-word answers (non-digit or in addition to digit) must be a single token.
	if wantsOneWord {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "one_word", Label: "exactly one word",
			Value: "1", Hint: "Your entire response must be exactly one word. No sentences, no punctuation, no extra explanation.",
			Check: "one_word",
		})
	}

	// Complex / multi-step reasoning question → inject step-by-step hint
	if reComplexQ.MatchString(userMessage) || reMultiStepQ.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "complex_reasoning", Label: "step-by-step reasoning",
			Value: "", Hint: "Break this question down step-by-step. Think through each part before answering.",
			Check: "none",
		})
	}

	// Factual / definition question → encourage confidence indication
	if reFactualQ.MatchString(userMessage) {
		rawConstraints = append(rawConstraints, Constraint{
			Type: "factual_confidence", Label: "confidence indication",
			Value: "", Hint: "If you are uncertain about any factual claim, state your confidence level (high/medium/low).",
			Check: "none",
		})
	}

	// ── Optimization: Deduplicate constraints ──
	constraints, dedupCount := deduplicateConstraints(rawConstraints)

	// ── Optimization: Detect conflicts ──
	conflicts := detectConflicts(constraints)

	// Only count hard constraints (Check != "none") for HasConstraints.
	// Soft hints (complex_reasoning, factual_confidence) do not set HasConstraints
	// so simple questions without formatting constraints still return HasConstraints=false.
	hasHardConstraints := false
	for _, c := range constraints {
		if c.Check != "none" {
			hasHardConstraints = true
			break
		}
	}

	if !hasHardConstraints {
		return Result{}
	}

	// ── Optimization: Priority-ordered hint block ──
	hintBlock := buildPrioritizedHintBlock(constraints, conflicts)

	return Result{
		Constraints:       constraints,
		HintBlock:         hintBlock,
		HasConstraints:    true,
		Conflicts:         conflicts,
		DeduplicatedCount: dedupCount,
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
// Returns a list of human-readable conflict descriptions.
func detectConflicts(constraints []Constraint) []string {
	var conflicts []string

	// Check for one_word + word_count mismatch
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
		conflicts = append(conflicts, fmt.Sprintf("CONFLICT: 'one word' but also 'exactly %d words' — these cannot both be satisfied", oneWordCount))
	}

	// Check for one_word + sentence_count mismatch
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

	// Check for JSON + one_word conflict
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

	// Check for digit_answer + one_word (non-math context)
	hasDigit := false
	for _, c := range constraints {
		if c.Type == "digit_answer" {
			hasDigit = true
			break
		}
	}
	if hasDigit && oneWord {
		conflicts = append(conflicts, "NOTE: 'digit answer' and 'one word' are compatible — a digit is a single word")
	}

	return conflicts
}

// buildPrioritizedHintBlock constructs a hint block with constraints ordered
// by priority (most critical first) and grouped logically.
func buildPrioritizedHintBlock(constraints []Constraint, conflicts []string) string {
	var parts []string

	// Hard constraints grouped by priority
	hardConstraints := make(map[int][]Constraint)
	var maxPriority int
	for _, c := range constraints {
		if c.Check == "none" {
			continue
		}
		p := constraintPriority[c.Type]
		if p > maxPriority {
			maxPriority = p
		}
		hardConstraints[p] = append(hardConstraints[p], c)
	}

	parts = append(parts, "CRITICAL: Follow ALL of these rules exactly:")

	// Emit constraints in priority order
	for p := 1; p <= maxPriority; p++ {
		group, ok := hardConstraints[p]
		if !ok {
			continue
		}
		for i, c := range group {
			parts = append(parts, fmt.Sprintf("%d. %s", len(parts), c.Hint))
			_ = i
		}
	}

	// Append soft hints after hard constraints
	softHints := make([]string, 0)
	for _, c := range constraints {
		if c.Check != "none" {
			continue
		}
		softHints = append(softHints, c.Hint)
	}
	if len(softHints) > 0 {
		parts = append(parts, "")
		parts = append(parts, "Additional guidance:")
		for _, h := range softHints {
			parts = append(parts, "- "+h)
		}
	}

	// Append conflict warnings if any
	if len(conflicts) > 0 {
		parts = append(parts, "")
		parts = append(parts, "⚠ WARNING: Some constraints may conflict. Resolve by following the most specific rule.")
		for _, c := range conflicts {
			parts = append(parts, "  "+c)
		}
	}

	parts = append(parts, "")
	parts = append(parts, "Before responding, verify each rule above is satisfied.")

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
// rules that conflict with verbose or step-by-step guidance (e.g. one_word,
// digit_answer, exact sentence/word/line counts).
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
	parts = append(parts, "YOUR PREVIOUS RESPONSE VIOLATED THESE RULES. FIX EACH ONE:")
	for i, v := range violations {
		parts = append(parts, fmt.Sprintf("%d. FAILED: %s", i+1, v))
	}
	parts = append(parts, "Try again. Follow ALL rules exactly this time.")
	if len(violations) > 1 {
		parts = append(parts, "TIP: Restate the question in your own words, then verify each requirement before output.")
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
	// Count last sentence even if no punctuation
	if count == 0 {
		count = 1
	} else if trimmed[len(trimmed)-1] != '.' && trimmed[len(trimmed)-1] != '!' && trimmed[len(trimmed)-1] != '?' {
		count++ // trailing text without punctuation
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

	// Check last word
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
	// Strip markdown fences if present (handles any language tag: ```json, ```python, etc.)
	if strings.HasPrefix(trimmed, "```") {
		if idx := strings.Index(trimmed, "\n"); idx >= 0 {
			fenceLine := trimmed[:idx]
			rest := strings.TrimSpace(trimmed[idx+1:])
			if fenceLine == "```" || (strings.HasPrefix(fenceLine, "```") && !strings.ContainsAny(fenceLine[3:], " \t")) {
				trimmed = rest
			}
		}
		// Strip the closing fence.
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
	// Simple check: look at last words of each line, if two end the same, flag it
	lines := nonEmptyLines(response)
	if len(lines) < 2 {
		return true, ""
	}
	for i := 0; i < len(lines)-1; i++ {
		for j := i + 1; j < len(lines); j++ {
			w1 := lastWord(lines[i])
			w2 := lastWord(lines[j])
			if w1 != "" && w2 != "" && len(w1) > 2 && len(w2) > 2 {
				// Check if last 3 chars match (simple rhyme detection)
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
	// Treat trailing punctuation as still one word ("Paris." / "4.").
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
	// Take the first field so "Four." / "4\n" normalize cleanly.
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
