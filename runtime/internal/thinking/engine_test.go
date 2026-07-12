package thinking

import (
	"strings"
	"testing"
)

func TestExtractAndStripNoReasoning(t *testing.T) {
	content := "This is a plain answer."
	result := ExtractAndStrip(content)
	if result.ReasoningPresent {
		t.Fatal("expected no reasoning")
	}
	if result.CleanContent != content {
		t.Fatalf("expected content preserved, got %q", result.CleanContent)
	}
	if result.ReasoningLength != 0 {
		t.Fatalf("expected zero reasoning length, got %d", result.ReasoningLength)
	}
}

func TestExtractAndStripXMLReasoning(t *testing.T) {
	content := "<thinking>Let me think...</thinking>The answer is 42."
	result := ExtractAndStrip(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if strings.Contains(result.CleanContent, "thinking") {
		t.Fatalf("expected tags stripped, got %q", result.CleanContent)
	}
	if !strings.Contains(result.CleanContent, "The answer is 42.") {
		t.Fatalf("expected answer preserved, got %q", result.CleanContent)
	}
	if result.ReasoningLength == 0 {
		t.Fatal("expected non-zero reasoning length")
	}
}

func TestExtractAndStripFencedReasoning(t *testing.T) {
	content := "```thinking\nStep one: parse the request.\n```\nDone."
	result := ExtractAndStrip(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if strings.Contains(result.CleanContent, "Step one") {
		t.Fatalf("expected fenced reasoning stripped, got %q", result.CleanContent)
	}
	if !strings.Contains(result.CleanContent, "Done.") {
		t.Fatalf("expected answer preserved, got %q", result.CleanContent)
	}
}

func TestExtractAndStripReasoningOnly(t *testing.T) {
	content := "```reasoning\nInternal monologue only.\n```"
	result := ExtractAndStrip(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != content {
		t.Fatalf("expected fallback to original when stripping leaves nothing, got %q", result.CleanContent)
	}
}

func TestExtractAndStripMultipleBlocks(t *testing.T) {
	content := "<thinking>A</thinking>Answer <reasoning>B</reasoning>42."
	result := ExtractAndStrip(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != "Answer 42." {
		t.Fatalf("expected multiple blocks stripped, got %q", result.CleanContent)
	}
}

func TestHasReasoning(t *testing.T) {
	if HasReasoning("plain") {
		t.Fatal("expected plain content to have no reasoning")
	}
	if !HasReasoning("<thinking>x</thinking>") {
		t.Fatal("expected reasoning detected")
	}
}

func TestExtractAndStripProseNoReasoning(t *testing.T) {
	content := "This is a plain answer."
	result := ExtractAndStripProse(content)
	if result.ReasoningPresent {
		t.Fatal("expected no reasoning")
	}
	if result.CleanContent != content {
		t.Fatalf("expected content preserved, got %q", result.CleanContent)
	}
	if result.ReasoningLength != 0 {
		t.Fatalf("expected zero reasoning length, got %d", result.ReasoningLength)
	}
}

func TestExtractAndStripProseLeadingReasoning(t *testing.T) {
	content := "I need to think about this carefully.\n\nThe answer is 42."
	result := ExtractAndStripProse(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != "The answer is 42." {
		t.Fatalf("expected reasoning stripped, got %q", result.CleanContent)
	}
	if result.ReasoningLength == 0 {
		t.Fatal("expected non-zero reasoning length")
	}
}

func TestExtractAndStripProseMultipleReasoningParagraphs(t *testing.T) {
	content := "I need to think about this.\n\nLet me check the context.\n\nThe answer is 42."
	result := ExtractAndStripProse(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != "The answer is 42." {
		t.Fatalf("expected all reasoning paragraphs stripped, got %q", result.CleanContent)
	}
}

func TestExtractAndStripProseStopsAtFirstNonReasoning(t *testing.T) {
	content := "I need to think.\n\nHere is the answer.\n\nI should also add this."
	result := ExtractAndStripProse(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != "Here is the answer.\n\nI should also add this." {
		t.Fatalf("expected non-reasoning and subsequent content preserved, got %q", result.CleanContent)
	}
}

func TestExtractAndStripProseAnswerTransitionKept(t *testing.T) {
	content := "I need to think about this.\n\nSo, to answer your question: the answer is 42."
	result := ExtractAndStripProse(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != "So, to answer your question: the answer is 42." {
		t.Fatalf("expected answer transition preserved, got %q", result.CleanContent)
	}
}

func TestExtractAndStripProseMixedCase(t *testing.T) {
	content := "I NEED TO think about this.\n\nThe answer is 42."
	result := ExtractAndStripProse(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != "The answer is 42." {
		t.Fatalf("expected case-insensitive reasoning stripped, got %q", result.CleanContent)
	}
}

func TestExtractAndStripProseFallbackToOriginal(t *testing.T) {
	content := "I need to think about this."
	result := ExtractAndStripProse(content)
	if !result.ReasoningPresent {
		t.Fatal("expected reasoning detected")
	}
	if result.CleanContent != content {
		t.Fatalf("expected fallback to original when everything is reasoning, got %q", result.CleanContent)
	}
	if result.ReasoningLength == 0 {
		t.Fatal("expected non-zero reasoning length")
	}
}

func TestExtractAndStripProseAllPatterns(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"first_person_planning", "I should check the data first.\n\nHere is the result."},
		{"first_person_will", "I will analyze this step by step.\n\nDone."},
		{"first_person_going_to", "I am going to break this down.\n\nResult: 42."},
		{"first_person_think", "I think the answer might be 42.\n\nActually, it is 42."},
		{"first_person_have_to", "I have to consider all options.\n\nThe best is 42."},
		{"let_me_think", "Let me think about this.\n\nAnswer: 42."},
		{"let_me_check", "Let me check the documentation.\n\nIt says 42."},
		{"let_me_look_at", "Let me look at the code.\n\nIt returns 42."},
		{"let_me_consider", "Let me consider the options.\n\nOption A is best."},
		{"user_is_asking", "The user is asking about X.\n\nX is 42."},
		{"user_wants", "The user wants to know Y.\n\nY is 42."},
		{"user_needs", "The user needs help with Z.\n\nZ is 42."},
		{"user_said", "The user said they need X.\n\nHere is X."},
		{"user_is_requesting", "The user is requesting help.\n\nHere is help."},
		{"okay_so", "Okay, so let me figure this out.\n\nIt is 42."},
		{"alright_let_me", "Alright, let me work through this.\n\nDone."},
		{"hmm_let_me", "Hmm, let me reconsider.\n\nIt is 42."},
		{"right_so", "Right, so the approach is clear.\n\nResult: 42."},
		{"well_first", "Well, first I need to parse the input.\n\nParsed."},
		{"first_let_me", "First, let me establish the facts.\n\nFact: 42."},
		{"now_let_me", "Now, let me apply the formula.\n\nFormula yields 42."},
		{"so_to_answer", "So, to answer this I need to calculate.\n\nCalculated: 42."},
		{"based_on_the", "Based on the data provided, the answer is 42.\n\nFinal: 42."},
		{"looking_at_the", "Looking at the request, I see a pattern.\n\nPattern: 42."},
		{"given_the_context", "Given the context of the question, the answer is 42.\n\nFinal: 42."},
		{"from_the_context", "From the context, I can determine the answer.\n\nAnswer: 42."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAndStripProse(tt.content)
			if !result.ReasoningPresent {
				t.Fatal("expected reasoning detected")
			}
			if result.ReasoningLength == 0 {
				t.Fatal("expected non-zero reasoning length")
			}
		})
	}
}

func TestExtractAndStripProseAllAnswerTransitions(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"heres_a_plan", "I need to think.\n\nHere's a plan: do X then Y."},
		{"hash_hash_answer", "I need to think.\n\n## Answer\nThe answer is 42."},
		{"hash_answer", "I need to think.\n\n# Answer\nThe answer is 42."},
		{"the_answer_is", "I need to think.\n\nThe answer is: 42."},
		{"in_summary", "I need to think.\n\nIn summary: the result is 42."},
		{"to_summarize", "I need to think.\n\nTo summarize: 42 is the answer."},
		{"therefore", "I need to think.\n\nTherefore, the answer is 42."},
		{"so_to_answer_your_question", "I need to think.\n\nSo, to answer your question: 42."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAndStripProse(tt.content)
			if !result.ReasoningPresent {
				t.Fatal("expected reasoning detected")
			}
			// The answer transition paragraph should be preserved.
			if strings.Contains(result.CleanContent, "I need to think") {
				t.Fatalf("expected reasoning stripped, got %q", result.CleanContent)
			}
			if result.ReasoningLength == 0 {
				t.Fatal("expected non-zero reasoning length")
			}
		})
	}
}
