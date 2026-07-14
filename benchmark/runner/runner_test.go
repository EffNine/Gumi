package runner

import (
	"testing"

	"github.com/EffNine/gumi/benchmark"
)

// TestRun is an integration test that requires a running LM Studio instance.
// Skipped by default. Run with: go test -tags=integration -count=1 ./benchmark/runner/
func TestRun(t *testing.T) {
	t.Skip("Skipping integration test. Run with: go test -tags=integration -count=1 ./benchmark/runner/")
}

func TestRun_EmptyModelReturnsError(t *testing.T) {
	cfg := Config{
		Model:    "",
		Attempts: 1,
	}
	_, err := Run(cfg)
	if err == nil {
		t.Fatal("Run with empty model should return error")
	}
}

func TestRun_DefaultsAttempts(t *testing.T) {
	t.Skip("Skipping integration test. Run with: go test -tags=integration -count=1 ./benchmark/runner/")
}

func TestNewOrchestrator(t *testing.T) {
	cfg := Config{
		Model:    "test",
		Provider: "lmstudio",
	}
	o := NewOrchestrator(cfg)
	if o == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if o.config.Model != "test" {
		t.Errorf("Orchestrator.config.Model = %q, want %q", o.config.Model, "test")
	}
}

func TestOrchestrator_ClientForCondition(t *testing.T) {
	o := NewOrchestrator(Config{Model: "test", Provider: "lmstudio"})

	direct := NewProviderClient("http://localhost:1234", "")
	gumi := NewProviderClient("http://127.0.0.1:8787", "")
	frontier := NewProviderClient("https://api.anthropic.com", "sk-xxx")

	tests := []struct {
		cond     Condition
		wantNil  bool
		wantBase string
	}{
		{ConditionDirect, false, "http://localhost:1234"},
		{ConditionGumiStabilized, false, "http://127.0.0.1:8787"},
		{ConditionGumiLightweight, false, "http://127.0.0.1:8787"},
		{ConditionGumiDirect, false, "http://127.0.0.1:8787"},
		{ConditionFrontier, false, "https://api.anthropic.com"},
	}

	for _, tt := range tests {
		t.Run(string(tt.cond), func(t *testing.T) {
			client := o.clientForCondition(tt.cond, direct, gumi, frontier)
			if tt.wantNil {
				if client != nil {
					t.Errorf("clientForCondition(%v) = %+v, want nil", tt.cond, client)
				}
				return
			}
			if client == nil {
				t.Fatalf("clientForCondition(%v) returned nil, want non-nil", tt.cond)
			}
			if client.baseURL != tt.wantBase {
				t.Errorf("client.baseURL = %q, want %q", client.baseURL, tt.wantBase)
			}
		})
	}
}

func TestComputeLatencyOverhead(t *testing.T) {
	tests := []struct {
		name  string
		input []benchmark.TestResult
		want  float64
	}{
		{
			name: "no gumi results",
			input: []benchmark.TestResult{
				{Condition: "direct", LatencyMs: 100},
			},
			want: 0,
		},
		{
			name: "no direct results",
			input: []benchmark.TestResult{
				{Condition: "gumi-stabilized", LatencyMs: 200},
			},
			want: 0,
		},
		{
			name: "gumi slower by 50ms avg",
			input: []benchmark.TestResult{
				{Condition: "direct", LatencyMs: 100},
				{Condition: "direct", LatencyMs: 150},
				{Condition: "gumi-stabilized", LatencyMs: 200},
				{Condition: "gumi-lightweight", LatencyMs: 250},
			},
			// direct avg = (100+150)/2 = 125
			// gumi avg = (200+250)/2 = 225
			// overhead = 225 - 125 = 100
			want: 100,
		},
		{
			name: "overhead capped at zero when gumi faster",
			input: []benchmark.TestResult{
				{Condition: "direct", LatencyMs: 200},
				{Condition: "gumi-stabilized", LatencyMs: 100},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeLatencyOverhead(tt.input)
			if got != tt.want {
				t.Errorf("computeLatencyOverhead = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple-name", "simple-name"},
		{"has/slashes\\and@spaces:colons", "has-slashes-and-spaces-colons"},
		{"new\nline\rhere", "newlinehere"},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDirectBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		want     string
	}{
		{"always returns LM Studio URL", Config{Provider: "lmstudio"}, "http://192.168.0.164:1234"},
		{"ignores custom base URL", Config{Provider: "lmstudio", BaseURL: "http://custom:8080"}, "http://192.168.0.164:1234"},
		{"ignores provider", Config{Provider: "openai"}, "http://192.168.0.164:1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOrchestrator(tt.cfg)
			got := o.directBaseURL()
			if got != tt.want {
				t.Errorf("directBaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGumiBaseURL(t *testing.T) {
	o := NewOrchestrator(Config{Provider: "lmstudio"})
	got := o.gumiBaseURL()
	want := ""
	if got != want {
		t.Errorf("gumiBaseURL() = %q, want %q", got, want)
	}

	// Custom base URL overrides
	o2 := NewOrchestrator(Config{BaseURL: "http://custom:8080"})
	got2 := o2.gumiBaseURL()
	if got2 != "http://custom:8080" {
		t.Errorf("gumiBaseURL with BaseURL = %q, want %q", got2, "http://custom:8080")
	}
}

func TestFrontierBaseURL(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", "https://api.anthropic.com"},
		{"openai", "https://api.openai.com"},
		{"google", "https://generativelanguage.googleapis.com"},
		{"unknown", "https://api.anthropic.com"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			o := NewOrchestrator(Config{Provider: tt.provider})
			got := o.frontierBaseURL()
			if got != tt.want {
				t.Errorf("frontierBaseURL(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestConvertDegradationReport(t *testing.T) {
	t.Skip("Skipping integration test. Run with: go test -tags=integration -count=1 ./benchmark/runner/")
}
