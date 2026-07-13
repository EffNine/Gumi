package memory

// memorySchema contains all SQL DDL for the memory engine database.
// These statements are executed in order when the memory store is opened.
var memorySchema = []string{
	// Facts table: key-value pairs with metadata for cross-session persistence.
	`CREATE TABLE IF NOT EXISTS facts (
		id          TEXT PRIMARY KEY,
		key         TEXT NOT NULL,
		value       TEXT NOT NULL,
		source      TEXT NOT NULL DEFAULT 'inferred',
		confidence  REAL NOT NULL DEFAULT 0.5,
		session_id  TEXT NOT NULL DEFAULT '',
		created_at  TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
		accessed_at TEXT NOT NULL DEFAULT (datetime('now')),
		access_count INTEGER NOT NULL DEFAULT 1,
		ttl_seconds INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_facts_key ON facts(key)`,
	`CREATE INDEX IF NOT EXISTS idx_facts_session ON facts(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_facts_accessed ON facts(accessed_at)`,

	// Episodes table: compressed step histories with outcomes.
	`CREATE TABLE IF NOT EXISTS episodes (
		id           TEXT PRIMARY KEY,
		session_id   TEXT NOT NULL,
		step         INTEGER NOT NULL,
		task         TEXT NOT NULL,
		difficulty   INTEGER NOT NULL DEFAULT 1,
		model_used   TEXT NOT NULL DEFAULT '',
		outcome      TEXT NOT NULL DEFAULT 'unknown',
		retries      INTEGER NOT NULL DEFAULT 0,
		latency_ms   INTEGER NOT NULL DEFAULT 0,
		tokens_used  INTEGER NOT NULL DEFAULT 0,
		compressed_summary TEXT NOT NULL DEFAULT '',
		errors_encountered TEXT NOT NULL DEFAULT '[]',
		key_facts_extracted TEXT NOT NULL DEFAULT '[]',
		created_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_episodes_session ON episodes(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_episodes_created ON episodes(created_at)`,

	// Model fit table: per-model performance tracking for the router feedback loop.
	`CREATE TABLE IF NOT EXISTS model_fit (
		model_id     TEXT NOT NULL,
		difficulty   INTEGER NOT NULL,
		task_type    TEXT NOT NULL DEFAULT 'general',
		attempts     INTEGER NOT NULL DEFAULT 0,
		successes    INTEGER NOT NULL DEFAULT 0,
		avg_latency_ms  INTEGER NOT NULL DEFAULT 0,
		avg_retries     REAL NOT NULL DEFAULT 0.0,
		repair_rate     REAL NOT NULL DEFAULT 0.0,
		last_updated TEXT NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (model_id, difficulty, task_type)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_model_fit_model ON model_fit(model_id)`,

	// Session metadata for tracking active sessions.
	`CREATE TABLE IF NOT EXISTS sessions (
		session_id    TEXT PRIMARY KEY,
		workspace_id  TEXT NOT NULL DEFAULT '',
		episode_count INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
		summary       TEXT NOT NULL DEFAULT ''
	)`,
}
