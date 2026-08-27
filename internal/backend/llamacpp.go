package backend

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// LlamaCLI runs verification through a llama.cpp llama-cli subprocess.
//
// Version drift is handled by probing --help once and by retrying with legacy
// flag forms when the backend rejects an argument.
type LlamaCLI struct {
	Bin       string   // path to llama-cli (resolved via PATH when empty)
	ExtraArgs []string // appended verbatim

	mu      sync.Mutex
	binPath string
	q       quirks
	caps    Capabilities
}

// NewLlamaCLI constructs a runner. Bin may be empty to use "llama-cli".
func NewLlamaCLI(bin string) *LlamaCLI {
	if bin == "" {
		bin = "llama-cli"
	}
	return &LlamaCLI{Bin: bin}
}

// Name implements Runner.
func (l *LlamaCLI) Name() string { return "llama.cpp" }

// Available checks the binary exists and probes flag quirks.
func (l *LlamaCLI) Available(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.q.helpChecked {
		return nil
	}
	path, err := exec.LookPath(l.Bin)
	if err != nil {
		return fmt.Errorf("%w: %s not found in PATH", ErrNotAvailable, l.Bin)
	}
	l.binPath = path

	helpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	outBytes, _ := exec.CommandContext(helpCtx, path, "--help").CombinedOutput()
	help := string(outBytes)
	if len(help) > 0 {
		l.q.fa = detectFAStyle(help)
		l.q.conv = detectConvStyle(help)
		l.caps = ParseCapabilities(help)
	} else {
		l.q.fa = faValue // modern default; retry chain covers drift
		l.q.conv = convSingleTurn
		l.caps = Capabilities{} // undiscovered: permissive-with-retry
	}
	l.q.helpChecked = true
	return nil
}

// detectConvStyle picks the conversation-mode disable flag. --single-turn is
// preferred when present: -no-cnv alone still loops on stdin EOF in recent
// builds.
func detectConvStyle(help string) convStyle {
	h := strings.ToLower(help)
	if strings.Contains(h, "--single-turn") {
		return convSingleTurn
	}
	if strings.Contains(h, "-no-cnv") || strings.Contains(h, "--no-conversation") {
		return convNoCNV
	}
	return convNone
}

// detectFAStyle inspects help output for flash-attention syntax.
func detectFAStyle(help string) faStyle {
	for _, line := range strings.Split(help, "\n") {
		ll := strings.ToLower(line)
		if !strings.HasPrefix(strings.TrimSpace(ll), "-fa") &&
			!strings.Contains(ll, "--flash-attn") {
			continue
		}
		if strings.Contains(ll, "on|off") || strings.Contains(ll, "auto") {
			return faValue
		}
		return faFlag
	}
	return faValue
}

// Run executes one verification run.
func (l *LlamaCLI) Run(ctx context.Context, spec RunSpec) (*Result, error) {
	l.mu.Lock()
	bin, q, caps := l.binPath, l.q, l.caps
	l.mu.Unlock()
	if bin == "" {
		return nil, ErrNotAvailable
	}
	if err := validateAgainstCaps(spec.Config, caps); err != nil {
		return nil, err
	}

	start := time.Now()
	res, err := l.execWithRetries(ctx, bin, spec, q)
	if err != nil {
		return nil, err
	}
	res.Metrics.Duration = time.Since(start)
	return res, nil
}

// validateAgainstCaps refuses configurations that demand features the
// discovered backend cannot express. Silently dropping such a flag would run
// a DIFFERENT configuration than the one recorded — corrupted evidence — so
// this fails loudly instead. Evidence-critical dimensions only: advisory
// knobs (mmap/mlock) may degrade silently because they do not change what is
// being measured.
func validateAgainstCaps(c Config, caps Capabilities) error {
	if !caps.Discovered {
		return nil // nothing probed: the retry chain is the arbiter
	}
	kv := strings.ToLower(c.KVCacheType)
	if kv != "" && kv != "f16" && !caps.SupportedKV(kv) {
		return fmt.Errorf("%w: KV cache type %q rejected by this backend build (accepted: %s)",
			ErrUnsupported, kv, strings.Join(caps.KVTypes, ","))
	}
	if c.ExpertsOnCPU && !caps.OverrideTensor {
		return fmt.Errorf("%w: expert tensor placement (-ot) not supported by this backend build", ErrUnsupported)
	}
	return nil
}

// attemptList builds the ordered quirk variants to try.
func attemptList(q quirks) []quirks {
	out := []quirks{q}
	add := func(v quirks) {
		for _, a := range out {
			if a == v {
				return
			}
		}
		out = append(out, v)
	}
	if q.conv == convSingleTurn {
		v := q
		v.conv = convNoCNV
		add(v)
	}
	if q.fa == faValue {
		v := q
		v.fa = faFlag
		add(v)
	}
	return out
}

// execWithRetries runs llama-cli, retrying with legacy flag forms when the
// binary rejects arguments it does not know.
func (l *LlamaCLI) execWithRetries(ctx context.Context, bin string, spec RunSpec, q quirks) (*Result, error) {
	attempts := attemptList(q)

	var lastErr error
	var lastRes *Result
	for i, attempt := range attempts {
		res, err := l.execOnce(ctx, bin, spec, attempt)
		if err == nil {
			return res, nil
		}
		lastErr, lastRes = err, res
		more := i < len(attempts)-1
		var exitErr *exec.ExitError
		if more && errors.As(err, &exitErr) && lastRes != nil &&
			looksLikeUnknownArg(lastRes.StderrTail) {
			continue
		}
		break
	}
	return nil, lastErr
}

// execOnce performs exactly one llama-cli invocation.
func (l *LlamaCLI) execOnce(ctx context.Context, bin string, spec RunSpec, q quirks) (*Result, error) {
	args := append(buildArgs(spec, q), l.ExtraArgs...)
	cmdCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, args...)
	cmd.SysProcAttr = procGroupAttrs()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &strings.Builder{}
	cmd.Stderr = stderr

	sampler := startSampler(cancel)

	if err := cmd.Start(); err != nil {
		sampler.Stop()
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	sampler.SetPid(cmd.Process.Pid)

	outBuf := &strings.Builder{}
	scannerDone := make(chan struct{})
	go func() {
		defer close(scannerDone)
		sc := bufio.NewScanner(stdoutPipe)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			outBuf.WriteString(sc.Text())
			outBuf.WriteByte('\n')
		}
	}()

	// Drain stdout fully, then collect the exit status. The scanner reaches
	// EOF when the process exits and the pipe write-end closes.
	<-scannerDone
	runErr := cmd.Wait()
	sampler.Stop()

	stderrText := stderr.String()
	tail := Truncate(stderrText, 4000)

	if runErr != nil {
		if ctx.Err() != nil {
			killProcessGroup(cmd.Process.Pid)
			return &Result{StderrTail: tail}, ErrTimedOut
		}
		if reason := classifyError(stderrText); reason != nil {
			return &Result{StderrTail: tail}, reason
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return &Result{StderrTail: tail},
				fmt.Errorf("llama-cli exited with code %d: %s", exitErr.ExitCode(), firstLine(stderrText))
		}
		return &Result{StderrTail: tail}, runErr
	}

	// Recent builds print perf summaries (and sometimes all init logs) on
	// stdout; older ones on stderr. Parse the combined stream.
	prefillTPS, decodeTPS, promptMs, ok := ParseTimings(outBuf.String() + "\n" + stderrText)
	if !ok {
		return &Result{StderrTail: tail}, fmt.Errorf("could not parse llama.cpp timing output")
	}

	vramPeak, ramPeak := sampler.Peaks()
	return &Result{
		Output: CleanOutput(outBuf.String(), spec.Prompt),
		Metrics: Metrics{
			PrefillTPS:    prefillTPS,
			DecodeTPS:     decodeTPS,
			PromptEvalMs:  promptMs,
			PeakVRAMBytes: vramPeak,
			PeakRAMBytes:  ramPeak,
		},
		StderrTail: tail,
	}, nil
}
