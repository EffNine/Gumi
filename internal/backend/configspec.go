package backend

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseConfigSpec parses an execution-only configuration spec used to admit
// human ("current") configurations into verification runs:
//
//	ngl=33,c=8192,kv=q8_0,fa,b=512,ub=128,exps-cpu
//
// Keys (all optional):
//
//	ngl       int | "max"        GPU layers (-ngl)
//	c         int                context tokens
//	kv        f16|q8_0|q4_0      KV cache precision
//	fa|no-fa  flag               flash attention
//	b, ub     int                batch / ubatch
//	exps-cpu | gpu-exps          MoE expert placement
//	mmap | no-mmap               mmap weights (default on)
//	mlock   flag                 lock weights in RAM
//	t       int                  threads
//
// Sampling knobs (temperature, top-p/k, seeds) are deliberately absent:
// paired verification always runs temperature 0 with the shared fixed seed,
// and this parser must never become a sampler-tuning side door.
func ParseConfigSpec(spec string) (Config, error) {
	cfg := Config{MMap: true}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return cfg, fmt.Errorf("empty config spec")
	}
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, val, hasVal := strings.Cut(field, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)

		boolKey := func(v bool) error { //nolint:unparam
			if hasVal {
				return fmt.Errorf("key %q takes no value", key)
			}
			return nil
		}
		switch key {
		case "ngl":
			if val == "max" || val == "999" {
				cfg.GPULayers = MaxGPULayers
				continue
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				return cfg, fmt.Errorf("ngl: expected non-negative int or \"max\", got %q", val)
			}
			cfg.GPULayers = n
		case "c":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return cfg, fmt.Errorf("c: expected positive int, got %q", val)
			}
			cfg.ContextTokens = n
		case "kv":
			switch strings.ToLower(val) {
			case "f16", "q8_0", "q4_0":
				cfg.KVCacheType = strings.ToLower(val)
			default:
				return cfg, fmt.Errorf("kv: supported precisions are f16, q8_0, q4_0; got %q", val)
			}
		case "fa":
			if err := boolKey(true); err != nil {
				return cfg, err
			}
			cfg.FlashAttention = true
		case "no-fa":
			if err := boolKey(false); err != nil {
				return cfg, err
			}
			cfg.FlashAttention = false
		case "b":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return cfg, fmt.Errorf("b: expected positive int, got %q", val)
			}
			cfg.BatchSize = n
		case "ub":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return cfg, fmt.Errorf("ub: expected positive int, got %q", val)
			}
			cfg.UBatchSize = n
		case "exps-cpu":
			if err := boolKey(true); err != nil {
				return cfg, err
			}
			cfg.ExpertsOnCPU = true
		case "gpu-exps":
			if err := boolKey(false); err != nil {
				return cfg, err
			}
			cfg.ExpertsOnCPU = false
		case "mmap":
			if err := boolKey(true); err != nil {
				return cfg, err
			}
			cfg.MMap = true
		case "no-mmap":
			if err := boolKey(false); err != nil {
				return cfg, err
			}
			cfg.MMap = false
		case "mlock":
			if err := boolKey(true); err != nil {
				return cfg, err
			}
			cfg.MLock = true
		case "t":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return cfg, fmt.Errorf("t: expected positive int, got %q", val)
			}
			cfg.Threads = n
		default:
			return cfg, fmt.Errorf("unknown key %q in config spec %q", key, spec)
		}
	}
	return cfg, nil
}
