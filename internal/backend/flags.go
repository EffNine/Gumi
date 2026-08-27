package backend

import (
	"fmt"
	"strconv"
	"strings"
)

// faStyle describes how the llama-cli binary expects flash attention flags.
type faStyle int

const (
	faUnknown faStyle = iota
	faValue           // -fa on|off (2025+ builds)
	faFlag            // bare -fa boolean flag (legacy builds)
)

// convStyle describes how the build disables interactive conversation mode.
// Recent builds accept -no-cnv but still enter the conversation loop for
// chat-template models (spinning on stdin EOF), while --single-turn exits
// cleanly after one generation; legacy builds only know -no-cnv.
type convStyle int

const (
	convUnknown    convStyle = iota
	convSingleTurn           // --single-turn / -st
	convNoCNV                // -no-cnv (legacy)
	convNone                 // no known disable flag
)

// quirks captures version-specific CLI behavior discovered at runtime.
type quirks struct {
	fa          faStyle
	conv        convStyle
	helpChecked bool
}

// buildArgs renders llama-cli arguments for a RunSpec.
func buildArgs(spec RunSpec, q quirks) []string {
	c := spec.Config
	args := []string{
		"-m", spec.ModelPath,
		"-p", spec.Prompt,
		"-n", strconv.Itoa(spec.MaxTokens),
		"--seed", strconv.FormatInt(c.Seed, 10),
		"--temp", strconv.FormatFloat(c.Temperature, 'f', -1, 64),
	}
	if c.ContextTokens > 0 {
		args = append(args, "-c", strconv.Itoa(c.ContextTokens))
	}
	if c.GPULayers > 0 {
		args = append(args, "-ngl", strconv.Itoa(c.EffectiveGPULayers()))
	}
	if c.Threads > 0 {
		args = append(args, "-t", strconv.Itoa(c.Threads))
	}
	if c.BatchSize > 0 {
		args = append(args, "-b", strconv.Itoa(c.BatchSize))
	}
	if c.UBatchSize > 0 {
		args = append(args, "-ub", strconv.Itoa(c.UBatchSize))
	}
	switch q.fa {
	case faValue:
		if c.FlashAttention {
			args = append(args, "-fa", "on")
		} else {
			args = append(args, "-fa", "off")
		}
	case faFlag:
		if c.FlashAttention {
			args = append(args, "-fa")
		}
	default:
		// Unknown style: omit and let the retry chain handle failures.
	}
	if c.KVCacheType != "" && c.KVCacheType != "f16" {
		args = append(args, "-ctk", c.KVCacheType, "-ctv", c.KVCacheType)
	}
	if !c.MMap {
		args = append(args, "--no-mmap")
	}
	if c.MLock {
		args = append(args, "--mlock")
	}
	if c.ExpertsOnCPU {
		args = append(args, "-ot", "exps=CPU")
	}
	switch q.conv {
	case convSingleTurn:
		args = append(args, "-st")
	case convNoCNV:
		args = append(args, "-no-cnv")
	case convNone:
		// Nothing to add; rely on stdin EOF terminating the run.
	}
	return args
}

// unknownArgMarkers are stderr fragments indicating a rejected flag.
var unknownArgMarkers = []string{
	"error: invalid argument",
	"unknown argument",
	"unrecognized argument",
	"invalid argument:",
}

func looksLikeUnknownArg(stderr string) bool {
	l := strings.ToLower(stderr)
	for _, m := range unknownArgMarkers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

// oomMarkers are stderr fragments indicating allocation failure.
var oomMarkers = []string{
	"out of memory",
	"failed to allocate",
	"ggml_backend_alloc",
	"cuda_error_out_of_memory",
	"cannot allocate memory",
	"std::bad_alloc",
	"out_of_memory",
}

// ClassifyError maps stderr text to a typed failure reason.
func classifyError(errText string) error {
	l := strings.ToLower(errText)
	for _, m := range oomMarkers {
		if strings.Contains(l, m) {
			return fmt.Errorf("%w: %s", ErrOutOfMemory, firstLine(errText))
		}
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
