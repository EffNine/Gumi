// Gumi is a local-first Local LLM Optimization Engine.
//
// Given a GGUF model and the user's hardware, gumi finds the best verified
// inference configuration for a target workload: it inspects the model,
// probes the machine, generates a small deterministic set of candidate
// configurations, verifies performance and capability with llama.cpp, and
// emits a report plus ready-to-use exports.
package main

import (
	"github.com/EffNine/gumi/internal/cli"
)

func main() {
	cli.Execute()
}
