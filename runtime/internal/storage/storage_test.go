package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "gumi.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
}

func TestOpenInMemory(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	if err := s.DB().Ping(); err != nil {
		t.Fatalf("in-memory database ping failed: %v", err)
	}
}

func TestSchemaCreatesRequiredTables(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	tables := []string{
		"runtime_info",
		"requests",
		"pipeline_events",
		"errors",
		"provider_health",
		"validation_reports",
		"repair_reports",
	}

	for _, name := range tables {
		var count int
		err := s.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", name, err)
		}
		if count != 1 {
			t.Errorf("expected table %s to exist", name)
		}
	}
}

func TestSchemaCreatesRecommendedIndexes(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	indexes := []string{
		"idx_requests_created_at",
		"idx_requests_status",
		"idx_requests_provider_model",
		"idx_pipeline_events_request_id",
		"idx_errors_request_id",
		"idx_provider_health_provider_checked_at",
	}

	for _, name := range indexes {
		var count int
		err := s.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check index %s: %v", name, err)
		}
		if count != 1 {
			t.Errorf("expected index %s to exist", name)
		}
	}
}

func TestMigrateAddsThinkingColumnsToExistingRequestsTable(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	if _, err := s.DB().Exec(`DROP TABLE requests`); err != nil {
		t.Fatalf("drop requests failed: %v", err)
	}
	if _, err := s.DB().Exec(`
		CREATE TABLE requests (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			runtime_mode TEXT NOT NULL,
			status TEXT NOT NULL,
			stream INTEGER NOT NULL DEFAULT 0
		);
	`); err != nil {
		t.Fatalf("create old requests table failed: %v", err)
	}

	if err := migrate(s.DB()); err != nil {
		t.Fatalf("migrate old requests table failed: %v", err)
	}

	for _, column := range []string{"thinking_enabled", "reasoning_content_present"} {
		var found bool
		rows, err := s.DB().Query("PRAGMA table_info(requests)")
		if err != nil {
			t.Fatalf("inspect requests columns failed: %v", err)
		}
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull int
			var defaultValue interface{}
			var primaryKey int
			if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &primaryKey); err != nil {
				_ = rows.Close()
				t.Fatalf("scan requests columns failed: %v", err)
			}
			if name == column {
				found = true
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close requests columns rows failed: %v", err)
		}
		if !found {
			t.Fatalf("expected migrated column %s", column)
		}
	}
}

func TestInsertRequestRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	_, err = s.DB().Exec(`
		INSERT INTO requests (id, created_at, workspace_id, runtime_mode, status, stream)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "req_123", "2026-07-10T00:00:00Z", "default", "stabilized", "success", 0)
	if err != nil {
		t.Fatalf("insert request failed: %v", err)
	}

	var status string
	if err := s.DB().QueryRow("SELECT status FROM requests WHERE id=?", "req_123").Scan(&status); err != nil {
		t.Fatalf("select request failed: %v", err)
	}
	if status != "success" {
		t.Errorf("expected status success, got %s", status)
	}
}

func TestInsertPipelineEventRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	res, err := s.DB().Exec(`
		INSERT INTO pipeline_events (request_id, timestamp, engine, event, severity, message)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "req_123", "2026-07-10T00:00:00Z", "pipeline", "request_received", "info", "request received")
	if err != nil {
		t.Fatalf("insert pipeline event failed: %v", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id failed: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero auto-increment id")
	}
}

func TestInsertErrorRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	_, err = s.DB().Exec(`
		INSERT INTO errors (request_id, created_at, code, type, engine, message, retryable)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "req_123", "2026-07-10T00:00:00Z", "PROVIDER_UNAVAILABLE", "provider_error", "provider", "ollama offline", 1)
	if err != nil {
		t.Fatalf("insert error failed: %v", err)
	}

	var retryable int
	if err := s.DB().QueryRow("SELECT retryable FROM errors WHERE request_id=?", "req_123").Scan(&retryable); err != nil {
		t.Fatalf("select error failed: %v", err)
	}
	if retryable != 1 {
		t.Errorf("expected retryable 1, got %d", retryable)
	}
}

func TestInsertProviderHealthRow(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory failed: %v", err)
	}
	defer s.Close()

	_, err = s.DB().Exec(`
		INSERT INTO provider_health (provider, checked_at, status, latency_ms, message, error_code)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "ollama", "2026-07-10T00:00:00Z", "ok", 12, "", "")
	if err != nil {
		t.Fatalf("insert provider health failed: %v", err)
	}

	var status string
	if err := s.DB().QueryRow("SELECT status FROM provider_health WHERE provider=?", "ollama").Scan(&status); err != nil {
		t.Fatalf("select provider health failed: %v", err)
	}
	if status != "ok" {
		t.Errorf("expected status ok, got %s", status)
	}
}

func TestDefaultPathUsesHomeDirectory(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Fatal("DefaultPath returned empty string")
	}
	if filepath.Base(path) != "gumi.db" {
		t.Errorf("expected basename gumi.db, got %s", filepath.Base(path))
	}
}

func TestStorageCloseNil(t *testing.T) {
	var s *Storage
	if err := s.Close(); err != nil {
		t.Fatalf("Close on nil storage should not error: %v", err)
	}
}

func TestStorageDBNil(t *testing.T) {
	var s *Storage
	if s.DB() != nil {
		t.Error("DB on nil storage should return nil")
	}
}
