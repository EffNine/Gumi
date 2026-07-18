package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
)

// ────────────────────────────────────────────────────────────────────────────
// Model Registry CRUD Handlers
// ────────────────────────────────────────────────────────────────────────────

// modelRegistryResponse is the envelope for GET /v1/gumi/models.
type modelRegistryResponse struct {
	Enabled bool                        `json:"enabled"`
	Models  []config.ModelRegistryEntry `json:"models"`
}

func (s *Server) effectiveConfigPath() string {
	if s.ConfigPath != "" {
		return s.ConfigPath
	}
	return s.configPath
}

func (s *Server) snapshotModels() []config.ModelRegistryEntry {
	s.modelsMu.RLock()
	defer s.modelsMu.RUnlock()
	if s.cfg.Models == nil {
		return []config.ModelRegistryEntry{}
	}
	return slices.Clone(s.cfg.Models)
}

func (s *Server) persistAndSyncModels(models []config.ModelRegistryEntry) error {
	if err := config.SaveModels(s.effectiveConfigPath(), models); err != nil {
		return err
	}
	s.manager.SetRegistry(models)
	if s.pipeline != nil {
		s.pipeline.RefreshProviderModels(models)
	}
	return nil
}

// mutateModels applies an in-memory update, persists it, and refreshes runtime
// wiring. The previous registry state is restored when persistence fails.
func (s *Server) mutateModels(update func([]config.ModelRegistryEntry) ([]config.ModelRegistryEntry, error)) ([]config.ModelRegistryEntry, error) {
	s.modelsMu.Lock()
	defer s.modelsMu.Unlock()

	before := slices.Clone(s.cfg.Models)
	updated, err := update(before)
	if err != nil {
		return nil, err
	}

	s.cfg.Models = updated
	if err := s.persistAndSyncModels(updated); err != nil {
		s.cfg.Models = before
		s.manager.SetRegistry(before)
		if s.pipeline != nil {
			s.pipeline.RefreshProviderModels(before)
		}
		return nil, err
	}

	return updated, nil
}

// handleGetModels returns the model registry entries.
func (s *Server) handleGetModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, modelRegistryResponse{
		Enabled: true,
		Models:  s.snapshotModels(),
	})
}

// handleCreateModel adds a new entry to the model registry.
func (s *Server) handleCreateModel(w http.ResponseWriter, r *http.Request) {
	var entry config.ModelRegistryEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}
	if entry.Alias == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "alias is required", requestIDFromContext(r.Context())))
		return
	}
	if entry.Provider == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "provider is required", requestIDFromContext(r.Context())))
		return
	}
	if entry.ModelID == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "model_id is required", requestIDFromContext(r.Context())))
		return
	}

	models, err := s.mutateModels(func(models []config.ModelRegistryEntry) ([]config.ModelRegistryEntry, error) {
		for _, m := range models {
			if strings.EqualFold(m.Alias, entry.Alias) {
				return nil, fmt.Errorf("duplicate alias %q", entry.Alias)
			}
		}

		if entry.Default {
			for i := range models {
				models[i].Default = false
			}
		}

		return append(models, entry), nil
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate alias") {
			s.writeError(w, http.StatusConflict, api.NewRequestError("DUPLICATE_ALIAS", fmt.Sprintf("a model with alias %q already exists", entry.Alias), requestIDFromContext(r.Context())))
			return
		}
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "failed to persist model registry: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	writeJSON(w, http.StatusCreated, modelRegistryResponse{
		Enabled: true,
		Models:  models,
	})
}

// handleUpdateModel updates an existing registry entry by alias.
func (s *Server) handleUpdateModel(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "alias path parameter is required", requestIDFromContext(r.Context())))
		return
	}

	var entry config.ModelRegistryEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("INVALID_REQUEST", "invalid JSON body", requestIDFromContext(r.Context())))
		return
	}

	models, err := s.mutateModels(func(models []config.ModelRegistryEntry) ([]config.ModelRegistryEntry, error) {
		idx := -1
		for i, m := range models {
			if strings.EqualFold(m.Alias, alias) {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, fmt.Errorf("not found: %s", alias)
		}

		if entry.Alias == "" {
			entry.Alias = alias
		}

		if entry.Default {
			for i := range models {
				models[i].Default = false
			}
		}

		models[idx] = entry
		return models, nil
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "not found:") {
			s.writeError(w, http.StatusNotFound, api.NewRequestError("NOT_FOUND", fmt.Sprintf("no model with alias %q found", alias), requestIDFromContext(r.Context())))
			return
		}
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "failed to persist model registry: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	writeJSON(w, http.StatusOK, modelRegistryResponse{
		Enabled: true,
		Models:  models,
	})
}

// handleDeleteModel removes a registry entry by alias.
func (s *Server) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "alias path parameter is required", requestIDFromContext(r.Context())))
		return
	}

	models, err := s.mutateModels(func(models []config.ModelRegistryEntry) ([]config.ModelRegistryEntry, error) {
		idx := -1
		for i, m := range models {
			if strings.EqualFold(m.Alias, alias) {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil, fmt.Errorf("not found: %s", alias)
		}
		return slices.Delete(models, idx, idx+1), nil
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "not found:") {
			s.writeError(w, http.StatusNotFound, api.NewRequestError("NOT_FOUND", fmt.Sprintf("no model with alias %q found", alias), requestIDFromContext(r.Context())))
			return
		}
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "failed to persist model registry: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	writeJSON(w, http.StatusOK, modelRegistryResponse{
		Enabled: true,
		Models:  models,
	})
}

// handleSetDefaultModel sets the given alias as the default, unsetting others.
func (s *Server) handleSetDefaultModel(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("alias")
	if alias == "" {
		s.writeError(w, http.StatusBadRequest, api.NewRequestError("MISSING_FIELDS", "alias path parameter is required", requestIDFromContext(r.Context())))
		return
	}

	models, err := s.mutateModels(func(models []config.ModelRegistryEntry) ([]config.ModelRegistryEntry, error) {
		found := false
		for i := range models {
			if strings.EqualFold(models[i].Alias, alias) {
				models[i].Default = true
				found = true
			} else {
				models[i].Default = false
			}
		}
		if !found {
			return nil, fmt.Errorf("not found: %s", alias)
		}
		return models, nil
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "not found:") {
			s.writeError(w, http.StatusNotFound, api.NewRequestError("NOT_FOUND", fmt.Sprintf("no model with alias %q found", alias), requestIDFromContext(r.Context())))
			return
		}
		s.writeError(w, http.StatusInternalServerError, api.NewRuntimeError("CONFIG_ERROR", "failed to persist model registry: "+err.Error(), requestIDFromContext(r.Context())))
		return
	}

	writeJSON(w, http.StatusOK, modelRegistryResponse{
		Enabled: true,
		Models:  models,
	})
}
