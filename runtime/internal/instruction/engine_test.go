package instruction

import (
	"strings"
	"testing"
)

func TestExtractSentences(t *testing.T) {
	e := New()
	result := e.Extract("Answer in exactly 2 sentences about AI.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if len(result.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(result.Constraints))
	}
	if result.Constraints[0].Type != "sentences" {
		t.Errorf("expected sentences, got %s", result.Constraints[0].Type)
	}
	if result.Constraints[0].Value != "2" {
		t.Errorf("expected value 2, got %s", result.Constraints[0].Value)
	}
	if !strings.Contains(result.HintBlock, "2 sentence") {
		t.Errorf("hint should mention 2 sentences: %s", result.HintBlock)
	}
}

func TestExtractWords(t *testing.T) {
	result := New().Extract("Respond in exactly 5 words.")
	if !result.HasConstraints || result.Constraints[0].Type != "words" || result.Constraints[0].Value != "5" {
		t.Errorf("failed words extraction: %+v", result)
	}
}

func TestExtractLines(t *testing.T) {
	result := New().Extract("Write a 4-line poem about coding.")
	if !result.HasConstraints || result.Constraints[0].Type != "lines" || result.Constraints[0].Value != "4" {
		t.Errorf("failed lines extraction: %+v", result)
	}
}

func TestExtractLinesAlt(t *testing.T) {
	result := New().Extract("Write a 3-line answer about the solar system.")
	if !result.HasConstraints || result.Constraints[0].Type != "lines" || result.Constraints[0].Value != "3" {
		t.Errorf("failed lines alt extraction: %+v", result)
	}
}

func TestExtractBullets(t *testing.T) {
	result := New().Extract("List 3 benefits. Use bullet points with dashes.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasBullets := false
	for _, c := range result.Constraints {
		if c.Type == "bullets" {
			hasBullets = true
			break
		}
	}
	if !hasBullets {
		t.Error("expected bullet constraint")
	}
}

func TestExtractNoWord(t *testing.T) {
	result := New().Extract("Summarize AI. Do not use the word 'artificial'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasNoWord := false
	for _, c := range result.Constraints {
		if c.Type == "no_word" && c.Value == "artificial" {
			hasNoWord = true
			break
		}
	}
	if !hasNoWord {
		t.Error("expected no_word constraint for 'artificial'")
	}
}

func TestExtractNoWordAlt(t *testing.T) {
	result := New().Extract("Explain ML. Avoid the term 'neural'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasNoWord := false
	for _, c := range result.Constraints {
		if c.Type == "no_word" && c.Value == "neural" {
			hasNoWord = true
			break
		}
	}
	if !hasNoWord {
		t.Error("expected no_word constraint for 'neural'")
	}
}

func TestExtractEndWith(t *testing.T) {
	result := New().Extract("Describe Python. End with the word 'programming.'")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasEnd := false
	for _, c := range result.Constraints {
		if c.Type == "end_with" {
			hasEnd = true
			break
		}
	}
	if !hasEnd {
		t.Error("expected end_with constraint")
	}
}

func TestExtractCapitalStart(t *testing.T) {
	result := New().Extract("Each line must start with a capital letter.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasCap := false
	for _, c := range result.Constraints {
		if c.Type == "capital_start" {
			hasCap = true
			break
		}
	}
	if !hasCap {
		t.Error("expected capital_start constraint")
	}
}

func TestExtractJSON(t *testing.T) {
	result := New().Extract("Return only valid JSON with name and value.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasJSON := false
	for _, c := range result.Constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Error("expected json constraint")
	}
}

func TestExtractJSONFormatResponse(t *testing.T) {
	// "Format your response as JSON" — used by Terminus-2 agent system prompts.
	result := New().Extract("Format your response as JSON with the following structure:")
	if !result.HasConstraints {
		t.Fatal("expected constraints for 'Format your response as JSON'")
	}
	hasJSON := false
	for _, c := range result.Constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Error("expected json constraint for 'Format your response as JSON'")
	}
}

func TestExtractJSONRespondIn(t *testing.T) {
	// "Respond in JSON" — another common agent pattern.
	result := New().Extract("You must respond in JSON format.")
	if !result.HasConstraints {
		t.Fatal("expected constraints for 'Respond in JSON'")
	}
	hasJSON := false
	for _, c := range result.Constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Error("expected json constraint for 'Respond in JSON'")
	}
}

func TestExtractJSONFromSystemPrompt(t *testing.T) {
	// Simulates Terminus-2 system prompt + user message.
	systemPrompt := `You are an AI assistant. Format your response as JSON with the following structure:
{
  "analysis": "...",
  "plan": "...",
  "commands": [...]
}`
	userMsg := "The user wants to fix a bug in separability_matrix."
	combined := systemPrompt + "\n" + userMsg
	result := New().Extract(combined)
	if !result.HasConstraints {
		t.Fatal("expected constraints from combined system+user message")
	}
	hasJSON := false
	for _, c := range result.Constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Error("expected json constraint from system prompt containing 'Format your response as JSON'")
	}
}

func TestExtractNoCommas(t *testing.T) {
	result := New().Extract("Write a paragraph. Do not use any commas.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasNoComma := false
	for _, c := range result.Constraints {
		if c.Type == "no_commas" {
			hasNoComma = true
			break
		}
	}
	if !hasNoComma {
		t.Error("expected no_commas constraint")
	}
}

func TestExtractNoRhyme(t *testing.T) {
	result := New().Extract("Write a 4-line poem. Do not rhyme.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasNoRhyme := false
	for _, c := range result.Constraints {
		if c.Type == "no_rhyme" {
			hasNoRhyme = true
			break
		}
	}
	if !hasNoRhyme {
		t.Error("expected no_rhyme constraint")
	}
}

func TestExtractMultiple(t *testing.T) {
	result := New().Extract("Write a 4-line poem about AI. Each line must start with a capital letter. Do not rhyme. Do not use the word 'robot'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if len(result.Constraints) < 3 {
		t.Errorf("expected at least 3 constraints, got %d: %+v", len(result.Constraints), result.Constraints)
	}
}

func TestExtractEmpty(t *testing.T) {
	result := New().Extract("")
	if result.HasConstraints {
		t.Error("expected no constraints for empty prompt")
	}
}

func TestExtractNoConstraints(t *testing.T) {
	result := New().Extract("What is the capital of France?")
	if result.HasConstraints {
		t.Error("expected no constraints for simple question")
	}
}

// ── Validation tests ──────────────────────────────────────────────

func TestValidateSentenceCount(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "sentences", Check: "sentence_count", Value: "2", Label: "2 sentences"}}

	v := e.Validate("First sentence. Second sentence.", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("Only one sentence.", constraints)
	if v.Passed {
		t.Error("expected fail for 1 sentence")
	}

	v = e.Validate("", constraints)
	if v.Passed {
		t.Error("expected fail for empty")
	}
}

func TestValidateWordCount(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "words", Check: "word_count", Value: "3", Label: "3 words"}}

	v := e.Validate("one two three", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("one two", constraints)
	if v.Passed {
		t.Error("expected fail for 2 words")
	}
}

func TestValidateLineCount(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "lines", Check: "line_count", Value: "4", Label: "4 lines"}}

	v := e.Validate("Line one\nLine two\nLine three\nLine four", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("Line one\nLine two", constraints)
	if v.Passed {
		t.Error("expected fail for 2 lines")
	}
}

func TestValidateDashBullets(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "bullets", Check: "dash_bullets", Label: "dash bullets"}}

	v := e.Validate("- First\n- Second\n- Third", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("* First\n* Second", constraints)
	if v.Passed {
		t.Error("expected fail for asterisk bullets")
	}
}

func TestValidateForbiddenWord(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "no_word", Check: "forbidden_word", Value: "health", Label: "no health"}}

	v := e.Validate("Exercise improves mood.", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("Exercise improves health.", constraints)
	if v.Passed {
		t.Error("expected fail for containing 'health'")
	}
}

func TestValidateEndWith(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "end_with", Check: "end_with", Value: "learning", Label: "end learning"}}

	v := e.Validate("Machine learning is a subset of AI focused on automated learning.", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("Machine learning is AI.", constraints)
	if v.Passed {
		t.Error("expected fail for ending with 'AI'")
	}
}

func TestValidateCapitalStart(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "capital_start", Check: "capital_start", Label: "capital"}}

	v := e.Validate("Logic builds code.\nFunctions call functions.\nErrors teach skills.", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("logic builds code.\nFunctions call functions.", constraints)
	if v.Passed {
		t.Error("expected fail for lowercase start")
	}
}

func TestValidateJSON(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "json", Check: "json", Label: "JSON"}}

	v := e.Validate(`{"name": "test", "value": 42}`, constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("Not JSON", constraints)
	if v.Passed {
		t.Error("expected fail for non-JSON")
	}
}

func TestValidateNoCommas(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "no_commas", Check: "no_commas", Label: "no commas"}}

	v := e.Validate("This is a sentence without commas.", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("This, has, commas.", constraints)
	if v.Passed {
		t.Error("expected fail for commas")
	}
}

func TestValidateMultiple(t *testing.T) {
	e := New()
	constraints := []Constraint{
		{Type: "sentences", Check: "sentence_count", Value: "2", Label: "2 sentences"},
		{Type: "no_word", Check: "forbidden_word", Value: "robot", Label: "no robot"},
	}

	// Both pass
	v := e.Validate("First sentence. Second sentence.", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	// One fails
	v = e.Validate("First about robot. Second sentence.", constraints)
	if v.Passed {
		t.Error("expected fail for robot word")
	}
	if len(v.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(v.Violations))
	}
}

func TestBuildRetryHint(t *testing.T) {
	e := New()
	hint := e.BuildRetryHint([]string{"expected 2 sentences, got 1"}, nil)
	if hint == "" {
		t.Error("expected retry hint")
	}
	if !strings.Contains(hint, "Fix these issues") {
		t.Errorf("hint should contain 'Fix these issues': %s", hint)
	}
}

func TestExtractDigitAnswerFromMathOneWord(t *testing.T) {
	result := New().Extract("What is 2+2? Answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasDigit := false
	hasOneWord := false
	for _, c := range result.Constraints {
		if c.Type == "digit_answer" {
			hasDigit = true
		}
		if c.Type == "one_word" {
			hasOneWord = true
		}
	}
	if !hasDigit {
		t.Error("expected digit_answer constraint for math + one-word prompt")
	}
	if !hasOneWord {
		t.Error("expected one_word constraint")
	}
	if !strings.Contains(result.HintBlock, "numeric digit") {
		t.Errorf("hint should mention numeric digit: %s", result.HintBlock)
	}
}

func TestExtractDigitAnswerExplicit(t *testing.T) {
	result := New().Extract("How many sides does a triangle have? Answer with a digit.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasDigit := false
	for _, c := range result.Constraints {
		if c.Type == "digit_answer" {
			hasDigit = true
		}
	}
	if !hasDigit {
		t.Error("expected digit_answer constraint")
	}
}

func TestExtractOneWordWithoutMath(t *testing.T) {
	result := New().Extract("What is the capital of France? Answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasDigit := false
	hasOneWord := false
	for _, c := range result.Constraints {
		if c.Type == "digit_answer" {
			hasDigit = true
		}
		if c.Type == "one_word" {
			hasOneWord = true
		}
	}
	if hasDigit {
		t.Error("did not expect digit_answer for factual one-word prompt")
	}
	if !hasOneWord {
		t.Error("expected one_word constraint")
	}
}

func TestValidateDigitAnswer(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "digit_answer", Check: "digit_answer", Label: "numeric digit only"}}

	if v := e.Validate("4", constraints); !v.Passed {
		t.Errorf("expected pass for '4': %v", v.Violations)
	}
	if v := e.Validate("4.", constraints); !v.Passed {
		t.Errorf("expected pass for '4.': %v", v.Violations)
	}
	if v := e.Validate("-12", constraints); !v.Passed {
		t.Errorf("expected pass for '-12': %v", v.Violations)
	}
	if v := e.Validate("Four", constraints); v.Passed {
		t.Error("expected fail for spelled 'Four'")
	}
	if v := e.Validate("Four.", constraints); v.Passed {
		t.Error("expected fail for spelled 'Four.'")
	}
	if v := e.Validate("the answer is 4", constraints); v.Passed {
		t.Error("expected fail for multi-word prose")
	}
}

func TestValidateOneWord(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "one_word", Check: "one_word", Label: "exactly one word"}}

	if v := e.Validate("Paris", constraints); !v.Passed {
		t.Errorf("expected pass for 'Paris': %v", v.Violations)
	}
	if v := e.Validate("Paris.", constraints); !v.Passed {
		t.Errorf("expected pass for 'Paris.': %v", v.Violations)
	}
	if v := e.Validate("The capital is Paris", constraints); v.Passed {
		t.Error("expected fail for multi-word answer")
	}
}

// ── Optimization tests ──────────────────────────────────────────────

func TestExtractPriorityOrdering(t *testing.T) {
	result := New().Extract("Return JSON. Answer in one word. Write exactly 3 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// All three hints should be present (JSON, one_word, sentences)
	hint := result.HintBlock
	if !strings.Contains(hint, "return valid JSON only") {
		t.Errorf("expected JSON hint in block: %s", hint)
	}
	if !strings.Contains(hint, "one word") {
		t.Errorf("expected one_word hint in block: %s", hint)
	}
	if !strings.Contains(hint, "exactly 3 sentences") {
		t.Errorf("expected sentences hint in block: %s", hint)
	}
}

func TestExtractDeduplication(t *testing.T) {
	// Prompt that would trigger two "no_word" constraints for the same word
	result := New().Extract("Do not use the word 'test'. Avoid the word 'test'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	noWordCount := 0
	for _, c := range result.Constraints {
		if c.Type == "no_word" {
			noWordCount++
		}
	}
	if noWordCount != 1 {
		t.Errorf("expected 1 deduplicated no_word constraint, got %d", noWordCount)
	}
	if result.DeduplicatedCount != 1 {
		t.Errorf("expected DeduplicatedCount=1, got %d", result.DeduplicatedCount)
	}
}

func TestExtractConflictDetection(t *testing.T) {
	result := New().Extract("Return JSON and answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict detected between JSON and one_word")
	}
	hasConflict := false
	for _, c := range result.Conflicts {
		if strings.Contains(c, "CONFLICT") && strings.Contains(c, "JSON") {
			hasConflict = true
			break
		}
	}
	if !hasConflict {
		t.Error("expected JSON/one_word conflict in conflicts list")
	}
}

func TestExtractConflictOneWordVsWordCount(t *testing.T) {
	result := New().Extract("Answer in exactly 5 words. Respond in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict between one_word and word_count")
	}
}

func TestIsFormatRestrictive(t *testing.T) {
	e := New()

	// one_word is format-restrictive
	if !e.IsFormatRestrictive([]Constraint{{Type: "one_word", Check: "one_word"}}) {
		t.Error("expected one_word to be format-restrictive")
	}

	// digit_answer is format-restrictive
	if !e.IsFormatRestrictive([]Constraint{{Type: "digit_answer", Check: "digit_answer"}}) {
		t.Error("expected digit_answer to be format-restrictive")
	}

	// sentence count is format-restrictive
	if !e.IsFormatRestrictive([]Constraint{{Type: "sentences", Check: "sentence_count", Value: "3"}}) {
		t.Error("expected sentences to be format-restrictive")
	}

	// JSON is NOT format-restrictive (allows verbose output)
	if e.IsFormatRestrictive([]Constraint{{Type: "json", Check: "json"}}) {
		t.Error("expected json to NOT be format-restrictive")
	}

	// empty constraints
	if e.IsFormatRestrictive([]Constraint{}) {
		t.Error("expected empty constraints to not be format-restrictive")
	}
}

func TestBuildPrioritizedHintBlockIncludesConflicts(t *testing.T) {
	result := New().Extract("Return JSON and answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Conflicts should be available in diagnostics but NOT in the hint block
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict detected between JSON and one_word")
	}
	// Hint block should NOT contain conflict warnings
	if strings.Contains(result.HintBlock, "CONFLICT") {
		t.Errorf("hint block should not contain conflict warnings: %s", result.HintBlock)
	}
}

func TestExtractHintBlockContainsVerificationStep(t *testing.T) {
	result := New().Extract("Answer in exactly 2 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Minimal hint block: just the constraint hint, no verification footer
	if result.HintBlock == "" {
		t.Error("expected non-empty hint block")
	}
	if strings.Contains(result.HintBlock, "verify each rule") {
		t.Error("hint block should not contain verification step")
	}
}

func TestExtractSoftHintsAppendedSeparately(t *testing.T) {
	result := New().Extract("Explain why the sky is blue. Answer in exactly 2 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Hard constraint should be present in hint block
	if result.HintBlock == "" {
		t.Error("expected non-empty hint block")
	}
	// Soft hints should NOT appear in the hint block
	if strings.Contains(result.HintBlock, "step-by-step") {
		t.Error("soft hints should not be injected into model prompt")
	}
}

// ── Sprint 17R2 regression tests ──────────────────────────────────

func TestNoHintForSimpleQuestion(t *testing.T) {
	result := New().Extract("What is the capital of France?")
	if result.HasConstraints {
		t.Error("expected no constraints for simple question")
	}
	if result.HintBlock != "" {
		t.Errorf("expected empty hint block for simple question: %q", result.HintBlock)
	}
}

func TestNoUnnecessaryHintInjection(t *testing.T) {
	result := New().Extract("Hello, how are you?")
	if result.HasConstraints {
		t.Error("expected no constraints for greeting")
	}
}

func TestConciseHintGeneration(t *testing.T) {
	result := New().Extract("Answer in exactly 3 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Hint block should be concise (no headers, no footers, no numbering)
	hint := result.HintBlock
	if strings.Contains(hint, "CRITICAL") {
		t.Error("hint block should not contain 'CRITICAL' header")
	}
	if strings.Contains(hint, "Before responding, verify") {
		t.Error("hint block should not contain verification footer")
	}
	if strings.Contains(hint, "Additional guidance") {
		t.Error("hint block should not contain soft hints section")
	}
	if strings.Contains(hint, "⚠ WARNING") {
		t.Error("hint block should not contain conflict warnings")
	}
	if strings.Contains(hint, "1.") || strings.Contains(hint, "2.") {
		t.Error("hint block should not use numbered list format")
	}
}

func TestSoftHintsDisabledByDefault(t *testing.T) {
	// Complex reasoning question should NOT have soft hints in hint block
	result := New().Extract("Explain why the sky is blue step by step.")
	if result.HasConstraints {
		t.Error("expected no hard constraints for complex reasoning question")
	}
	if result.HintBlock != "" {
		t.Errorf("expected empty hint block when only soft hints present: %q", result.HintBlock)
	}
}

func TestConflictHandlingDiagnosticOnly(t *testing.T) {
	result := New().Extract("Return JSON and answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Conflicts should be in diagnostics
	if len(result.Conflicts) == 0 {
		t.Error("expected conflicts to be detected")
	}
	// But NOT in the hint block
	if strings.Contains(result.HintBlock, "CONFLICT") || strings.Contains(result.HintBlock, "WARNING") {
		t.Errorf("conflicts should not appear in hint block: %q", result.HintBlock)
	}
}

func TestJSONPreservation(t *testing.T) {
	result := New().Extract("Return valid JSON with name and age fields.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasJSON := false
	for _, c := range result.Constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Error("expected json constraint")
	}
	// Hint should be minimal, not competing with JSON format
	if strings.Contains(result.HintBlock, "markdown") {
		t.Error("JSON hint should not mention markdown")
	}
	if strings.Contains(result.HintBlock, "explain") {
		t.Error("JSON hint should not mention explanation")
	}
}

func TestOneWordPreservation(t *testing.T) {
	result := New().Extract("What is 2+2? Answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasOneWord := false
	for _, c := range result.Constraints {
		if c.Type == "one_word" {
			hasOneWord = true
			break
		}
	}
	if !hasOneWord {
		t.Error("expected one_word constraint")
	}
	// Hint should be minimal
	if strings.Contains(result.HintBlock, "No sentences") {
		t.Error("one_word hint should be concise")
	}
}

func TestDigitPreservation(t *testing.T) {
	result := New().Extract("How many sides does a triangle have? Answer with a digit.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasDigit := false
	for _, c := range result.Constraints {
		if c.Type == "digit_answer" {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		t.Error("expected digit_answer constraint")
	}
}

func TestExactCountPreservation(t *testing.T) {
	result := New().Extract("Write a 5-line poem about cats.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hasLines := false
	for _, c := range result.Constraints {
		if c.Type == "lines" {
			hasLines = true
			break
		}
	}
	if !hasLines {
		t.Error("expected lines constraint")
	}
	if result.HintBlock != "5 lines" {
		t.Errorf("expected concise hint '5 lines', got: %q", result.HintBlock)
	}
}

func TestNoDuplicatedConstraints(t *testing.T) {
	result := New().Extract("Do not use the word 'test'. Avoid the word 'test'. Do not use the word 'test'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	noWordCount := 0
	for _, c := range result.Constraints {
		if c.Type == "no_word" {
			noWordCount++
		}
	}
	if noWordCount != 1 {
		t.Errorf("expected 1 deduplicated no_word constraint, got %d", noWordCount)
	}
}

func TestNoDuplicatedInstructions(t *testing.T) {
	// Same constraint extracted twice should produce single hint line
	result := New().Extract("Answer in exactly 2 sentences. Reply in exactly 2 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Count how many times "sentences" appears in hint block
	count := strings.Count(result.HintBlock, "sentences")
	if count > 1 {
		t.Errorf("expected at most 1 sentence hint, got %d occurrences: %q", count, result.HintBlock)
	}
}

func TestNoGenericStepByStepAutoInjection(t *testing.T) {
	result := New().Extract("Tell me about quantum computing.")
	// Factual question — soft hint should not be injected
	if result.HasConstraints {
		t.Error("expected no hard constraints for factual question")
	}
	if strings.Contains(result.HintBlock, "confidence") {
		t.Error("soft factual hint should not be injected")
	}
}

func TestNoGenericConfidenceAutoInjection(t *testing.T) {
	result := New().Extract("Who invented the telephone?")
	if result.HasConstraints {
		t.Error("expected no hard constraints for simple factual question")
	}
	if strings.Contains(result.HintBlock, "confidence") {
		t.Error("soft confidence hint should not be injected")
	}
}

func TestHintBlockTokenBudget(t *testing.T) {
	// Multi-constraint prompt should still produce concise hint
	result := New().Extract("Return JSON with name and value. Do not use the word 'test'. Answer in exactly 2 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// Hint block should be bounded — no more than ~50 chars per constraint line
	lines := strings.Split(result.HintBlock, "\n")
	for _, line := range lines {
		if len(line) > 80 {
			t.Errorf("hint line too long (%d chars): %q", len(line), line)
		}
	}
	// Total hint should not exceed 300 chars for 3 constraints
	if len(result.HintBlock) > 300 {
		t.Errorf("hint block too verbose (%d chars): %q", len(result.HintBlock), result.HintBlock)
	}
}

func TestRetryBehavior(t *testing.T) {
	e := New()
	violations := []string{"expected 2 sentences, got 1", "contains forbidden word 'test'"}
	constraints := []Constraint{
		{Type: "sentences", Check: "sentence_count", Value: "2", Label: "2 sentences"},
		{Type: "no_word", Check: "forbidden_word", Value: "test", Label: "no test"},
	}
	hint := e.BuildRetryHint(violations, constraints)
	if hint == "" {
		t.Error("expected retry hint for violations")
	}
	// Should not contain verbose preamble
	if strings.Contains(hint, "YOUR PREVIOUS RESPONSE VIOLATED") {
		t.Error("retry hint should not contain verbose preamble")
	}
	if strings.Contains(hint, "TIP:") {
		t.Error("retry hint should not contain tip")
	}
}

func TestNoModelSpecificWording(t *testing.T) {
	result := New().Extract("Return JSON with name and age.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	hint := result.HintBlock
	// Should not contain model-specific terms
	for _, term := range []string{"Ollama", "LM Studio", "LLaMA", "GPT", "Claude"} {
		if strings.Contains(hint, term) {
			t.Errorf("hint should not contain model-specific term '%s': %q", term, hint)
		}
	}
}

func TestExtractNoConstraintsSimplePrompt(t *testing.T) {
	simplePrompts := []string{
		"Hello",
		"What is AI?",
		"Tell me a joke",
		"Explain photosynthesis",
		"Write a story",
	}
	for _, prompt := range simplePrompts {
		result := New().Extract(prompt)
		if result.HasConstraints {
			t.Errorf("expected no constraints for simple prompt: %q", prompt)
		}
	}
}

func TestExtractJSONWithSystemPrompt(t *testing.T) {
	// When system prompt has JSON instructions, they should be detected
	systemPrompt := "You are an API. Return JSON responses only."
	userMsg := "Get user data."
	result := New().Extract(systemPrompt + "\n" + userMsg)
	if !result.HasConstraints {
		t.Fatal("expected constraints from system prompt")
	}
	hasJSON := false
	for _, c := range result.Constraints {
		if c.Type == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		t.Error("expected json constraint from combined system+user prompt")
	}
}

func TestValidateExactWordCount(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "words", Check: "word_count", Value: "3", Label: "3 words"}}

	v := e.Validate("one two three", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("one two three four", constraints)
	if v.Passed {
		t.Error("expected fail for 4 words")
	}
}

func TestValidateExactLineCount(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "lines", Check: "line_count", Value: "3", Label: "3 lines"}}

	v := e.Validate("line one\nline two\nline three", constraints)
	if !v.Passed {
		t.Errorf("expected pass: %v", v.Violations)
	}

	v = e.Validate("line one\nline two", constraints)
	if v.Passed {
		t.Error("expected fail for 2 lines")
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Error("empty string should have 0 tokens")
	}
	// Rough check: 100 chars should be ~25 tokens
	tokens := EstimateTokens("this is a test string with about thirty words in it")
	if tokens <= 0 {
		t.Error("estimate should return positive token count")
	}
}

// ── Adaptive Hint Strategy Tests (Sprint 17R3) ────────────────────

func TestSelectProfileNoConstraints(t *testing.T) {
	e := New()
	profile, score := e.SelectProfile(nil, nil)
	if profile != ProfileNone {
		t.Errorf("expected NONE profile for no constraints, got %s", profile)
	}
	if score != 0 {
		t.Errorf("expected score 0 for no constraints, got %d", score)
	}
}

func TestSelectProfileSingleSimpleConstraint(t *testing.T) {
	e := New()
	constraints := []Constraint{{Type: "sentences", Check: "sentence_count", Value: "2", Label: "exactly 2 sentences", Hint: "exactly 2 sentences"}}
	profile, score := e.SelectProfile(constraints, nil)
	if profile != ProfileMinimal {
		t.Errorf("expected MINIMAL profile for single constraint, got %s", profile)
	}
	if score != 1 {
		t.Errorf("expected score 1 for single constraint, got %d", score)
	}
}

func TestSelectProfileMultipleConstraints(t *testing.T) {
	e := New()
	constraints := []Constraint{
		{Type: "sentences", Check: "sentence_count", Value: "2", Label: "exactly 2 sentences", Hint: "exactly 2 sentences"},
		{Type: "no_word", Check: "forbidden_word", Value: "test", Label: "do not use 'test'", Hint: "no 'test'"},
	}
	profile, score := e.SelectProfile(constraints, nil)
	if profile != ProfileStandard {
		t.Errorf("expected STANDARD profile for 2 constraints, got %s", profile)
	}
	if score != 2 {
		t.Errorf("expected score 2 for 2 constraints, got %d", score)
	}
}

func TestSelectProfileComplexConstraints(t *testing.T) {
	e := New()
	constraints := []Constraint{
		{Type: "sentences", Check: "sentence_count", Value: "3", Label: "exactly 3 sentences", Hint: "exactly 3 sentences"},
		{Type: "no_word", Check: "forbidden_word", Value: "technology", Label: "do not use 'technology'", Hint: "no 'technology'"},
		{Type: "capital_start", Check: "capital_start", Label: "start with capital", Hint: "each line starts with capital"},
		{Type: "end_with", Check: "end_with", Value: "future", Label: "end with 'future'", Hint: "end with 'future'"},
	}
	profile, score := e.SelectProfile(constraints, nil)
	if profile != ProfileExplicit {
		t.Errorf("expected EXPLICIT profile for 4 constraints, got %s", profile)
	}
	if score != 4 {
		t.Errorf("expected score 4 for 4 constraints, got %d", score)
	}
}

func TestSelectProfileWithConflicts(t *testing.T) {
	e := New()
	constraints := []Constraint{
		{Type: "json", Check: "json", Label: "JSON only", Hint: "return valid JSON only"},
		{Type: "one_word", Check: "one_word", Label: "exactly one word", Hint: "one word"},
	}
	conflicts := []string{"CONFLICT: 'JSON output' and 'one word' are incompatible"}
	profile, score := e.SelectProfile(constraints, conflicts)
	// 2 constraints + 1 conflict + 1 JSON bonus = 4 → EXPLICIT
	if profile != ProfileExplicit {
		t.Errorf("expected EXPLICIT profile for 2 constraints + 1 conflict + JSON bonus, got %s", profile)
	}
	if score != 4 {
		t.Errorf("expected score 4 for 2 constraints + 1 conflict + JSON bonus, got %d", score)
	}
}

func TestSelectProfileJSONWithOtherConstraints(t *testing.T) {
	e := New()
	constraints := []Constraint{
		{Type: "json", Check: "json", Label: "JSON only", Hint: "return valid JSON only"},
		{Type: "no_word", Check: "forbidden_word", Value: "test", Label: "do not use 'test'", Hint: "no 'test'"},
		{Type: "no_markdown", Check: "no_markdown", Label: "no markdown", Hint: "no markdown"},
	}
	profile, score := e.SelectProfile(constraints, nil)
	if profile != ProfileExplicit {
		t.Errorf("expected EXPLICIT profile for JSON + 2 other constraints, got %s", profile)
	}
	if score != 4 {
		t.Errorf("expected score 4 for JSON+others, got %d", score)
	}
}

func TestExtractProfileSelectionDeterministic(t *testing.T) {
	e := New()

	// Same prompt should always produce the same profile
	prompts := []string{
		"Answer in exactly 2 sentences.",
		"Explain Go in exactly 2 sentences. Do not use the word 'programming'.",
		"Write a short paragraph about AI. Requirements: exactly 3 sentences, no 'technology', each sentence starts with capital, end with 'future'.",
		"Return valid JSON with name and age. No markdown fences.",
	}

	for _, prompt := range prompts {
		r1 := e.Extract(prompt)
		r2 := e.Extract(prompt)
		if r1.SelectedProfile != r2.SelectedProfile {
			t.Errorf("non-deterministic profile for prompt %q: %s vs %s", prompt, r1.SelectedProfile, r2.SelectedProfile)
		}
		if r1.ComplexityScore != r2.ComplexityScore {
			t.Errorf("non-deterministic score for prompt %q: %d vs %d", prompt, r1.ComplexityScore, r2.ComplexityScore)
		}
	}
}

func TestExtractProfileNoneForSimplePrompt(t *testing.T) {
	e := New()
	result := e.Extract("What is the capital of France?")
	if result.HasConstraints {
		t.Error("expected no constraints for simple question")
	}
	if result.SelectedProfile != ProfileNone {
		t.Errorf("expected NONE profile, got %s", result.SelectedProfile)
	}
	if result.HintBlock != "" {
		t.Errorf("expected empty hint block, got %q", result.HintBlock)
	}
}

func TestExtractProfileMinimalForSingleConstraint(t *testing.T) {
	e := New()
	result := e.Extract("Answer in exactly 2 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if result.SelectedProfile != ProfileMinimal {
		t.Errorf("expected MINIMAL profile, got %s", result.SelectedProfile)
	}
	if result.ComplexityScore != 1 {
		t.Errorf("expected complexity score 1, got %d", result.ComplexityScore)
	}
	if result.HintBlock != "exactly 2 sentences" {
		t.Errorf("expected minimal hint, got %q", result.HintBlock)
	}
}

func TestExtractProfileStandardForMultipleConstraints(t *testing.T) {
	e := New()
	result := e.Extract("Explain Go in exactly 2 sentences. Do not use the word 'programming'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if result.SelectedProfile != ProfileStandard {
		t.Errorf("expected STANDARD profile, got %s", result.SelectedProfile)
	}
	if result.ComplexityScore != 2 {
		t.Errorf("expected complexity score 2, got %d", result.ComplexityScore)
	}
	// Standard profile should have label: hint format
	if !strings.Contains(result.HintBlock, "exactly 2 sentences: exactly 2 sentences") {
		t.Errorf("expected standard format with labels, got %q", result.HintBlock)
	}
}

func TestExtractProfileExplicitForComplexConstraints(t *testing.T) {
	e := New()
	// 4 distinct constraints: sentences, no_word, capital_start, end_with
	result := e.Extract("Write a short paragraph about AI. Requirements: exactly 3 sentences, do not use the word 'technology', each line must start with a capital, end with the word 'future'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	if result.SelectedProfile != ProfileExplicit {
		t.Errorf("expected EXPLICIT profile, got %s", result.SelectedProfile)
	}
	if result.ComplexityScore < 4 {
		t.Errorf("expected complexity score >= 4, got %d", result.ComplexityScore)
	}
	// Explicit profile should have numbered list + verification
	if !strings.Contains(result.HintBlock, "1.") {
		t.Errorf("expected numbered list in explicit profile, got %q", result.HintBlock)
	}
	if !strings.Contains(result.HintBlock, "verify all requirements") {
		t.Errorf("expected verification reminder in explicit profile, got %q", result.HintBlock)
	}
}

func TestHintBlockTokenBudgetStandard(t *testing.T) {
	e := New()
	result := e.Extract("Explain Go in exactly 2 sentences. Do not use the word 'programming'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	tokenCount := EstimateTokens(result.HintBlock)
	if tokenCount > 40 {
		t.Errorf("standard hint block exceeds 40 tokens: %d tokens, block=%q", tokenCount, result.HintBlock)
	}
}

func TestHintBlockTokenBudgetExplicit(t *testing.T) {
	e := New()
	result := e.Extract("Write a short paragraph about AI. Requirements: exactly 3 sentences, no 'technology', each sentence starts with capital, end with 'future'.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	tokenCount := EstimateTokens(result.HintBlock)
	if tokenCount > 100 {
		t.Errorf("explicit hint block exceeds 100 tokens: %d tokens, block=%q", tokenCount, result.HintBlock)
	}
}

func TestExtractOneWordPlusDigit(t *testing.T) {
	e := New()
	result := e.Extract("What is 2+2? Answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// one_word + digit_answer → 2 constraints, score=2 → STANDARD
	if result.SelectedProfile != ProfileStandard {
		t.Errorf("expected STANDARD profile for one_word+digit, got %s", result.SelectedProfile)
	}
}

func TestExtractExactSentenceWordLineCounts(t *testing.T) {
	e := New()

	// Single exact-count constraint → MINIMAL
	r1 := e.Extract("Answer in exactly 3 sentences.")
	if r1.SelectedProfile != ProfileMinimal {
		t.Errorf("expected MINIMAL for single sentence count, got %s", r1.SelectedProfile)
	}

	r2 := e.Extract("Respond in exactly 5 words.")
	if r2.SelectedProfile != ProfileMinimal {
		t.Errorf("expected MINIMAL for single word count, got %s", r2.SelectedProfile)
	}

	r3 := e.Extract("Write a 4-line poem.")
	if r3.SelectedProfile != ProfileMinimal {
		t.Errorf("expected MINIMAL for single line count, got %s", r3.SelectedProfile)
	}
}

func TestExtractJSONWithSystemPromptProfile(t *testing.T) {
	e := New()
	systemPrompt := "You are an API. Return JSON responses only."
	userMsg := "Get user data."
	result := e.Extract(systemPrompt + "\n" + userMsg)
	if !result.HasConstraints {
		t.Fatal("expected constraints from system prompt")
	}
	// Single JSON constraint → MINIMAL
	if result.SelectedProfile != ProfileMinimal {
		t.Errorf("expected MINIMAL for single JSON constraint, got %s", result.SelectedProfile)
	}
}

func TestExtractJSONWithAdditionalConstraintsProfile(t *testing.T) {
	e := New()
	// "Use plain text" won't trigger no_markdown (needs "markdown" keyword).
	// Use a prompt that clearly triggers JSON + one other constraint.
	result := e.Extract("Return valid JSON with name and age. Answer in exactly 2 sentences.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// JSON + sentences = 2 constraints + 1(JSON bonus) = score 3 → STANDARD
	if result.SelectedProfile != ProfileStandard {
		t.Errorf("expected STANDARD for JSON+other, got %s", result.SelectedProfile)
	}
}

func TestExtractConflictingConstraintsProfile(t *testing.T) {
	e := New()
	result := e.Extract("Return JSON and answer in one word.")
	if !result.HasConstraints {
		t.Fatal("expected constraints")
	}
	// JSON + one_word = 2 constraints + 1 conflict + 1(JSON bonus) = score 4 → EXPLICIT
	if result.SelectedProfile != ProfileExplicit {
		t.Errorf("expected EXPLICIT for conflicting constraints, got %s", result.SelectedProfile)
	}
	if len(result.Conflicts) == 0 {
		t.Error("expected conflict detected")
	}
}

func TestNoModelSpecificWordingInAnyProfile(t *testing.T) {
	e := New()
	prompts := []string{
		"Answer in exactly 2 sentences.",
		"Explain Go in exactly 2 sentences. Do not use the word 'programming'.",
		"Write a short paragraph about AI. Requirements: exactly 3 sentences, no 'technology', each sentence starts with capital, end with 'future'.",
		"Return valid JSON with name and age. Do not use markdown fences.",
	}
	for _, prompt := range prompts {
		result := e.Extract(prompt)
		hint := result.HintBlock
		for _, term := range []string{"Ollama", "LM Studio", "LLaMA", "GPT", "Claude", "Gemma", "Qwen"} {
			if strings.Contains(hint, term) {
				t.Errorf("hint should not contain model-specific term '%s' for prompt %q: %q", term, prompt, hint)
			}
		}
	}
}

func TestAdaptiveStrategyNoSoftHintInjection(t *testing.T) {
	e := New()
	// Complex reasoning question with no hard constraints
	result := e.Extract("Explain why the sky is blue step by step.")
	if result.HasConstraints {
		t.Error("expected no hard constraints for complex reasoning question")
	}
	if result.SelectedProfile != ProfileNone {
		t.Errorf("expected NONE profile, got %s", result.SelectedProfile)
	}
	if result.HintBlock != "" {
		t.Errorf("expected empty hint block, got %q", result.HintBlock)
	}
}

func TestRetryFreeNormalPath(t *testing.T) {
	e := New()
	// All profiles should produce valid hint blocks that don't require retries
	testCases := []string{
		"Answer in exactly 2 sentences.",
		"Explain Go in exactly 2 sentences. Do not use the word 'programming'.",
		"Write a short paragraph about AI. Requirements: exactly 3 sentences, no 'technology', each sentence starts with capital, end with 'future'.",
		"Return valid JSON with name and age. Do not use markdown fences.",
		"What is 2+2? Answer in one word.",
		"List 3 benefits. Use bullet points with dashes.",
	}
	for _, prompt := range testCases {
		result := e.Extract(prompt)
		if result.HasConstraints && result.HintBlock == "" {
			t.Errorf("expected non-empty hint block for prompt with constraints: %q", prompt)
		}
		// Hint block should not contain any conflict warnings
		if strings.Contains(result.HintBlock, "CONFLICT") || strings.Contains(result.HintBlock, "WARNING") {
			t.Errorf("hint block should not contain conflict warnings: %q", result.HintBlock)
		}
		// Hint block should not contain soft hints
		if strings.Contains(result.HintBlock, "step-by-step") || strings.Contains(result.HintBlock, "confidence") {
			t.Errorf("hint block should not contain soft hints: %q", result.HintBlock)
		}
	}
}
