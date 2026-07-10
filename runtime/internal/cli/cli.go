// Package cli implements the Novexa command-line interface.
//
// Sprint 1 supports two commands:
//
//   - version: prints the runtime version.
//   - start:   loads configuration, prints startup information, runs a
//     placeholder loop, and shuts down gracefully on SIGINT/SIGTERM.
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
	"github.com/novexa/novexa/runtime/internal/dashboard"
	"github.com/novexa/novexa/runtime/internal/gateway"
	"github.com/novexa/novexa/runtime/internal/logger"
	"github.com/novexa/novexa/runtime/internal/version"
)

// Version is the current Novexa runtime version.
//
// The default comes from the version package and can be overridden at build time
// with ldflags so release pipelines do not need to edit source files.
var Version = version.Version

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
	case "status", "doctor", "providers", "models", "benchmark", "logs":
		runUtilityCommand(os.Args[1], os.Args[2:])
	case "config":
		if len(os.Args) >= 3 && os.Args[2] == "show" {
			runConfigShow(os.Args[3:])
			return
		}
		fmt.Fprintln(os.Stderr, "usage: novexa config show [--json]")
		os.Exit(1)
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
	fmt.Println("  novexa version [--verbose]")
	fmt.Println("  novexa start [flags]")
	fmt.Println("  novexa status [--json]")
	fmt.Println("  novexa doctor [--json]")
	fmt.Println("  novexa config show [--json]")
	fmt.Println("  novexa providers [--json]")
	fmt.Println("  novexa models [--json]")
	fmt.Println("  novexa benchmark [--json]")
	fmt.Println("  novexa logs [--tail int]")
	fmt.Println()
	fmt.Println("Flags for start:")
	fmt.Println("  --config string         Path to configuration file")
	fmt.Println("  --port int              Override API port")
	fmt.Println("  --host string           Override API bind host")
	fmt.Println("  --dashboard-port int    Override dashboard port")
	fmt.Println("  --dashboard-host string Override dashboard bind host")
	fmt.Println("  --provider string       Override default provider")
	fmt.Println("  --model string          Override default model")
	fmt.Println("  --mode string           Override runtime mode")
	fmt.Println("  --verbose               Enable debug logging")
	fmt.Println("  --quiet                 Suppress non-error output")
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "show build metadata")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	fmt.Printf("Novexa Runtime %s\n", Version)
	if *verbose {
		fmt.Printf("Commit: %s\n", version.Commit)
		fmt.Printf("Build Date: %s\n", version.BuildDate)
	}
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)

	var configPath string
	var portOverride int
	var dashboardPortOverride int
	var hostOverride string
	var dashboardHostOverride string
	var providerOverride string
	var modelOverride string
	var modeOverride string
	var verbose bool
	var quiet bool

	fs.StringVar(&configPath, "config", "", "path to configuration file")
	fs.IntVar(&portOverride, "port", 0, "override API port")
	fs.IntVar(&dashboardPortOverride, "dashboard-port", 0, "override dashboard port")
	fs.StringVar(&hostOverride, "host", "", "override API bind host")
	fs.StringVar(&dashboardHostOverride, "dashboard-host", "", "override dashboard bind host")
	fs.StringVar(&providerOverride, "provider", "", "override default provider")
	fs.StringVar(&modelOverride, "model", "", "override default model")
	fs.StringVar(&modeOverride, "mode", "", "override runtime mode")
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
	if dashboardPortOverride > 0 {
		cfg.Dashboard.Port = dashboardPortOverride
	}
	if hostOverride != "" {
		cfg.Runtime.Host = hostOverride
	}
	if dashboardHostOverride != "" {
		cfg.Dashboard.Host = dashboardHostOverride
	}
	if modeOverride != "" {
		cfg.Runtime.Mode = modeOverride
	}
	if providerOverride != "" {
		cfg.Provider.Default = providerOverride
	}
	if modelOverride != "" {
		settings, ok := cfg.Providers[cfg.Provider.Default]
		if !ok {
			fmt.Fprintf(os.Stderr, "provider %q is not configured\n", cfg.Provider.Default)
			os.Exit(1)
		}
		settings.DefaultModel = modelOverride
		cfg.Providers[cfg.Provider.Default] = settings
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

	srv := gateway.New(cfg, log)
	srvErr := srv.Start()
	var dashboardSrv *dashboard.Server
	var dashboardErr <-chan error
	if cfg.Dashboard.Enabled {
		dashboardSrv = dashboard.New(cfg, log)
		dashboardErr = dashboardSrv.Start()
	}

	select {
	case <-ctx.Done():
	case err := <-srvErr:
		if err != nil {
			log.Error("gateway failed to start", err)
			os.Exit(1)
		}
	case err := <-dashboardErr:
		if err != nil {
			log.Error("dashboard failed to start", err)
		}
	}

	log.Info("Novexa Runtime shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("gateway shutdown error", err)
	}
	if dashboardSrv != nil {
		if err := dashboardSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("dashboard shutdown error", err)
		}
	}
	log.Info("Novexa Runtime stopped")
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
