package backend

import (
	"sort"
	"strings"
)

// TunableKVTypes lists the KV cache precisions Gumi V1 explores, ordered by
// descending fidelity. The list is deliberately small: every entry must be
// gated by the capability battery, and wider exploration multiplies tuning
// cost without identified upside.
var TunableKVTypes = []string{"q8_0", "q4_0"}

// Capabilities describes what the installed llama.cpp build supports, as
// discovered by probing its --help output at startup. Gumi never assumes a
// feature exists: unsupported parameters are suppressed (with the reason
// recorded) and tuning continues on the remaining dimensions.
//
// A zero-value Capabilities means "unknown" — callers treat unknown as
// permissive-with-retry rather than restrictive, because the legacy-flag
// retry chain remains the final arbiter of what a build accepts.
type Capabilities struct {
	Discovered     bool     // false when the help probe produced nothing usable
	FlashAttention bool     // -fa / --flash-attn present
	FAValueStyle   bool     // expects "on|off|auto" values (2025+ builds)
	KVTypes        []string // cache types this build accepts (parsed choices)
	GPULayers      bool     // -ngl present
	Batch          bool     // -b/--batch-size present
	UBatch         bool     // -ub/--ubatch-size present
	MMap           bool     // --no-mmap/--mmap family present
	MLock          bool     // --mlock present
	OverrideTensor bool     // -ot/--override-tensor present (expert placement)
	SingleTurn     bool     // --single-turn present (clean one-shot runs)
}

// SupportedKV reports whether the build accepts `kv` as a K/V cache type.
// f16 is always accepted (it is the default). When discovery failed, every
// tunable type counts as supported so the retry chain can prove otherwise.
func (c Capabilities) SupportedKV(kv string) bool {
	if kv == "" || kv == "f16" || !c.Discovered {
		return true
	}
	for _, t := range c.KVTypes {
		if t == kv {
			return true
		}
	}
	return false
}

// SupportedKVTunables returns the subset of Gumi's tunable KV types this
// backend accepts, fidelity-descending.
func (c Capabilities) SupportedKVTunables() []string {
	var out []string
	for _, t := range TunableKVTypes {
		if c.SupportedKV(t) {
			out = append(out, t)
		}
	}
	return out
}

// ParseCapabilities extracts backend capabilities from llama-cli --help
// output. Pure function: same help text, same capabilities.
func ParseCapabilities(help string) Capabilities {
	caps := Capabilities{}
	if strings.TrimSpace(help) == "" {
		return caps // nothing usable; stays undiscovered
	}
	caps.Discovered = true
	lower := strings.ToLower(help)

	caps.FlashAttention = strings.Contains(lower, "--flash-attn") ||
		strings.Contains(lower, "-fa,")
	if caps.FlashAttention {
		caps.FAValueStyle = strings.Contains(lower, "on|off") ||
			strings.Contains(lower, "'auto'")
	}
	caps.GPULayers = strings.Contains(lower, "--gpu-layers") ||
		strings.Contains(lower, "-ngl")
	caps.Batch = strings.Contains(lower, "--batch-size") || strings.Contains(lower, "-b,")
	caps.UBatch = strings.Contains(lower, "--ubatch-size") || strings.Contains(lower, "-ub,")
	caps.MMap = strings.Contains(lower, "--no-mmap") || strings.Contains(lower, "--mmap")
	caps.MLock = strings.Contains(lower, "--mlock")
	caps.OverrideTensor = strings.Contains(lower, "--override-tensor") || strings.Contains(lower, "-ot,")
	caps.SingleTurn = strings.Contains(lower, "--single-turn")

	caps.KVTypes = parseKVChoices(help)
	return caps
}

// parseKVChoices finds the allowed cache-type values listed next to
// -ctk/--cache-type-k. Modern builds enumerate choices explicitly; builds
// that do not return nil (unknown), and unknown is treated permissively.
func parseKVChoices(help string) []string {
	lines := strings.Split(help, "\n")
	for i, line := range lines {
		ll := strings.ToLower(line)
		if !strings.Contains(ll, "--cache-type-k") && !strings.HasPrefix(strings.TrimSpace(ll), "-ctk,") {
			continue
		}
		// The choices may sit on the same line or within the next few lines.
		for j := i; j < len(lines) && j <= i+3; j++ {
			vals, ok := extractAllowedValues(lines[j])
			if ok {
				return vals
			}
		}
	}
	return nil
}

// extractAllowedValues parses an "allowed values: a, b, c" fragment,
// returning entries that look like ggml type names.
func extractAllowedValues(line string) ([]string, bool) {
	ll := strings.ToLower(line)
	idx := strings.Index(ll, "allowed values")
	if idx < 0 {
		return nil, false
	}
	rest := line[idx+len("allowed values")+1:]
	rest = strings.TrimPrefix(strings.TrimSpace(rest), ":")
	var out []string
	for _, tok := range strings.Split(rest, ",") {
		tok = strings.TrimSpace(tok)
		if isGGMLTypeName(tok) {
			out = append(out, tok)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func isGGMLTypeName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
		default:
			return false
		}
	}
	return true
}

// CapabilitySource is implemented by runners that can report their
// discovered capabilities. Runners that do not implement it are treated as
// fully capable; their retry chains handle rejected flags.
type CapabilitySource interface {
	Capabilities() Capabilities
}

// Capabilities returns the probed capabilities of the wrapped llama-cli
// binary. Before a successful Available() call the result is undiscovered.
func (l *LlamaCLI) Capabilities() Capabilities {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.caps
}
