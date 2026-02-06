package db

import (
	"testing"
	"time"
)

// TestCleanupHistory_KeepsRecent24h verifies that history entries less than 24 hours old
// are never deleted, regardless of event type.
func TestCleanupHistory_KeepsRecent24h(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Insert history entries from 1 hour ago (various event types)
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	eventTypes := []string{"created", "title_changed", "description_changed", "status_changed"}
	for _, eventType := range eventTypes {
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', ?, ?)
		`, eventType, sqlTime(oneHourAgo))
		if err != nil {
			t.Fatalf("failed to insert %s event: %v", eventType, err)
		}
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: No entries should be deleted (all within 24h)
	if result.DeletedCount != 0 {
		t.Errorf("expected 0 deleted, got %d", result.DeletedCount)
	}

	// Verify all entries still exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 history entries, got %d", count)
	}
}

// TestCleanupHistory_KeepsStatusChanges30d verifies that status-related events
// (status_changed, completed, canceled, reopened) are kept for 30 days.
func TestCleanupHistory_KeepsStatusChanges30d(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'done')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Insert status-related events from 15 days ago (within 30-day retention)
	fifteenDaysAgo := time.Now().Add(-15 * 24 * time.Hour)
	statusEvents := []string{"status_changed", "completed", "canceled", "reopened"}
	for _, eventType := range statusEvents {
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', ?, ?)
		`, eventType, sqlTime(fifteenDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert %s event: %v", eventType, err)
		}
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: No entries should be deleted (status events within 30 days)
	if result.DeletedCount != 0 {
		t.Errorf("expected 0 deleted, got %d", result.DeletedCount)
	}

	// Verify all entries still exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 history entries, got %d", count)
	}
}

// TestCleanupHistory_DeletesOldMinorEvents verifies that non-status events
// older than 7 days are deleted.
func TestCleanupHistory_DeletesOldMinorEvents(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Insert minor events from 10 days ago (beyond 7-day retention for non-status)
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	minorEvents := []string{"title_changed", "description_changed", "priority_changed", "parent_changed"}
	for _, eventType := range minorEvents {
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', ?, ?)
		`, eventType, sqlTime(tenDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert %s event: %v", eventType, err)
		}
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: All minor events should be deleted (>7 days old)
	if result.DeletedCount != 4 {
		t.Errorf("expected 4 deleted, got %d", result.DeletedCount)
	}

	// Verify no entries remain
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 history entries, got %d", count)
	}
}

// TestCleanupHistory_DeletesStatusChangesAfter30d verifies that status events
// older than 30 days are deleted.
func TestCleanupHistory_DeletesStatusChangesAfter30d(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'done')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Insert status events from 45 days ago (beyond 30-day retention)
	fortyFiveDaysAgo := time.Now().Add(-45 * 24 * time.Hour)
	statusEvents := []string{"status_changed", "completed", "canceled", "reopened"}
	for _, eventType := range statusEvents {
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', ?, ?)
		`, eventType, sqlTime(fortyFiveDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert %s event: %v", eventType, err)
		}
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: All status events should be deleted (>30 days old)
	if result.DeletedCount != 4 {
		t.Errorf("expected 4 deleted, got %d", result.DeletedCount)
	}

	// Verify no entries remain
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 history entries, got %d", count)
	}
}

// TestCleanupHistory_DryRun verifies dry run mode counts but doesn't delete.
func TestCleanupHistory_DryRun(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Insert old minor events (should be eligible for deletion)
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	for i := 0; i < 5; i++ {
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', 'title_changed', ?)
		`, sqlTime(tenDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert event %d: %v", i, err)
		}
	}

	// Act: Run in dry-run mode
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: true})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: Should report 5 would be deleted
	if result.DeletedCount != 5 {
		t.Errorf("expected DeletedCount=5 in dry run, got %d", result.DeletedCount)
	}

	// Assert: Entries should still exist (not actually deleted)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 5 {
		t.Errorf("dry run should not delete - expected 5 entries, got %d", count)
	}
}

// TestCleanupHistory_EmptyHistory verifies cleanup handles empty history gracefully.
func TestCleanupHistory_EmptyHistory(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Act: Run cleanup on empty history
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed on empty history: %v", err)
	}

	// Assert: Should report 0 deleted
	if result.DeletedCount != 0 {
		t.Errorf("expected 0 deleted for empty history, got %d", result.DeletedCount)
	}
	if result.TotalBefore != 0 {
		t.Errorf("expected TotalBefore=0 for empty history, got %d", result.TotalBefore)
	}
}

// TestCleanupHistory_MixedRetention verifies correct handling of mixed event types
// with different ages, ensuring only the appropriate ones are deleted.
func TestCleanupHistory_MixedRetention(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	now := time.Now()
	testCases := []struct {
		eventType    string
		age          time.Duration
		shouldDelete bool
	}{
		// Recent events (within 24h) - never deleted
		{"title_changed", 1 * time.Hour, false},
		{"status_changed", 12 * time.Hour, false},

		// Minor events between 7-30 days - should be deleted
		{"title_changed", 10 * 24 * time.Hour, true},
		{"description_changed", 15 * 24 * time.Hour, true},
		{"priority_changed", 8 * 24 * time.Hour, true},

		// Status events between 7-30 days - should NOT be deleted
		{"status_changed", 10 * 24 * time.Hour, false},
		{"completed", 20 * 24 * time.Hour, false},
		{"canceled", 25 * 24 * time.Hour, false},

		// Status events older than 30 days - should be deleted
		{"status_changed", 35 * 24 * time.Hour, true},
		{"completed", 40 * 24 * time.Hour, true},

		// Minor events at boundary (exactly 7 days - should be kept)
		{"title_changed", 7 * 24 * time.Hour, false},
	}

	expectedDeleted := 0
	expectedKept := 0
	for _, tc := range testCases {
		createdAt := now.Add(-tc.age)
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', ?, ?)
		`, tc.eventType, sqlTime(createdAt))
		if err != nil {
			t.Fatalf("failed to insert %s event (age %v): %v", tc.eventType, tc.age, err)
		}
		if tc.shouldDelete {
			expectedDeleted++
		} else {
			expectedKept++
		}
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: Correct number deleted
	if result.DeletedCount != expectedDeleted {
		t.Errorf("expected %d deleted, got %d", expectedDeleted, result.DeletedCount)
	}

	// Verify remaining entries
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history WHERE item_id = 'ts-test1'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != expectedKept {
		t.Errorf("expected %d entries remaining, got %d", expectedKept, count)
	}
}

// TestCleanupHistory_ResultBreakdown verifies the result struct contains
// accurate breakdown of what was deleted by category.
func TestCleanupHistory_ResultBreakdown(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	now := time.Now()

	// Insert: 2 recent (kept), 3 old minor (deleted), 2 old status (deleted)

	// Recent entries (1 hour ago) - should be kept
	oneHourAgo := now.Add(-1 * time.Hour)
	_, err = db.Exec(`INSERT INTO history (item_id, event_type, created_at) VALUES ('ts-test1', 'title_changed', ?)`, sqlTime(oneHourAgo))
	if err != nil {
		t.Fatalf("failed to insert recent: %v", err)
	}
	_, err = db.Exec(`INSERT INTO history (item_id, event_type, created_at) VALUES ('ts-test1', 'status_changed', ?)`, sqlTime(oneHourAgo))
	if err != nil {
		t.Fatalf("failed to insert recent: %v", err)
	}

	// Old minor events (10 days) - should be deleted
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	for i := 0; i < 3; i++ {
		_, err = db.Exec(`INSERT INTO history (item_id, event_type, created_at) VALUES ('ts-test1', 'title_changed', ?)`, sqlTime(tenDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert old minor: %v", err)
		}
	}

	// Old status events (45 days) - should be deleted
	fortyFiveDaysAgo := now.Add(-45 * 24 * time.Hour)
	_, err = db.Exec(`INSERT INTO history (item_id, event_type, created_at) VALUES ('ts-test1', 'completed', ?)`, sqlTime(fortyFiveDaysAgo))
	if err != nil {
		t.Fatalf("failed to insert old status: %v", err)
	}
	_, err = db.Exec(`INSERT INTO history (item_id, event_type, created_at) VALUES ('ts-test1', 'status_changed', ?)`, sqlTime(fortyFiveDaysAgo))
	if err != nil {
		t.Fatalf("failed to insert old status: %v", err)
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: Verify totals
	if result.TotalBefore != 7 {
		t.Errorf("expected TotalBefore=7, got %d", result.TotalBefore)
	}
	if result.DeletedCount != 5 {
		t.Errorf("expected DeletedCount=5, got %d", result.DeletedCount)
	}

	// Assert: Verify breakdown
	if result.DeletedRecent != 0 {
		t.Errorf("expected DeletedRecent=0 (never deletes recent), got %d", result.DeletedRecent)
	}
	if result.DeletedOther != 3 {
		t.Errorf("expected DeletedOther=3 (old minor events), got %d", result.DeletedOther)
	}
	if result.DeletedStatus != 2 {
		t.Errorf("expected DeletedStatus=2 (old status events), got %d", result.DeletedStatus)
	}
}

// TestCleanupHistory_MultipleItems verifies cleanup works across multiple items.
func TestCleanupHistory_MultipleItems(t *testing.T) {
	// Arrange
	db := setupTestDB(t)

	// Create multiple items
	for i := 1; i <= 3; i++ {
		_, err := db.Exec(`
			INSERT INTO items (id, project, type, title, status)
			VALUES (?, 'test', 'task', 'Test', 'open')
		`, "ts-test"+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("failed to create item %d: %v", i, err)
		}
	}

	// Insert old events for each item
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	for i := 1; i <= 3; i++ {
		itemID := "ts-test" + string(rune('0'+i))
		_, err := db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES (?, 'title_changed', ?)
		`, itemID, sqlTime(tenDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert event for item %d: %v", i, err)
		}
	}

	// Act
	result, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("CleanupHistory failed: %v", err)
	}

	// Assert: All 3 should be deleted
	if result.DeletedCount != 3 {
		t.Errorf("expected 3 deleted across multiple items, got %d", result.DeletedCount)
	}

	// Verify history is empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM history").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count history: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 history entries, got %d", count)
	}
}

// TestCleanupHistory_DryRunMatchesActual verifies dry run reports same count as actual delete.
func TestCleanupHistory_DryRunMatchesActual(t *testing.T) {
	// This test runs dry-run first, then actual, comparing results

	// Arrange
	db := setupTestDB(t)

	// Create an item
	_, err := db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test1', 'test', 'task', 'Test', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Insert a mix of events
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	for i := 0; i < 5; i++ {
		_, err = db.Exec(`
			INSERT INTO history (item_id, event_type, created_at)
			VALUES ('ts-test1', 'title_changed', ?)
		`, sqlTime(tenDaysAgo))
		if err != nil {
			t.Fatalf("failed to insert event %d: %v", i, err)
		}
	}

	// Act: First do dry run
	dryResult, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	// Act: Then do actual cleanup
	actualResult, err := db.CleanupHistory(CleanupHistoryOptions{DryRun: false})
	if err != nil {
		t.Fatalf("actual cleanup failed: %v", err)
	}

	// Assert: Counts should match
	if dryResult.DeletedCount != actualResult.DeletedCount {
		t.Errorf("dry run reported %d, actual deleted %d", dryResult.DeletedCount, actualResult.DeletedCount)
	}
	if dryResult.TotalBefore != actualResult.TotalBefore {
		t.Errorf("dry run TotalBefore=%d, actual TotalBefore=%d", dryResult.TotalBefore, actualResult.TotalBefore)
	}
}
