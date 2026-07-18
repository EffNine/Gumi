package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/memory"
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
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("MEMORY_LIST_ERROR", "failed to list facts: "+err.Error(), requestIDFromContext(r.Context())))
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
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("MEMORY_MODEL_FIT_ERROR", "failed to list model fit: "+err.Error(), requestIDFromContext(r.Context())))
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
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("MEMORY_CLEAR_ERROR", "failed to clear memory: "+err.Error(), requestIDFromContext(r.Context())))
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
		"enabled":           true,
		"database_path":     dbPath,
		"facts_count":       len(facts),
		"model_fit_entries": len(fit),
		"injection_budget":  s.cfg.Memory.InjectionBudgetTokens,
	})
}

// handleSelfTuning returns the current self-tuning snapshot for the dashboard.
func (s *Server) handleSelfTuning(w http.ResponseWriter, r *http.Request) {
	pipe := s.pipeline
	if pipe == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"reason":  "pipeline not available",
		})
		return
	}

	routerEngine := pipe.CodingRouter()
	if routerEngine == nil || routerEngine.SelfTuner() == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"reason":  "self-tuning not configured",
		})
		return
	}

	snapshot := routerEngine.SelfTuner().Snapshot()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":   true,
		"snapshot":  snapshot,
		"config":    s.cfg.Routing.SelfTuning,
		"generated": snapshot.GeneratedAt,
	})
}

// handleMemoryCreateFact creates a new memory fact via POST.
func (s *Server) handleMemoryCreateFact(w http.ResponseWriter, r *http.Request) {
	mem := s.pipeline.MemoryEngine()
	if mem == nil {
		s.writeError(w, http.StatusServiceUnavailable, api.NewRuntimeError("MEMORY_DISABLED", "memory engine is not enabled", requestIDFromContext(r.Context())))
		return
	}

	var req struct {
		Key        string  `json:"key"`
		Value      string  `json:"value"`
		Source     string  `json:"source,omitempty"`
		Confidence float64 `json:"confidence,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}
	if req.Key == "" || req.Value == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "key and value are required", requestIDFromContext(r.Context())))
		return
	}
	if req.Confidence == 0 {
		req.Confidence = 0.7
	}
	fact := memory.MemoryFact{Key: req.Key, Value: req.Value, Source: req.Source, Confidence: req.Confidence}
	if err := mem.StoreFact(fact); err != nil {
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("MEMORY_FACT_CREATE_ERROR", "failed to store fact: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "ok", "fact": fact})
}

// writeJSON is a helper to write a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
