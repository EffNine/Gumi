// Package cli implements the Novexa command-line interface.
//
// Sprint 1 supports two commands:
//
//   - version: prints the runtime version.
//   - start:   loads configuration, prints startup information, runs a
//              placeholder loop, and shuts down gracefully on SIGINT/SIGTERM.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/novexa/novexa/runtime/internal/config"
	"github.com/novexa/novexa/runtime/internal/logger"
)

// Version is the current Novexa runtime version.
const Version = "0.1.0"

// Execute parses the CLI arguments and dispatches to the appropriate command.
func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		runVersion(os.Args[2:])
	case "start":
		runStart(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Novexa Runtime")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  novexa version")
	fmt.Println("  novexa start [flags]")
	fmt.Println()
	fmt.Println("Flags for start:")
	fmt.Println("  --config string   Path to configuration file")
	fmt.Println("  --port int        Override API port")
	fmt.Println("  --verbose         Enable debug logging")
	fmt.Println("  --quiet           Suppress non-error output")
}

func runVersion(args []string) {
	fmt.Printf("Novexa Runtime %s\n", Version)
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)

	var configPath string
	var portOverride int
	var verbose bool
	var quiet bool

	fs.StringVar(&configPath, "config", "", "path to configuration file")
	fs.IntVar(&portOverride, "port", 0, "override API port")
	fs.BoolVar(&verbose, "verbose", false, "enable debug logging")
	fs.BoolVar(&quiet, "quiet", false, "suppress non-error output")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		cfg.Runtime.LogLevel = "debug"
	}
	if quiet {
		cfg.Runtime.LogLevel = "error"
	}

	if portOverride > 0 {
		cfg.Runtime.Port = portOverride
	}

	log := logger.New(cfg.Runtime.LogLevel)

	printStartupBanner(cfg)
	log.Info("Novexa Runtime started",
		"version", Version,
		"mode", cfg.Runtime.Mode,
		"host", cfg.Runtime.Host,
		"port", cfg.Runtime.Port,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Novexa Runtime shutting down gracefully")
			// Give any future cleanup a moment to finish.
			time.Sleep(100 * time.Millisecond)
			log.Info("Novexa Runtime stopped")
			return
		case <-ticker.C:
			log.Debug("placeholder runtime loop tick")
		}
	}
}

func printStartupBanner(cfg *config.Config) {
	provider := cfg.Provider.Default
	model := "local:auto"
	if p, ok := cfg.Providers[provider]; ok {
		model = p.DefaultModel
	}

	fmt.Printf("Novexa Runtime %s\n\n", Version)
	fmt.Printf("API        http://%s:%d/v1\n", cfg.Runtime.Host, cfg.Runtime.Port)
	fmt.Printf("Dashboard  http://%s:%d\n", cfg.Dashboard.Host, cfg.Dashboard.Port)
	fmt.Printf("Mode       %s\n", cfg.Runtime.Mode)
	fmt.Printf("Provider   %s\n", provider)
	fmt.Printf("Model      %s\n", model)
	fmt.Println()
	fmt.Println("Status     ready")
	fmt.Println()
	fmt.Println("Use with OpenAI-compatible clients:")
	fmt.Println()
	fmt.Printf("export OPENAI_BASE_URL=http://%s:%d/v1\n", cfg.Runtime.Host, cfg.Runtime.Port)
	fmt.Printf("export OPENAI_API_KEY=%s\n", cfg.Auth.LocalKey)
	fmt.Println()
}
