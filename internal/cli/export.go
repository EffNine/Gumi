package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/EffNine/gumi/internal/backend"
)

type candidatesFile struct {
	Candidates []struct {
		ID     string         `json:"id"`
		Name   string         `json:"name"`
		Config backend.Config `json:"config"`
	} `json:"candidates"`
}

// parseCandidatesFile accepts both artifact shapes: the bare array written
// by writeAuxArtifacts and a {"candidates":[...]} wrapper.
func parseCandidatesFile(data []byte) (candidatesFile, error) {
	var cf candidatesFile
	if err := json.Unmarshal(data, &cf); err == nil && len(cf.Candidates) > 0 {
		return cf, nil
	}
	var arr candidatesFile
	if err := json.Unmarshal(data, &arr.Candidates); err == nil && len(arr.Candidates) > 0 {
		return arr, nil
	}
	return cf, fmt.Errorf("no candidates found in config file")
}

func runExport(args []string) {
	fs := newFlagSet("export")
	configPath := fs.String("config", "", "path to candidates.json from an optimize run")
	id := fs.String("id", "", "candidate id (e.g. balanced)")
	target := fs.String("target", "llama.cpp", "llama.cpp | lmstudio | ollama")
	modelPath := fs.String("model", "", "override model path used in exports")
	if err := fs.Parse(args); err != nil {
		osExit(2)
	}
	if *configPath == "" || *id == "" {
		fs.Usage()
		osExit(2)
	}
	data, err := os.ReadFile(*configPath)
	if err != nil {
		fail("read config: %v", err)
	}
	cf, err := parseCandidatesFile(data)
	if err != nil {
		fail("parse config: %v", err)
	}
	for _, c := range cf.Candidates {
		if c.ID != *id {
			continue
		}
		model := *modelPath
		if model == "" {
			model = "<model.gguf>"
		}
		switch *target {
		case "llama.cpp":
			fmt.Println(backend.RenderExports(model, c.Config).LlamaCLI)
		case "llama-server":
			fmt.Println(backend.RenderExports(model, c.Config).LlamaServer)
		case "lmstudio":
			fmt.Println(backend.RenderExports(model, c.Config).LMStudio)
		case "ollama":
			fmt.Println(backend.RenderExports(model, c.Config).Ollama)
		default:
			fail("unknown target %q (llama.cpp|llama-server|lmstudio|ollama)", *target)
		}
		return
	}
	fail("candidate %q not found in %s", *id, *configPath)
}
