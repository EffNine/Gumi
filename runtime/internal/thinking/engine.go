// Package thinking manages model reasoning/thinking content for Gumi.
//
// Local models with reasoning support can emit internal reasoning traces.
// Gumi's job is to detect those traces, strip them from the final response,
// and record only safe metadata (presence, length). Actual reasoning text is
// never stored.
package thinking

import (
	"regexp"
	"strings"
)

// Result describes the output of reasoning detection and stripping.
type Result struct {
	CleanContent     string
	ReasoningPresent bool
	ReasoningLength  int
}

// ExtractAndStrip removes common reasoning/thinking markers from assistant
// content and returns the cleaned content plus metadata.
func ExtractAndStrip(content string) Result {
	original := content
	reasoning := extractReasoning(content)
	clean := strings.TrimSpace(removeReasoning(content))

	if clean == "" {
		// If stripping removed everything, fall back to the original content so
		// the response is not empty. This is a safety guard.
		clean = strings.TrimSpace(original)
	}

	return Result{
		CleanContent:     clean,
		ReasoningPresent: reasoning != "",
		ReasoningLength:  len([]rune(reasoning)),
	}
}

var (
	// Match fenced reasoning blocks such as:
	//   <thinking>...</thinking>
	//   <reasoning>...</reasoning>
	//   ```thinking ... ```
	//   ```reasoning ... ```
	fencedReasoning = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<thinking>.*?</thinking>`),
		regexp.MustCompile(`(?is)<reasoning>.*?</reasoning>`),
		regexp.MustCompile("(?is)```thinking\\s*\\n?.*?```"),
		regexp.MustCompile("(?is)```reasoning\\s*\\n?.*?```"),
	}

	// Inline markers that often wrap reasoning text.
	inlineMarkers = []string{
		"<thinking>", "</thinking>",
		"<reasoning>", "</reasoning>",
		"[thinking]", "[/thinking]",
		"[reasoning]", "[/reasoning]",
	}
)

// extractReasoning returns only the reasoning text found in the content.
func extractReasoning(content string) string {
	var parts []string
	remaining := content
	for _, re := range fencedReasoning {
		for _, m := range re.FindAllString(remaining, -1) {
			parts = append(parts, m)
		}
		remaining = re.ReplaceAllString(remaining, "")
	}
	return strings.Join(parts, "\n")
}

// removeReasoning removes reasoning markers and fenced blocks from content.
func removeReasoning(content string) string {
	clean := content
	for _, re := range fencedReasoning {
		clean = re.ReplaceAllString(clean, "")
	}
	for _, marker := range inlineMarkers {
		clean = strings.ReplaceAll(clean, marker, "")
	}
	return clean
}

// ExtractAndStripProse detects and strips free-form reasoning prose from local
// model responses that do not use explicit reasoning tags. It splits content
// into paragraphs and removes leading contiguous paragraphs that match known
// reasoning patterns. If everything would be stripped, the original content is
// preserved as a safety fallback.
func ExtractAndStripProse(content string) Result {
	original := content
	paragraphs := splitParagraphs(content)

	var reasoningParts []string
	var cleanParts []string
	stripping := true

	for _, p := range paragraphs {
		if stripping && isReasoningProse(p) {
			reasoningParts = append(reasoningParts, p)
		} else {
			stripping = false
			cleanParts = append(cleanParts, p)
		}
	}

	clean := strings.TrimSpace(strings.Join(cleanParts, "\n\n"))
	if clean == "" {
		clean = strings.TrimSpace(original)
	}

	reasoningText := strings.Join(reasoningParts, "\n\n")

	return Result{
		CleanContent:     clean,
		ReasoningPresent: len(reasoningParts) > 0,
		ReasoningLength:  len([]rune(reasoningText)),
	}
}

// splitParagraphs splits content into paragraphs separated by one or more
// blank lines. Empty paragraphs are discarded.
func splitParagraphs(content string) []string {
	parts := paragraphSplitter.Split(strings.TrimSpace(content), -1)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// isReasoningProse checks whether a paragraph looks like reasoning prose by
// matching against known reasoning prefixes. Answer-transition patterns are
// checked first so they are never mistaken for reasoning.
func isReasoningProse(paragraph string) bool {
	if answerTransitionPattern.MatchString(paragraph) {
		return false
	}
	return reasoningProsePattern.MatchString(paragraph)
}

var (
	// paragraphSplitter splits on one or more blank lines.
	paragraphSplitter = regexp.MustCompile(`\n\n+`)

	// reasoningProsePattern matches paragraphs that begin with known reasoning
	// prose prefixes (case-insensitive, leading whitespace tolerant).
	reasoningProsePattern = regexp.MustCompile(`(?i)^\s*(` + reasoningProsePrefixes + `)`)

	// answerTransitionPattern matches paragraphs that begin with answer
	// transition phrases. These indicate the reasoning block has ended and the
	// actual answer is starting, so they are never stripped.
	answerTransitionPattern = regexp.MustCompile(`(?i)^\s*(` + answerTransitionPrefixes + `)`)

	reasoningProsePrefixes = strings.Join([]string{
		// First-person planning
		"I need to",
		"I should",
		"I will",
		"I am going to",
		"I think",
		"I have to",
		"Let me think",
		"Let me check",
		"Let me look at",
		"Let me consider",
		// User-focused
		"The user is asking",
		"The user wants",
		"The user needs",
		"The user said",
		"The user is requesting",
		// Discourse markers
		"Okay, so",
		"Alright, let me",
		"Hmm, let me",
		"Right, so",
		"Well, first",
		"First, let me",
		"Now, let me",
		"So, to answer",
		// Contextual
		"Based on the",
		"Looking at the",
		"Given the context",
		"From the context",
		// Model-specific reasoning markers
		"Thinking Process:",
		"Key points:",
	}, "|")

	answerTransitionPrefixes = strings.Join([]string{
		"Here's a plan:",
		"## Answer",
		"# Answer",
		"The answer is:",
		"In summary:",
		"To summarize:",
		"Therefore,",
		"So, to answer your question:",
	}, "|")
)

// HasReasoning is a convenience check for telemetry or guard decisions.
func HasReasoning(content string) bool {
	return ExtractAndStrip(content).ReasoningPresent
}
