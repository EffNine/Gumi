// Package cli implements the Novexa command-line interface.
//
// Sprint 1 supports four commands:
//
//   - version: prints the runtime version.
//   - start:   loads configuration, starts the gateway and dashboard servers,
//     and shuts down gracefully on SIGINT/SIGTERM.
//   - stop:    sends SIGTERM to the running runtime, with a 30s graceful
//     timeout before falling back to SIGKILL.
//   - restart: stops the running runtime and starts a new instance.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	case "stop":
		runStop(os.Args[2:])
	case "restart":
		runRestart(os.Args[2:])
	case "status", "doctor", "providers", "models", "benchmark", "logs":
		runUtilityCommand(os.Args[1], os.Args[2:])
	case "config":
		if len(os.Args) >= 3 && os.Args[2] == "show" {
			runConfigShow(os.Args[3:])
			return
		}
		fmt.Fprintln(os.Stderr, "usage: novexa config show [--json]")
		os.Exit(1)
	case "lmstudio":
		runLMStudio(os.Args[2:])
	case "memory":
		runMemory(os.Args[2:])
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
	fmt.Println("  novexa stop")
	fmt.Println("  novexa restart [flags]")
	fmt.Println("  novexa status [--json]")
	fmt.Println("  novexa doctor [--json]")
	fmt.Println("  novexa config show [--json]")
	fmt.Println("  novexa providers [--json]")
	fmt.Println("  novexa models [--json]")
	fmt.Println("  novexa benchmark [--json]")
	fmt.Println("  novexa logs [--tail int]")
	fmt.Println("  novexa lmstudio [status|load|unload|models] [flags]")
	fmt.Println("  novexa memory [status|facts|clear] [flags]")
	fmt.Println()
	fmt.Println("Flags for start and restart:")
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

// pidFilePath returns the canonical PID file path (~/.novexa/novexa.pid).
func pidFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".novexa", "novexa.pid")
}

// ensurePidDir creates the ~/.novexa directory if it does not exist.
func ensurePidDir() {
	dir := filepath.Dir(pidFilePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create PID directory %s: %v\n", dir, err)
	}
}

// writePidFile writes the current process PID to the PID file.
func writePidFile() {
	ensurePidDir()
	pid := os.Getpid()
	if err := os.WriteFile(pidFilePath(), []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write PID file: %v\n", err)
	}
}

// removePidFile deletes the PID file, logging a warning on failure.
func removePidFile() {
	if err := os.Remove(pidFilePath()); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: could not remove PID file: %v\n", err)
	}
}

// checkExistingPid checks whether a PID file exists with a live process.
// If so, it prints an error and exits with code 1.
func checkExistingPid() {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid); err != nil {
		return
	}
	if isProcessRunning(pid) {
		fmt.Fprintf(os.Stderr, "Novexa is already running (PID %d). Use 'novexa stop' to stop it first.\n", pid)
		os.Exit(1)
	}
}

// runStart parses flags, loads config, checks for an existing running instance,
// writes the PID file, starts the servers, and blocks until shutdown.
func runStart(args []string) {
	cfg, log := parseStartFlags(args)
	checkExistingPid()
	writePidFile()
	startServers(cfg, log)
}

// parseStartFlags parses the common start/restart flags and returns the
// resolved config and logger.
func parseStartFlags(args []string) (*config.Config, *logger.Logger) {
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
	return cfg, log
}

// startServers creates the gateway and dashboard servers, starts them, blocks
// until a signal or startup error, then shuts down gracefully. It also removes
// the PID file on shutdown.
func startServers(cfg *config.Config, log *logger.Logger) {
	defer removePidFile()

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

// runStop sends SIGTERM to the running runtime process, waits up to 30 seconds
// for graceful shutdown, and falls back to SIGKILL if needed.
func runStop(args []string) {
	_ = args // stop takes no flags
	if !stopProcess() {
		os.Exit(1)
	}
	os.Exit(0)
}

// runRestart stops the running runtime and starts a new instance with the
// provided flags.
func runRestart(args []string) {
	stopProcess()
	cfg, log := parseStartFlags(args)
	writePidFile()
	startServers(cfg, log)
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
