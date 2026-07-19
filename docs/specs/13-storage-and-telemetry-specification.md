# Gumi Storage & Telemetry Specification

Version: 1.0  
Status: Draft  
Scope: Local storage, telemetry, request history, privacy rules, and observability data for Gumi Runtime V1

---

# 1. Purpose

This document defines how Gumi stores local runtime data and telemetry.

Storage and telemetry support:

- local observability
- request debugging
- dashboard metrics
- CLI diagnostics
- runtime health checks
- future session and memory features

Gumi telemetry must be local-first and privacy-first.

---

# 2. Core Philosophy

Gumi should be observable without being invasive.

The runtime should record enough metadata to help users debug issues, but it must not store sensitive prompt or response content by default.

Default rule:

```text
Store metadata.
Do not store full prompt.
Do not store full response.
Do not send external telemetry.
```

---

# 3. Storage Goals

V1 storage must support:

- runtime settings cache
- provider health history
- request metadata
- pipeline events
- telemetry metrics
- validation reports
- repair reports
- error records
- recent logs
- basic session tracking

---

# 4. Storage Non-Goals for V1

V1 does not need:

- cloud sync
- hosted database
- team data sharing
- enterprise audit log
- distributed storage
- vector database
- long-term memory graph
- remote telemetry export

These can be added later.

---

# 5. Default Storage Engine

V1 should use:

```text
SQLite
```

Why SQLite:

- local-first
- lightweight
- no server required
- easy backup
- cross-platform
- sufficient for V1 telemetry
- simple for dashboard queries

---

# 6. Storage Location

Default storage directory:

```text
~/.gumi/
```

Suggested structure:

```text
~/.gumi/
├── gumi.yaml
├── gumi.db
├── logs/
│   └── gumi.log
├── profiles/
├── plugins/
├── cache/
└── sessions/
```

Project-local mode may use:

```text
./.gumi/
```

---

# 7. Database File

Default database:

```text
~/.gumi/gumi.db
```

Config override:

```yaml
storage:
  database_path: ~/.gumi/gumi.db
```

---

# 8. Storage Configuration

```yaml
storage:
  engine: sqlite
  database_path: ~/.gumi/gumi.db
  retain_days: 14
  max_size_mb: 500
  vacuum_on_startup: false

telemetry:
  local: true
  external: false
  log_prompts: false
  log_responses: false
  retain_days: 14
```

---

# 9. Privacy Defaults

Default privacy settings:

```yaml
telemetry:
  local: true
  external: false
  log_prompts: false
  log_responses: false
```

This means Gumi stores metadata only.

Full prompt and response content are not stored unless the user explicitly enables them.

---

# 10. Sensitive Data Rules

Gumi must redact:

- API keys
- provider tokens
- authorization headers
- full prompts by default
- full responses by default
- memory content by default
- filesystem paths if configured as sensitive
- environment variable secrets

Redacted value:

```text
***REDACTED***
```

---

# 11. Telemetry Categories

Telemetry should be grouped into:

```text
request telemetry
pipeline telemetry
provider telemetry
context telemetry
prompt telemetry
validation telemetry
repair telemetry
plugin telemetry
system telemetry
```

---

# 12. Required Tables

V1 SQLite tables:

```text
runtime_info
requests
pipeline_events
provider_events
validation_reports
repair_reports
errors
logs
sessions
provider_health
```

Optional V1 tables:

```text
model_profiles
config_snapshots
benchmarks
```

Future tables:

```text
memory_items
memory_links
plugin_registry
workspaces
```

---

# 13. Table: runtime_info

Stores runtime metadata.

```sql
CREATE TABLE runtime_info (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

Example keys:

```text
version
started_at
last_shutdown_at
config_schema_version
database_schema_version
```

---

# 14. Table: requests

Stores one row per API request.

```sql
CREATE TABLE requests (
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
  response_preview TEXT
);
```

## 14.1 Notes

`prompt_preview` and `response_preview` should be short and optional.

Default:

```text
null
```

If preview is enabled, limit to safe small length.

Example:

```text
first 120 characters
```

---

# 15. Table: pipeline_events

Stores detailed request lifecycle events.

```sql
CREATE TABLE pipeline_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL,
  timestamp TEXT NOT NULL,
  engine TEXT NOT NULL,
  event TEXT NOT NULL,
  severity TEXT NOT NULL,
  message TEXT,
  metadata_json TEXT,
  FOREIGN KEY (request_id) REFERENCES requests(id)
);
```

---

# 16. Table: provider_events

Stores provider-specific events.

```sql
CREATE TABLE provider_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT,
  timestamp TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT,
  event TEXT NOT NULL,
  status TEXT,
  latency_ms INTEGER,
  error_code TEXT,
  metadata_json TEXT
);
```

---

# 17. Table: provider_health

Stores provider health check history.

```sql
CREATE TABLE provider_health (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  provider TEXT NOT NULL,
  checked_at TEXT NOT NULL,
  status TEXT NOT NULL,
  latency_ms INTEGER,
  message TEXT,
  error_code TEXT,
  metadata_json TEXT
);
```

---

# 18. Table: validation_reports

Stores validation result metadata.

```sql
CREATE TABLE validation_reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  passed INTEGER NOT NULL,
  severity TEXT,
  repairable INTEGER,
  suggested_repair_strategy TEXT,
  issues_json TEXT,
  metadata_json TEXT,
  FOREIGN KEY (request_id) REFERENCES requests(id)
);
```

---

# 19. Table: repair_reports

Stores repair result metadata.

```sql
CREATE TABLE repair_reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  attempted INTEGER NOT NULL,
  strategy TEXT,
  success INTEGER,
  changes_json TEXT,
  remaining_issues_json TEXT,
  retry_requested INTEGER NOT NULL DEFAULT 0,
  metadata_json TEXT,
  FOREIGN KEY (request_id) REFERENCES requests(id)
);
```

---

# 20. Table: errors

Stores structured runtime errors.

```sql
CREATE TABLE errors (
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
);
```

---

# 21. Table: logs

Stores recent structured logs if configured.

```sql
CREATE TABLE logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT NOT NULL,
  level TEXT NOT NULL,
  engine TEXT,
  request_id TEXT,
  message TEXT NOT NULL,
  metadata_json TEXT
);
```

---

# 22. Table: sessions

Stores basic session metadata.

```sql
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  workspace_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  title TEXT,
  message_count INTEGER NOT NULL DEFAULT 0,
  summary TEXT,
  persistent INTEGER NOT NULL DEFAULT 0
);
```

## 22.1 Session Privacy

Session summary should be disabled unless session persistence is enabled.

If enabled, summary content is local only.

---

# 23. Optional Table: model_profiles

Caches loaded model profiles.

```sql
CREATE TABLE model_profiles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  family TEXT,
  size TEXT,
  version INTEGER,
  source TEXT,
  valid INTEGER NOT NULL,
  loaded_at TEXT NOT NULL,
  warnings_json TEXT,
  metadata_json TEXT
);
```

---

# 24. Optional Table: config_snapshots

Stores config snapshots for debugging.

```sql
CREATE TABLE config_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT NOT NULL,
  request_id TEXT,
  config_json TEXT NOT NULL
);
```

Config snapshots must redact secrets before storing.

---

# 25. Optional Table: benchmarks

Stores benchmark results.

```sql
CREATE TABLE benchmarks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT NOT NULL,
  provider TEXT,
  model TEXT,
  runtime_mode TEXT,
  gateway_latency_ms INTEGER,
  context_latency_ms INTEGER,
  prompt_latency_ms INTEGER,
  validation_latency_ms INTEGER,
  provider_latency_ms INTEGER,
  total_latency_ms INTEGER,
  tokens_per_second REAL,
  metadata_json TEXT
);
```

---

# 26. Request Status Values

Allowed request status:

```text
success
failed
repaired
retried
streaming
cancelled
```

---

# 27. Event Severity Values

Allowed severity:

```text
debug
info
warning
error
fatal
```

---

# 28. Error Storage Rule

Errors must be stored even when telemetry is minimal.

However, error details must be redacted.

---

# 29. Prompt and Response Logging

## 29.1 Default

```text
Do not store full prompt.
Do not store full response.
```

## 29.2 If Enabled

Config:

```yaml
telemetry:
  log_prompts: true
  log_responses: true
```

Then Gumi may store full content in future dedicated tables.

Suggested future tables:

```text
request_prompts
request_responses
```

V1 can avoid full content storage entirely.

---

# 30. Preview Logging

Optional safe previews:

```yaml
telemetry:
  prompt_preview_chars: 0
  response_preview_chars: 0
```

Default:

```text
0
```

If user sets:

```yaml
telemetry:
  prompt_preview_chars: 120
  response_preview_chars: 120
```

Gumi may store short previews.

---

# 31. Telemetry Retention

Default:

```yaml
telemetry:
  retain_days: 14
```

Retention cleanup should delete old rows from:

```text
requests
pipeline_events
provider_events
validation_reports
repair_reports
errors
logs
provider_health
benchmarks
```

---

# 32. Retention Cleanup

Cleanup should run:

```text
on startup
daily while runtime is active
manual via future CLI command
```

Future CLI:

```bash
gumi telemetry clean
```

---

# 33. Database Indexes

Recommended indexes:

```sql
CREATE INDEX idx_requests_created_at ON requests(created_at);
CREATE INDEX idx_requests_status ON requests(status);
CREATE INDEX idx_requests_provider_model ON requests(provider, model);
CREATE INDEX idx_pipeline_events_request_id ON pipeline_events(request_id);
CREATE INDEX idx_provider_events_provider ON provider_events(provider);
CREATE INDEX idx_provider_health_provider_time ON provider_health(provider, checked_at);
CREATE INDEX idx_errors_request_id ON errors(request_id);
CREATE INDEX idx_logs_created_at ON logs(created_at);
CREATE INDEX idx_sessions_workspace ON sessions(workspace_id);
```

---

# 34. Dashboard Query Patterns

Dashboard needs efficient queries for:

```text
recent requests
recent errors
request detail
provider status
latency trends
validation failures
repair events
retry counts
provider health
runtime overview
```

---

# 35. Recent Requests Query

Example:

```sql
SELECT
  id,
  created_at,
  runtime_mode,
  provider,
  model,
  status,
  latency_ms,
  validation_passed,
  repair_applied,
  retry_count,
  error_code
FROM requests
ORDER BY created_at DESC
LIMIT 100;
```

---

# 36. Request Detail Query

Request detail includes:

```text
request row
pipeline events
validation reports
repair reports
errors
provider events
```

---

# 37. Provider Status Query

Latest provider health:

```sql
SELECT *
FROM provider_health
WHERE provider = ?
ORDER BY checked_at DESC
LIMIT 1;
```

---

# 38. Metrics Query Examples

Requests in last 15 minutes:

```sql
SELECT COUNT(*)
FROM requests
WHERE created_at >= datetime('now', '-15 minutes');
```

Average latency:

```sql
SELECT AVG(latency_ms)
FROM requests
WHERE status IN ('success', 'repaired', 'retried');
```

Repair count:

```sql
SELECT COUNT(*)
FROM requests
WHERE repair_applied = 1;
```

---

# 39. Telemetry API

Dashboard should access telemetry through API.

Required API:

```http
GET /v1/gumi/telemetry/recent
```

Future APIs:

```http
GET /v1/gumi/telemetry/summary
GET /v1/gumi/trace/{request_id}
GET /v1/gumi/errors/recent
GET /v1/gumi/provider-health
```

Dashboard must not read SQLite directly.

---

# 40. Trace Data

Future `gumi trace` should reconstruct lifecycle from:

```text
requests
pipeline_events
provider_events
validation_reports
repair_reports
errors
```

Trace output should remain content-safe by default.

---

# 41. External Telemetry

Default:

```yaml
telemetry:
  external: false
```

External telemetry is out of scope for V1.

Future external telemetry must be:

- opt-in
- documented
- redacted
- disableable
- inspectable

---

# 42. Logs

Runtime should write local logs to:

```text
~/.gumi/logs/gumi.log
```

Log levels:

```text
debug
info
warning
error
fatal
```

Logs must follow same privacy rules as telemetry.

---

# 43. Log Rotation

Default:

```yaml
logs:
  max_size_mb: 50
  max_files: 5
```

Future config:

```yaml
logs:
  directory: ~/.gumi/logs
  max_size_mb: 50
  max_files: 5
  level: info
```

---

# 44. Storage Doctor Checks

`gumi doctor` should check:

```text
database exists or can be created
database writable
schema version valid
telemetry retention valid
log directory writable
storage size below configured max
```

Example warning:

```text
Telemetry database is 620MB, above configured max_size_mb 500.
Suggestion: run gumi telemetry clean.
```

---

# 45. Database Migration

V1 should include schema versioning.

Store version in `runtime_info`:

```text
database_schema_version = 1
```

Future command:

```bash
gumi db migrate
```

Migration rule:

- never delete user data silently
- backup before destructive migrations
- show migration errors clearly

---

# 46. Backup

Future command:

```bash
gumi db backup
```

Backup output:

```text
~/.gumi/backups/gumi-2026-07-10.db
```

Not required in V1, but storage layout should allow it.

---

# 47. Memory Storage Future

Future memory tables may include:

```text
memory_items
memory_embeddings
memory_links
memory_sources
```

V1 should not require vector database.

Memory Engine can start with session summaries only.

---

# 48. Security Considerations

SQLite database may contain local metadata.

Gumi should warn users if they enable:

```yaml
telemetry:
  log_prompts: true
  log_responses: true
```

Warning:

```text
Full prompt/response logging is enabled. Sensitive data may be stored locally in Gumi telemetry.
```

---

# 49. Testing Requirements

Storage tests:

- database creation
- schema migration
- insert request
- insert pipeline event
- insert validation report
- insert repair report
- insert error
- retention cleanup
- secret redaction
- prompt logging disabled by default
- config snapshot redaction
- provider health insert/query

Telemetry tests:

- event creation
- event severity
- request summary
- dashboard recent telemetry output
- no prompt content stored by default
- error still stored with redaction

---

# 50. V1 Implementation Priority

Implement in this order:

```text
1. SQLite connection
2. Schema creation
3. runtime_info table
4. requests table
5. pipeline_events table
6. errors table
7. provider_health table
8. validation_reports table
9. repair_reports table
10. recent telemetry API
11. retention cleanup
12. logs table
13. sessions table
14. config_snapshots table
```

---

# 51. Anti-Patterns

Avoid:

```text
Storing full prompts by default
Sending telemetry externally by default
Dashboard reading SQLite directly
Mixing logs and telemetry without structure
Storing API keys in plaintext telemetry
No retention policy
No schema versioning
No indexes for dashboard queries
Hardcoding storage paths
Making SQLite mandatory for basic gateway operation
```

---

# 52. Final Storage & Telemetry Statement

Gumi telemetry should make the runtime explainable without making it invasive.

The default experience stores useful metadata, not sensitive content.

Storage exists to help developers understand and improve local AI behaviour while preserving local-first privacy.