package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EffNine/gumi/runtime/internal/config"
	"github.com/EffNine/gumi/runtime/internal/memory"
)

func runMemory(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: gumi memory [status|facts|clear]")
		os.Exit(1)
	}

	// Resolve memory DB path.
	dbPath := resolveMemoryDBPath()

	// Initialize memory engine.
	cfg := MemoryConfig()
	mem, err := memory.New(cfg, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot open memory database: %v\n", err)
		os.Exit(1)
	}
	defer mem.Close()

	switch args[0] {
	case "status":
		runMemoryStatus(mem, args[1:])
	case "facts":
		runMemoryFacts(mem, args[1:])
	case "clear":
		runMemoryClear(mem, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown memory subcommand: %q\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: gumi memory [status|facts|clear]")
		os.Exit(1)
	}
}

func runMemoryStatus(mem *memory.MemoryEngine, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx

	facts, err := mem.ListFacts(0)
	factCount := 0
	if err == nil {
		factCount = len(facts)
	}

	fit, err := mem.ListModelFit()
	fitCount := 0
	if err == nil {
		fitCount = len(fit)
	}

	// Check if JSON output requested.
	useJSON := false
	for _, a := range args {
		if a == "--json" || a == "-j" {
			useJSON = true
			break
		}
	}

	if useJSON {
		output := map[string]interface{}{
			"facts_count":       factCount,
			"model_fit_entries": fitCount,
			"database_path":     resolveMemoryDBPath(),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
		return
	}

	fmt.Println("Memory Engine Status")
	fmt.Println("====================")
	fmt.Printf("Database:     %s\n", resolveMemoryDBPath())
	fmt.Printf("Facts stored: %d\n", factCount)
	fmt.Printf("Model fit entries: %d\n", fitCount)
	fmt.Println()
}

func runMemoryFacts(mem *memory.MemoryEngine, args []string) {
	useJSON := false
	searchQuery := ""
	for _, a := range args {
		if a == "--json" || a == "-j" {
			useJSON = true
		} else if !strings.HasPrefix(a, "-") {
			searchQuery = a
		}
	}

	var facts []memory.MemoryFact
	var err error

	if searchQuery != "" {
		facts, err = mem.SearchFacts(searchQuery, 50)
	} else {
		facts, err = mem.ListFacts(50)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(facts)
		return
	}

	if len(facts) == 0 {
		fmt.Println("No facts stored.")
		return
	}

	fmt.Println("Memory Facts")
	fmt.Println("============")
	for _, f := range facts {
		fmt.Printf("  %-30s = %s  (confidence: %.2f, source: %s)\n",
			f.Key, truncateString(f.Value, 50), f.Confidence, f.Source)
	}
	fmt.Printf("\n%d facts shown\n", len(facts))
}

func runMemoryClear(mem *memory.MemoryEngine, args []string) {
	// Require confirmation.
	if len(args) == 0 || args[0] != "--force" {
		fmt.Fprintln(os.Stderr, "Warning: This will delete ALL memory data (facts, episodes, model fit).")
		fmt.Fprintln(os.Stderr, "Use --force to confirm.")
		os.Exit(1)
	}

	if err := mem.ClearAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Error clearing memory: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Memory cleared.")
}

// resolveMemoryDBPath returns the default path for the memory database.
func resolveMemoryDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.gumi/memory.db"
}

// MemoryConfig returns a default memory config for CLI use.
func MemoryConfig() *config.MemoryConfig {
	return &config.MemoryConfig{}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
