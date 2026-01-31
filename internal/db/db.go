// Package db provides SQLite database operations for the tpg task system.
//
// The database is stored under .tpg/ in the project root.
// Use Open() to connect and Init() to create the schema.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// MaxRetries is the maximum number of retries for transient database errors.
const MaxRetries = 5

// RetryBaseDelay is the base delay for exponential backoff.
const RetryBaseDelay = 50 * time.Millisecond

// sqlTime formats a time.Time as a SQLite-compatible UTC string.
// This ensures consistent timestamp format across inserts and queries,
// avoiding comparison issues between Go's time.Time serialization
// and SQLite's datetime functions.
func sqlTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// SchemaVersion is the current schema version.
// Increment this when adding new migrations.
const SchemaVersion = 3

// baseSchema is the original schema (version 1).
// New tables should be added via migrations, not here.
const baseSchema = `
CREATE TABLE IF NOT EXISTS items (
	id TEXT PRIMARY KEY,
	project TEXT NOT NULL,
	type TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'open',
	priority INTEGER DEFAULT 2,
	parent_id TEXT REFERENCES items(id),
	agent_id TEXT,
	agent_last_active DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS deps (
	item_id TEXT REFERENCES items(id),
	depends_on TEXT REFERENCES items(id),
	PRIMARY KEY (item_id, depends_on)
);

CREATE TABLE IF NOT EXISTS logs (
	id INTEGER PRIMARY KEY,
	item_id TEXT REFERENCES items(id),
	message TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
	name TEXT PRIMARY KEY,
	description TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS concepts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	project TEXT NOT NULL,
	summary TEXT,
	last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (name, project)
);

CREATE TABLE IF NOT EXISTS learnings (
	id TEXT PRIMARY KEY,
	project TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	task_id TEXT REFERENCES items(id),
	summary TEXT NOT NULL,
	detail TEXT,
	files TEXT,
	status TEXT DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS learning_concepts (
	learning_id TEXT REFERENCES learnings(id),
	concept_id TEXT REFERENCES concepts(id),
	PRIMARY KEY (learning_id, concept_id)
);

CREATE VIRTUAL TABLE IF NOT EXISTS learnings_fts USING fts5(
	summary,
	detail,
	content='learnings',
	content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS learnings_ai AFTER INSERT ON learnings BEGIN
	INSERT INTO learnings_fts(rowid, summary, detail)
	VALUES (NEW.rowid, NEW.summary, NEW.detail);
END;

CREATE TRIGGER IF NOT EXISTS learnings_ad AFTER DELETE ON learnings BEGIN
	INSERT INTO learnings_fts(learnings_fts, rowid, summary, detail)
	VALUES ('delete', OLD.rowid, OLD.summary, OLD.detail);
END;

CREATE TRIGGER IF NOT EXISTS learnings_au AFTER UPDATE ON learnings BEGIN
	INSERT INTO learnings_fts(learnings_fts, rowid, summary, detail)
	VALUES ('delete', OLD.rowid, OLD.summary, OLD.detail);
	INSERT INTO learnings_fts(rowid, summary, detail)
	VALUES (NEW.rowid, NEW.summary, NEW.detail);
END;

CREATE INDEX IF NOT EXISTS idx_items_project ON items(project);
CREATE INDEX IF NOT EXISTS idx_items_status ON items(status);
CREATE INDEX IF NOT EXISTS idx_items_parent ON items(parent_id);
CREATE INDEX IF NOT EXISTS idx_items_agent_id ON items(agent_id) WHERE agent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_logs_item ON logs(item_id);
CREATE INDEX IF NOT EXISTS idx_learnings_project ON learnings(project);
CREATE INDEX IF NOT EXISTS idx_learnings_task ON learnings(task_id);
CREATE INDEX IF NOT EXISTS idx_learnings_status ON learnings(status);
CREATE INDEX IF NOT EXISTS idx_learning_concepts_concept ON learning_concepts(concept_id);

CREATE TABLE IF NOT EXISTS agent_sessions (
	agent_id TEXT NOT NULL,
	project TEXT NOT NULL,
	last_active DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (agent_id, project)
);
`

// migrations defines incremental schema changes.
// Each migration upgrades from version N-1 to N.
// Index 0 is migration to version 2, index 1 is migration to version 3, etc.
var migrations = []string{
	// Version 2: Add labels system
	`
CREATE TABLE IF NOT EXISTS labels (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	project TEXT NOT NULL,
	color TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (name, project)
);

CREATE TABLE IF NOT EXISTS item_labels (
	item_id TEXT REFERENCES items(id),
	label_id TEXT REFERENCES labels(id),
	PRIMARY KEY (item_id, label_id)
);

CREATE INDEX IF NOT EXISTS idx_labels_project ON labels(project);
CREATE INDEX IF NOT EXISTS idx_item_labels_item ON item_labels(item_id);
CREATE INDEX IF NOT EXISTS idx_item_labels_label ON item_labels(label_id);
`,
	// Version 3: Add template metadata and results
	`
ALTER TABLE items ADD COLUMN template_id TEXT;
ALTER TABLE items ADD COLUMN step_index INTEGER;
ALTER TABLE items ADD COLUMN variables TEXT;
ALTER TABLE items ADD COLUMN template_hash TEXT;
ALTER TABLE items ADD COLUMN results TEXT;
`,
}

// DB wraps a SQL database connection with task-specific operations.
type DB struct {
	*sql.DB
}

// ExecRetry executes a statement with retry logic for transient errors.
func (db *DB) ExecRetry(query string, args ...any) (sql.Result, error) {
	return withRetry(func() (sql.Result, error) {
		return db.Exec(query, args...)
	})
}

// QueryRowRetry executes a query that returns a single row with retry logic.
func (db *DB) QueryRowRetry(query string, args ...any) *sql.Row {
	// sql.Row doesn't support retry directly, but the busy_timeout handles it
	return db.QueryRow(query, args...)
}

// QueryRetry executes a query with retry logic for transient errors.
func (db *DB) QueryRetry(query string, args ...any) (*sql.Rows, error) {
	return withRetry(func() (*sql.Rows, error) {
		return db.Query(query, args...)
	})
}

// Open opens or creates the database at the given path
func Open(path string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	var sqlDB *sql.DB
	var err error

	// Retry opening the database with exponential backoff
	err = withRetryNoResult(func() error {
		sqlDB, err = sql.Open("sqlite", path)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		// Set busy timeout FIRST before any other operations
		// This ensures retries work for subsequent PRAGMA calls
		if _, err := sqlDB.Exec("PRAGMA busy_timeout=5000"); err != nil {
			_ = sqlDB.Close()
			return fmt.Errorf("failed to set busy timeout: %w", err)
		}

		// Enable foreign keys
		if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
			_ = sqlDB.Close()
			return fmt.Errorf("failed to enable foreign keys: %w", err)
		}

		// Enable WAL mode for better concurrency (allows concurrent readers during writes)
		// This may fail if another process has the database open in non-WAL mode,
		// but will succeed on retry once that process closes or switches to WAL
		if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
			_ = sqlDB.Close()
			return fmt.Errorf("failed to enable WAL mode: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &DB{sqlDB}, nil
}

// isRetryableError checks if an error is a transient SQLite error that can be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// SQLITE_BUSY (5), SQLITE_LOCKED (6)
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "SQLITE_BUSY") ||
		strings.Contains(errStr, "SQLITE_LOCKED")
}

// withRetry executes a function with exponential backoff retry on transient errors.
func withRetry[T any](fn func() (T, error)) (T, error) {
	var result T
	var err error
	delay := RetryBaseDelay

	for attempt := 0; attempt < MaxRetries; attempt++ {
		result, err = fn()
		if err == nil || !isRetryableError(err) {
			return result, err
		}

		// Exponential backoff with jitter
		time.Sleep(delay)
		delay *= 2
		if delay > 2*time.Second {
			delay = 2 * time.Second
		}
	}

	return result, fmt.Errorf("failed after %d retries: %w", MaxRetries, err)
}

// withRetryNoResult executes a function with retry that returns only an error.
func withRetryNoResult(fn func() error) error {
	_, err := withRetry(func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// Init creates the schema for a fresh database.
// For existing databases, use Migrate() instead.
func (db *DB) Init() error {
	_, err := db.Exec(baseSchema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run all migrations to bring to current version
	if err := db.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Migrate existing projects from items table
	if err := db.migrateProjects(); err != nil {
		return fmt.Errorf("failed to migrate projects: %w", err)
	}

	return nil
}

// Migrate runs any pending schema migrations.
// Safe to call on every startup - only runs migrations newer than current version.
func (db *DB) Migrate() error {
	currentVersion, err := db.getSchemaVersion()
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	// If version is 0 but tables exist, this is a legacy database (v1)
	if currentVersion == 0 {
		exists, err := db.tableExists("items")
		if err != nil {
			return fmt.Errorf("failed to check tables: %w", err)
		}
		if exists {
			currentVersion = 1
			if err := db.setSchemaVersion(1); err != nil {
				return fmt.Errorf("failed to set legacy version: %w", err)
			}
		}
	}

	// Run pending migrations
	for i, migration := range migrations {
		targetVersion := i + 2 // migrations[0] upgrades to v2
		if currentVersion >= targetVersion {
			continue
		}

		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration to v%d failed: %w", targetVersion, err)
		}

		if err := db.setSchemaVersion(targetVersion); err != nil {
			return fmt.Errorf("failed to update version to %d: %w", targetVersion, err)
		}
		currentVersion = targetVersion
	}

	return nil
}

// getSchemaVersion returns the current schema version using PRAGMA user_version.
func (db *DB) getSchemaVersion() (int, error) {
	var version int
	err := db.QueryRow("PRAGMA user_version").Scan(&version)
	return version, err
}

// setSchemaVersion sets the schema version using PRAGMA user_version.
func (db *DB) setSchemaVersion(version int) error {
	_, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
}

// tableExists checks if a table exists in the database.
func (db *DB) tableExists(name string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		name,
	).Scan(&count)
	return count > 0, err
}

// migrateProjects populates the projects table from existing items.
func (db *DB) migrateProjects() error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO projects (name, created_at, updated_at)
		SELECT DISTINCT project, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		FROM items
		WHERE project != ''
	`)
	return err
}
