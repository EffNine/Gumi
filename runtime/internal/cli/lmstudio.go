package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/novexa/novexa/runtime/internal/config"
)

// runLMStudio dispatches lmstudio subcommands.
// Usage: novexa lmstudio [status|load|unload|models] [flags]
func runLMStudio(args []string) {
	if len(args) < 1 {
		printLMStudioUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "status":
		runLMStudioStatus(subArgs)
	case "load":
		runLMStudioLoad(subArgs)
	case "unload":
		runLMStudioUnload(subArgs)
	case "models":
		runLMStudioListModels(subArgs)
	case "help", "--help", "-h":
		printLMStudioUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown lmstudio subcommand: %q\n", subcommand)
		printLMStudioUsage()
		os.Exit(1)
	}
}

func printLMStudioUsage() {
	fmt.Println("LM Studio management commands:")
	fmt.Println()
	fmt.Println("  novexa lmstudio status [--url <base>] [--json]")
	fmt.Println("    Show the currently loaded model on LM Studio.")
	fmt.Println()
	fmt.Println("  novexa lmstudio load <model> [flags]")
	fmt.Println("    Load a model on LM Studio.")
	fmt.Println("    Flags: --url, --context-length, --flash-attention, --offload-kv-cache, --json")
	fmt.Println()
	fmt.Println("  novexa lmstudio unload <instance-id>")
	fmt.Println("    Unload a model instance from LM Studio.")
	fmt.Println("    Flags: --url, --json")
	fmt.Println()
	fmt.Println("  novexa lmstudio models [--url <base>] [--json]")
	fmt.Println("    List all models available on LM Studio (on disk).")
	fmt.Println()
	fmt.Println("  Use --url to override the LM Studio API base URL.")
	fmt.Println("  Default: http://localhost:1234/v1")
}

// resolveLMStudioURL returns the LM Studio API base URL from flag, config, or default.
func resolveLMStudioURL(providedURL string) string {
	if providedURL != "" {
		return providedURL
	}
	// Try config file.
	cfg, err := config.Load("")
	if err == nil {
		if s, ok := cfg.Providers["lmstudio"]; ok && s.URL != "" {
			return s.URL
		}
	}
	return "http://localhost:1234/v1"
}

// mgmtURL converts an OpenAI-compatible base URL to the v1 REST API base.
// e.g. "http://localhost:1234/v1" → "http://localhost:1234/api/v1"
func mgmtURL(baseURL string) string {
	base := strings.TrimSuffix(baseURL, "/v1")
	base = strings.TrimSuffix(base, "/")
	return base + "/api/v1"
}

// lmstudioAPIGet performs a GET to an LM Studio management endpoint.
func lmstudioAPIGet(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LM Studio unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// lmstudioAPIPost performs a POST to an LM Studio management endpoint.
func lmstudioAPIPost(url string, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second} // model loading can take 10+ seconds
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LM Studio unreachable: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

// ────────────────────────────────────────────────────────────────────────────
// novexa lmstudio status
// ────────────────────────────────────────────────────────────────────────────

func runLMStudioStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	apiURL := fs.String("url", "", "LM Studio API base URL")
	jsonOutput := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	base := resolveLMStudioURL(*apiURL)
	mgmtBase := mgmtURL(base)

	// Try to get the currently loaded model by calling /api/v1/models.
	// LM Studio doesn't have a dedicated "get loaded" endpoint, so we
	// check if the loaded model info is available.
	body, err := lmstudioAPIGet(mgmtBase + "/models")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// The /api/v1/models response has a "data" array of available models.
	// For status, we show the available models and note that the CLI
	// tracks the loaded model via the load command.
	var result struct {
		Data []struct {
			Model string `json:"model"`
			Type  string `json:"type"`
			Path  string `json:"path,omitempty"`
			Size  string `json:"size,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		fmt.Println(string(body))
		return
	}

	fmt.Printf("LM Studio: %s\n", base)
	fmt.Printf("Models on disk: %d\n", len(result.Data))
	fmt.Println()
	if len(result.Data) == 0 {
		fmt.Println("No models found. Download models via LM Studio GUI.")
		return
	}
	fmt.Println("Available models:")
	for _, m := range result.Data {
		size := m.Size
		if size == "" {
			size = "unknown"
		}
		fmt.Printf("  - %s  (%s, %s)\n", m.Model, m.Type, size)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// novexa lmstudio load
// ────────────────────────────────────────────────────────────────────────────

func runLMStudioLoad(args []string) {
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	apiURL := fs.String("url", "", "LM Studio API base URL")
	contextLength := fs.Int("context-length", 0, "max context tokens")
	flashAttention := fs.Bool("flash-attention", false, "enable flash attention")
	offloadKVCache := fs.Bool("offload-kv-cache", false, "offload KV cache to GPU")
	jsonOutput := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	modelID := fs.Arg(0)
	if modelID == "" {
		fmt.Fprintln(os.Stderr, "Error: model ID is required")
		fmt.Fprintln(os.Stderr, "Usage: novexa lmstudio load <model> [flags]")
		os.Exit(1)
	}

	base := resolveLMStudioURL(*apiURL)
	mgmtBase := mgmtURL(base)

	// Build load request payload.
	payload := map[string]interface{}{
		"model":            modelID,
		"echo_load_config": true,
	}
	if *contextLength > 0 {
		payload["context_length"] = *contextLength
	}
	if *flashAttention {
		payload["flash_attention"] = true
	}
	if *offloadKVCache {
		payload["offload_kv_cache_to_gpu"] = true
	}

	fmt.Printf("Loading model %q on LM Studio...\n", modelID)
	body, err := lmstudioAPIPost(mgmtBase+"/models/load", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		fmt.Println(string(body))
		return
	}

	var resp struct {
		Type            string  `json:"type"`
		InstanceID      string  `json:"instance_id"`
		LoadTimeSeconds float64 `json:"load_time_seconds"`
		Status          string  `json:"status"`
		LoadConfig      map[string]interface{} `json:"load_config,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Instance:     %s\n", resp.InstanceID)
	fmt.Printf("  Status:       %s\n", resp.Status)
	fmt.Printf("  Load time:    %.1fs\n", resp.LoadTimeSeconds)
	fmt.Printf("  Type:         %s\n", resp.Type)
	if resp.LoadConfig != nil {
		fmt.Println("  Config applied:")
		for k, v := range resp.LoadConfig {
			fmt.Printf("    %s: %v\n", k, v)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// novexa lmstudio unload
// ────────────────────────────────────────────────────────────────────────────

func runLMStudioUnload(args []string) {
	fs := flag.NewFlagSet("unload", flag.ContinueOnError)
	apiURL := fs.String("url", "", "LM Studio API base URL")
	jsonOutput := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	instanceID := fs.Arg(0)
	if instanceID == "" {
		fmt.Fprintln(os.Stderr, "Error: instance ID is required")
		fmt.Fprintln(os.Stderr, "Usage: novexa lmstudio unload <instance-id>")
		os.Exit(1)
	}

	base := resolveLMStudioURL(*apiURL)
	mgmtBase := mgmtURL(base)

	payload := map[string]string{"instance_id": instanceID}
	body, err := lmstudioAPIPost(mgmtBase+"/models/unload", payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		fmt.Println(string(body))
		return
	}

	var resp struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Unloaded: %s\n", resp.InstanceID)
}

// ────────────────────────────────────────────────────────────────────────────
// novexa lmstudio models
// ────────────────────────────────────────────────────────────────────────────

func runLMStudioListModels(args []string) {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	apiURL := fs.String("url", "", "LM Studio API base URL")
	jsonOutput := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	base := resolveLMStudioURL(*apiURL)
	mgmtBase := mgmtURL(base)

	body, err := lmstudioAPIGet(mgmtBase + "/models")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		fmt.Println(string(body))
		return
	}

	var result struct {
		Data []struct {
			Model string `json:"model"`
			Type  string `json:"type"`
			Path  string `json:"path,omitempty"`
			Size  string `json:"size,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("LM Studio models (disk): %d\n", len(result.Data))
	for _, m := range result.Data {
		size := m.Size
		if size == "" {
			size = "unknown"
		}
		fmt.Printf("  %s  (%s, %s)\n", m.Model, m.Type, size)
	}
}
