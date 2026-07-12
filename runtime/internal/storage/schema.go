package storage

import (
	"database/sql"
	"fmt"
)

// migrate applies the telemetry schema to the database.
func migrate(db *sql.DB) error {
	for i, stmt := range schemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration step %d failed: %w", i+1, err)
		}
	}
	for _, column := range requestColumns {
		if err := ensureColumn(db, "requests", column.name, column.definition); err != nil {
			return err
		}
	}
	for i, stmt := range indexStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("index migration step %d failed: %w", i+1, err)
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table string, column string, definition string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue interface{}
		var primaryKey int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan table %s columns: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table %s columns: %w", table, err)
	}

	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

type columnDefinition struct {
	name       string
	definition string
}

var requestColumns = []columnDefinition{
	{name: "session_id", definition: "TEXT"},
	{name: "provider", definition: "TEXT"},
	{name: "model", definition: "TEXT"},
	{name: "provider_latency_ms", definition: "INTEGER"},
	{name: "prompt_tokens", definition: "INTEGER"},
	{name: "completion_tokens", definition: "INTEGER"},
	{name: "total_tokens", definition: "INTEGER"},
	{name: "context_compressed", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "validation_passed", definition: "INTEGER"},
	{name: "repair_applied", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "retry_count", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "error_code", definition: "TEXT"},
	{name: "prompt_logged", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "response_logged", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "prompt_preview", definition: "TEXT"},
	{name: "response_preview", definition: "TEXT"},
	{name: "thinking_enabled", definition: "TEXT"},
	{name: "reasoning_content_present", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "agent_step_count", definition: "INTEGER NOT NULL DEFAULT 0"},
	{name: "agent_loop_detected", definition: "INTEGER NOT NULL DEFAULT 0"},
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS runtime_info (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS requests (
		id TEXT PRIMARY KEY,
		created_at TEXT NOT NULL,
		workspace_id TEXT NOT NULL,
		session_id TEXT,
		runtime_mode TEXT NOT NULL,
		provider TEXT,
		model TEXT,
		status TEXT NOT NULL,
		stream INTEGER NOT NULL DEFAULT 0,
		latency_ms INTEGER,
		provider_latency_ms INTEGER,
		prompt_tokens INTEGER,
		completion_tokens INTEGER,
		total_tokens INTEGER,
		context_compressed INTEGER NOT NULL DEFAULT 0,
		validation_passed INTEGER,
		repair_applied INTEGER NOT NULL DEFAULT 0,
		retry_count INTEGER NOT NULL DEFAULT 0,
		error_code TEXT,
		prompt_logged INTEGER NOT NULL DEFAULT 0,
		response_logged INTEGER NOT NULL DEFAULT 0,
		prompt_preview TEXT,
		response_preview TEXT,
		thinking_enabled TEXT,
		reasoning_content_present INTEGER NOT NULL DEFAULT 0
	);`,

	`CREATE TABLE IF NOT EXISTS pipeline_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id TEXT NOT NULL,
		timestamp TEXT NOT NULL,
		engine TEXT NOT NULL,
		event TEXT NOT NULL,
		severity TEXT NOT NULL,
		message TEXT,
		metadata_json TEXT
	);`,

	`CREATE TABLE IF NOT EXISTS errors (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id TEXT,
		created_at TEXT NOT NULL,
		code TEXT NOT NULL,
		type TEXT NOT NULL,
		engine TEXT NOT NULL,
		message TEXT NOT NULL,
		retryable INTEGER NOT NULL,
		suggestion TEXT,
		details_json TEXT
	);`,

	`CREATE TABLE IF NOT EXISTS provider_health (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL,
		checked_at TEXT NOT NULL,
		status TEXT NOT NULL,
		latency_ms INTEGER,
		message TEXT,
		error_code TEXT,
		metadata_json TEXT
	);`,

	`CREATE TABLE IF NOT EXISTS validation_reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		passed INTEGER NOT NULL,
		severity TEXT,
		repairable INTEGER,
		suggested_repair_strategy TEXT,
		issues_json TEXT,
		metadata_json TEXT
	);`,

	`CREATE TABLE IF NOT EXISTS repair_reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		attempted INTEGER NOT NULL,
		strategy TEXT,
		success INTEGER,
		changes_json TEXT,
		remaining_issues_json TEXT,
		retry_requested INTEGER NOT NULL DEFAULT 0,
		metadata_json TEXT
	);`,
}

var indexStatements = []string{
	`CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at);`,
	`CREATE INDEX IF NOT EXISTS idx_requests_status ON requests(status);`,
	`CREATE INDEX IF NOT EXISTS idx_requests_provider_model ON requests(provider, model);`,
	`CREATE INDEX IF NOT EXISTS idx_pipeline_events_request_id ON pipeline_events(request_id);`,
	`CREATE INDEX IF NOT EXISTS idx_errors_request_id ON errors(request_id);`,
	`CREATE INDEX IF NOT EXISTS idx_provider_health_provider_checked_at ON provider_health(provider, checked_at);`,
}
