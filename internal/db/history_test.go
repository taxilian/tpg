package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// setupTestDBWithHistory creates a test database with history table and test data.
func setupTestDBWithHistory(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

// insertTestItem creates a test item in the database.
// Note: This uses CreateItem which now records a "created" history event.
func insertTestItem(t *testing.T, db *DB, id, project string) {
	t.Helper()
	now := time.Now()
	item := &model.Item{
		ID:        id,
		Project:   project,
		Type:      model.ItemTypeTask,
		Title:     "Test Item " + id,
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create test item %s: %v", id, err)
	}
}

// insertTestItemWithoutHistory creates a test item directly in the database,
// bypassing CreateItem to avoid recording a "created" history event.
// Use this when you need precise control over history entries.
func insertTestItemWithoutHistory(t *testing.T, db *DB, id, project string) {
	t.Helper()
	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, project, model.ItemTypeTask, "Test Item "+id, model.StatusOpen, 2, sqlTime(now), sqlTime(now))
	if err != nil {
		t.Fatalf("failed to create test item %s: %v", id, err)
	}
}

// insertHistoryEntry creates a history entry for testing.
func insertHistoryEntry(t *testing.T, db *DB, itemID, eventType, actorID, actorType string, changes map[string]any, createdAt time.Time) {
	t.Helper()
	var changesJSON []byte
	var err error
	if changes != nil {
		changesJSON, err = json.Marshal(changes)
		if err != nil {
			t.Fatalf("failed to marshal changes: %v", err)
		}
	}

	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type, actor_id, actor_type, changes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, itemID, eventType, actorID, actorType, string(changesJSON), sqlTime(createdAt))
	if err != nil {
		t.Fatalf("failed to insert history entry: %v", err)
	}
}

// TestGetItemHistory_Basic verifies that history entries for a specific item are returned.
func TestGetItemHistory_Basic(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", map[string]any{"title": "Test"}, now.Add(-2*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", map[string]any{"old": "open", "new": "in_progress"}, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "completed", "agent-1", "subagent", nil, now)

	// Act
	entries, err := db.GetItemHistory("ts-test1", 50)

	// Assert
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(entries))
	}

	// Should be sorted by created_at DESC (newest first)
	if len(entries) > 0 && entries[0].EventType != "completed" {
		t.Errorf("expected newest entry first (completed), got %s", entries[0].EventType)
	}
	if len(entries) > 2 && entries[2].EventType != "created" {
		t.Errorf("expected oldest entry last (created), got %s", entries[2].EventType)
	}
}

// TestGetItemHistory_WithLimit verifies the limit parameter works correctly.
func TestGetItemHistory_WithLimit(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	// Insert 10 history entries
	for i := 0; i < 10; i++ {
		insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(time.Duration(-i)*time.Hour))
	}

	// Act
	entries, err := db.GetItemHistory("ts-test1", 5)

	// Assert
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 history entries (limit), got %d", len(entries))
	}
}

// TestGetItemHistory_Empty verifies empty result when item has no history.
func TestGetItemHistory_Empty(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// Act
	entries, err := db.GetItemHistory("ts-test1", 50)

	// Assert
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 history entries, got %d", len(entries))
	}
}

// TestGetRecentHistory_Basic verifies recent history across all items is returned.
func TestGetRecentHistory_Basic(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")
	insertTestItemWithoutHistory(t, db, "ts-test2", "test")
	insertTestItemWithoutHistory(t, db, "ts-test3", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-2*time.Hour))
	insertHistoryEntry(t, db, "ts-test2", "status_changed", "agent-2", "primary", nil, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test3", "completed", "agent-1", "subagent", nil, now)

	// Act
	entries, err := db.GetHistory(HistoryQueryOptions{Limit: 50})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 history entries across all items, got %d", len(entries))
	}

	// Should be sorted by created_at DESC (newest first)
	if len(entries) > 0 && entries[0].ItemID != "ts-test3" {
		t.Errorf("expected newest entry first (ts-test3), got %s", entries[0].ItemID)
	}
}

// TestGetRecentHistory_FilterByEventType verifies filtering by event type.
func TestGetRecentHistory_FilterByEventType(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-3*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(-2*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "completed", "agent-1", "subagent", nil, now)

	// Act
	entries, err := db.GetHistory(HistoryQueryOptions{
		EventTypes: []string{"status_changed"},
		Limit:      50,
	})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 status_changed entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.EventType != "status_changed" {
			t.Errorf("expected event_type=status_changed, got %s", e.EventType)
		}
	}
}

// TestGetRecentHistory_FilterByActor verifies filtering by actor ID.
func TestGetRecentHistory_FilterByActor(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")
	insertTestItem(t, db, "ts-test2", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-2*time.Hour))
	insertHistoryEntry(t, db, "ts-test2", "created", "agent-2", "primary", nil, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now)

	// Act
	entries, err := db.GetHistory(HistoryQueryOptions{
		ActorID: "agent-1",
		Limit:   50,
	})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries from agent-1, got %d", len(entries))
	}
	for _, e := range entries {
		if e.ActorID != "agent-1" {
			t.Errorf("expected actor_id=agent-1, got %s", e.ActorID)
		}
	}
}

// TestGetRecentHistory_FilterBySince verifies filtering by time.
func TestGetRecentHistory_FilterBySince(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-48*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "completed", "agent-1", "subagent", nil, now)

	// Act
	entries, err := db.GetHistory(HistoryQueryOptions{
		Since: now.Add(-24 * time.Hour),
		Limit: 50,
	})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries within last 24 hours, got %d", len(entries))
	}
}

// TestGetRecentHistory_CombinedFilters verifies multiple filters work together.
func TestGetRecentHistory_CombinedFilters(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")
	insertTestItem(t, db, "ts-test2", "test")

	now := time.Now()
	// agent-1 events
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-48*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(-1*time.Hour))
	// agent-2 events
	insertHistoryEntry(t, db, "ts-test2", "status_changed", "agent-2", "primary", nil, now.Add(-30*time.Minute))
	// agent-1 completed event
	insertHistoryEntry(t, db, "ts-test1", "completed", "agent-1", "subagent", nil, now)

	// Act: Filter by agent-1 AND since last 24 hours AND event_type status_changed
	entries, err := db.GetHistory(HistoryQueryOptions{
		ActorID:    "agent-1",
		Since:      now.Add(-24 * time.Hour),
		EventTypes: []string{"status_changed"},
		Limit:      50,
	})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry matching all filters, got %d", len(entries))
	}
	if len(entries) > 0 {
		if entries[0].ActorID != "agent-1" {
			t.Errorf("expected actor_id=agent-1, got %s", entries[0].ActorID)
		}
		if entries[0].EventType != "status_changed" {
			t.Errorf("expected event_type=status_changed, got %s", entries[0].EventType)
		}
	}
}

// TestGetRecentHistory_FilterByItemID verifies filtering by specific item.
func TestGetRecentHistory_FilterByItemID(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")
	insertTestItem(t, db, "ts-test2", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-2*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test2", "created", "agent-2", "primary", nil, now)

	// Act
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID: "ts-test1",
		Limit:  50,
	})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for ts-test1, got %d", len(entries))
	}
	for _, e := range entries {
		if e.ItemID != "ts-test1" {
			t.Errorf("expected item_id=ts-test1, got %s", e.ItemID)
		}
	}
}

// TestGetRecentHistory_MultipleEventTypes verifies filtering by multiple event types.
func TestGetRecentHistory_MultipleEventTypes(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	insertHistoryEntry(t, db, "ts-test1", "created", "agent-1", "subagent", nil, now.Add(-3*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(-2*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "title_changed", "agent-1", "subagent", nil, now.Add(-1*time.Hour))
	insertHistoryEntry(t, db, "ts-test1", "completed", "agent-1", "subagent", nil, now)

	// Act: Filter by created OR completed
	entries, err := db.GetHistory(HistoryQueryOptions{
		EventTypes: []string{"created", "completed"},
		Limit:      50,
	})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (created + completed), got %d", len(entries))
	}
}

// TestGetRecentHistory_DefaultLimit verifies default limit is applied.
func TestGetRecentHistory_DefaultLimit(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	// Insert 100 history entries
	for i := 0; i < 100; i++ {
		insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", nil, now.Add(time.Duration(-i)*time.Hour))
	}

	// Act: Call with Limit=0 (should use default 50)
	entries, err := db.GetHistory(HistoryQueryOptions{Limit: 0})

	// Assert
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 50 {
		t.Errorf("expected 50 entries (default limit), got %d", len(entries))
	}
}

// TestGetRecentHistory_ChangesJSON verifies JSON changes are parsed correctly.
func TestGetRecentHistory_ChangesJSON(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	now := time.Now()
	changes := map[string]any{
		"old_status": "open",
		"new_status": "in_progress",
		"reason":     "started work",
	}
	insertHistoryEntry(t, db, "ts-test1", "status_changed", "agent-1", "subagent", changes, now)

	// Act
	entries, err := db.GetItemHistory("ts-test1", 50)

	// Assert
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Changes == nil {
		t.Fatal("expected Changes to be non-nil")
	}
	if entry.Changes["old_status"] != "open" {
		t.Errorf("expected old_status=open, got %v", entry.Changes["old_status"])
	}
	if entry.Changes["new_status"] != "in_progress" {
		t.Errorf("expected new_status=in_progress, got %v", entry.Changes["new_status"])
	}
}

// TestGetRecentHistory_MalformedJSON verifies graceful handling of malformed JSON in changes.
func TestGetRecentHistory_MalformedJSON(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// Insert entry with malformed JSON directly
	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO history (item_id, event_type, actor_id, actor_type, changes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "ts-test1", "status_changed", "agent-1", "subagent", "{invalid json", sqlTime(now))
	if err != nil {
		t.Fatalf("failed to insert history entry: %v", err)
	}

	// Act
	entries, err := db.GetItemHistory("ts-test1", 50)

	// Assert: Should not fail, Changes should be nil or empty
	if err != nil {
		t.Fatalf("GetItemHistory should handle malformed JSON gracefully: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Malformed JSON should result in nil Changes (graceful degradation)
	if entries[0].Changes != nil && len(entries[0].Changes) > 0 {
		t.Logf("note: Changes is non-nil, this is acceptable if implementation chooses to return empty map")
	}
}

// TestGetRecentlyClosed_Basic verifies recently closed items are returned.
func TestGetRecentlyClosed_Basic(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)

	now := time.Now()

	// Create and close some items
	for i, status := range []struct {
		id     string
		status model.Status
		closed bool
	}{
		{"ts-open1", model.StatusOpen, false},
		{"ts-done1", model.StatusDone, true},
		{"ts-done2", model.StatusDone, true},
		{"ts-canceled1", model.StatusCanceled, true},
		{"ts-progress1", model.StatusInProgress, false},
	} {
		item := &model.Item{
			ID:        status.id,
			Project:   "test",
			Type:      model.ItemTypeTask,
			Title:     "Test Item " + status.id,
			Status:    status.status,
			Priority:  2,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.CreateItem(item); err != nil {
			t.Fatalf("failed to create test item: %v", err)
		}
		// Set closed_at for closed items
		if status.closed {
			closedAt := now.Add(time.Duration(-i) * time.Hour)
			_, err := db.Exec("UPDATE items SET closed_at = ? WHERE id = ?", sqlTime(closedAt), status.id)
			if err != nil {
				t.Fatalf("failed to set closed_at: %v", err)
			}
		}
	}

	// Act
	items, err := db.GetRecentlyClosed(10, time.Time{})

	// Assert
	if err != nil {
		t.Fatalf("GetRecentlyClosed failed: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 closed items, got %d", len(items))
	}

	// Should be sorted by closed_at DESC (most recently closed first)
	// ts-done1 was closed most recently (i=1, -1 hour)
	if len(items) > 0 && items[0].ID != "ts-done1" {
		t.Errorf("expected most recently closed item first (ts-done1), got %s", items[0].ID)
	}
}

// TestGetRecentlyClosed_WithSince verifies the since filter works.
func TestGetRecentlyClosed_WithSince(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)

	now := time.Now()

	// Create items closed at different times
	items := []struct {
		id       string
		closedAt time.Time
	}{
		{"ts-old", now.Add(-48 * time.Hour)}, // Old closure
		{"ts-recent1", now.Add(-1 * time.Hour)},
		{"ts-recent2", now.Add(-30 * time.Minute)},
	}

	for _, tc := range items {
		item := &model.Item{
			ID:        tc.id,
			Project:   "test",
			Type:      model.ItemTypeTask,
			Title:     "Test Item",
			Status:    model.StatusDone,
			Priority:  2,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.CreateItem(item); err != nil {
			t.Fatalf("failed to create test item: %v", err)
		}
		_, err := db.Exec("UPDATE items SET closed_at = ? WHERE id = ?", sqlTime(tc.closedAt), tc.id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act: Get items closed in last 24 hours
	result, err := db.GetRecentlyClosed(10, now.Add(-24*time.Hour))

	// Assert
	if err != nil {
		t.Fatalf("GetRecentlyClosed failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 items closed in last 24 hours, got %d", len(result))
	}
}

// TestGetRecentlyClosed_WithLimit verifies the limit parameter works.
func TestGetRecentlyClosed_WithLimit(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)

	now := time.Now()

	// Create 10 closed items
	for i := 0; i < 10; i++ {
		id := model.GenerateID(model.ItemTypeTask)
		item := &model.Item{
			ID:        id,
			Project:   "test",
			Type:      model.ItemTypeTask,
			Title:     "Test Item",
			Status:    model.StatusDone,
			Priority:  2,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.CreateItem(item); err != nil {
			t.Fatalf("failed to create test item: %v", err)
		}
		closedAt := now.Add(time.Duration(-i) * time.Hour)
		_, err := db.Exec("UPDATE items SET closed_at = ? WHERE id = ?", sqlTime(closedAt), id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act
	items, err := db.GetRecentlyClosed(5, time.Time{})

	// Assert
	if err != nil {
		t.Fatalf("GetRecentlyClosed failed: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("expected 5 items (limit), got %d", len(items))
	}
}

// TestGetRecentlyClosed_Empty verifies empty result when no closed items exist.
func TestGetRecentlyClosed_Empty(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-open1", "test")
	insertTestItem(t, db, "ts-open2", "test")

	// Act
	items, err := db.GetRecentlyClosed(10, time.Time{})

	// Assert
	if err != nil {
		t.Fatalf("GetRecentlyClosed failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 closed items, got %d", len(items))
	}
}

// TestGetRecentlyClosed_ReturnsFullItem verifies full Item objects are returned.
func TestGetRecentlyClosed_ReturnsFullItem(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)

	now := time.Now()
	item := &model.Item{
		ID:          "ts-test1",
		Project:     "myproject",
		Type:        model.ItemTypeTask,
		Title:       "My Test Task",
		Description: "A detailed description",
		Status:      model.StatusDone,
		Priority:    1,
		Results:     "Task completed successfully",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create test item: %v", err)
	}
	_, err := db.Exec("UPDATE items SET closed_at = ? WHERE id = ?", sqlTime(now), "ts-test1")
	if err != nil {
		t.Fatalf("failed to set closed_at: %v", err)
	}

	// Act
	items, err := db.GetRecentlyClosed(10, time.Time{})

	// Assert
	if err != nil {
		t.Fatalf("GetRecentlyClosed failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	result := items[0]
	if result.ID != "ts-test1" {
		t.Errorf("expected ID=ts-test1, got %s", result.ID)
	}
	if result.Project != "myproject" {
		t.Errorf("expected Project=myproject, got %s", result.Project)
	}
	if result.Title != "My Test Task" {
		t.Errorf("expected Title='My Test Task', got %s", result.Title)
	}
	if result.Description != "A detailed description" {
		t.Errorf("expected Description set, got %s", result.Description)
	}
	if result.Status != model.StatusDone {
		t.Errorf("expected Status=done, got %s", result.Status)
	}
	if result.Priority != 1 {
		t.Errorf("expected Priority=1, got %d", result.Priority)
	}
	if result.Results != "Task completed successfully" {
		t.Errorf("expected Results set, got %s", result.Results)
	}
}

// TestHistoryEntry_NullFields verifies nullable fields are handled correctly.
func TestHistoryEntry_NullFields(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// Insert entry with NULL actor_id, actor_type, and changes
	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO history (item_id, event_type, created_at)
		VALUES (?, ?, ?)
	`, "ts-test1", "created", sqlTime(now))
	if err != nil {
		t.Fatalf("failed to insert history entry: %v", err)
	}

	// Act
	entries, err := db.GetItemHistory("ts-test1", 50)

	// Assert
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.ActorID != "" {
		t.Errorf("expected empty ActorID for NULL, got %s", entry.ActorID)
	}
	if entry.ActorType != "" {
		t.Errorf("expected empty ActorType for NULL, got %s", entry.ActorType)
	}
	if entry.Changes != nil && len(entry.Changes) > 0 {
		t.Errorf("expected nil or empty Changes for NULL, got %v", entry.Changes)
	}
}

// ============================================================================
// RecordHistory Tests
// ============================================================================

// TestRecordHistory_BasicEvent verifies that RecordHistory creates a history entry.
func TestRecordHistory_BasicEvent(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act
	err := db.RecordHistory("ts-test1", EventTypeCreated, map[string]any{
		"title": "Test Task",
	})

	// Assert
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	// Verify entry was created (1 from insertTestItem + 1 from RecordHistory = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (insertTestItem + manual), got %d", len(entries))
	}

	// Find the entry with the specific title change (the manual one)
	var found bool
	for _, entry := range entries {
		if entry.EventType == EventTypeCreated && entry.Changes != nil && entry.Changes["title"] == "Test Task" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find a 'created' event with title='Test Task'")
	}
}

// TestRecordHistory_WithActorContext verifies that agent context is recorded.
func TestRecordHistory_WithActorContext(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Set agent environment variables
	t.Setenv("AGENT_ID", "agent-xyz-123")
	t.Setenv("AGENT_TYPE", "subagent")

	// Act
	err := db.RecordHistory("ts-test1", EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Assert
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	// Query specifically for status_changed events
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID:     "ts-test1",
		EventTypes: []string{EventTypeStatusChanged},
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 status_changed entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.ActorID != "agent-xyz-123" {
		t.Errorf("expected actor_id='agent-xyz-123', got '%s'", entry.ActorID)
	}
	if entry.ActorType != "subagent" {
		t.Errorf("expected actor_type='subagent', got '%s'", entry.ActorType)
	}
}

// TestRecordHistory_WithoutActorContext verifies recording works without agent context.
func TestRecordHistory_WithoutActorContext(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Ensure no agent context
	t.Setenv("AGENT_ID", "")
	t.Setenv("AGENT_TYPE", "")

	// Act
	err := db.RecordHistory("ts-test1", EventTypeCompleted, map[string]any{
		"results": "Done",
	})

	// Assert
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	// Query specifically for completed events
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID:     "ts-test1",
		EventTypes: []string{EventTypeCompleted},
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 completed entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.ActorID != "" {
		t.Errorf("expected empty actor_id when no env set, got '%s'", entry.ActorID)
	}
	if entry.ActorType != "" {
		t.Errorf("expected empty actor_type when no env set, got '%s'", entry.ActorType)
	}
}

// TestRecordHistory_StatusChanged verifies status change events are recorded correctly.
func TestRecordHistory_StatusChanged(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act
	err := db.RecordHistory("ts-test1", EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Assert
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	// Query specifically for status_changed events
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID:     "ts-test1",
		EventTypes: []string{EventTypeStatusChanged},
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 status_changed entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.EventType != EventTypeStatusChanged {
		t.Errorf("expected event_type=%s, got %s", EventTypeStatusChanged, entry.EventType)
	}
	if entry.Changes["old"] != "open" {
		t.Errorf("expected changes.old='open', got %v", entry.Changes["old"])
	}
	if entry.Changes["new"] != "in_progress" {
		t.Errorf("expected changes.new='in_progress', got %v", entry.Changes["new"])
	}
}

// TestRecordHistory_NilChanges verifies recording works with nil changes.
func TestRecordHistory_NilChanges(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act
	err := db.RecordHistory("ts-test1", EventTypeReopened, nil)

	// Assert
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	// Query specifically for reopened events
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID:     "ts-test1",
		EventTypes: []string{EventTypeReopened},
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 reopened entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.EventType != EventTypeReopened {
		t.Errorf("expected event_type=%s, got %s", EventTypeReopened, entry.EventType)
	}
	// Changes can be nil or empty map
	if entry.Changes != nil && len(entry.Changes) > 0 {
		t.Errorf("expected nil or empty changes for nil input, got %v", entry.Changes)
	}
}

// TestRecordHistory_MultipleEvents verifies multiple events can be recorded for one item.
func TestRecordHistory_MultipleEvents(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act: Record multiple events (note: insertTestItem already recorded 1 "created")
	_ = db.RecordHistory("ts-test1", EventTypeStatusChanged, map[string]any{"old": "open", "new": "in_progress"})
	_ = db.RecordHistory("ts-test1", EventTypeCompleted, map[string]any{"results": "Done"})

	// Assert: 1 from insertTestItem + 2 manual = 3
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(entries))
	}

	// Verify all expected event types are present
	eventTypes := make(map[string]bool)
	for _, entry := range entries {
		eventTypes[entry.EventType] = true
	}
	if !eventTypes[EventTypeCreated] {
		t.Error("expected 'created' event to be present")
	}
	if !eventTypes[EventTypeStatusChanged] {
		t.Error("expected 'status_changed' event to be present")
	}
	if !eventTypes[EventTypeCompleted] {
		t.Error("expected 'completed' event to be present")
	}
}

// TestRecordHistory_DependencyEvents verifies dependency events are recorded correctly.
func TestRecordHistory_DependencyEvents(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" for ts-test1
	insertTestItem(t, db, "ts-test2", "test") // Records "created" for ts-test2

	// Act
	err := db.RecordHistory("ts-test1", EventTypeDependencyAdded, map[string]any{
		"depends_on": "ts-test2",
	})

	// Assert
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}

	// Query specifically for dependency_added events on ts-test1
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID:     "ts-test1",
		EventTypes: []string{EventTypeDependencyAdded},
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 dependency_added entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.EventType != EventTypeDependencyAdded {
		t.Errorf("expected event_type=%s, got %s", EventTypeDependencyAdded, entry.EventType)
	}
	if entry.Changes["depends_on"] != "ts-test2" {
		t.Errorf("expected changes.depends_on='ts-test2', got %v", entry.Changes["depends_on"])
	}
}

// TestRecordHistory_AllEventTypes verifies all event type constants work.
func TestRecordHistory_AllEventTypes(t *testing.T) {
	eventTypes := []string{
		EventTypeCreated,
		EventTypeStatusChanged,
		EventTypeTitleChanged,
		EventTypeDescriptionChanged,
		EventTypePriorityChanged,
		EventTypeParentChanged,
		EventTypeAssigned,
		EventTypeCompleted,
		EventTypeCanceled,
		EventTypeReopened,
		EventTypeDependencyAdded,
		EventTypeDependencyRemoved,
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			// Arrange
			db := setupTestDBWithHistory(t)
			// insertTestItemWithoutHistory bypasses CreateItem, so no auto-recorded "created" event
			insertTestItemWithoutHistory(t, db, "ts-test1", "test")

			// Act
			err := db.RecordHistory("ts-test1", eventType, nil)

			// Assert
			if err != nil {
				t.Fatalf("RecordHistory failed for %s: %v", eventType, err)
			}

			// Query for just the event type we're testing
			entries, err := db.GetHistory(HistoryQueryOptions{
				ItemID:     "ts-test1",
				EventTypes: []string{eventType},
				Limit:      50,
			})
			if err != nil {
				t.Fatalf("GetHistory failed: %v", err)
			}

			// All event types should have exactly 1 entry from RecordHistory
			if len(entries) != 1 {
				t.Fatalf("expected 1 '%s' history entry, got %d", eventType, len(entries))
			}
			// Verify the entry has the right event type
			if entries[0].EventType != eventType {
				t.Errorf("expected event_type=%s, got %s", eventType, entries[0].EventType)
			}
		})
	}
}

// ============================================================================
// Integration Tests: History recording from item operations
// These tests verify that item modification methods record history entries
// ============================================================================

// TestIntegration_CreateItemRecordsHistory verifies CreateItem records a history entry.
func TestIntegration_CreateItemRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	item := &model.Item{
		ID:        "ts-new-item",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "New Test Item",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Act
	err := db.CreateItem(item)

	// Assert
	if err != nil {
		t.Fatalf("CreateItem failed: %v", err)
	}

	// Verify history was recorded
	entries, err := db.GetItemHistory("ts-new-item", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry after CreateItem, got %d", len(entries))
	}

	entry := entries[0]
	if entry.EventType != EventTypeCreated {
		t.Errorf("expected event_type=%s, got %s", EventTypeCreated, entry.EventType)
	}
	if entry.Changes["title"] != "New Test Item" {
		t.Errorf("expected changes.title='New Test Item', got %v", entry.Changes["title"])
	}
}

// TestIntegration_UpdateStatusRecordsHistory verifies UpdateStatus records a history entry.
func TestIntegration_UpdateStatusRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // This records a "created" event

	// Act
	err := db.UpdateStatus("ts-test1", model.StatusInProgress, AgentContext{}, false)

	// Assert
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for status_changed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + status_changed), got %d", len(entries))
	}

	// Find the status_changed entry (should be first since sorted by DESC)
	var statusEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeStatusChanged {
			statusEntry = &entries[i]
			break
		}
	}
	if statusEntry == nil {
		t.Fatal("expected to find a status_changed event")
	}
	if statusEntry.Changes["old"] != "open" {
		t.Errorf("expected changes.old='open', got %v", statusEntry.Changes["old"])
	}
	if statusEntry.Changes["new"] != "in_progress" {
		t.Errorf("expected changes.new='in_progress', got %v", statusEntry.Changes["new"])
	}
}

// TestIntegration_CompleteItemRecordsHistory verifies CompleteItem records a history entry.
func TestIntegration_CompleteItemRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // This records a "created" event

	// Act
	err := db.CompleteItem("ts-test1", "Task completed successfully", AgentContext{})

	// Assert
	if err != nil {
		t.Fatalf("CompleteItem failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for completed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + completed), got %d", len(entries))
	}

	// Find the completed entry
	var completedEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeCompleted {
			completedEntry = &entries[i]
			break
		}
	}
	if completedEntry == nil {
		t.Fatal("expected to find a completed event")
	}
	if completedEntry.Changes["results"] != "Task completed successfully" {
		t.Errorf("expected changes.results='Task completed successfully', got %v", completedEntry.Changes["results"])
	}
}

// TestIntegration_CompleteItemSetsClosedAt verifies CompleteItem sets closed_at.
func TestIntegration_CompleteItemSetsClosedAt(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// Act
	err := db.CompleteItem("ts-test1", "Done", AgentContext{})

	// Assert
	if err != nil {
		t.Fatalf("CompleteItem failed: %v", err)
	}

	// Verify closed_at is set
	var closedAt sql.NullTime
	err = db.QueryRow(`SELECT closed_at FROM items WHERE id = ?`, "ts-test1").Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at: %v", err)
	}
	if !closedAt.Valid {
		t.Error("expected closed_at to be set after CompleteItem")
	}
}

// TestIntegration_UpdateStatusToDoneSetsClosedAt verifies UpdateStatus to done sets closed_at.
func TestIntegration_UpdateStatusToDoneSetsClosedAt(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// Act
	err := db.UpdateStatus("ts-test1", model.StatusDone, AgentContext{}, false)

	// Assert
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Verify closed_at is set
	var closedAt sql.NullTime
	err = db.QueryRow(`SELECT closed_at FROM items WHERE id = ?`, "ts-test1").Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at: %v", err)
	}
	if !closedAt.Valid {
		t.Error("expected closed_at to be set after UpdateStatus to done")
	}
}

// TestIntegration_UpdateStatusToCanceledSetsClosedAt verifies UpdateStatus to canceled sets closed_at.
func TestIntegration_UpdateStatusToCanceledSetsClosedAt(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// Act
	err := db.UpdateStatus("ts-test1", model.StatusCanceled, AgentContext{}, false)

	// Assert
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Verify closed_at is set
	var closedAt sql.NullTime
	err = db.QueryRow(`SELECT closed_at FROM items WHERE id = ?`, "ts-test1").Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at: %v", err)
	}
	if !closedAt.Valid {
		t.Error("expected closed_at to be set after UpdateStatus to canceled")
	}
}

// TestIntegration_ReopenClearsClosedAt verifies reopening clears closed_at.
func TestIntegration_ReopenClearsClosedAt(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItemWithoutHistory(t, db, "ts-test1", "test")

	// First, complete the item
	err := db.CompleteItem("ts-test1", "Done", AgentContext{})
	if err != nil {
		t.Fatalf("CompleteItem failed: %v", err)
	}

	// Verify closed_at is set
	var closedAt sql.NullTime
	err = db.QueryRow(`SELECT closed_at FROM items WHERE id = ?`, "ts-test1").Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at: %v", err)
	}
	if !closedAt.Valid {
		t.Fatal("expected closed_at to be set after CompleteItem")
	}

	// Act: Reopen by setting status to open
	err = db.UpdateStatus("ts-test1", model.StatusOpen, AgentContext{}, false)

	// Assert
	if err != nil {
		t.Fatalf("UpdateStatus to open failed: %v", err)
	}

	// Verify closed_at is cleared
	err = db.QueryRow(`SELECT closed_at FROM items WHERE id = ?`, "ts-test1").Scan(&closedAt)
	if err != nil {
		t.Fatalf("failed to query closed_at: %v", err)
	}
	if closedAt.Valid {
		t.Error("expected closed_at to be cleared after reopening")
	}

	// Verify reopened event was recorded
	entries, err := db.GetHistory(HistoryQueryOptions{
		ItemID:     "ts-test1",
		EventTypes: []string{EventTypeReopened},
		Limit:      50,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 reopened event, got %d", len(entries))
	}
}

// TestIntegration_AddDepRecordsHistory verifies AddDep records a history entry.
func TestIntegration_AddDepRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" for ts-test1
	insertTestItem(t, db, "ts-test2", "test") // Records "created" for ts-test2

	// Act
	err := db.AddDep("ts-test1", "ts-test2")

	// Assert
	if err != nil {
		t.Fatalf("AddDep failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for dependency_added = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + dependency_added), got %d", len(entries))
	}

	// Find the dependency_added entry
	var depEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeDependencyAdded {
			depEntry = &entries[i]
			break
		}
	}
	if depEntry == nil {
		t.Fatal("expected to find a dependency_added event")
	}
	if depEntry.Changes["depends_on"] != "ts-test2" {
		t.Errorf("expected changes.depends_on='ts-test2', got %v", depEntry.Changes["depends_on"])
	}
}

// TestIntegration_RemoveDepRecordsHistory verifies RemoveDep records a history entry.
func TestIntegration_RemoveDepRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" for ts-test1
	insertTestItem(t, db, "ts-test2", "test") // Records "created" for ts-test2
	err := db.AddDep("ts-test1", "ts-test2")  // Records "dependency_added" for ts-test1
	if err != nil {
		t.Fatalf("AddDep failed: %v", err)
	}

	// Act
	err = db.RemoveDep("ts-test1", "ts-test2")

	// Assert
	if err != nil {
		t.Fatalf("RemoveDep failed: %v", err)
	}

	// Verify history was recorded (1 created + 1 add + 1 remove = 3)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 history entries (created + add + remove), got %d", len(entries))
	}

	// Find the remove event
	var removeEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeDependencyRemoved {
			removeEntry = &entries[i]
			break
		}
	}
	if removeEntry == nil {
		t.Fatal("expected to find a dependency_removed event")
	}
	if removeEntry.Changes["depends_on"] != "ts-test2" {
		t.Errorf("expected changes.depends_on='ts-test2', got %v", removeEntry.Changes["depends_on"])
	}
}

// TestIntegration_SetTitleRecordsHistory verifies SetTitle records a history entry.
func TestIntegration_SetTitleRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act
	err := db.SetTitle("ts-test1", "New Title")

	// Assert
	if err != nil {
		t.Fatalf("SetTitle failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for title_changed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + title_changed), got %d", len(entries))
	}

	// Find the title_changed entry
	var titleEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeTitleChanged {
			titleEntry = &entries[i]
			break
		}
	}
	if titleEntry == nil {
		t.Fatal("expected to find a title_changed event")
	}
	if titleEntry.Changes["old"] != "Test Item ts-test1" {
		t.Errorf("expected changes.old='Test Item ts-test1', got %v", titleEntry.Changes["old"])
	}
	if titleEntry.Changes["new"] != "New Title" {
		t.Errorf("expected changes.new='New Title', got %v", titleEntry.Changes["new"])
	}
}

// TestIntegration_SetDescriptionRecordsHistory verifies SetDescription records a history entry.
func TestIntegration_SetDescriptionRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act
	err := db.SetDescription("ts-test1", "New description text")

	// Assert
	if err != nil {
		t.Fatalf("SetDescription failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for description_changed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + description_changed), got %d", len(entries))
	}

	// Find the description_changed entry
	var descEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeDescriptionChanged {
			descEntry = &entries[i]
			break
		}
	}
	if descEntry == nil {
		t.Fatal("expected to find a description_changed event")
	}
	if descEntry.Changes["new"] != "New description text" {
		t.Errorf("expected changes.new='New description text', got %v", descEntry.Changes["new"])
	}
}

// TestIntegration_UpdatePriorityRecordsHistory verifies UpdatePriority records a history entry.
func TestIntegration_UpdatePriorityRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)
	insertTestItem(t, db, "ts-test1", "test") // Records "created" event with priority 2

	// Act
	err := db.UpdatePriority("ts-test1", 1)

	// Assert
	if err != nil {
		t.Fatalf("UpdatePriority failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for priority_changed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + priority_changed), got %d", len(entries))
	}

	// Find the priority_changed entry
	var priorityEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypePriorityChanged {
			priorityEntry = &entries[i]
			break
		}
	}
	if priorityEntry == nil {
		t.Fatal("expected to find a priority_changed event")
	}
	// Note: JSON unmarshals numbers as float64
	if priorityEntry.Changes["old"] != float64(2) {
		t.Errorf("expected changes.old=2, got %v", priorityEntry.Changes["old"])
	}
	if priorityEntry.Changes["new"] != float64(1) {
		t.Errorf("expected changes.new=1, got %v", priorityEntry.Changes["new"])
	}
}

// TestIntegration_SetParentRecordsHistory verifies SetParent records a history entry.
func TestIntegration_SetParentRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)

	// Create an epic to be the parent
	epic := &model.Item{
		ID:        "ep-parent",
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	insertTestItem(t, db, "ts-test1", "test") // Records "created" event

	// Act
	err := db.SetParent("ts-test1", "ep-parent")

	// Assert
	if err != nil {
		t.Fatalf("SetParent failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for parent_changed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + parent_changed), got %d", len(entries))
	}

	// Find the parent_changed entry
	var parentEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeParentChanged {
			parentEntry = &entries[i]
			break
		}
	}
	if parentEntry == nil {
		t.Fatal("expected to find a parent_changed event")
	}
	if parentEntry.Changes["old"] != "" {
		t.Errorf("expected changes.old='', got %v", parentEntry.Changes["old"])
	}
	if parentEntry.Changes["new"] != "ep-parent" {
		t.Errorf("expected changes.new='ep-parent', got %v", parentEntry.Changes["new"])
	}
}

// TestIntegration_ClearParentRecordsHistory verifies ClearParent records a history entry.
func TestIntegration_ClearParentRecordsHistory(t *testing.T) {
	// Arrange
	db := setupTestDBWithHistory(t)

	// Create an epic to be the parent
	epic := &model.Item{
		ID:        "ep-parent",
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	// Create task with parent
	parentID := "ep-parent"
	task := &model.Item{
		ID:        "ts-test1",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test Task",
		Status:    model.StatusOpen,
		Priority:  2,
		ParentID:  &parentID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Act
	err := db.ClearParent("ts-test1")

	// Assert
	if err != nil {
		t.Fatalf("ClearParent failed: %v", err)
	}

	// Verify history was recorded (1 for created + 1 for parent_changed = 2)
	entries, err := db.GetItemHistory("ts-test1", 50)
	if err != nil {
		t.Fatalf("GetItemHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries (created + parent_changed), got %d", len(entries))
	}

	// Find the parent_changed entry
	var parentEntry *HistoryEntry
	for i := range entries {
		if entries[i].EventType == EventTypeParentChanged {
			parentEntry = &entries[i]
			break
		}
	}
	if parentEntry == nil {
		t.Fatal("expected to find a parent_changed event")
	}
	if parentEntry.Changes["old"] != "ep-parent" {
		t.Errorf("expected changes.old='ep-parent', got %v", parentEntry.Changes["old"])
	}
	if parentEntry.Changes["new"] != "" {
		t.Errorf("expected changes.new='', got %v", parentEntry.Changes["new"])
	}
}
