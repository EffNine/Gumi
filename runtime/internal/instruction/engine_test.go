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
	if !strings.Contains(hint, "FIX EACH ONE") {
		t.Errorf("hint should contain 'FIX EACH ONE': %s", hint)
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
