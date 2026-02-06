// Package db provides SQLite database operations for the tpg task system.
//
// The database is stored under .tpg/ in the project root.
// Use Open() to connect and Init() to create the schema.
package db

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
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
const SchemaVersion = 6

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
	// Version 4: Add worktree metadata for epics
	`
ALTER TABLE items ADD COLUMN worktree_branch TEXT;
ALTER TABLE items ADD COLUMN worktree_base TEXT;
`,
	// Version 5: Add epic shared context and closing instructions
	// This migration is handled specially in runMigrationV5 to be idempotent
	"", // Empty placeholder - actual logic in runMigrationV5
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
// Creates a backup before running any migrations.
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

	// Check if any migrations need to run
	needsMigration := false
	for i := range migrations {
		targetVersion := i + 2 // migrations[0] upgrades to v2
		if currentVersion < targetVersion {
			needsMigration = true
			break
		}
	}
	// Also check v6 migration
	if currentVersion == 5 {
		needsMigration = true
	}

	// Create backup before running any migrations
	if needsMigration {
		backupPath, err := db.Backup()
		if err != nil {
			return fmt.Errorf("failed to create pre-migration backup: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Created pre-migration backup: %s\n", backupPath)
	}

	// Run pending SQL schema migrations
	for i, migration := range migrations {
		targetVersion := i + 2 // migrations[0] upgrades to v2
		if currentVersion >= targetVersion {
			continue
		}

		// Handle v5 migration specially (idempotent column addition)
		if targetVersion == 5 {
			if err := db.runMigrationV5(); err != nil {
				return fmt.Errorf("migration to v5 failed: %w", err)
			}
		} else {
			if _, err := db.Exec(migration); err != nil {
				return fmt.Errorf("migration to v%d failed: %w", targetVersion, err)
			}
		}

		if err := db.setSchemaVersion(targetVersion); err != nil {
			return fmt.Errorf("failed to update version to %d: %w", targetVersion, err)
		}
		currentVersion = targetVersion
	}

	// Run v6 data migration (convert legacy types to labels)
	if currentVersion == 5 {
		if err := db.migrateV6(); err != nil {
			return fmt.Errorf("migration v6 failed: %w", err)
		}
		if err := db.setSchemaVersion(6); err != nil {
			return fmt.Errorf("failed to update version to 6: %w", err)
		}
	}

	return nil
}

// CheckIntegrity runs PRAGMA integrity_check on the database.
// If issues are found and they relate to FTS5, it creates a backup
// and attempts to rebuild the FTS5 index.
// Returns nil if the database is OK or if repairs succeed.
func (db *DB) CheckIntegrity() error {
	return db.checkDatabaseIntegrity()
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

// columnExists checks if a column exists in a table.
func (db *DB) columnExists(table, column string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?",
		table, column,
	).Scan(&count)
	return count > 0, err
}

// runMigrationV5 adds shared_context and closing_instructions columns idempotently.
// This migration is idempotent - it checks if columns exist before adding them.
func (db *DB) runMigrationV5() error {
	// Check if shared_context column exists
	exists, err := db.columnExists("items", "shared_context")
	if err != nil {
		return fmt.Errorf("failed to check shared_context column: %w", err)
	}
	if !exists {
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN shared_context TEXT"); err != nil {
			return fmt.Errorf("failed to add shared_context column: %w", err)
		}
	}

	// Check if closing_instructions column exists
	exists, err = db.columnExists("items", "closing_instructions")
	if err != nil {
		return fmt.Errorf("failed to check closing_instructions column: %w", err)
	}
	if !exists {
		if _, err := db.Exec("ALTER TABLE items ADD COLUMN closing_instructions TEXT"); err != nil {
			return fmt.Errorf("failed to add closing_instructions column: %w", err)
		}
	}

	return nil
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

// slugify converts a type name to a label-safe format.
// - Lowercase
// - Spaces and special chars replaced with hyphens
// - Leading/trailing hyphens removed
// - Multiple hyphens collapsed
// Examples: "User Story" → "user-story", "Bug Fix" → "bug-fix"
func slugify(s string) string {
	s = strings.ToLower(s)

	// Replace non-alphanumeric chars with hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	// Collapse multiple hyphens and trim
	slug := result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")

	return slug
}

// checkDatabaseIntegrity runs PRAGMA integrity_check and returns any errors found.
// It also attempts to detect and fix common FTS5 corruption issues.
// Creates a backup before attempting any repairs.
func (db *DB) checkDatabaseIntegrity() error {
	var result string
	err := db.QueryRow("PRAGMA integrity_check").Scan(&result)
	if err != nil {
		return fmt.Errorf("integrity check query failed: %w", err)
	}
	if result != "ok" {
		// Try to recover FTS5 if that's the issue
		if strings.Contains(result, "fts5") || strings.Contains(result, "learnings_fts") {
			// Create backup before attempting repair
			backupPath, backupErr := db.Backup()
			if backupErr != nil {
				// Log but don't fail - we still want to try the repair
				fmt.Fprintf(os.Stderr, "warning: failed to create pre-repair backup: %v\n", backupErr)
			} else {
				fmt.Fprintf(os.Stderr, "Created pre-repair backup: %s\n", backupPath)
			}
			if err := db.rebuildFTS5(); err != nil {
				return fmt.Errorf("database integrity check failed: %s; FTS5 rebuild also failed: %w", result, err)
			}
			// Re-check integrity after rebuild
			err = db.QueryRow("PRAGMA integrity_check").Scan(&result)
			if err != nil {
				return fmt.Errorf("integrity check after FTS5 rebuild failed: %w", err)
			}
			if result != "ok" {
				return fmt.Errorf("database integrity check failed even after FTS5 rebuild: %s", result)
			}
			return nil
		}
		return fmt.Errorf("database integrity check failed: %s", result)
	}
	return nil
}

// rebuildFTS5 rebuilds the FTS5 virtual table to fix corruption.
// This deletes and recreates the FTS5 index from the source data.
func (db *DB) rebuildFTS5() error {
	// Delete all FTS5 content and rebuild from source
	_, err := db.Exec("DELETE FROM learnings_fts")
	if err != nil {
		return fmt.Errorf("failed to clear FTS5 table: %w", err)
	}
	// Re-insert all learnings data to trigger FTS5 rebuild via triggers
	_, err = db.Exec(`
		INSERT INTO learnings_fts(rowid, summary, detail)
		SELECT rowid, summary, detail FROM learnings
	`)
	if err != nil {
		return fmt.Errorf("failed to rebuild FTS5 index: %w", err)
	}
	return nil
}

// migrateV6 converts legacy item types to labels.
// For each item with type not in ('task', 'epic'):
// 1. Store old type name
// 2. Update type to 'task'
// 3. Create label with slugified old type name
// 4. Attach label to item
func (db *DB) migrateV6() error {
	// Check database integrity before migration
	if err := db.checkDatabaseIntegrity(); err != nil {
		return fmt.Errorf("database integrity check failed before migration: %w\n\nTo recover:\n1. Backup your database: cp .tpg/tpg.db .tpg/tpg.db.backup\n2. Try running: sqlite3 .tpg/tpg.db 'REINDEX; VACUUM;'\n3. If still failing, the database may need manual recovery", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Find all items with non-standard types
	// Use a simpler query that's less likely to hit corruption issues
	rows, err := tx.Query(`
		SELECT id, project, type
		FROM items
		WHERE type NOT IN ('task', 'epic')
	`)
	if err != nil {
		// If we get a malformed error, try to provide helpful recovery info
		if strings.Contains(err.Error(), "malformed") {
			return fmt.Errorf("failed to query legacy types (database may be corrupted): %w\n\nRecovery steps:\n1. Backup: cp .tpg/tpg.db .tpg/tpg.db.backup\n2. Try recovery: sqlite3 .tpg/tpg.db '.recover' | sqlite3 .tpg/tpg.db.recovered\n3. Replace: mv .tpg/tpg.db.recovered .tpg/tpg.db", err)
		}
		return fmt.Errorf("failed to query legacy types: %w", err)
	}

	// Collect items to migrate (can't modify while iterating)
	type itemToMigrate struct {
		id      string
		project string
		oldType string
	}
	var items []itemToMigrate
	for rows.Next() {
		var item itemToMigrate
		if err := rows.Scan(&item.id, &item.project, &item.oldType); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan item: %w", err)
		}
		items = append(items, item)
	}
	rows.Close()

	// Migrate each item
	for _, item := range items {
		// Update type to task
		_, err := tx.Exec(`UPDATE items SET type = 'task' WHERE id = ?`, item.id)
		if err != nil {
			return fmt.Errorf("failed to update item %s type: %w", item.id, err)
		}

		// Create slugified label name
		labelName := slugify(item.oldType)
		if labelName == "" {
			continue // Skip if type produces empty slug
		}

		// Ensure label exists (create if not)
		var labelID string
		err = tx.QueryRow(`SELECT id FROM labels WHERE name = ? AND project = ?`, labelName, item.project).Scan(&labelID)
		if err != nil {
			// Label doesn't exist, create it
			labelID = fmt.Sprintf("lb-%s", generateShortID())
			_, err = tx.Exec(`
				INSERT INTO labels (id, name, project, created_at, updated_at)
				VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			`, labelID, labelName, item.project)
			if err != nil {
				return fmt.Errorf("failed to create label %s: %w", labelName, err)
			}
		}

		// Attach label to item (ignore if already exists)
		_, err = tx.Exec(`INSERT OR IGNORE INTO item_labels (item_id, label_id) VALUES (?, ?)`, item.id, labelID)
		if err != nil {
			return fmt.Errorf("failed to attach label to item %s: %w", item.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

// generateShortID generates a short random ID for labels.
// This is used during migration; normally model.GenerateLabelID() is used.
func generateShortID() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	alphabetLen := big.NewInt(int64(len(alphabet)))
	b := make([]byte, 6)
	for i := range b {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		b[i] = alphabet[idx.Int64()]
	}
	return string(b)
}
