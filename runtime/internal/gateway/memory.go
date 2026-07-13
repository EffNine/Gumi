package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/novexa/novexa/runtime/internal/memory"
)

// handleMemoryFacts returns stored memory facts.
func (s *Server) handleMemoryFacts(w http.ResponseWriter, r *http.Request) {
	mem := s.pipeline.MemoryEngine()
	if mem == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"facts":   []memory.MemoryFact{},
		})
		return
	}

	searchQuery := r.URL.Query().Get("search")
	limit := 100

	var facts []memory.MemoryFact
	var err error
	if searchQuery != "" {
		facts, err = mem.SearchFacts(searchQuery, limit)
	} else {
		facts, err = mem.ListFacts(limit)
	}

	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list facts: "+err.Error())
		return
	}

	if facts == nil {
		facts = []memory.MemoryFact{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"facts":   facts,
		"count":   len(facts),
	})
}

// handleMemoryModelFit returns model performance data for the dashboard.
func (s *Server) handleMemoryModelFit(w http.ResponseWriter, r *http.Request) {
	mem := s.pipeline.MemoryEngine()
	if mem == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"entries": []memory.ModelFitEntry{},
		})
		return
	}

	entries, err := mem.ListModelFit()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list model fit: "+err.Error())
		return
	}

	if entries == nil {
		entries = []memory.ModelFitEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"entries": entries,
		"count":   len(entries),
	})
}

// handleMemoryClear clears all memory data.
func (s *Server) handleMemoryClear(w http.ResponseWriter, r *http.Request) {
	mem := s.pipeline.MemoryEngine()
	if mem == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"message": "memory engine not enabled, nothing to clear",
		})
		return
	}

	if err := mem.ClearAll(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to clear memory: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "memory cleared",
	})
}

// handleMemoryStatus returns memory engine status.
func (s *Server) handleMemoryStatus(w http.ResponseWriter, r *http.Request) {
	mem := s.pipeline.MemoryEngine()
	if mem == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled":       false,
			"database_path": "",
		})
		return
	}

	facts, _ := mem.ListFacts(0)
	fit, _ := mem.ListModelFit()

	dbPath := ""
	if s.cfg.Memory.DBPath != "" {
		dbPath = s.cfg.Memory.DBPath
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":             true,
		"database_path":       dbPath,
		"facts_count":         len(facts),
		"model_fit_entries":   len(fit),
		"injection_budget":    s.cfg.Memory.InjectionBudgetTokens,
	})
}

// writeJSON is a helper to write a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeJSONError is a helper to write a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
