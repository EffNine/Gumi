package verify

import (
	"strings"

	"math/rand"
)

const fillerSeed = 7

// FillerPrompt builds a deterministic neutral filler prompt of approximately
// targetTokens tokens. Used for performance probing where content does not
// matter but prompt size does (prefill measurement scales with it).
func FillerPrompt(targetTokens int) string {
	if targetTokens < 16 {
		targetTokens = 16
	}
	rng := rand.New(rand.NewSource(fillerSeed))
	var b strings.Builder
	for b.Len()/4 < targetTokens { // ~4 chars per token heuristic
		n := 8 + rng.Intn(6)
		var sb strings.Builder
		for w := 0; w < n; w++ {
			sb.WriteString(fillerWords[rng.Intn(len(fillerWords))])
			sb.WriteByte(' ')
		}
		b.WriteString(sb.String())
		b.WriteString("\n")
	}
	return b.String() +
		"\nSummarize the text above in one short sentence."
}
