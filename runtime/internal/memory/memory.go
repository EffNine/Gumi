// Package memory provides a persistent, zero-VRAM memory engine for agentic
// coding agents. It stores facts, episode summaries, and model-fit data in
// SQLite, shared across all models and surviving session boundaries.
package memory

import (
	"container/list"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/EffNine/gumi/runtime/internal/api"
	"github.com/EffNine/gumi/runtime/internal/config"
)

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// MemoryFact is a single piece of structured knowledge.
type MemoryFact struct {
	ID          string  `json:"id"`
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Source      string  `json:"source"`
	Confidence  float64 `json:"confidence"`
	SessionID   string  `json:"session_id"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	AccessedAt  string  `json:"accessed_at"`
	AccessCount int     `json:"access_count"`
	TTLSeconds  int     `json:"ttl_seconds"`
}

// MemoryEpisode is a compressed step summary.
type MemoryEpisode struct {
	ID                string   `json:"id"`
	SessionID         string   `json:"session_id"`
	Step              int      `json:"step"`
	Task              string   `json:"task"`
	Difficulty        int      `json:"difficulty"`
	ModelUsed         string   `json:"model_used"`
	Outcome           string   `json:"outcome"`
	Retries           int      `json:"retries"`
	LatencyMs         int64    `json:"latency_ms"`
	TokensUsed        int      `json:"tokens_used"`
	CompressedSummary string   `json:"compressed_summary"`
	ErrorsEncountered []string `json:"errors_encountered"`
	KeyFactsExtracted []string `json:"key_facts_extracted"`
	CreatedAt         string   `json:"created_at"`
}

// ModelFitEntry records performance for one (model, difficulty, task_type) combo.
type ModelFitEntry struct {
	ModelID      string  `json:"model_id"`
	Difficulty   int     `json:"difficulty"`
	TaskType     string  `json:"task_type"`
	Attempts     int     `json:"attempts"`
	Successes    int     `json:"successes"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	AvgRetries   float64 `json:"avg_retries"`
	LastUpdated  string  `json:"last_updated"`
}

// ---------------------------------------------------------------------------
// MemoryEngine
// ---------------------------------------------------------------------------

// MemoryEngine is the top-level memory engine providing facts, episodes,
// model-fit tracking, injection, and extraction. It uses an internal SQLite
// database at the configured path.
type MemoryEngine struct {
	mu  sync.RWMutex
	db  *sql.DB
	cfg *config.MemoryConfig

	// Hot cache for frequently accessed facts (L1).
	hotCache map[string]*MemoryFact

	// hotCacheList is an LRU ordering of hot cache keys. The front is the
	// least-recently-used entry; the back is the most-recently-used.
	hotCacheList *list.List

	// telemetryHook is called for significant memory events (eviction, etc.).
	// Set via SetTelemetryHook after construction.
	telemetryHook func(event string, metadata map[string]string)
}

// SetTelemetryHook attaches a callback for significant memory events.
// The hook is called with event names like "memory_eviction" and a metadata map.
func (m *MemoryEngine) SetTelemetryHook(hook func(event string, metadata map[string]string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.telemetryHook = hook
}

// New creates a MemoryEngine, opening (or creating) the SQLite database at the
// given path and applying the memory schema. If dbPath is empty, memory is
// purely in-memory (for testing).
func New(cfg *config.MemoryConfig, dbPath string) (*MemoryEngine, error) {
	if dbPath == "" {
		db, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
		if err != nil {
			return nil, fmt.Errorf("open in-memory sqlite: %w", err)
		}
		if err := applyMemorySchema(db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply memory schema: %w", err)
		}
		return &MemoryEngine{
			db:           db,
			cfg:          cfg,
			hotCache:     make(map[string]*MemoryFact),
			hotCacheList: list.New(),
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create memory directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("open memory sqlite: %w", err)
	}
	if err := applyMemorySchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply memory schema: %w", err)
	}

	return &MemoryEngine{
		db:           db,
		cfg:          cfg,
		hotCache:     make(map[string]*MemoryFact),
		hotCacheList: list.New(),
	}, nil
}

// Close closes the underlying database connection.
func (m *MemoryEngine) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// DB returns the underlying *sql.DB for direct use (e.g., by dashboard queries).
func (m *MemoryEngine) DB() *sql.DB {
	return m.db
}

// ---------------------------------------------------------------------------
// Fact store
// ---------------------------------------------------------------------------

// StoreFact inserts or updates a fact. If a fact with the same key exists and
// the value is identical, the confidence is bumped if higher. If the value
// differs, both are kept as alternatives — the higher-confidence one is
// primary and the other is stored with an `:alt` suffix.
func (m *MemoryEngine) StoreFact(fact MemoryFact) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fact.ID == "" {
		fact.ID = fmt.Sprintf("fact_%d", time.Now().UnixNano())
	}
	if fact.CreatedAt == "" {
		fact.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	fact.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Check if a fact with the same key already exists.
	var existingID string
	var existingConfidence float64
	var existingValue string
	err := m.db.QueryRow(
		"SELECT id, confidence, value FROM facts WHERE key = ?", fact.Key,
	).Scan(&existingID, &existingConfidence, &existingValue)

	if err == sql.ErrNoRows {
		// Insert new.
		if fact.AccessCount <= 0 {
			fact.AccessCount = 1
		}
		_, err = m.db.Exec(
			`INSERT INTO facts (id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fact.ID, fact.Key, fact.Value, fact.Source, fact.Confidence,
			fact.SessionID, fact.CreatedAt, fact.UpdatedAt, fact.UpdatedAt,
			fact.AccessCount, fact.TTLSeconds,
		)
		return err
	}
	if err != nil {
		return fmt.Errorf("check existing fact: %w", err)
	}

	// Existing fact with same key.
	if fact.Value == existingValue {
		// Same value — just bump confidence if higher.
		if fact.Confidence > existingConfidence {
			_, err = m.db.Exec(`UPDATE facts SET confidence=?, updated_at=? WHERE id=?`,
				fact.Confidence, fact.UpdatedAt, existingID)
		}
		return err
	}

	// Value differs — keep both. Ensure the higher-confidence one is primary.
	if fact.Confidence > existingConfidence {
		// Demote existing to :alt, insert new as primary.
		_, err = m.db.Exec(`UPDATE facts SET key = key || ':alt' WHERE id = ?`, existingID)
		if err != nil {
			return fmt.Errorf("demote existing fact: %w", err)
		}
		if fact.AccessCount <= 0 {
			fact.AccessCount = 1
		}
		_, err = m.db.Exec(
			`INSERT INTO facts (id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fact.ID, fact.Key, fact.Value, fact.Source, fact.Confidence,
			fact.SessionID, fact.CreatedAt, fact.UpdatedAt, fact.UpdatedAt, fact.AccessCount, fact.TTLSeconds)
	} else {
		// New is lower confidence — store as :alt.
		fact.Key = fact.Key + ":alt"
		if fact.AccessCount <= 0 {
			fact.AccessCount = 1
		}
		_, err = m.db.Exec(
			`INSERT INTO facts (id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fact.ID, fact.Key, fact.Value, fact.Source, fact.Confidence,
			fact.SessionID, fact.CreatedAt, fact.UpdatedAt, fact.UpdatedAt, fact.AccessCount, fact.TTLSeconds)
	}

	// Emit telemetry for fact conflicts (value differs from existing).
	if m.telemetryHook != nil {
		m.telemetryHook("fact_conflict", map[string]string{
			"key":            fact.Key,
			"existing_value": existingValue,
			"new_value":      fact.Value,
		})
	}
	return err
}

// hotCacheMaxSize returns the configured hot cache size limit, falling back to
// the default if not set.
func (m *MemoryEngine) hotCacheMaxSize() int {
	if m.cfg != nil && m.cfg.HotCacheMaxSize > 0 {
		return m.cfg.HotCacheMaxSize
	}
	return config.DefaultHotCacheMaxSize
}

// evictHotCacheEntry removes the least-recently-used entry from the hot cache.
// Must be called with m.mu held.
func (m *MemoryEngine) evictHotCacheEntry() {
	elem := m.hotCacheList.Front()
	if elem == nil {
		return
	}
	key := elem.Value.(string)
	delete(m.hotCache, key)
	m.hotCacheList.Remove(elem)

	if m.telemetryHook != nil {
		m.telemetryHook("hot_cache_eviction", map[string]string{
			"key": key,
		})
	}
}

// promoteHotCacheKey moves the given key to the back (most-recently-used end)
// of the LRU list. Must be called with m.mu held.
func (m *MemoryEngine) promoteHotCacheKey(key string) {
	for e := m.hotCacheList.Front(); e != nil; e = e.Next() {
		if e.Value.(string) == key {
			m.hotCacheList.MoveToBack(e)
			return
		}
	}
}

// insertHotCache adds a fact to the hot cache, enforcing the LRU size limit.
// Must be called with m.mu held.
func (m *MemoryEngine) insertHotCache(key string, f *MemoryFact) {
	m.hotCache[key] = f
	m.hotCacheList.PushBack(key)

	// Evict oldest if over capacity.
	maxSize := m.hotCacheMaxSize()
	for m.hotCacheList.Len() > maxSize {
		m.evictHotCacheEntry()
	}
}

// GetFact retrieves a fact by key. Updates access_count and accessed_at.
func (m *MemoryEngine) GetFact(key string) (*MemoryFact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check hot cache first.
	if cached, ok := m.hotCache[key]; ok {
		go m.touchFact(cached.ID) // async update access time
		m.promoteHotCacheKey(key)
		factCopy := *cached // copy to avoid data race on the pointer
		return &factCopy, nil
	}

	var f MemoryFact
	err := m.db.QueryRow(
		`SELECT id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds
		 FROM facts WHERE key = ?`, key,
	).Scan(&f.ID, &f.Key, &f.Value, &f.Source, &f.Confidence, &f.SessionID,
		&f.CreatedAt, &f.UpdatedAt, &f.AccessedAt, &f.AccessCount, &f.TTLSeconds)
	if err != nil {
		return nil, err
	}

	// Update access tracking.
	_, _ = m.db.Exec(
		`UPDATE facts SET access_count = access_count + 1, accessed_at = datetime('now') WHERE id = ?`,
		f.ID,
	)
	f.AccessCount++

	// Add to hot cache if frequently accessed.
	if f.AccessCount >= 3 {
		m.insertHotCache(key, &f)
	}

	return &f, nil
}

// SearchFacts searches facts by key or value, returning ranked results.
func (m *MemoryEngine) SearchFacts(query string, limit int) ([]MemoryFact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := m.db.Query(
		`SELECT id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds
		 FROM facts WHERE key LIKE ? OR value LIKE ?
		 ORDER BY confidence DESC, access_count DESC LIMIT ?`,
		"%"+query+"%", "%"+query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search facts: %w", err)
	}
	defer rows.Close()

	var facts []MemoryFact
	for rows.Next() {
		var f MemoryFact
		if err := rows.Scan(&f.ID, &f.Key, &f.Value, &f.Source, &f.Confidence,
			&f.SessionID, &f.CreatedAt, &f.UpdatedAt, &f.AccessedAt,
			&f.AccessCount, &f.TTLSeconds); err != nil {
			return nil, fmt.Errorf("scan fact: %w", err)
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

// DeleteFact removes a fact by key.
func (m *MemoryEngine) DeleteFact(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.hotCache, key)
	m.removeFromHotCacheList(key)
	_, err := m.db.Exec("DELETE FROM facts WHERE key = ?", key)
	return err
}

// removeFromHotCacheList removes a key from the LRU list if present.
// Must be called with m.mu held.
func (m *MemoryEngine) removeFromHotCacheList(key string) {
	for e := m.hotCacheList.Front(); e != nil; e = e.Next() {
		if e.Value.(string) == key {
			m.hotCacheList.Remove(e)
			return
		}
	}
}

// ListFacts returns all facts, ordered by recency.
func (m *MemoryEngine) ListFacts(limit int) ([]MemoryFact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := m.db.Query(
		`SELECT id, key, value, source, confidence, session_id, created_at, updated_at, accessed_at, access_count, ttl_seconds
		 FROM facts ORDER BY updated_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list facts: %w", err)
	}
	defer rows.Close()

	var facts []MemoryFact
	for rows.Next() {
		var f MemoryFact
		if err := rows.Scan(&f.ID, &f.Key, &f.Value, &f.Source, &f.Confidence,
			&f.SessionID, &f.CreatedAt, &f.UpdatedAt, &f.AccessedAt,
			&f.AccessCount, &f.TTLSeconds); err != nil {
			return nil, fmt.Errorf("scan fact: %w", err)
		}
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

func (m *MemoryEngine) touchFact(id string) {
	m.mu.Lock()
	_, _ = m.db.Exec(
		`UPDATE facts SET access_count = access_count + 1, accessed_at = datetime('now') WHERE id = ?`,
		id,
	)
	m.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Episodes
// ---------------------------------------------------------------------------

// StoreEpisode appends an episode to the history.
func (m *MemoryEngine) StoreEpisode(ep MemoryEpisode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ep.ID == "" {
		ep.ID = fmt.Sprintf("ep_%d", time.Now().UnixNano())
	}
	if ep.CreatedAt == "" {
		ep.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	errorsJSON := "[]"
	if len(ep.ErrorsEncountered) > 0 {
		errorsJSON = `["` + strings.Join(ep.ErrorsEncountered, `","`) + `"]`
	}
	factsJSON := "[]"
	if len(ep.KeyFactsExtracted) > 0 {
		factsJSON = `["` + strings.Join(ep.KeyFactsExtracted, `","`) + `"]`
	}

	_, err := m.db.Exec(
		`INSERT INTO episodes (id, session_id, step, task, difficulty, model_used, outcome, retries, latency_ms, tokens_used, compressed_summary, errors_encountered, key_facts_extracted, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ep.ID, ep.SessionID, ep.Step, ep.Task, ep.Difficulty, ep.ModelUsed,
		ep.Outcome, ep.Retries, ep.LatencyMs, ep.TokensUsed,
		ep.CompressedSummary, errorsJSON, factsJSON, ep.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store episode: %w", err)
	}

	// Update session episode count.
	_, _ = m.db.Exec(
		`INSERT INTO sessions (session_id, episode_count, created_at, updated_at)
		 VALUES (?, 1, datetime('now'), datetime('now'))
		 ON CONFLICT(session_id) DO UPDATE SET episode_count = episode_count + 1, updated_at = datetime('now')`,
		ep.SessionID,
	)

	return nil
}

// GetRecentEpisodes returns the N most recent episodes for a session.
func (m *MemoryEngine) GetRecentEpisodes(sessionID string, n int) ([]MemoryEpisode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n <= 0 {
		n = 10
	}

	rows, err := m.db.Query(
		`SELECT id, session_id, step, task, difficulty, model_used, outcome, retries, latency_ms, tokens_used, compressed_summary, errors_encountered, key_facts_extracted, created_at
		 FROM episodes WHERE session_id = ? ORDER BY created_at DESC LIMIT ?`,
		sessionID, n,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent episodes: %w", err)
	}
	defer rows.Close()

	var episodes []MemoryEpisode
	for rows.Next() {
		var ep MemoryEpisode
		var errorsStr, factsStr string
		if err := rows.Scan(&ep.ID, &ep.SessionID, &ep.Step, &ep.Task,
			&ep.Difficulty, &ep.ModelUsed, &ep.Outcome, &ep.Retries,
			&ep.LatencyMs, &ep.TokensUsed, &ep.CompressedSummary,
			&errorsStr, &factsStr, &ep.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		ep.ErrorsEncountered = parseJSONList(errorsStr)
		ep.KeyFactsExtracted = parseJSONList(factsStr)
		episodes = append(episodes, ep)
	}
	return episodes, rows.Err()
}

// SummarizeEpisodes returns a compressed string of recent episode summaries.
func (m *MemoryEngine) SummarizeEpisodes(sessionID string, maxEpisodes int) (string, error) {
	episodes, err := m.GetRecentEpisodes(sessionID, maxEpisodes)
	if err != nil {
		return "", err
	}
	if len(episodes) == 0 {
		return "", nil
	}

	// Reverse so oldest first.
	for i, j := 0, len(episodes)-1; i < j; i, j = i+1, j-1 {
		episodes[i], episodes[j] = episodes[j], episodes[i]
	}

	var b strings.Builder
	for _, ep := range episodes {
		icon := "✓"
		if ep.Outcome != "success" {
			icon = "✗"
		}
		fmt.Fprintf(&b, "Step %d: %s %s (retries: %d)\n", ep.Step, icon, ep.CompressedSummary, ep.Retries)
	}
	return b.String(), nil
}

// ---------------------------------------------------------------------------
// Model fit (router feedback)
// ---------------------------------------------------------------------------

// RecordOutcome records a model's performance for a given task.
func (m *MemoryEngine) RecordOutcome(modelID string, difficulty int, taskType string, success bool, latencyMs int64, retries int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if taskType == "" {
		taskType = "general"
	}

	var entry ModelFitEntry
	err := m.db.QueryRow(
		`SELECT attempts, successes, avg_latency_ms, avg_retries
		 FROM model_fit WHERE model_id = ? AND difficulty = ? AND task_type = ?`,
		modelID, difficulty, taskType,
	).Scan(&entry.Attempts, &entry.Successes, &entry.AvgLatencyMs, &entry.AvgRetries)

	if err == sql.ErrNoRows {
		// First observation.
		successes := 0
		if success {
			successes = 1
		}
		_, err = m.db.Exec(
			`INSERT INTO model_fit (model_id, difficulty, task_type, attempts, successes, avg_latency_ms, avg_retries, last_updated)
			 VALUES (?, ?, ?, 1, ?, ?, ?, datetime('now'))`,
			modelID, difficulty, taskType, successes, latencyMs, float64(retries),
		)
		return err
	}
	if err != nil {
		return fmt.Errorf("query model fit: %w", err)
	}

	// Update with exponential decay weighting.
	decay := m.cfg.ModelFitDecay
	if decay <= 0 {
		decay = 0.95
	}
	newAttempts := entry.Attempts + 1
	newSuccesses := entry.Successes
	if success {
		newSuccesses++
	}

	// Weighted moving average for latency and retries.
	weight := decay / (1 - math.Pow(decay, float64(newAttempts)))
	ewmaLatency := int64(weight*float64(latencyMs) + (1-weight)*float64(entry.AvgLatencyMs))
	ewmaRetries := weight*float64(retries) + (1-weight)*entry.AvgRetries

	_, err = m.db.Exec(
		`UPDATE model_fit SET attempts = ?, successes = ?, avg_latency_ms = ?, avg_retries = ?, last_updated = datetime('now')
		 WHERE model_id = ? AND difficulty = ? AND task_type = ?`,
		newAttempts, newSuccesses, ewmaLatency, ewmaRetries,
		modelID, difficulty, taskType,
	)
	return err
}

// GetBestModel returns the model with the highest success rate for a given
// difficulty and task type, with a minimum attempt threshold.
func (m *MemoryEngine) GetBestModel(difficulty int, taskType string, minAttempts int) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if taskType == "" {
		taskType = "general"
	}
	if minAttempts <= 0 {
		minAttempts = 3
	}

	var modelID string
	err := m.db.QueryRow(
		`SELECT model_id FROM model_fit
		 WHERE difficulty = ? AND task_type = ? AND attempts >= ?
		 ORDER BY CAST(successes AS REAL) / MAX(attempts, 1) DESC, avg_latency_ms ASC
		 LIMIT 1`,
		difficulty, taskType, minAttempts,
	).Scan(&modelID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get best model: %w", err)
	}
	return modelID, nil
}

// GetBestModelForRouter returns the best model ID and success rate for a given
// difficulty and task type. It matches the ModelFitLookup interface used by
// the router engine. Returns ("", 0, false) if no model has enough data.
func (m *MemoryEngine) GetBestModelForRouter(difficulty int, taskType string) (modelID string, successRate float64, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if taskType == "" {
		taskType = "general"
	}

	var id string
	var attempts, successes int
	err := m.db.QueryRow(
		`SELECT model_id, attempts, successes FROM model_fit
		 WHERE difficulty = ? AND task_type = ? AND attempts >= 3
		 ORDER BY CAST(successes AS REAL) / MAX(attempts, 1) DESC, avg_latency_ms ASC
		 LIMIT 1`,
		difficulty, taskType,
	).Scan(&id, &attempts, &successes)
	if err == sql.ErrNoRows {
		return "", 0, false
	}
	if err != nil {
		return "", 0, false
	}
	rate := float64(successes) / float64(attempts)
	return id, rate, true
}

// GetModelProfile returns the aggregated stats for a model across all
// difficulty/task_type combinations.
func (m *MemoryEngine) GetModelProfile(modelID string) ([]ModelFitEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(
		`SELECT model_id, difficulty, task_type, attempts, successes, avg_latency_ms, avg_retries, last_updated
		 FROM model_fit WHERE model_id = ? ORDER BY difficulty ASC, task_type ASC`,
		modelID,
	)
	if err != nil {
		return nil, fmt.Errorf("get model profile: %w", err)
	}
	defer rows.Close()

	var entries []ModelFitEntry
	for rows.Next() {
		var e ModelFitEntry
		if err := rows.Scan(&e.ModelID, &e.Difficulty, &e.TaskType,
			&e.Attempts, &e.Successes, &e.AvgLatencyMs, &e.AvgRetries,
			&e.LastUpdated); err != nil {
			return nil, fmt.Errorf("scan model fit: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListModelFit returns all model-fit data for the dashboard.
func (m *MemoryEngine) ListModelFit() ([]ModelFitEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(
		`SELECT model_id, difficulty, task_type, attempts, successes, avg_latency_ms, avg_retries, last_updated
		 FROM model_fit ORDER BY model_id ASC, difficulty ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list model fit: %w", err)
	}
	defer rows.Close()

	var entries []ModelFitEntry
	for rows.Next() {
		var e ModelFitEntry
		if err := rows.Scan(&e.ModelID, &e.Difficulty, &e.TaskType,
			&e.Attempts, &e.Successes, &e.AvgLatencyMs, &e.AvgRetries,
			&e.LastUpdated); err != nil {
			return nil, fmt.Errorf("scan model fit: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ---------------------------------------------------------------------------
// Memory injection
// ---------------------------------------------------------------------------

// FormatInjection formats facts, episodes, and model-fit data into a system
// message block that fits within the token budget.
// Output order: (1) Model Performance, (2) This Session, (3) Project Knowledge.
func (m *MemoryEngine) FormatInjection(ctx context.Context, facts []MemoryFact, episodeSummary string, fitData []ModelFitEntry, budget int) string {
	if budget <= 0 {
		budget = 1200
	}

	var b strings.Builder
	b.WriteString("[Memory]\n")

	// 1. Model fit data (router feedback) — highest priority.
	if len(fitData) > 0 {
		b.WriteString("--- Model Performance ---\n")
		for _, entry := range fitData {
			rate := 0.0
			if entry.Attempts > 0 {
				rate = float64(entry.Successes) / float64(entry.Attempts) * 100
			}
			line := fmt.Sprintf("%s (diff %d, %s): %.0f%% success (%d/%d)\n",
				entry.ModelID, entry.Difficulty, entry.TaskType, rate,
				entry.Successes, entry.Attempts)
			if estimateTokens(line) <= budget {
				b.WriteString(line)
			}
		}
	}

	// 2. Episode summaries (this session).
	if episodeSummary != "" {
		b.WriteString("--- This Session ---\n")
		lineTokens := estimateTokens(episodeSummary)
		if lineTokens <= budget {
			b.WriteString(episodeSummary)
		}
	}

	// 3. Cross-session facts (project knowledge).
	if len(facts) > 0 {
		b.WriteString("--- Project Knowledge ---\n")
		tokens := 0
		for _, f := range facts {
			line := fmt.Sprintf("%s: %s\n", f.Key, f.Value)
			lineTokens := estimateTokens(line)
			if tokens+lineTokens > budget {
				break
			}
			b.WriteString(line)
			tokens += lineTokens
		}
	}

	return b.String()
}

// SelectRelevantFacts finds the most relevant facts for a given request context.
func (m *MemoryEngine) SelectRelevantFacts(requestText string, maxFacts int) []MemoryFact {
	if maxFacts <= 0 {
		maxFacts = 20
	}

	facts, err := m.ListFacts(100)
	if err != nil || len(facts) == 0 {
		return nil
	}

	// Score each fact for relevance.
	type scored struct {
		fact  MemoryFact
		score float64
	}
	var scoredFacts []scored

	requestLower := strings.ToLower(requestText)
	for _, f := range facts {
		score := 0.0

		// Boost if fact key appears in request text.
		if strings.Contains(requestLower, strings.ToLower(f.Key)) {
			score += 0.5
		}
		// Boost if fact value appears in request text.
		if strings.Contains(requestLower, strings.ToLower(f.Value)) {
			score += 0.3
		}
		// Confidence factor.
		score *= f.Confidence
		// Access frequency boost (frequently accessed = important).
		score *= 1.0 + math.Log2(float64(f.AccessCount+1))*0.1
		// Recency factor: recency = 1.0 / (1.0 + hours_since(accessed_at))
		if f.AccessedAt != "" {
			accessed, perr := time.Parse(time.RFC3339, f.AccessedAt)
			if perr == nil {
				hours := time.Since(accessed).Hours()
				if hours < 0 {
					hours = 0
				}
				recency := 1.0 / (1.0 + hours)
				score *= 0.5 + 0.5*recency // blend so recency doesn't dominate
			}
		}

		if score > 0 {
			scoredFacts = append(scoredFacts, scored{fact: f, score: score})
		}
	}

	// Sort by score descending.
	sort.Slice(scoredFacts, func(i, j int) bool {
		return scoredFacts[i].score > scoredFacts[j].score
	})

	if len(scoredFacts) > maxFacts {
		scoredFacts = scoredFacts[:maxFacts]
	}

	result := make([]MemoryFact, 0, len(scoredFacts))
	for _, s := range scoredFacts {
		result = append(result, s.fact)
	}
	return result
}

// ---------------------------------------------------------------------------
// Fact extraction
// ---------------------------------------------------------------------------

// ExtractFactsFromResponse extracts structured facts from a model response
// using structural patterns — no model inference needed.
func (m *MemoryEngine) ExtractFactsFromResponse(request api.ChatCompletionRequest, response *api.ChatCompletionResponse, sessionID string) []MemoryFact {
	if response == nil || len(response.Choices) == 0 {
		return nil
	}

	contentStr, ok := response.Choices[0].Message.Content.(string)
	if !ok || contentStr == "" {
		return nil
	}

	var facts []MemoryFact
	seen := map[string]bool{}

	// Pattern: File paths (e.g., "path/to/file.go").
	for _, match := range extractFilePaths(contentStr) {
		key := "file:" + match
		if seen[key] {
			continue
		}
		seen[key] = true
		facts = append(facts, MemoryFact{
			Key:        key,
			Value:      match,
			Source:     "extracted_from_response",
			Confidence: 0.6,
			SessionID:  sessionID,
		})
	}

	// Pattern: Error messages.
	for _, match := range extractErrors(contentStr) {
		key := "error:" + match
		if seen[key] {
			continue
		}
		seen[key] = true
		facts = append(facts, MemoryFact{
			Key:        key,
			Value:      match,
			Source:     "extracted_from_response",
			Confidence: 0.7,
			SessionID:  sessionID,
		})
	}

	// Pattern: Import statements.
	for _, match := range extractImports(contentStr) {
		key := "import:" + match
		if seen[key] {
			continue
		}
		seen[key] = true
		facts = append(facts, MemoryFact{
			Key:        key,
			Value:      match,
			Source:     "extracted_from_response",
			Confidence: 0.8,
			SessionID:  sessionID,
		})
	}

	return facts
}

// extractFilePaths finds file-like paths in text.
// Absolute paths (starting with /) and URLs (starting with http) are excluded
// because they are typically system paths, OS error messages, or build-tool
// paths rather than project files the agent should remember.
func extractFilePaths(text string) []string {
	var paths []string
	seen := map[string]bool{}
	// Look for patterns like: path/to/file.ext, path/to/file.go:42, etc.
	words := strings.Fields(text)
	for _, w := range words {
		w = strings.Trim(w, `"',.;:()[]{}\n`)
		if strings.Contains(w, "/") && strings.Contains(w, ".") {
			// Basic heuristic: has path separator and extension.
			if !strings.HasPrefix(w, "http") && !strings.HasPrefix(w, "/") {
				if _, ok := seen[w]; !ok {
					paths = append(paths, w)
					seen[w] = true
				}
			}
		}
	}
	return paths
}

// extractErrors finds error-like patterns in text.
func extractErrors(text string) []string {
	var errors []string
	seen := map[string]bool{}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "panic") ||
			strings.Contains(lower, "failed") || strings.Contains(lower, "exception") {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 10 && len(trimmed) < 200 {
				if _, ok := seen[trimmed]; !ok {
					errors = append(errors, trimmed)
					seen[trimmed] = true
				}
			}
		}
	}
	return errors
}

// extractImports finds import-like patterns in text.
func extractImports(text string) []string {
	var imports []string
	seen := map[string]bool{}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "require(") ||
			strings.Contains(trimmed, "from ") && strings.Contains(trimmed, "import") {
			if _, ok := seen[trimmed]; !ok {
				imports = append(imports, trimmed)
				seen[trimmed] = true
			}
		}
	}
	return imports
}

// ---------------------------------------------------------------------------
// Garbage collection
// ---------------------------------------------------------------------------

// GarbageCollectExpired removes expired facts and old episodes.
func (m *MemoryEngine) GarbageCollectExpired() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove TTL-expired facts.
	res, err := m.db.Exec(
		`DELETE FROM facts WHERE ttl_seconds > 0 AND
		 datetime('now') > datetime(created_at, '+' || CAST(ttl_seconds AS TEXT) || ' seconds')`,
	)
	if err != nil {
		return 0, fmt.Errorf("gc facts: %w", err)
	}
	factCount, _ := res.RowsAffected()

	// Evict LRU facts if over max_facts limit.
	maxFacts := m.cfg.MaxFacts
	if maxFacts <= 0 {
		maxFacts = 10000
	}

	// Select to-be-evicted facts first for telemetry.
	overLimit := 0
	_ = m.db.QueryRow("SELECT MAX(0, (SELECT COUNT(*) FROM facts) - ?)", maxFacts).Scan(&overLimit)
	if overLimit > 0 {
		rows, err := m.db.Query(
			`SELECT id, key FROM facts ORDER BY access_count ASC, accessed_at ASC LIMIT ?`, overLimit,
		)
		if err == nil {
			for rows.Next() {
				var id, key string
				if rows.Scan(&id, &key) == nil && m.telemetryHook != nil {
					m.telemetryHook("memory_eviction", map[string]string{
						"fact_key": key,
						"reason":   "lru_max_facts",
					})
				}
			}
			rows.Close()
		}
	}

	_, _ = m.db.Exec(
		`DELETE FROM facts WHERE id IN (
			SELECT id FROM facts ORDER BY access_count ASC, accessed_at ASC LIMIT MAX(0, (SELECT COUNT(*) FROM facts) - ?)
		)`, maxFacts,
	)

	// Reset hot cache after GC; let it repopulate on access.
	m.hotCache = make(map[string]*MemoryFact)
	m.hotCacheList.Init()

	// Model fit retention: delete old entries.
	retainDays := m.cfg.ModelFitRetentionDays
	if retainDays <= 0 {
		retainDays = 30
	}
	if _, err := m.db.Exec(
		`DELETE FROM model_fit WHERE datetime('now') > datetime(last_updated, '+' || CAST(? AS TEXT) || ' days')`, retainDays,
	); err != nil {
		return factCount, fmt.Errorf("gc model fit: %w", err)
	}

	return factCount, nil
}

// ClearAll removes all memory data. Use with caution.
func (m *MemoryEngine) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hotCache = make(map[string]*MemoryFact)
	m.hotCacheList.Init()
	_, err := m.db.Exec("DELETE FROM facts")
	if err != nil {
		return err
	}
	_, err = m.db.Exec("DELETE FROM episodes")
	if err != nil {
		return err
	}
	_, err = m.db.Exec("DELETE FROM model_fit")
	if err != nil {
		return err
	}
	_, err = m.db.Exec("DELETE FROM sessions")
	return err
}

// ClearSession removes all episodes and the session row for the given session
// ID, but keeps facts (which are cross-session). This is used for per-request
// session reset via gumi.memory.reset_session.
func (m *MemoryEngine) ClearSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.db.Exec("DELETE FROM episodes WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("clear session episodes: %w", err)
	}
	if _, err := m.db.Exec("DELETE FROM sessions WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("clear session row: %w", err)
	}
	return nil
}

// ObservationCount returns how many times a fact key has been observed (but
// not necessarily stored). Used for MinObservationCount gating.
func (m *MemoryEngine) ObservationCount(key string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int
	err := m.db.QueryRow(
		"SELECT count FROM fact_observations WHERE key = ?", key,
	).Scan(&count)
	if err == sql.ErrNoRows {
		return 0
	}
	return count
}

// IncrementObservation increments the observation counter for a fact key.
// This is called before a fact is stored, to track how many times it has been
// observed. Returns the new count.
func (m *MemoryEngine) IncrementObservation(key string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(
		`INSERT INTO fact_observations (key, count, last_seen) VALUES (?, 1, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET count = count + 1, last_seen = datetime('now')`,
		key,
	)
	if err != nil {
		return 0, fmt.Errorf("increment observation: %w", err)
	}

	var count int
	_ = m.db.QueryRow("SELECT count FROM fact_observations WHERE key = ?", key).Scan(&count)
	return count, nil
}

// ObserveAndCheck atomically increments the observation counter for a fact key
// and returns true if the observation count has reached the minimum threshold.
// This avoids the read-then-write race of calling ObservationCount then
// IncrementObservation separately.
func (m *MemoryEngine) ObserveAndCheck(key string, minCount int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(
		`INSERT INTO fact_observations (key, count, last_seen) VALUES (?, 1, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET count = count + 1, last_seen = datetime('now')`,
		key,
	)
	if err != nil {
		return false, fmt.Errorf("increment observation: %w", err)
	}

	var count int
	_ = m.db.QueryRow("SELECT count FROM fact_observations WHERE key = ?", key).Scan(&count)
	return count >= minCount, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

// applyMemorySchema runs all DDL statements to create memory tables.
// Index creation failures are logged but non-fatal — the core tables are usable
// without them, and indexes will be created next time if the schema changes.
func applyMemorySchema(db *sql.DB) error {
	for i, stmt := range memorySchema {
		if _, err := db.Exec(stmt); err != nil {
			if strings.HasPrefix(stmt, "CREATE INDEX") {
				continue // index creation is non-critical
			}
			return fmt.Errorf("schema step %d: %w", i+1, err)
		}
	}
	return nil
}

// estimateTokens is a rough token estimator (4 chars per token).
func estimateTokens(s string) int {
	return (len(s) + 3) / 4
}

// parseJSONList parses a JSON string array like `["a","b"]` using proper JSON
// deserialization so that elements containing commas or quotes are handled correctly.
func parseJSONList(s string) []string {
	var result []string
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil
	}
	return result
}
