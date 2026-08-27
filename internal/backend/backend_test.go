package backend

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBuildArgsModern(t *testing.T) {
	q := quirks{fa: faValue, conv: convSingleTurn}
	spec := RunSpec{
		ModelPath: "/models/test.gguf",
		Prompt:    "hello",
		MaxTokens: 128,
		Config: Config{
			GPULayers: MaxGPULayers, ContextTokens: 16384,
			KVCacheType: "q8_0", FlashAttention: true,
			BatchSize: 1024, UBatchSize: 512, Threads: 8,
			MMap: false, MLock: true, ExpertsOnCPU: true,
			Seed: 42, Temperature: 0,
		},
	}
	args := buildArgs(spec, q)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-m /models/test.gguf", "-p hello", "-n 128",
		"-c 16384", "-ngl 999", "-t 8", "-b 1024", "-ub 512",
		"-fa on", "-ctk q8_0 -ctv q8_0", "--no-mmap", "--mlock",
		"-ot exps=CPU", "--seed 42", "--temp 0", "-st",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q in %q", want, joined)
		}
	}
}

func TestBuildArgsLegacyFA(t *testing.T) {
	q := quirks{fa: faFlag, conv: convNoCNV}
	spec := RunSpec{
		ModelPath: "m.gguf", Prompt: "x", MaxTokens: 8,
		Config: Config{FlashAttention: false, KVCacheType: "f16"},
	}
	args := buildArgs(spec, q)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "-fa") {
		t.Errorf("legacy off must omit -fa entirely: %q", joined)
	}
	spec.Config.FlashAttention = true
	if got := buildArgs(spec, q); !containsExact(got, "-fa") {
		t.Errorf("legacy on must include bare -fa: %v", got)
	}
}

func containsExact(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestParseTimingsNewFormat(t *testing.T) {
	stderr := `
load_tensors: loading model tensors
prompt eval time:   12345.67 ms /   256 tokens (   48.21 tokens per second)
eval time:   21000.00 ms /   512 tokens (   24.38 tokens per second)
total time:   33345.67 ms
`
	prefill, decode, promptMs, ok := ParseTimings(stderr)
	if !ok {
		t.Fatal("timings not found")
	}
	if prefill != 48.21 || decode != 24.38 || promptMs != 12345.67 {
		t.Errorf("prefill=%v decode=%v promptMs=%v", prefill, decode, promptMs)
	}
}

func TestParseTimingsOldFormat(t *testing.T) {
	stderr := "prompt eval time =    88.12 ms /     8 runs   (    10.00 tokens per second)\n" +
		"eval time =        2100.00 ms /    20 runs   (     5.71 tokens per second)"
	prefill, decode, _, ok := ParseTimings(stderr)
	if !ok || prefill != 10.0 || decode != 5.71 {
		t.Errorf("old format parse: prefill=%v decode=%v ok=%v", prefill, decode, ok)
	}
}

func TestParseTimingsCompactFormat(t *testing.T) {
	out := "some init noise\n[ Prompt: 52.5 t/s | Generation: 30.7 t/s ]\nExiting..."
	prefill, decode, promptMs, ok := ParseTimings(out)
	if !ok || prefill != 52.5 || decode != 30.7 || promptMs != 0 {
		t.Errorf("compact parse: prefill=%v decode=%v promptMs=%v ok=%v", prefill, decode, promptMs, ok)
	}
}

func TestParseTimingsAbsent(t *testing.T) {
	if _, _, _, ok := ParseTimings("nothing useful here"); ok {
		t.Error("expected ok=false")
	}
}

func TestClassifyOOM(t *testing.T) {
	if err := classifyError("ggml_backend_cuda_buffer::alloc_buffer failed to allocate 100 MB"); err == nil {
		t.Error("OOM not detected")
	} else if !errors.Is(err, ErrOutOfMemory) {
		t.Errorf("wrong error type: %v", err)
	}
	if classifyError("all good") != nil {
		t.Error("false positive OOM")
	}
}

func TestCleanOutputPreservesLines(t *testing.T) {
	prompt := "List three colors:"
	stdout := prompt + "\n- red\n- green\n- blue\n"
	got := CleanOutput(stdout, prompt)
	want := "- red\n- green\n- blue"
	if got != want {
		t.Errorf("CleanOutput = %q, want %q", got, want)
	}
	// No echo present: output returned as-is.
	if got := CleanOutput("- red\n- blue", "not-in-output"); got != "- red\n- blue" {
		t.Errorf("fallback path broken: %q", got)
	}
}

func TestDetectFAStyle(t *testing.T) {
	modern := "-fa, --flash-attn [on|off|auto] whether to use FlashAttention"
	legacy := "-fa, --flash-attn           use FlashAttention during inference"
	if detectFAStyle(modern) != faValue {
		t.Error("modern help must map to faValue")
	}
	if detectFAStyle(legacy) != faFlag {
		t.Error("legacy help must map to faFlag")
	}
}

type stubRunner struct{ name string }

func (s *stubRunner) Name() string                                  { return s.name }
func (s *stubRunner) Available(context.Context) error               { return nil }
func (s *stubRunner) Run(context.Context, RunSpec) (*Result, error) { return nil, nil }

func TestRunnerInterfaceSatisfied(t *testing.T) {
	var r Runner = &stubRunner{name: "stub"}
	if r.Name() != "stub" {
		t.Fail()
	}
	var _ = LlamaCLI{} // compile-time check that LlamaCLI exists
}

func TestCleanOutputStripsThinkingSpan(t *testing.T) {
	raw := "[Start thinking]\n\nOkay, the user wants json.\n[End thinking]\n{\"status\":\"ok\"}"
	got := CleanOutput(raw, "Return a JSON object with key status ok.")
	if got != `{"status":"ok"}` {
		t.Errorf("thinking span not stripped: %q", got)
	}
	// Unterminated thinking: no answer produced -> empty output -> fail.
	if got := CleanOutput("[Start thinking]\nhalf a thought", "p"); got != "" {
		t.Errorf("unterminated thinking should empty output, got %q", got)
	}
}

// Regression: real b10360 stdout anatomy — spinner with backspaces, UTF-8
// banner, "> "-prefixed prompt echo, thinking block, perf summary.
func TestCleanOutputRealBuildAnatomy(t *testing.T) {
	raw := "\n\nLoading model... |\b-\b\\\b|\b/^H-^H\b|\n" +
		"\n\xE2\x94\x82 banner \xE2\x94\x82\n\n" +
		"> Compute 47*83. Reply with only the number.\n" +
		"[Start thinking]\n\nOkay, let's see. 47*83...\n[End thinking]\n\n3901\n\n" +
		"[ Prompt: 59.1 t/s | Generation: 32.7 t/s ]\n\nExiting..."
	got := CleanOutput(raw, "Compute 47*83. Reply with only the number.")
	if got != "3901" {
		t.Errorf("CleanOutput = %q, want %q", got, "3901")
	}
}

// Banner-only capture (no prompt echo, empty generation) must not be
// returned as model output.
func TestCleanOutputBannerOnly(t *testing.T) {
	raw := "Loading model... |\b-\b\\\b|\n\xE2\x96\x84\xE2\x96\x84 \xE2\x96\x84\xE2\x96\x84\n\xE2\x96\x88\xE2\x96\x88 \xE2\x96\x88\xE2\x96\x88\n"
	if got := CleanOutput(raw, "long retrieval haystack prompt"); got != "" {
		t.Errorf("banner-only output should clean to empty, got %q", got)
	}
}

// Chat templates that render turn scaffolding into stdout must not pollute
// extracted answers (DeepSeek-style "User:/Assistant:" plumbing).
func TestCleanOutputStripsTurnMarkers(t *testing.T) {
	raw := "User:\n\nAssistant:\n\nThe JSON object is:\n\n{\"status\":\"ok\"}\n\nAssistant:\n\n"
	got := CleanOutput(raw, `Return a JSON object with the key status set to ok.`)
	if got != "The JSON object is:\n\n{\"status\":\"ok\"}" {
		t.Errorf("turn markers not stripped cleanly: %q", got)
	}
}
