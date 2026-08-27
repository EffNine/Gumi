// Package backend defines inference backends that verify candidate
// configurations. Gumi treats backends as measurement instruments: the same
// prompts, seeds, and sampling must be reproducible across candidates.
//
// MVP ships a llama.cpp (llama-cli) subprocess runner plus static export
// renderers for LM Studio and Ollama. API-driven integrations plug into the
// Runner interface later.
package backend

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors surfaced by runners.
var (
	ErrOutOfMemory  = errors.New("backend ran out of memory")
	ErrTimedOut     = errors.New("backend run timed out")
	ErrLoadFailed   = errors.New("backend failed to load model")
	ErrUnsupported  = errors.New("backend unsupported in this build")
	ErrNotAvailable = errors.New("backend binary not available")
)

// Config is one inference configuration under test. Every knob is in the
// SAFE-AUTO set from the product spec; Gumi never changes weights, active
// expert count, RoPE scaling, system prompts, or reasoning behavior.
type Config struct {
	GPULayers      int     // -ngl; >= MaxGPULayers sentinel means "maximum"
	ContextTokens  int     // -c
	KVCacheType    string  // f16 | q8_0 | q4_0 (applies to K and V)
	FlashAttention bool    // -fa
	BatchSize      int     // -b
	UBatchSize     int     // -ub
	Threads        int     // -t
	MMap           bool    // mmap enabled (default true)
	MLock          bool    // --mlock
	ExpertsOnCPU   bool    // -ot "exps=CPU" (MoE expert placement)
	Seed           int64   // fixed for reproducible paired verification
	Temperature    float64 // 0 => greedy decoding
}

// MaxGPULayers is the sentinel meaning "offload everything possible".
const MaxGPULayers = 999

// EffectiveGPULayers clamps the max sentinel to what llama.cpp accepts.
func (c Config) EffectiveGPULayers() int {
	if c.GPULayers >= MaxGPULayers {
		return MaxGPULayers
	}
	return c.GPULayers
}

// RunSpec describes a single verification run.
type RunSpec struct {
	ModelPath string
	Config    Config
	Prompt    string
	MaxTokens int
	Purpose   string // free-form label recorded in logs, e.g. "perf", "task:math_mult"
}

// Metrics holds performance measurements from one run.
type Metrics struct {
	PrefillTPS    float64       `json:"prefill_tps"`
	DecodeTPS     float64       `json:"decode_tps"`
	PromptEvalMs  float64       `json:"prompt_eval_ms"`
	PeakVRAMBytes uint64        `json:"peak_vram_bytes,omitempty"` // 0 = unknown
	PeakRAMBytes  uint64        `json:"peak_ram_bytes,omitempty"`  // 0 = unknown
	Duration      time.Duration `json:"-"`
}

// Result is the outcome of one run.
type Result struct {
	Output     string  // generated text (cleaned)
	Metrics    Metrics `json:"metrics"`
	StderrTail string  `json:"stderr_tail,omitempty"` // diagnostics for reports/logs
}

// Runner abstracts an inference backend used for verification.
type Runner interface {
	Name() string
	Available(ctx context.Context) error
	Run(ctx context.Context, spec RunSpec) (*Result, error)
}
