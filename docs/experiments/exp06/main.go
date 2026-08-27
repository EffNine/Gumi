// Experiment 06 — agentic context economics (Phase 8).
//
// Measures whether an agentic coding workload actually NEEDS large active
// context, using a simulated coding-agent session whose context grows as
// the agent works (file reads, test logs, search results, review comments),
// then asks one final task that requires synthesizing information planted
// across the session:
//
//	RULE  — early-session fact   (~10% depth): active pricing rule ID
//	TEST  — mid-session fact     (~50% depth): failing test name
//	FIX   — late-session fact    (~90% depth): approved fix phrase
//
// Strategies differ ONLY in the active context window delivered to the
// backend (8k / 16k / 24k / 32k, all else identical to the Phase-6 anchor
// shape). When a session exceeds a strategy's visible budget, the OLDEST
// blocks are dropped first (sliding window = worst-case memory management;
// smart compaction could only do better, never worse). This models the
// optimizer-harness reality: no compaction subsystem exists outside the
// frozen pre-pivot runtime (AgentConfig.ContextCompactionThreshold), so the
// experiment measures small-window economics under naive eviction.
//
// Usage:
//
//	go run ./docs/experiments/exp06 -model <gguf> [-out dir] [-quick]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EffNine/gumi/internal/backend"
	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/verify"
)

var (
	flagModel  = flag.String("model", "", "path to GGUF model")
	flagOut    = flag.String("out", "reports/phase8", "output directory")
	flagQuick  = flag.Bool("quick", false, "smoke mode: 2 sizes x 1 rep (for harness validation)")
	flagProbes = flag.Int("probes", 3, "perf probe repetitions per strategy")
	flagBin    = flag.String("backend-bin", "", "path to llama-cli (default: PATH)")
	flagGensz  = flag.Int("gen", 768, "generation cap for final answers (ceiling, not target; thinking-style models need headroom)")
)

// ---- session fixture ---------------------------------------------------

const sessionSeed = 42

// Expected answers; every session instance plants the same facts so all
// cells are paired.
const (
	wantRule = "EU-VAT-DEFERRED-1987"
	wantTest = "TestBulkDiscount_OffByOne"
	wantFix  = "clamp negative amounts to zero"
	ruleBps  = "1875"
)

type block struct {
	kind string // briefing | read_file | test_log | grep | review | gitlog
	text string
}

// Token-density calibration. Synthetic code/log content tokenizes denser
// than prose and the density is TOKENIZER-DEPENDENT (measured on this
// harness: llama-3.1 ≈ 2.9 chars/token, Qwen3 ≈ 2.2). The driver therefore
// calibrates the factor against the actual model before the sweep — one
// untruncated run of a mid-size session at the largest window, deriving
//
//	density = prompt_chars / actual_prompt_tokens (from timing telemetry)
//
// and shares the calibrated value across ALL strategies so every strategy
// sees byte-identical sessions (paired comparison preserved). Until
// calibrated the factor starts conservative; eviction additionally applies
// an 8% margin. The backend (llama-cli v10360) ERRORS rather than truncates
// when a prompt exceeds -c in single-turn mode, so overflow must never be
// delegated to the backend.
var (
	tokDensity    = 2.2 // chars per token; replaced by calibration
	evictionSlack = 0.92
)

func estTokens(chars int) int { return int(float64(chars) / tokDensity) }

// buildSession renders a deterministic agent session of approximately
// targetTokens tokens (chars/4 heuristic, consistent with internal/verify
// fixtures). Cross-session facts are planted at fixed relative DEPTHS
// (fraction of total blocks), so every size variant carries identical task
// semantics: RULE at the very start (dropped first under eviction),
// TEST near the middle, FIX at the end (always inside any window).
func buildSession(targetTokens int) string {
	rng := rand.New(rand.NewSource(int64(sessionSeed + targetTokens)))

	var filler []block
	kindCycle := []string{"read_file", "test_log", "grep", "review", "gitlog"}
	// Budget the filler below the nominal target so the FIXED blocks
	// (briefing, final code, approval, preamble, question) land the whole
	// session near the target even for small sizes.
	fixedChars := len(briefingBlock().text) + len(finalCodeBlock().text) +
		len(approvalBlock().text) + 900 // preamble + question block
	fillerTarget := targetTokens - estTokens(fixedChars)
	if fillerTarget < 1200 {
		fillerTarget = 1200
	}
	for i := 0; float64(charsOf(filler))/tokDensity < float64(fillerTarget); i++ {
		filler = append(filler, contentBlock(kindCycle[i%len(kindCycle)], rng, i))
	}

	blocks := []block{briefingBlock()} // RULE fact lives here (position 0)
	insertAt := func(frac float64, b block) {
		pos := int(float64(len(blocks)+len(filler)) * frac)
		if pos <= len(blocks) {
			pos = len(blocks)
		} else {
			pos -= len(blocks)
			if pos > len(filler) {
				pos = len(filler)
			}
		}
		idx := pos
		filler = append(filler, block{})
		copy(filler[idx+1:], filler[idx:])
		filler[idx] = b
	}
	insertAt(0.18, reviewBlock())      // early reinforcement of the rule
	insertAt(0.50, failingTestBlock()) // TEST fact @~mid depth
	blocks = append(blocks, filler...)
	blocks = append(blocks, finalCodeBlock())
	blocks = append(blocks, approvalBlock()) // FIX fact (newest content)

	var b strings.Builder
	b.WriteString("You are a coding agent. Below is your accumulated work session.\n\n")
	for j, bl := range blocks {
		fmt.Fprintf(&b, "--- STEP %d [%s] ---\n%s\n\n", j+1, strings.ToUpper(bl.kind), bl.text)
	}
	b.WriteString(`=== SESSION COMPLETE ===

Answer the final task using ONLY the session above. Reply with EXACTLY three
lines and nothing else, in this order:
RULE=<pricing rule id declared ACTIVE in this session>
TEST=<exact Go test name that FAILED during this session>
FIX=<exact approved-fix phrase from the latest review comment>
`)
	return b.String()
}

func charsOf(bs []block) int {
	n := 0
	for _, b := range bs {
		n += len(b.text)
	}
	return n
}

func briefingBlock() block {
	return block{kind: "briefing", text: fmt.Sprintf(`
TASK: migrate the legacy pricing pipeline in src/catalog/pricing to the new
engine without changing customer-visible prices.

Project layout:
  src/catalog/pricing/legacy.go      (old engine, being replaced)
  src/catalog/pricing/engine.go      (new engine skeleton)
  src/catalog/pricing/engine_test.go (contract tests)
  src/catalog/discount/bulk.go       (shared bulk-discount helper)

COMPLIANCE NOTE (binding for this project):
Active pricing rule: PRICING_RULE_ID=%s
Legacy rate basis: RATE_BASIS_POINTS=%s (basis points, applied to net amount)
All migrations MUST preserve RATE_BASIS_POINTS unless a newer rule supersedes it.
Nothing later in this session supersedes it.`, wantRule, ruleBps)}
}

func failingTestBlock() block {
	return block{kind: "test_log", text: fmt.Sprintf(`
$ go test ./src/catalog/discount/ -run Bulk -v
=== RUN   TestBulkDiscount_Plain
--- PASS: TestBulkDiscount_Plain (0.01s)
=== RUN   %s
    bulk_test.go:42: got -150 cents for negative amount, want 0
--- FAIL: %s (0.00s)
FAIL
FAIL	src/catalog/discount	0.08s`, wantTest, wantTest)}
}

func reviewBlock() block {
	return block{kind: "review", text: `
reviewer (ada-q): checked legacy.go line 88 — the old engine hard-codes
RATE_BASIS_POINTS=1875 too, so the compliance note matches the code.
Keep it in the migration.`}
}

func finalCodeBlock() block {
	return block{kind: "read_file", text: `
tool output: src/catalog/pricing/engine.go (current working state)

package pricing

// ApplyNet computes the final charge in cents.
func ApplyNet(amountCents int64, bulk bool) int64 {
	if bulk {
		return applyBulkDiscount(amountCents)
	}
	return applyRate(amountCents)
}

func applyBulkDiscount(amountCents int64) int64 {
	disc := amountCents * 95 / 100
	return applyRate(disc)
}`}
}

func approvalBlock() block {
	return block{kind: "review", text: fmt.Sprintf(`
reviewer (ada-q): LGTM pending one fix.
Approved fix: %s BEFORE applying bulk discount.
Land it with the contract tests green.`, wantFix)}
}

// contentBlock emits deterministic filler resembling real agent traffic.
func contentBlock(kind string, rng *rand.Rand, seq int) block {
	switch kind {
	case "read_file":
		var b strings.Builder
		fmt.Fprintf(&b, "tool output: src/catalog/pricing/gen_%02d.go\n\npackage pricing\n\n", seq)
		for l := 0; l < 26+seq%9; l++ {
			fn := []string{"normalize", "roundTo", "splitTax", "mergeLine", "auditRow"}[seq%5]
			fmt.Fprintf(&b, "func %s_%02d(x int64) int64 { return (x*%d + %d) %% 100000 }\n",
				fn, seq, 97+rng.Intn(800), rng.Intn(500))
		}
		return block{kind: kind, text: b.String()}
	case "test_log":
		var b strings.Builder
		fmt.Fprintf(&b, "$ go test ./src/catalog/pricing/ -count=1\n")
		for l := 0; l < 14+seq%7; l++ {
			fmt.Fprintf(&b, "--- PASS: TestGen%02d_Case%d (%d.%02ds)\n",
				seq, l, rng.Intn(3)/10+1, rng.Intn(99))
		}
		b.WriteString("ok\tsrc/catalog/pricing\t2.418s\n")
		return block{kind: kind, text: b.String()}
	case "grep":
		var b strings.Builder
		fmt.Fprintf(&b, "$ grep -rn \"rate\" src/catalog/ | head -30\n")
		for l := 0; l < 22+seq%11; l++ {
			fmt.Fprintf(&b, "src/catalog/pricing/file%02d.go:%d: rateHints[%d] = %d // nolint\n",
				seq%97, l, rng.Intn(64), 1000+rng.Intn(9000))
		}
		return block{kind: kind, text: b.String()}
	case "review":
		return block{kind: kind, text: fmt.Sprintf(`
reviewer (kim-w): left inline comments on gen_%02d.go — naming only,
no functional findings. Please prefix helpers with the package name.`, seq)}
	default: // gitlog
		var b strings.Builder
		for l := 0; l < 10+seq%5; l++ {
			fmt.Fprintf(&b, "%08x refactor(pricing): batch %d — mechanical renames (%d files)\n",
				rng.Int63(), seq*10+l, 3+rng.Intn(20))
		}
		return block{kind: kind, text: b.String()}
	}
}

// truncateToVisible drops the OLDEST whole blocks until the session fits the
// visible budget (chars/4 <= visibleTokens). The header and the final
// question are always preserved.
func truncateToVisible(session string, visibleTokens int) (string, int) {
	limitChars := int(float64(visibleTokens) * tokDensity * evictionSlack)
	if len(session) <= limitChars {
		return session, estTokens(len(session))
	}
	marker := "\n--- STEP "
	headEnd := strings.Index(session, marker)
	if headEnd < 0 {
		return session[:limitChars], limitChars / 4
	}
	header := session[:headEnd]
	tail := session[headEnd:]
	// Drop leading STEP blocks from the tail while over budget. The final
	// question block (after "SESSION COMPLETE") is protected.
	qi := strings.Index(tail, "=== SESSION COMPLETE ===")
	body, question := tail[:qi], tail[qi:]
	for len(body) > limitChars-len(header)-len(question) {
		nl := strings.Index(body[len(marker):], marker)
		if nl < 0 {
			break
		}
		body = body[len(marker)+nl:]
	}
	out := header + body + question
	return out, estTokens(len(out))
}

// grade splits the answer into RULE=/TEST=/FIX= lines ("=" or ":" labels
// both accepted) and grades each part, so partial failures localize the
// capacity loss.
func grade(out string) map[string]bool {
	res := map[string]bool{"RULE": false, "TEST": false, "FIX": false}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		for _, part := range []struct {
			key  string
			want string
		}{{"RULE", wantRule}, {"TEST", wantTest}, {"FIX", wantFix}} {
			p1, p2 := part.key+"=", part.key+":"
			if !strings.HasPrefix(upper, p1) && !strings.HasPrefix(upper, p2) {
				continue
			}
			norm := strings.ToLower(strings.Join(strings.Fields(line), " "))
			if strings.Contains(norm, strings.ToLower(part.want)) {
				res[part.key] = true
			}
		}
	}
	return res
}

// ---- measurement -------------------------------------------------------

var sessionSizes = []int{4000, 7000, 10000, 14000, 18000, 24000}
var quickSizes = []int{4000, 7000}
var strategyWindows = []int{8192, 16384, 24576, 32768}

type cellResult struct {
	Strategy        int     `json:"strategy_window"`
	SessionTarget   int     `json:"session_target_tokens"`
	VisibleEst      int     `json:"visible_tokens_est"`
	Truncated       bool    `json:"truncated"`
	PromptEvalSec   float64 `json:"prompt_eval_sec"`
	EvalSec         float64 `json:"eval_sec"`
	WallSec         float64 `json:"wall_sec"`
	PrefillTPS      float64 `json:"prefill_tps"`
	DecodeTPS       float64 `json:"decode_tps"`
	PeakVRAMBytes   uint64  `json:"peak_vram_bytes,omitempty"`
	PeakRAMBytes    uint64  `json:"peak_ram_bytes,omitempty"`
	RuleOK          bool    `json:"rule_ok"`
	TestOK          bool    `json:"test_ok"`
	FixOK           bool    `json:"fix_ok"`
	Complete        bool    `json:"complete"` // all three parts
	Error           string  `json:"error,omitempty"`
	OutputSnippet   string  `json:"output_snippet,omitempty"`
	ConfirmAttempt2 *bool   `json:"confirm_attempt2,omitempty"` // re-run verdict for failed cells
}

type strategyResult struct {
	Window     int                `json:"window"`
	Probe      []*backend.Metrics `json:"perf_probes"`
	Cells      []*cellResult      `json:"cells"`
	ConfigJSON backend.Config     `json:"config"`
}

func main() {
	flag.Parse()
	if *flagModel == "" {
		fmt.Fprintln(os.Stderr, "-model is required")
		os.Exit(2)
	}
	ctx := context.Background()

	model, err := gguf.Inspect(*flagModel)
	must(err)
	hw, err := hardware.Detect(hardware.Options{ModelPath: *flagModel})
	must(err)
	runner := backend.NewLlamaCLI(*flagBin)
	must(runner.Available(ctx))
	engine := verify.NewEngine(runner, *flagModel)

	sizes := sessionSizes
	if *flagQuick {
		sizes = quickSizes
	}
	threads := hw.CPU.Threads()
	if threads <= 0 {
		threads = 1
	}

	// ---- density calibration (one deliberate micro-overflow probe) ------
	//
	// This llama.cpp build prints no per-phase ms/token counts in
	// single-turn mode, but its overflow error reports the EXACT prompt
	// token count ("request (8637 tokens) exceeds ..."). Feed a tiny
	// session to a 512-token window, read the true count from the error,
	// and derive the tokenizer-specific density shared by all strategies.
	probe := buildSession(2000)
	if n := probeOverflowTokens(ctx, runner, *flagModel, threads, model.ExpertBytes > 0); n > 0 {
		if d := float64(len(probe)) / float64(n); d >= 1.6 && d <= 5.5 {
			tokDensity = d
			fmt.Printf("calibration: probe counted %d tokens -> density %.2f chars/token\n", n, tokDensity)
		} else {
			fmt.Printf("calibration: implausible density %.2f ignored; keeping %.2f\n", d, tokDensity)
		}
	} else {
		fmt.Printf("calibration: backend did not report a token count; keeping conservative %.2f\n", tokDensity)
	}

	results := map[string]*strategyResult{}
	for _, win := range strategyWindows {
		cfg := backend.Config{
			GPULayers:      backend.MaxGPULayers,
			ContextTokens:  win,
			KVCacheType:    "f16",
			FlashAttention: true,
			Threads:        threads,
			MMap:           true,
			ExpertsOnCPU:   model.ExpertBytes > 0,
			BatchSize:      2048,
			UBatchSize:     512,
			Seed:           42,
			Temperature:    0,
		}
		sr := &strategyResult{Window: win, ConfigJSON: cfg}

		fmt.Printf("\n=== STRATEGY c=%d ===\n", win)
		for p := 0; p < *flagProbes; p++ {
			m, err := engine.MeasurePerf(ctx, cfg, 10240, 160)
			if err != nil {
				fmt.Printf("  perf probe %d: ERROR %v\n", p+1, err)
				continue
			}
			sr.Probe = append(sr.Probe, m)
			fmt.Printf("  perf probe %d: prefill %.0f t/s decode %.1f t/s vram %.2fGB\n",
				p+1, m.PrefillTPS, m.DecodeTPS, float64(m.PeakVRAMBytes)/(1<<30))
		}

		visible := win - *flagGensz - 512 // generation reserve + safety margin
		for _, sz := range sizes {
			full := buildSession(sz)
			prompt, visEst := truncateToVisible(full, visible)
			cell := runCell(ctx, engine, cfg, sr.Window, sz, visEst, prompt != full, prompt, *flagGensz)
			if cell.Error != "" && strings.Contains(cell.Error, "exceeds the available context") {
				// Estimation residual overflowed despite calibration:
				// shrink harder and retry once so the cell measures
				// capability at this window rather than a harness artifact.
				prompt2, vis2 := truncateToVisible(full, visible*85/100)
				cell = runCell(ctx, engine, cfg, sr.Window, sz, vis2, true, prompt2, *flagGensz)
				cell.Error = strings.TrimSpace(cell.Error + " (after shrink-retry)")
			}
			sr.Cells = append(sr.Cells, cell)
			status := "PASS"
			if !cell.Complete {
				status = "FAIL"
				// Flake discrimination: one confirming re-run of failures.
				retry := runCell(ctx, engine, cfg, sr.Window, sz, visEst, prompt != full, prompt, *flagGensz)
				cell.ConfirmAttempt2 = &retry.Complete
				if retry.Complete {
					status = "FAIL(flaky)"
					cell.RuleOK, cell.TestOK, cell.FixOK = retry.RuleOK, retry.TestOK, retry.FixOK
					cell.Complete = retry.Complete
				}
			}
			fmt.Printf("  session ~%5dk: visible ~%5dk %s -> %s (rule=%v test=%v fix=%v, wall %.1fs)\n",
				sz/1000, visEst/1000, truncTag(cell.Truncated), status,
				cell.RuleOK, cell.TestOK, cell.FixOK, cell.WallSec)
			if !cell.Complete && status == "FAIL" && cell.OutputSnippet != "" {
				fmt.Printf("      output: %q\n", cell.OutputSnippet)
			}
		}
		results[fmt.Sprint(win)] = sr
	}

	writeArtifacts(*flagOut, results)
	renderSummary(results)
}

type rawRun struct {
	err                error
	actualPromptTokens int
	metrics            *backend.Metrics
	output             string
}

var overflowCountRe = regexp.MustCompile(`request \((\d+) tokens\)`)

// probeOverflowTokens runs a deliberately oversized prompt against a tiny
// window and extracts the exact token count from the backend's overflow
// error. Returns 0 when the backend truncates silently instead.
func probeOverflowTokens(ctx context.Context, runner backend.Runner, modelPath string, threads int, expsCPU bool) int {
	cfg := backend.Config{
		GPULayers: backend.MaxGPULayers, ContextTokens: 512,
		KVCacheType: "f16", FlashAttention: true, Threads: threads,
		MMap: true, ExpertsOnCPU: expsCPU,
		BatchSize: 2048, UBatchSize: 512, Seed: 42, Temperature: 0,
	}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	res, err := runner.Run(runCtx, backend.RunSpec{
		ModelPath: modelPath, Config: cfg, Prompt: buildSession(2000),
		MaxTokens: 8, Purpose: "exp06:calibration",
	})
	// The overflow report surfaces inconsistently across builds/modes:
	// sometimes as a run error, sometimes inside stdout output with a nil
	// error, sometimes on stderr. Scan every surface before giving up.
	var surfaces []string
	if err != nil {
		surfaces = append(surfaces, err.Error())
	}
	if res != nil {
		surfaces = append(surfaces, res.Output, res.StderrTail)
	}
	for _, s := range surfaces {
		if m := overflowCountRe.FindStringSubmatch(s); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n
		}
	}
	return 0
}

func runCell(ctx context.Context, engine *verify.Engine, cfg backend.Config,
	win, target, visEst int, truncated bool, prompt string, genTok int) *cellResult {

	spec := backend.RunSpec{
		ModelPath: modelPath(),
		Config:    cfg,
		Prompt:    prompt,
		MaxTokens: genTok,
		Purpose:   fmt.Sprintf("exp06:c%d:s%d", win, target),
	}
	cell := &cellResult{Strategy: win, SessionTarget: target, VisibleEst: visEst, Truncated: truncated}
	start := time.Now()
	res, err := engine.Runner().Run(ctx, spec)
	cell.WallSec = time.Since(start).Seconds()
	// Surface backend overflow reports (they arrive inconsistently via
	// stdout or stderr, with or without a run error) so the caller can
	// shrink-retry instead of misreading a harness artifact as failure.
	if res != nil {
		for _, s := range []string{res.StderrTail, res.Output} {
			if m := overflowCountRe.FindStringSubmatch(s); m != nil {
				cell.Error = "context overflow: request was " + m[1] + " tokens"
				break
			}
		}
	}
	if err != nil && cell.Error == "" {
		cell.Error = err.Error()
	}
	if res != nil {
		cell.PeakVRAMBytes = res.Metrics.PeakVRAMBytes
		cell.PeakRAMBytes = res.Metrics.PeakRAMBytes
	}
	if cell.Error != "" {
		return cell
	}
	m := res.Metrics
	cell.PrefillTPS, cell.DecodeTPS = m.PrefillTPS, m.DecodeTPS
	cell.PromptEvalSec = m.PromptEvalMs / 1000
	cell.EvalSec = 0
	if m.DecodeTPS > 0 {
		cell.EvalSec = float64(genTok) / m.DecodeTPS
	}
	cell.PeakVRAMBytes, cell.PeakRAMBytes = m.PeakVRAMBytes, m.PeakRAMBytes
	g := grade(res.Output)
	cell.RuleOK, cell.TestOK, cell.FixOK = g["RULE"], g["TEST"], g["FIX"]
	cell.Complete = g["RULE"] && g["TEST"] && g["FIX"]
	cell.OutputSnippet = snippet(res.Output)
	return cell
}

func truncTag(b bool) string {
	if b {
		return "TRUNC"
	}
	return "full "
}

func snippet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		return s[len(s)-160:]
	}
	return s
}

func writeArtifacts(outDir string, results map[string]*strategyResult) {
	must(os.MkdirAll(outDir, 0o755))
	windows := make([]string, 0, len(results))
	for w := range results {
		windows = append(windows, w)
	}
	sort.Strings(windows)
	ordered := map[string]*strategyResult{}
	for _, w := range windows {
		ordered[w] = results[w]
	}
	b, err := json.MarshalIndent(struct {
		GeneratedAt time.Time                  `json:"generated_at"`
		TokDensity  float64                    `json:"chars_per_token_calibrated"`
		Strategies  map[string]*strategyResult `json:"strategies"`
	}{time.Now().UTC(), tokDensity, ordered}, "", "  ")
	must(err)
	path := filepath.Join(outDir, "exp06-results.json")
	must(os.WriteFile(path, b, 0o644))
	fmt.Printf("\nartifacts: %s\n", path)
}

func renderSummary(results map[string]*strategyResult) {
	fmt.Println("\n=== COMPLETION MATRIX (full-task = all three parts) ===")
	fmt.Printf("%-8s", "ctx")
	for _, sz := range sessionSizes {
		fmt.Printf(" %6dk", sz/1000)
	}
	fmt.Println()
	for _, w := range strategyWindows {
		sr := results[fmt.Sprint(w)]
		if sr == nil {
			continue
		}
		fmt.Printf("%-8d", w)
		for _, sz := range sessionSizes {
			found := false
			for _, c := range sr.Cells {
				if c.SessionTarget == sz {
					mark := "."
					switch {
					case c.Complete:
						mark = "P"
					case c.ConfirmAttempt2 != nil && *c.ConfirmAttempt2:
						mark = "f"
					}
					fmt.Printf(" %6s", mark)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf(" %6s", "-")
			}
		}
		fmt.Println()
	}
	fmt.Println("\nP=complete  f=failed once then passed on confirm  .=failed twice  -=not run")
}

func modelPath() string { return *flagModel }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "exp06:", err)
		os.Exit(1)
	}
}
