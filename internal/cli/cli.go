// Package cli implements the gumi optimizer command-line interface.
//
// Commands:
//
//	gumi tune <model.gguf> [--workload W] [--min-decode X]  V1 auto-tuner
//	gumi optimize <model.gguf> --workload <profile>         alias of tune
//	gumi inspect <model.gguf> [--json]
//	gumi probe [--model path] [--bandwidth] [--json]
//	gumi profiles [--json]
//	gumi export --config candidates.json --id <id> --target llama.cpp|lmstudio|ollama [--model path]
//	gumi version
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/EffNine/gumi/internal/version"
)

const usage = `gumi — Local LLM Auto-Tuner

Give Gumi a GGUF model; it experiments on this CUDA machine, measures real
configurations, verifies them against its defined evidence battery, and
recommends the configurations supported by measured evidence. You do not need to
know llama.cpp flags. Gumi does not prove general model intelligence and does
not guarantee global optimality.

Usage:
  gumi tune <model.gguf> [--workload agentic_coding|chat] [--min-decode N] [flags]
  gumi inspect <model.gguf> [--json]
  gumi probe [--model path] [--bandwidth] [--json]
  gumi profiles [--json]
  gumi export --config candidates.json --id <id> --target llama.cpp|lmstudio|ollama [--model path]
  gumi version

Run 'gumi <command> -h' for command-specific flags.`

// Execute parses arguments and dispatches.
// osExit is a seam for tests (replaced to capture exits).
var osExit = os.Exit

// normalizeArgs moves flags (and their values) ahead of positional arguments,
// so both `gumi optimize model.gguf --workload chat` and
// `gumi optimize --workload chat model.gguf` parse identically despite the
// stdlib flag package stopping at the first positional.
func normalizeArgs(args []string, valueFlags map[string]bool) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		isFlag := strings.HasPrefix(a, "-") && a != "-" && a != "--"
		if !isFlag {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue // value inline (--workload=chat)
		}
		if valueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func Execute() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		osExit(2)
	}
	switch os.Args[1] {
	case "tune":
		runTune(os.Args[2:], "tune")
	case "optimize":
		// Documented alias of tune (pre-V1 command name).
		runTune(os.Args[2:], "optimize")
	case "inspect":
		runInspect(os.Args[2:])
	case "probe":
		runProbe(os.Args[2:])
	case "profiles":
		runProfiles(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("gumi %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s\n", os.Args[1], usage)
		osExit(2)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	osExit(1)
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail("marshal json: %v", err)
	}
	fmt.Println(string(b))
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gumi %s [flags]\n\n", name)
		fs.PrintDefaults()
	}
	return fs
}
