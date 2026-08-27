package backend

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// timingRe matches llama.cpp perf lines, e.g.
//
//	prompt eval time:  1234.56 ms /   128 tokens (   10.37 tokens per second)
//	eval time =        2100.00 ms /    20 runs   (    5.71 tokens per second)
var timingRe = regexp.MustCompile(
	`time\s*[=:]\s*([0-9.]+)\s*ms\s*/\s*([0-9]+)\s*\S+\s*\(\s*([0-9.]+)\s*tokens per second`)

// compactTimingRe matches the newer one-line perf summary, e.g.
//
//	[ Prompt: 52.5 t/s | Generation: 30.7 t/s ]
//
// printed by recent llama.cpp builds on stdout. It carries no prompt-eval
// millisecond figure.
var compactTimingRe = regexp.MustCompile(
	`\[\s*Prompt:\s*([0-9.]+)\s*t/s\s*\|\s*Generation:\s*([0-9.]+)\s*t/s\s*\]`)

// ParseTimings extracts prefill and decode throughput from llama.cpp output.
// Both the historical verbose stderr lines and the newer compact summary are
// supported. Returns (prefillTPS, decodeTPS, promptEvalMs, found).
func ParseTimings(output string) (float64, float64, float64, bool) {
	var prefill, decode, promptMs float64
	found := false
	sc := bufio.NewScanner(strings.NewReader(output))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.Contains(line, "prompt eval time"):
			if m := timingRe.FindStringSubmatch(line); m != nil {
				promptMs = parseF(m[1])
				prefill = parseF(m[3])
				found = true
			}
		case strings.Contains(line, "eval time"):
			if m := timingRe.FindStringSubmatch(line); m != nil {
				decode = parseF(m[3])
				found = true
			}
		default:
			if m := compactTimingRe.FindStringSubmatch(line); m != nil {
				prefill = parseF(m[1])
				decode = parseF(m[2])
				found = true
			}
		}
	}
	return prefill, decode, promptMs, found
}

func parseF(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%g", &f)
	return f
}

// containsAlnum reports whether s contains any ASCII letter or digit.
func containsAlnum(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// thinkingSpanRe removes explicit reasoning blocks some chat templates emit
// as literal text ("[Start thinking] … [End thinking]"). Validators run on
// the answer only; an unterminated block means no answer was produced.
var thinkingSpanRe = regexp.MustCompile(`(?is)\[start thinking\].*?(\[end thinking\]|$)`)

// spinnerRe matches load-progress artifacts: a "Loading model…" line whose
// animated frames are separated by backspace characters.
var spinnerRe = regexp.MustCompile(`(?m)^.*Loading model[^\n]*$`)

// controlRe removes backspace and other C0 control characters except newline
// and tab (spinner animation frames).
var controlRe = regexp.MustCompile(`[\x00-\x08\x0B-\x1F\x7F]`)

// turnMarkerRe matches bare chat-scaffold lines some templates render into
// stdout ("User:", "Assistant:", "System:", optionally "> "-prefixed).
// These are template plumbing, not model output.
var turnMarkerRe = regexp.MustCompile(`(?im)^\s*>?\s*(user|assistant|system)\s*:?\s*$`)

// CleanOutput turns raw llama-cli stdout into the model's answer text:
// strips control/spinner artifacts, the init banner, explicit thinking
// blocks, conversation markers, and the echoed prompt.
func CleanOutput(stdout, prompt string) string {
	out := strings.TrimSpace(controlRe.ReplaceAllString(stdout, ""))
	out = spinnerRe.ReplaceAllString(out, "")
	out = turnMarkerRe.ReplaceAllString(out, "")
	// Everything from the perf summary onward is post-run reporting.
	if i := strings.Index(out, "[ Prompt:"); i >= 0 {
		out = out[:i]
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return out
	}
	out = strings.TrimSpace(thinkingSpanRe.ReplaceAllString(out, ""))
	// No verbatim prompt echo: recent builds may still print an init banner
	// before a "> " marker; keep only what follows the last marker.
	if prompt != "" && !strings.Contains(out, strings.TrimSpace(prompt)) {
		if i := strings.LastIndex(out, "\n> "); i >= 0 {
			out = out[i+3:]
		} else if strings.HasPrefix(out, "> ") {
			out = out[2:]
		}
	}
	// Drop leading conversation-marker lines ("", ">", "> ") and pure
	// decoration (banner glyphs carry no alphanumeric content).
	for {
		lines := strings.SplitN(out, "\n", 2)
		head := strings.TrimSpace(lines[0])
		if head == "" || head == ">" {
			if len(lines) == 1 {
				return ""
			}
			out = strings.TrimSpace(lines[1])
			continue
		}
		if !containsAlnum(head) {
			if len(lines) == 1 {
				return ""
			}
			out = strings.TrimSpace(lines[1])
			continue
		}
		break
	}
	if prompt != "" {
		prompt = strings.TrimSpace(prompt)
		if idx := strings.LastIndex(out, prompt); idx >= 0 {
			out = strings.TrimSpace(out[idx+len(prompt):])
			if out == "" {
				return out
			}
		} else {
			// Prompt may be wrapped; fall back to normalized prefix strip,
			// slicing the ORIGINAL text so line structure survives for
			// format validators.
			norm := normalizeWS(out)
			tail := normalizeWS(prompt)
			if strings.HasPrefix(norm, tail) && len(norm) > len(tail) {
				out = strings.TrimSpace(out[len(out)-(len(norm)-len(tail)):])
			}
		}
	}
	return out
}

// normalizeWS collapses all whitespace runs to single spaces.
func normalizeWS(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// Truncate returns at most n bytes of s with an ellipsis marker.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
