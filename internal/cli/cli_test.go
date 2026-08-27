package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EffNine/gumi/internal/gguf"
	"github.com/EffNine/gumi/internal/hardware"
	"github.com/EffNine/gumi/internal/testgguf"
)

// runCLI executes the real command dispatcher with patched args/stdout and
// returns captured stdout. It fails the test when os.Exit is triggered.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	oldArgs := os.Args
	oldStdout := os.Stdout
	defer func() { os.Args = oldArgs; os.Stdout = oldStdout }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	os.Args = append([]string{"gumi"}, args...)

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	exited := make(chan int, 1)
	origExit := osExit
	osExit = func(code int) { exited <- code }
	defer func() { osExit = origExit }()

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		Execute()
	}()

	select {
	case code := <-exited:
		t.Fatalf("unexpected os.Exit(%d) for args %v", code, args)
	case <-finished:
	}

	w.Close()
	return <-done
}

var _ = filepath.Join

func fixtureModel(t *testing.T) string {
	t.Helper()
	b := testgguf.New("qwen3moe").Arch().
		Geometry(48, 40960, 2048, 16, 4).
		Rope(1000000).
		MoE(128, 8, 768).
		FileType(15)
	b.Tensor("token_embd.weight", []uint64{2048, 151936}, 12)
	b.Tensor("blk.0.attn_q.weight", []uint64{2048, 2048}, 1)
	b.Tensor("blk.0.ffn_gate_exps.weight", []uint64{768, 2048, 128}, 12)
	return b.WriteFile(t)
}

func TestVersionCommand(t *testing.T) {
	out := runCLI(t, "version")
	if !strings.Contains(out, "gumi v") {
		t.Errorf("version output = %q", out)
	}
}

func TestInspectHumanOutput(t *testing.T) {
	model := fixtureModel(t)
	out := runCLI(t, "inspect", model)
	for _, want := range []string{
		"Architecture: qwen3moe",
		"Quantization: Q4_K_M",
		"Layers:       48",
		"MoE experts:  128 total, 8 active",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inspect output missing %q:\n%s", want, out)
		}
	}
}

func TestInspectJSONRoundTrip(t *testing.T) {
	model := fixtureModel(t)
	out := runCLI(t, "inspect", "--json", model)
	var info gguf.ModelInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if info.Architecture != "qwen3moe" || info.LayerCount != 48 || info.QuantLabel != "Q4_K_M" {
		t.Errorf("round-trip mismatch: %+v", info)
	}
	if info.MoE == nil || info.MoE.TotalExperts != 128 {
		t.Errorf("MoE missing in JSON: %+v", info.MoE)
	}
}

func TestProbeJSON(t *testing.T) {
	out := runCLI(t, "probe", "--json")
	var hw hardware.Info
	if err := json.Unmarshal([]byte(out), &hw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if hw.OS == "" || hw.Arch == "" {
		t.Errorf("probe must report OS/arch: %+v", hw)
	}
}

func TestProfilesCommand(t *testing.T) {
	out := runCLI(t, "profiles")
	if !strings.Contains(out, "agentic_coding") || !strings.Contains(out, "chat") {
		t.Errorf("profiles output incomplete:\n%s", out)
	}
}

func TestOptimizeDryRunEndToEnd(t *testing.T) {
	model := fixtureModel(t)
	outDir := t.TempDir()

	out := runCLI(t, "optimize", model,
		"--workload", "agentic_coding",
		"--dry-run",
		"--out", outDir)

	if !strings.Contains(out, "GUMI OPTIMIZATION REPORT") {
		t.Errorf("stdout missing report:\n%s", out)
	}
	for _, name := range []string{"report.md", "report.json", "candidates.json", "hardware.json"} {
		full := filepath.Join(outDir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("missing artifact %s: %v", name, err)
			continue
		}
		if name == "report.md" && !strings.Contains(string(data), "## Hardware") {
			t.Error("report.md missing hardware section")
		}
	}
}
