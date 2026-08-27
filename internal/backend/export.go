package backend

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExportBlock holds ready-to-use launch configurations for every supported
// target. These are static transforms of a Config — no backend required.
type ExportBlock struct {
	LlamaCLI    string `json:"llama_cpp_cli"`
	LlamaServer string `json:"llama_cpp_server"`
	LMStudio    string `json:"lm_studio"`
	Ollama      string `json:"ollama"`
}

// RenderExports renders all export targets for a model + config.
func RenderExports(modelPath string, c Config) ExportBlock {
	return ExportBlock{
		LlamaCLI:    renderLlamaCLI(modelPath, c),
		LlamaServer: renderLlamaServer(modelPath, c),
		LMStudio:    renderLMStudio(c),
		Ollama:      renderOllama(modelPath, c),
	}
}

func commonFlags(c Config) []string {
	var f []string
	add := func(format string, args ...any) {
		f = append(f, fmt.Sprintf(format, args...))
	}
	add("-c %d", c.ContextTokens)
	if c.GPULayers > 0 {
		if c.GPULayers >= MaxGPULayers {
			f = append(f, "-ngl 999")
		} else {
			add("-ngl %d", c.GPULayers)
		}
	}
	switch c.KVCacheType {
	case "q8_0", "q4_0":
		add("-ctk %s -ctv %s", c.KVCacheType, c.KVCacheType)
	}
	if c.FlashAttention {
		f = append(f, "-fa on")
	}
	if c.BatchSize > 0 {
		add("-b %d", c.BatchSize)
	}
	if c.UBatchSize > 0 {
		add("-ub %d", c.UBatchSize)
	}
	if c.Threads > 0 {
		add("-t %d", c.Threads)
	}
	if !c.MMap {
		f = append(f, "--no-mmap")
	}
	if c.MLock {
		f = append(f, "--mlock")
	}
	if c.ExpertsOnCPU {
		f = append(f, "-ot exps=CPU")
	}
	return f
}

func renderLlamaCLI(modelPath string, c Config) string {
	flags := append([]string{"-m " + shellQuote(modelPath)}, commonFlags(c)...)
	return "llama-cli " + strings.Join(flags, " ")
}

func renderLlamaServer(modelPath string, c Config) string {
	flags := append([]string{
		"-m " + shellQuote(modelPath),
		"--host 127.0.0.1 --port 8080",
	}, commonFlags(c)...)
	return "llama-server " + strings.Join(flags, " ")
}

// lmStudioSettings is the load-configuration payload accepted by LM Studio's
// REST API (POST /api/v1/models/load). KV quantization is not exposed by that
// API today; when set it is noted so users can apply it in the UI.
type lmStudioSettings struct {
	Model             string `json:"model"`
	ContextLength     int    `json:"context_length"`
	GPUOffload        any    `json:"gpu_offload"` // "max" | int layers
	FlashAttention    bool   `json:"flash_attention"`
	OffloadKVCacheGPU bool   `json:"offload_kv_cache_to_gpu"`
	NumThreads        int    `json:"num_threads,omitempty"`
	KVQuantNote       string `json:"kv_quant_note,omitempty"`
}

func renderLMStudio(c Config) string {
	s := lmStudioSettings{
		ContextLength:     c.ContextTokens,
		FlashAttention:    c.FlashAttention,
		OffloadKVCacheGPU: true,
	}
	if c.GPULayers >= MaxGPULayers {
		s.GPUOffload = "max"
	} else if c.GPULayers > 0 {
		s.GPUOffload = c.GPULayers
	} else {
		s.GPUOffload = 0
	}
	if c.Threads > 0 {
		s.NumThreads = c.Threads
	}
	if c.KVCacheType != "" && c.KVCacheType != "f16" {
		s.KVQuantNote = "set KV cache quantization to " + strings.ToUpper(c.KVCacheType) +
			" in the LM Studio model loader UI (not exposed via API)"
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return string(b)
}

func renderOllama(modelPath string, c Config) string {
	var b strings.Builder
	b.WriteString("# Gumi optimized Modelfile\n")
	fmt.Fprintf(&b, "FROM %s\n\n", modelPath)
	fmt.Fprintf(&b, "PARAMETER num_ctx %d\n", c.ContextTokens)
	if c.GPULayers > 0 {
		fmt.Fprintf(&b, "PARAMETER num_gpu %d\n", c.EffectiveGPULayers())
	} else {
		b.WriteString("PARAMETER num_gpu 0\n")
	}
	if c.Threads > 0 {
		fmt.Fprintf(&b, "PARAMETER num_thread %d\n", c.Threads)
	}
	if c.KVCacheType != "" && c.KVCacheType != "f16" {
		b.WriteString("# note: ollama does not expose KV cache quantization; KV settings apply to llama.cpp only\n")
	}
	return b.String()
}

func shellQuote(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\"'\\$") {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return s
}
