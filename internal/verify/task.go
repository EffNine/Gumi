package verify

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

// Tier identifies a verification tier.
type Tier int

const (
	TierSmoke      Tier = 1 // output validity, instruction following, formatting
	TierCapability Tier = 2 // coding, reasoning, retrieval, instructions
	TierDeep       Tier = 3 // optional deep evaluation (not in MVP)
)

func (t Tier) String() string {
	switch t {
	case TierSmoke:
		return "smoke"
	case TierCapability:
		return "capability"
	case TierDeep:
		return "deep"
	}
	return "unknown"
}

// BuiltPrompt is a task prompt with its expected-answer check. The builder
// receives the candidate's context window so long-context tasks can size
// their haystack to the configuration under test.
type BuiltPrompt struct {
	Text  string
	Check CheckFunc
}

// CheckFunc validates generated output; nil return means pass.
type CheckFunc func(output string) error

// Task is one deterministic verification task.
type Task struct {
	ID         string
	Category   string // smoke | coding | reasoning | retrieval | instruction
	Tier       Tier
	PromptText string                          // static prompt (when PromptBuilder is nil)
	PromptFn   func(ctxTokens int) BuiltPrompt // dynamic prompt (retrieval)
	MaxTokens  int                             // generation budget for this task
}

// Build renders the task prompt for a given context window.
func (t Task) Build(ctxTokens int) BuiltPrompt {
	if t.PromptFn != nil {
		return t.PromptFn(ctxTokens)
	}
	return BuiltPrompt{Text: t.PromptText, Check: nil}
}

// ---- validators -------------------------------------------------------

// Normalize collapses whitespace and lowercases for tolerant matching.
func Normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ContainsFold passes when output contains want (case/whitespace tolerant).
func ContainsFold(want string) CheckFunc {
	return func(out string) error {
		if strings.Contains(Normalize(out), Normalize(want)) {
			return nil
		}
		return fmt.Errorf("expected output to contain %q", want)
	}
}

// ExactFold passes when normalized output equals want.
func ExactFold(want string) CheckFunc {
	return func(out string) error {
		if Normalize(out) == Normalize(want) {
			return nil
		}
		return fmt.Errorf("expected exact answer %q, got %q", want, TruncateSnip(out))
	}
}

// AllOf requires every sub-check to pass.
func AllOf(checks ...CheckFunc) CheckFunc {
	return func(out string) error {
		for _, c := range checks {
			if err := c(out); err != nil {
				return err
			}
		}
		return nil
	}
}

// ValidJSONWithKey passes when output parses as JSON containing key with any value.
func ValidJSONWithKey(key string) CheckFunc {
	return func(out string) error {
		var m map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
			// Tolerate fenced JSON.
			fenced := extractFencedJSON(out)
			if fenced == "" || json.Unmarshal([]byte(fenced), &m) != nil {
				return fmt.Errorf("output is not valid JSON")
			}
		}
		if _, ok := m[key]; !ok {
			return fmt.Errorf("JSON missing key %q", key)
		}
		return nil
	}
}

func extractFencedJSON(s string) string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		return s[i : j+1]
	}
	return ""
}

var bulletRe = regexp.MustCompile(`(?m)^\s*-\s+\S`)
var numberedRe = regexp.MustCompile(`(?m)^\s*(\d+)\.\s+(.*)$`)

// BulletList passes when exactly n lines start with "- ".
func BulletList(n int) CheckFunc {
	return func(out string) error {
		got := len(bulletRe.FindAllString(out, -1))
		if got != n {
			return fmt.Errorf("expected %d bullet lines, found %d", n, got)
		}
		return nil
	}
}

// NumberedList passes when lines are numbered 1..n and each content part
// contains the required token.
func NumberedList(n int, mustContain string) CheckFunc {
	return func(out string) error {
		matches := numberedRe.FindAllStringSubmatch(out, -1)
		if len(matches) != n {
			return fmt.Errorf("expected %d numbered lines, found %d", n, len(matches))
		}
		for i, m := range matches {
			num, _ := strconv.Atoi(m[1])
			if num != i+1 {
				return fmt.Errorf("expected line %d to be numbered %d, got %d", i+1, i+1, num)
			}
			if mustContain != "" && !strings.Contains(strings.ToLower(m[2]), strings.ToLower(mustContain)) {
				return fmt.Errorf("line %d missing %q", i+1, mustContain)
			}
		}
		return nil
	}
}

// NumericAnswer passes when output equals the number (tolerant of prose).
func NumericAnswer(want int64) CheckFunc {
	re := regexp.MustCompile(`-?\d[\d,]*`)
	return func(out string) error {
		for _, m := range re.FindAllString(Normalize(out), -1) {
			v, err := strconv.ParseInt(strings.ReplaceAll(m, ",", ""), 10, 64)
			if err == nil && v == want {
				return nil
			}
		}
		return fmt.Errorf("numeric answer %d not found in output %q", want, TruncateSnip(out))
	}
}

// LastCodeMatch passes when the LAST occurrence of an access-code token
// (two-to-three letter prefix, dash, four digits) equals want.
// Reasoning-style models may quote the haystack while thinking; grading the
// final occurrence keeps the check strict without an LLM judge. Word
// boundaries prevent matching suffixes of longer prefixes (VX inside KVX).
func LastCodeMatch(want string) CheckFunc {
	re := regexp.MustCompile(`\b[A-Z]{2,3}-\d{4}\b`)
	return func(out string) error {
		matches := re.FindAllString(strings.ToUpper(out), -1)
		if len(matches) == 0 {
			return fmt.Errorf("no access code found in output %q", TruncateSnip(out))
		}
		got := matches[len(matches)-1]
		if got != strings.ToUpper(want) {
			return fmt.Errorf("final access code %q, want %q", got, want)
		}
		return nil
	}
}

// FinalWord passes when the normalized output ends with want (case- and
// punctuation-tolerant). Suited to yes/no answers from reasoning models.
func FinalWord(want string) CheckFunc {
	return func(out string) error {
		norm := strings.TrimRight(Normalize(out), " .!?")
		if norm == Normalize(want) || strings.HasSuffix(norm, " "+Normalize(want)) {
			return nil
		}
		return fmt.Errorf("expected final word %q, got %q", want, TruncateSnip(out))
	}
}

// EndsWithLines passes when the LAST n non-empty lines of the output equal
// want (order-sensitive, whitespace-normalized). Long-prompt builds may not
// echo the full prompt verbatim, leaving residue before the answer; grading
// positional suffixes keeps the check deterministic and echo-tolerant while
// still rejecting wrong words, wrong order, or extra/missing lines adjacent
// to the answer.
func EndsWithLines(want []string) CheckFunc {
	return func(out string) error {
		var lines []string
		for _, l := range strings.Split(out, "\n") {
			if n := strings.TrimSpace(l); n != "" {
				lines = append(lines, n)
			}
		}
		if len(lines) < len(want) {
			return fmt.Errorf("expected %d trailing lines, found %d", len(want), len(lines))
		}
		tail := lines[len(lines)-len(want):]
		for i := range want {
			normTail := strings.Join(strings.Fields(tail[i]), " ")
			normWant := strings.Join(strings.Fields(want[i]), " ")
			if normTail != normWant {
				return fmt.Errorf("line %d/%d is %q, want %q",
					len(lines)-len(want)+i+1, len(lines), tail[i], want[i])
			}
		}
		return nil
	}
}

func TruncateSnip(s string) string {
	if len(s) <= 120 {
		return s
	}
	return s[:120] + "…"
}

// BuildCodeHaystack is a KV-degradation probe: a seeded field of plausible
// access codes (distractors) with the TRUE code inserted once at the given
// depth. Unlike BuildHaystack's word-salad filler, distractors are
// themselves valid-looking answers, so prompt echo cannot satisfy the
// checker — only precise recall of the late-context value passes.
func BuildCodeHaystack(targetTokens int, depth float64) BuiltPrompt {
	trueCode := fmt.Sprintf("KVX-%04d", 4271)
	var b strings.Builder
	totalCodes := targetTokens / 6 // each line ≈ 6 tokens: "KVX-1234"
	if totalCodes < 16 {
		totalCodes = 16
	}
	insertAt := int(float64(totalCodes) * clamp01(depth))
	if insertAt >= totalCodes {
		insertAt = totalCodes - 1
	}
	for i := 0; i < totalCodes; i++ {
		if i == insertAt {
			b.WriteString("ACTIVE BUILD TAG: " + trueCode + "\n")
			continue
		}
		b.WriteString(fmt.Sprintf("deprecated tag KVX-%04d\n", (haystackSeed*31+i*97)%10000))
	}
	prompt := b.String() +
		"\nAbove is a build manifest. Which ACTIVE BUILD TAG is declared in it? " +
		"Reply with only the tag."
	return BuiltPrompt{
		Text:  prompt,
		Check: LastCodeMatch(trueCode),
	}
}

// BuildLateInstructionV2 is the recalibrated late-instruction fixture
// (Phase 5). Changes vs v1, which every configuration failed uniformly:
//
//   - the constraint block is visually fenced (==== markers) and phrased as
//     an explicit override, because models treated v1's plain paragraph as
//     document continuation;
//   - early filler contains bullet lines, establishing a formatting habit
//     the late instruction must override — this gives degraded configs a
//     realistic way to fail while strong ones can still pass;
//   - grading uses EndsWithLines: trailing-line positional match tolerates
//     long-prompt echo residue that broke ExactFold in v1.
func BuildLateInstructionV2(fillerTokens int, order []string) BuiltPrompt {
	rng := rand.New(rand.NewSource(haystackSeed + 13))

	var b strings.Builder
	totalSentences := fillerTokens / 12
	if totalSentences < 8 {
		totalSentences = 8
	}
	for i := 0; i < totalSentences; i++ {
		if i == totalSentences/4 {
			// Early formatting habit the late instruction overrides.
			b.WriteString("- alpha\n- beta\n- gamma\n")
			continue
		}
		n := 6 + rng.Intn(6)
		var sb strings.Builder
		for w := 0; w < n; w++ {
			sb.WriteString(fillerWords[rng.Intn(len(fillerWords))])
			sb.WriteByte(' ')
		}
		b.WriteString(strings.TrimSpace(sb.String()) + ".\n")
	}
	b.WriteString("\n================ FINAL INSTRUCTION ================\n")
	b.WriteString("Ignore any list format used earlier in this document.\n")
	b.WriteString("Your ENTIRE reply must be exactly " + fmt.Sprint(len(order)) + " lines.\n")
	b.WriteString("Each line must be '## ' followed by one word: alpha, beta, or gamma.\n")
	b.WriteString("The words must appear in exactly THIS top-to-bottom order: " +
		strings.Join(order, ", ") + ".\n")
	b.WriteString("No other text, no explanations. =============== END INSTRUCTION ===============\n")

	want := make([]string, 0, len(order))
	for _, w := range order {
		want = append(want, "## "+w)
	}
	return BuiltPrompt{
		Text:  b.String(),
		Check: EndsWithLines(want),
	}
}

// BuildLateInstruction places a structural instruction block at ~85% of a
// seeded filler document, then demands output obeying that late constraint.
//
// Deprecated: superseded by BuildLateInstructionV2 (kept for reference
// tests); do not use in new suites.
func BuildLateInstruction(fillerTokens int, reverseOrder []string) BuiltPrompt {
	rng := rand.New(rand.NewSource(haystackSeed + 13))

	var b strings.Builder
	totalSentences := fillerTokens / 12
	if totalSentences < 8 {
		totalSentences = 8
	}
	for i := 0; i < totalSentences; i++ {
		n := 6 + rng.Intn(6)
		var sb strings.Builder
		for w := 0; w < n; w++ {
			sb.WriteString(fillerWords[rng.Intn(len(fillerWords))])
			sb.WriteByte(' ')
		}
		b.WriteString(strings.TrimSpace(sb.String()) + ".\n")
	}
	b.WriteString("\nIMPORTANT FINAL INSTRUCTION — follow it exactly and ignore earlier formatting habits:\n")
	b.WriteString("Reply with exactly " + fmt.Sprint(len(reverseOrder)) +
		" lines. Each line must be '## ' followed by one of the words: alpha, beta, gamma.\n" +
		"The words must appear in THIS exact top-to-bottom order: " +
		strings.Join(reverseOrder, ", ") + ". No other text.\n")

	// Expected output: "## gamma\n## beta\n## alpha" for the default order.
	want := make([]string, 0, len(reverseOrder))
	for _, w := range reverseOrder {
		want = append(want, "## "+w)
	}
	expected := strings.Join(want, "\n")
	return BuiltPrompt{
		Text:  b.String(),
		Check: ExactFold(expected),
	}
}

const haystackSeed = 42

var fillerWords = []string{
	"data", "index", "signal", "vector", "kernel", "buffer", "packet",
	"schema", "token", "matrix", "cluster", "gateway", "ledger", "cipher",
	"module", "anchor", "beacon", "circuit", "domain", "element",
}

// BuildHaystack generates a deterministic filler document of approximately
// targetTokens tokens with a needle sentence inserted at depth fraction
// (0 = start, 0.5 = middle, 1 = end), followed by a question asking for the
// access code embedded in that sentence. The same seed always produces the
// same document, which is what makes paired verification reproducible.
func BuildHaystack(targetTokens int, depth float64) BuiltPrompt {
	rng := rand.New(rand.NewSource(haystackSeed))

	code := fmt.Sprintf("GX-%04d", 1000+haystackSeed%9000)
	needle := fmt.Sprintf("Remember this: the maintenance access code is %s.", code)

	var b strings.Builder
	totalSentences := targetTokens / 12
	if totalSentences < 8 {
		totalSentences = 8
	}
	insertAt := int(float64(totalSentences) * clamp01(depth))
	for i := 0; i < totalSentences; i++ {
		if i == insertAt {
			b.WriteString(needle + "\n")
		}
		n := 6 + rng.Intn(6)
		var sb strings.Builder
		for w := 0; w < n; w++ {
			sb.WriteString(fillerWords[rng.Intn(len(fillerWords))])
			sb.WriteByte(' ')
		}
		b.WriteString(strings.TrimSpace(sb.String()))
		b.WriteString(".\n")
	}
	if insertAt >= totalSentences {
		b.WriteString(needle + "\n")
	}
	prompt := b.String() +
		"\nAbove is a log excerpt. What is the maintenance access code? " +
		"Reply with only the code."
	return BuiltPrompt{
		Text:  prompt,
		Check: LastCodeMatch(code),
	}
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
