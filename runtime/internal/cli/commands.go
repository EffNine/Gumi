package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/config"
)

func runUtilityCommand(command string, args []string) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "output machine-readable JSON")
	apiURL := fs.String("api-url", "", "runtime API base URL")
	tail := fs.Int("tail", 100, "number of log lines")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	cfg, _ := config.Load("")
	base := *apiURL
	if base == "" {
		base = fmt.Sprintf("http://%s:%d", cfg.Runtime.Host, cfg.Runtime.Port)
	}
	path := map[string]string{
		"status": "/v1/novexa/status", "doctor": "/v1/novexa/doctor",
		"providers": "/v1/novexa/status", "models": "/v1/models",
	}[command]

	if command == "logs" {
		runLogs(*tail)
		return
	}
	if command == "benchmark" {
		runBenchmark(base, cfg.Auth.LocalKey, *jsonOutput)
		return
	}

	body, err := apiGet(base+path, cfg.Auth.LocalKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Novexa %s failed: %v\nSuggestion: start the runtime with 'novexa start'.\n", command, err)
		os.Exit(1)
	}
	if *jsonOutput {
		fmt.Println(string(body))
		return
	}
	printHuman(command, body)
}

func apiGet(endpoint, key string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("runtime returned %s", resp.Status)
	}
	return body, nil
}

func printHuman(command string, body []byte) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		fmt.Println(string(body))
		return
	}
	pretty, _ := json.MarshalIndent(value, "", "  ")
	labels := map[string]string{"status": "Novexa Status", "doctor": "Novexa Doctor", "providers": "Novexa Providers", "models": "Novexa Models"}
	fmt.Println(labels[command])
	fmt.Println(string(pretty))
}

func runConfigShow(args []string) {
	fs := flag.NewFlagSet("config show", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "output machine-readable JSON")
	configPath := fs.String("config", "", "path to configuration file")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	redacted := map[string]any{
		"runtime": cfg.Runtime, "dashboard": cfg.Dashboard,
		"auth":     map[string]any{"mode": cfg.Auth.Mode, "local_key": "***REDACTED***"},
		"provider": cfg.Provider, "providers": cfg.Providers,
		"storage": cfg.Storage, "telemetry": cfg.Telemetry,
	}
	body, _ := json.MarshalIndent(redacted, "", "  ")
	if !*jsonOutput {
		fmt.Println("Resolved Novexa Config")
	}
	fmt.Println(string(body))
}

func runLogs(tail int) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".novexa", "logs", "novexa.log")
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("No local log file found at %s. Runtime logs currently remain in the terminal.\n", path)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if tail > 0 && len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	fmt.Println(strings.Join(lines, "\n"))
}

func runBenchmark(base, key string, jsonOutput bool) {
	start := time.Now()
	_, err := apiGet(base+"/health", key)
	latency := time.Since(start)
	status := "ok"
	if err != nil {
		status = "failed"
	}
	result := map[string]any{"status": status, "gateway_latency_ms": latency.Milliseconds(), "endpoint": base}
	body, _ := json.MarshalIndent(result, "", "  ")
	if !jsonOutput {
		fmt.Println("Novexa Benchmark")
	}
	fmt.Println(string(body))
	if err != nil {
		os.Exit(1)
	}
}
