package db

import (
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestCountOldItems(t *testing.T) {
	db := setupTestDB(t)

	// Create some items with different statuses and ages
	now := time.Now()
	oldTime := now.AddDate(0, 0, -60) // 60 days ago

	// Create an old done item
	oldDone := &model.Item{
		ID:        "ts-old1",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Old done task",
		Status:    model.StatusDone,
		Priority:  2,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}
	if err := db.CreateItem(oldDone); err != nil {
		t.Fatalf("failed to create old done item: %v", err)
	}
	// Manually update the timestamp to be old
	_, err := db.Exec("UPDATE items SET updated_at = ? WHERE id = ?", sqlTime(oldTime), oldDone.ID)
	if err != nil {
		t.Fatalf("failed to update timestamp: %v", err)
	}

	// Create a recent done item
	recentDone := &model.Item{
		ID:        "ts-new1",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Recent done task",
		Status:    model.StatusDone,
		Priority:  2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateItem(recentDone); err != nil {
		t.Fatalf("failed to create recent done item: %v", err)
	}

	// Create an old canceled item
	oldCanceled := &model.Item{
		ID:        "ts-old2",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Old canceled task",
		Status:    model.StatusCanceled,
		Priority:  2,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}
	if err := db.CreateItem(oldCanceled); err != nil {
		t.Fatalf("failed to create old canceled item: %v", err)
	}
	_, err = db.Exec("UPDATE items SET updated_at = ? WHERE id = ?", sqlTime(oldTime), oldCanceled.ID)
	if err != nil {
		t.Fatalf("failed to update timestamp: %v", err)
	}

	// Create an open item (should never be counted)
	openItem := &model.Item{
		ID:        "ts-open",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Open task",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}
	if err := db.CreateItem(openItem); err != nil {
		t.Fatalf("failed to create open item: %v", err)
	}
	_, err = db.Exec("UPDATE items SET updated_at = ? WHERE id = ?", sqlTime(oldTime), openItem.ID)
	if err != nil {
		t.Fatalf("failed to update timestamp: %v", err)
	}

	// Test counting old done items (30 day threshold)
	cutoff := now.AddDate(0, 0, -30)
	count, err := db.CountOldItems(cutoff, model.StatusDone)
	if err != nil {
		t.Fatalf("CountOldItems failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 old done item, got %d", count)
	}

	// Test counting old canceled items
	count, err = db.CountOldItems(cutoff, model.StatusCanceled)
	if err != nil {
		t.Fatalf("CountOldItems failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 old canceled item, got %d", count)
	}

	// Test counting old open items (should be 1 but we shouldn't delete open items)
	count, err = db.CountOldItems(cutoff, model.StatusOpen)
	if err != nil {
		t.Fatalf("CountOldItems failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 old open item, got %d", count)
	}
}

func TestDeleteOldItems(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	oldTime := now.AddDate(0, 0, -60)

	// Create an old done item with logs and dependencies
	oldDone := &model.Item{
		ID:        "ts-old1",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Old done task",
		Status:    model.StatusDone,
		Priority:  2,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}
	if err := db.CreateItem(oldDone); err != nil {
		t.Fatalf("failed to create old done item: %v", err)
	}
	_, err := db.Exec("UPDATE items SET updated_at = ? WHERE id = ?", sqlTime(oldTime), oldDone.ID)
	if err != nil {
		t.Fatalf("failed to update timestamp: %v", err)
	}

	// Add a log to the old item
	if err := db.AddLog(oldDone.ID, "test log"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}
	// Reset the updated_at after adding log
	_, err = db.Exec("UPDATE items SET updated_at = ? WHERE id = ?", sqlTime(oldTime), oldDone.ID)
	if err != nil {
		t.Fatalf("failed to update timestamp: %v", err)
	}

	// Create another item that depends on the old one
	dependent := &model.Item{
		ID:        "ts-dep",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Dependent task",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.CreateItem(dependent); err != nil {
		t.Fatalf("failed to create dependent item: %v", err)
	}
	if err := db.AddDep(dependent.ID, oldDone.ID); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Delete old done items
	cutoff := now.AddDate(0, 0, -30)
	deleted, err := db.DeleteOldItems(cutoff, model.StatusDone)
	if err != nil {
		t.Fatalf("DeleteOldItems failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted item, got %d", deleted)
	}

	// Verify item is gone
	_, err = db.GetItem(oldDone.ID)
	if err == nil {
		t.Error("expected item to be deleted")
	}

	// Verify logs are gone
	logs, err := db.GetLogs(oldDone.ID)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}

	// Verify dependency is gone
	deps, err := db.GetDeps(dependent.ID)
	if err != nil {
		t.Fatalf("GetDeps failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps))
	}
}

func TestOrphanedLogs(t *testing.T) {
	db := setupTestDB(t)

	// Create an item
	item := &model.Item{
		ID:        "ts-test",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test task",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Add a log
	if err := db.AddLog(item.ID, "test log"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	// Temporarily disable foreign keys to insert an orphaned log
	_, err := db.Exec("PRAGMA foreign_keys = OFF")
	if err != nil {
		t.Fatalf("failed to disable foreign keys: %v", err)
	}

	// Manually insert an orphaned log (referencing non-existent item)
	_, err = db.Exec("INSERT INTO logs (item_id, message) VALUES (?, ?)", "ts-nonexistent", "orphaned log")
	if err != nil {
		t.Fatalf("failed to insert orphaned log: %v", err)
	}

	// Re-enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("failed to re-enable foreign keys: %v", err)
	}

	// Count orphaned logs
	count, err := db.CountOrphanedLogs()
	if err != nil {
		t.Fatalf("CountOrphanedLogs failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 orphaned log, got %d", count)
	}

	// Delete orphaned logs
	deleted, err := db.DeleteOrphanedLogs()
	if err != nil {
		t.Fatalf("DeleteOrphanedLogs failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted log, got %d", deleted)
	}

	// Verify orphaned log is gone
	count, err = db.CountOrphanedLogs()
	if err != nil {
		t.Fatalf("CountOrphanedLogs failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 orphaned logs, got %d", count)
	}

	// Verify valid log still exists
	logs, err := db.GetLogs(item.ID)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(logs))
	}
}

func TestVacuum(t *testing.T) {
	db := setupTestDB(t)

	// Just verify vacuum doesn't error
	if err := db.Vacuum(); err != nil {
		t.Fatalf("Vacuum failed: %v", err)
	}
}

func TestGetDatabaseSize(t *testing.T) {
	db := setupTestDB(t)

	size, err := db.GetDatabaseSize()
	if err != nil {
		t.Fatalf("GetDatabaseSize failed: %v", err)
	}
	if size <= 0 {
		t.Errorf("expected positive database size, got %d", size)
	}
}

func TestDeleteOldItemsWithLabels(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	oldTime := now.AddDate(0, 0, -60)

	// Create an old done item
	oldDone := &model.Item{
		ID:        "ts-old1",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Old done task",
		Status:    model.StatusDone,
		Priority:  2,
		CreatedAt: oldTime,
		UpdatedAt: oldTime,
	}
	if err := db.CreateItem(oldDone); err != nil {
		t.Fatalf("failed to create old done item: %v", err)
	}
	_, err := db.Exec("UPDATE items SET updated_at = ? WHERE id = ?", sqlTime(oldTime), oldDone.ID)
	if err != nil {
		t.Fatalf("failed to update timestamp: %v", err)
	}

	// Add a label to the item
	if err := db.AddLabelToItem(oldDone.ID, "test", "bug"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}

	// Delete old done items
	cutoff := now.AddDate(0, 0, -30)
	deleted, err := db.DeleteOldItems(cutoff, model.StatusDone)
	if err != nil {
		t.Fatalf("DeleteOldItems failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted item, got %d", deleted)
	}

	// Verify item is gone
	_, err = db.GetItem(oldDone.ID)
	if err == nil {
		t.Error("expected item to be deleted")
	}

	// Verify item_labels association is gone (check by querying directly)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM item_labels WHERE item_id = ?", oldDone.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query item_labels: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 item_labels, got %d", count)
	}
}
