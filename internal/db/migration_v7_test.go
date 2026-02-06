package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// timestampsEqual compares two timestamp strings, handling different formats.
// SQLite may return timestamps in different formats depending on how they were inserted.
func timestampsEqual(a, b string) bool {
	// Try parsing both with different formats
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
	}

	parseTime := func(s string) (time.Time, bool) {
		for _, f := range formats {
			if t, err := time.Parse(f, s); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}

	ta, okA := parseTime(a)
	tb, okB := parseTime(b)

	if !okA || !okB {
		// If parsing fails, fall back to string comparison
		return strings.TrimSuffix(a, "Z") == strings.TrimSuffix(b, "Z")
	}

	return ta.Equal(tb)
}

// TestMigrationV7_AddsClosedAtColumn verifies the migration adds closed_at column to items table.
func TestMigrationV7_AddsClosedAtColumn(t *testing.T) {
	// Arrange: Create a v6 database
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Initialize with current schema (up to v6)
	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Insert a test item
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status, created_at, updated_at)
		VALUES ('ts-test1', 'test', 'task', 'Test Item', 'open', '2025-01-01 10:00:00', '2025-01-01 10:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	db.Close()

	// Act: Reopen and migrate
	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Assert: Verify schema version is 7
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 7 {
		t.Errorf("schema version = %d, want 7", version)
	}

	// Assert: Verify closed_at column exists and is queryable
	var closedAt sql.NullString
	err = db.QueryRow("SELECT closed_at FROM items WHERE id = 'ts-test1'").Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at column: %v", err)
	}

	// Open items should have NULL closed_at
	if closedAt.Valid {
		t.Errorf("expected closed_at to be NULL for open item, got %v", closedAt.String)
	}

	// Assert: Verify the partial index exists
	var indexCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type = 'index' AND name = 'idx_items_closed_at'
	`).Scan(&indexCount)
	if err != nil {
		t.Fatalf("failed to query index: %v", err)
	}
	if indexCount != 1 {
		t.Errorf("expected idx_items_closed_at index to exist, found %d", indexCount)
	}
}

// TestMigrationV7_BackfillsClosedAt verifies existing done/canceled items get closed_at set from updated_at.
func TestMigrationV7_BackfillsClosedAt(t *testing.T) {
	// Arrange: Create a v6 database with done/canceled items
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Reset to v6 to test v7 migration
	if err := db.setSchemaVersion(6); err != nil {
		t.Fatalf("failed to set schema version to 6: %v", err)
	}

	// Insert items in various states
	testCases := []struct {
		id        string
		status    string
		updatedAt string
	}{
		{"ts-done1", "done", "2025-01-15 14:30:00"},
		{"ts-done2", "done", "2025-01-16 09:00:00"},
		{"ts-canceled1", "canceled", "2025-01-14 11:00:00"},
		{"ts-open1", "open", "2025-01-10 08:00:00"},
		{"ts-progress1", "in_progress", "2025-01-17 12:00:00"},
		{"ts-blocked1", "blocked", "2025-01-18 16:00:00"},
	}

	for _, tc := range testCases {
		_, err = db.Exec(`
			INSERT INTO items (id, project, type, title, status, created_at, updated_at)
			VALUES (?, 'test', 'task', 'Test Item', ?, '2025-01-01 10:00:00', ?)
		`, tc.id, tc.status, tc.updatedAt)
		if err != nil {
			t.Fatalf("failed to insert %s: %v", tc.id, err)
		}
	}

	db.Close()

	// Act: Reopen and migrate
	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Assert: Verify backfill results
	for _, tc := range testCases {
		var closedAt sql.NullString
		err = db.QueryRow("SELECT closed_at FROM items WHERE id = ?", tc.id).Scan(&closedAt)
		if err != nil {
			t.Fatalf("failed to query %s: %v", tc.id, err)
		}

		shouldHaveClosedAt := tc.status == "done" || tc.status == "canceled"

		if shouldHaveClosedAt {
			if !closedAt.Valid {
				t.Errorf("item %s (status=%s) should have closed_at, got NULL", tc.id, tc.status)
			} else if !timestampsEqual(closedAt.String, tc.updatedAt) {
				t.Errorf("item %s closed_at = %q, want %q (from updated_at)", tc.id, closedAt.String, tc.updatedAt)
			}
		} else {
			if closedAt.Valid {
				t.Errorf("item %s (status=%s) should have NULL closed_at, got %q", tc.id, tc.status, closedAt.String)
			}
		}
	}
}

// TestMigrationV7_CreatesHistoryTable verifies the history table is created with correct schema.
func TestMigrationV7_CreatesHistoryTable(t *testing.T) {
	// Arrange: Create and migrate database
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Assert: Verify history table exists
	exists, err := db.tableExists("history")
	if err != nil {
		t.Fatalf("failed to check history table: %v", err)
	}
	if !exists {
		t.Fatal("expected history table to exist")
	}

	// Assert: Verify table structure by inserting and querying
	// First create an item to satisfy foreign key
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create test item: %v", err)
	}

	// Insert a history entry with all expected columns
	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type, actor_id, actor_type, changes, created_at)
		VALUES ('ts-test1', 'created', 'agent-123', 'subagent', '{"title":"Test"}', '2025-01-20 10:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert history entry: %v", err)
	}

	// Query it back
	var id int64
	var itemID, eventType, actorID, actorType, changes, createdAt string
	err = db.QueryRow(`
		SELECT id, item_id, event_type, actor_id, actor_type, changes, created_at
		FROM history WHERE item_id = 'ts-test1'
	`).Scan(&id, &itemID, &eventType, &actorID, &actorType, &changes, &createdAt)
	if err != nil {
		t.Fatalf("failed to query history: %v", err)
	}

	if id <= 0 {
		t.Error("expected history.id to be auto-incremented")
	}
	if itemID != "ts-test1" {
		t.Errorf("item_id = %q, want 'ts-test1'", itemID)
	}
	if eventType != "created" {
		t.Errorf("event_type = %q, want 'created'", eventType)
	}
	if actorID != "agent-123" {
		t.Errorf("actor_id = %q, want 'agent-123'", actorID)
	}
	if actorType != "subagent" {
		t.Errorf("actor_type = %q, want 'subagent'", actorType)
	}
	if changes != `{"title":"Test"}` {
		t.Errorf("changes = %q, want '{\"title\":\"Test\"}'", changes)
	}

	// Assert: Verify all three indexes exist
	indexes := []string{
		"idx_history_item_time",
		"idx_history_actor_time",
		"idx_history_recent",
	}
	for _, indexName := range indexes {
		var count int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master 
			WHERE type = 'index' AND name = ?
		`, indexName).Scan(&count)
		if err != nil {
			t.Fatalf("failed to query index %s: %v", indexName, err)
		}
		if count != 1 {
			t.Errorf("expected index %s to exist", indexName)
		}
	}
}

// TestMigrationV7_HistoryCascadeDelete verifies history entries are deleted when their item is deleted.
func TestMigrationV7_HistoryCascadeDelete(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Create item and history
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-delete1', 'test', 'task', 'To Delete', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type, actor_id, actor_type)
		VALUES ('ts-delete1', 'created', 'agent-1', 'subagent')
	`)
	if err != nil {
		t.Fatalf("failed to create history: %v", err)
	}

	// Verify history exists
	var historyCount int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-delete1'").Scan(&historyCount)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("expected 1 history entry, got %d", historyCount)
	}

	// Act: Delete the item
	_, err = db.Exec("DELETE FROM items WHERE id = 'ts-delete1'")
	if err != nil {
		t.Fatalf("failed to delete item: %v", err)
	}

	// Assert: History should be cascade deleted
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-delete1'").Scan(&historyCount)
	if err != nil {
		t.Fatalf("failed to count history after delete: %v", err)
	}
	if historyCount != 0 {
		t.Errorf("expected history to be cascade deleted, got %d entries", historyCount)
	}
}

// TestMigrationV7_Idempotent verifies the migration can be run multiple times safely.
func TestMigrationV7_Idempotent(t *testing.T) {
	// Arrange: Create a v6 database
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status, updated_at)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'done', '2025-01-15 10:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// Insert history entry
	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type, actor_id, actor_type)
		VALUES ('ts-test1', 'completed', 'agent-1', 'subagent')
	`)
	if err != nil {
		t.Fatalf("failed to insert history: %v", err)
	}

	db.Close()

	// Act: Reopen and migrate again (should be safe)
	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	// Run migrate multiple times
	for i := 0; i < 3; i++ {
		if err := db.Migrate(); err != nil {
			t.Fatalf("migrate attempt %d failed: %v", i+1, err)
		}
	}

	// Assert: Schema version is still 7
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 7 {
		t.Errorf("schema version = %d, want 7", version)
	}

	// Assert: Data is preserved
	var title string
	err = db.QueryRow("SELECT title FROM items WHERE id = 'ts-test1'").Scan(&title)
	if err != nil {
		t.Fatalf("failed to query item: %v", err)
	}
	if title != "Test" {
		t.Errorf("title = %q, want 'Test'", title)
	}

	// Assert: History entry is preserved
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 history entry, got %d", count)
	}

	// Assert: Only one index of each type (not duplicated)
	indexes := []string{"idx_items_closed_at", "idx_history_item_time", "idx_history_actor_time", "idx_history_recent"}
	for _, idx := range indexes {
		var indexCount int
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", idx).Scan(&indexCount)
		if err != nil {
			t.Fatalf("failed to count index %s: %v", idx, err)
		}
		if indexCount != 1 {
			t.Errorf("expected exactly 1 %s index, got %d", idx, indexCount)
		}
	}
}

// TestMigrationV7_FreshDatabase verifies a fresh database gets v7 schema correctly.
func TestMigrationV7_FreshDatabase(t *testing.T) {
	// Arrange & Act: Create a fresh database
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Assert: Version is 7
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 7 {
		t.Errorf("schema version = %d, want 7", version)
	}

	// Assert: closed_at column exists
	exists, err := db.columnExists("items", "closed_at")
	if err != nil {
		t.Fatalf("failed to check closed_at column: %v", err)
	}
	if !exists {
		t.Error("expected closed_at column to exist")
	}

	// Assert: history table exists
	exists, err = db.tableExists("history")
	if err != nil {
		t.Fatalf("failed to check history table: %v", err)
	}
	if !exists {
		t.Error("expected history table to exist")
	}
}

// TestMigrationV7_HistoryNullableFields verifies actor_id, actor_type, and changes can be NULL.
func TestMigrationV7_HistoryNullableFields(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Create item
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Act: Insert history with NULL optional fields
	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type)
		VALUES ('ts-test1', 'created')
	`)
	if err != nil {
		t.Fatalf("failed to insert history with NULL fields: %v", err)
	}

	// Assert: Query and verify NULLs are allowed
	var actorID, actorType, changes sql.NullString
	err = db.QueryRow(`
		SELECT actor_id, actor_type, changes FROM history WHERE item_id = 'ts-test1'
	`).Scan(&actorID, &actorType, &changes)
	if err != nil {
		t.Fatalf("failed to query history: %v", err)
	}

	if actorID.Valid {
		t.Errorf("expected actor_id to be NULL, got %q", actorID.String)
	}
	if actorType.Valid {
		t.Errorf("expected actor_type to be NULL, got %q", actorType.String)
	}
	if changes.Valid {
		t.Errorf("expected changes to be NULL, got %q", changes.String)
	}
}

// TestMigrationV7_HistoryDefaultTimestamp verifies created_at defaults to current timestamp.
func TestMigrationV7_HistoryDefaultTimestamp(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Create item
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Record time before insert
	beforeInsert := time.Now().UTC().Add(-1 * time.Second)

	// Act: Insert history without created_at
	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type)
		VALUES ('ts-test1', 'created')
	`)
	if err != nil {
		t.Fatalf("failed to insert history: %v", err)
	}

	// Record time after insert
	afterInsert := time.Now().UTC().Add(1 * time.Second)

	// Assert: created_at should be set to current time
	var createdAtStr string
	err = db.QueryRow("SELECT created_at FROM history WHERE item_id = 'ts-test1'").Scan(&createdAtStr)
	if err != nil {
		t.Fatalf("failed to query created_at: %v", err)
	}

	// Try multiple timestamp formats
	var createdAt time.Time
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
	}
	var parseErr error
	for _, f := range formats {
		createdAt, parseErr = time.Parse(f, createdAtStr)
		if parseErr == nil {
			break
		}
	}
	if parseErr != nil {
		t.Fatalf("failed to parse created_at %q with any format: %v", createdAtStr, parseErr)
	}

	if createdAt.Before(beforeInsert) || createdAt.After(afterInsert) {
		t.Errorf("created_at %v should be between %v and %v", createdAt, beforeInsert, afterInsert)
	}
}

// TestMigrationV7_ClosedAtSetOnComplete verifies items completed after migration get closed_at set.
func TestMigrationV7_ClosedAtSetOnComplete(t *testing.T) {
	// This test verifies the schema supports setting closed_at when completing items.
	// The actual logic to set closed_at on status change would be in the application layer.

	// Arrange
	db := setupTestDB(t)

	// Create an open item
	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Verify closed_at is NULL initially
	var closedAt sql.NullString
	err := db.QueryRow("SELECT closed_at FROM items WHERE id = ?", item.ID).Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at: %v", err)
	}
	if closedAt.Valid {
		t.Error("expected closed_at to be NULL for new open item")
	}

	// Act: Update to done with closed_at
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = db.Exec(`
		UPDATE items SET status = 'done', closed_at = ? WHERE id = ?
	`, now, item.ID)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Assert: closed_at is set
	err = db.QueryRow("SELECT closed_at FROM items WHERE id = ?", item.ID).Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at after update: %v", err)
	}
	if !closedAt.Valid {
		t.Error("expected closed_at to be set after completing item")
	}
}

// TestMigrationV7_FromV6 verifies migration from v6 to v7 works correctly.
func TestMigrationV7_FromV6(t *testing.T) {
	// Arrange: Create a v6 database manually
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Create v6 schema (without closed_at and history)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'open',
			priority INTEGER DEFAULT 2,
			parent_id TEXT,
			agent_id TEXT,
			agent_last_active DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			template_id TEXT,
			step_index INTEGER,
			variables TEXT,
			template_hash TEXT,
			results TEXT,
			worktree_branch TEXT,
			worktree_base TEXT,
			shared_context TEXT,
			closing_instructions TEXT
		);
		CREATE TABLE IF NOT EXISTS labels (id TEXT PRIMARY KEY, name TEXT NOT NULL, project TEXT NOT NULL, color TEXT);
		CREATE TABLE IF NOT EXISTS item_labels (item_id TEXT, label_id TEXT, PRIMARY KEY (item_id, label_id));
	`)
	if err != nil {
		t.Fatalf("failed to create v6 schema: %v", err)
	}

	// Set version to 6
	if err := db.setSchemaVersion(6); err != nil {
		t.Fatalf("failed to set schema version to 6: %v", err)
	}

	// Insert test data with various statuses
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status, updated_at) VALUES
			('ts-open', 'test', 'task', 'Open Task', 'open', '2025-01-10 10:00:00'),
			('ts-done', 'test', 'task', 'Done Task', 'done', '2025-01-15 15:00:00'),
			('ts-canceled', 'test', 'task', 'Canceled Task', 'canceled', '2025-01-12 12:00:00')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	db.Close()

	// Act: Reopen and migrate
	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Assert: Version is 7
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 7 {
		t.Errorf("schema version = %d, want 7", version)
	}

	// Assert: closed_at column added
	exists, err := db.columnExists("items", "closed_at")
	if err != nil {
		t.Fatalf("failed to check closed_at: %v", err)
	}
	if !exists {
		t.Error("closed_at column should exist after migration")
	}

	// Assert: history table created
	exists, err = db.tableExists("history")
	if err != nil {
		t.Fatalf("failed to check history table: %v", err)
	}
	if !exists {
		t.Error("history table should exist after migration")
	}

	// Assert: Backfill worked correctly
	testCases := []struct {
		id           string
		expectClosed bool
		expectedTime string
	}{
		{"ts-open", false, ""},
		{"ts-done", true, "2025-01-15 15:00:00"},
		{"ts-canceled", true, "2025-01-12 12:00:00"},
	}

	for _, tc := range testCases {
		var closedAt sql.NullString
		err = db.QueryRow("SELECT closed_at FROM items WHERE id = ?", tc.id).Scan(&closedAt)
		if err != nil {
			t.Fatalf("failed to query %s: %v", tc.id, err)
		}

		if tc.expectClosed {
			if !closedAt.Valid {
				t.Errorf("%s: expected closed_at to be set", tc.id)
			} else if !timestampsEqual(closedAt.String, tc.expectedTime) {
				t.Errorf("%s: closed_at = %q, want %q", tc.id, closedAt.String, tc.expectedTime)
			}
		} else {
			if closedAt.Valid {
				t.Errorf("%s: expected closed_at to be NULL, got %q", tc.id, closedAt.String)
			}
		}
	}
}
