package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

// setupClosedTestDB creates a test database for closed command tests.
// Unlike setupTestDB, this also sets TPG_DB env var for CLI testing.
func setupClosedTestDB(t *testing.T) *db.DB {
	t.Helper()
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "test.db")
	t.Setenv("TPG_DB", path)

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// resetClosedCmdFlags resets the closed command flags to defaults.
func resetClosedCmdFlags() {
	flagClosedLimit = 0
	flagClosedSince = ""
	flagClosedStatus = ""
}

// TestClosedCmd_ShowsRecentlyClosed verifies the closed command shows recently closed tasks.
func TestClosedCmd_ShowsRecentlyClosed(t *testing.T) {
	// Arrange: Create tasks and close some
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	// Create and close some tasks
	now := time.Now()
	items := []struct {
		id     string
		title  string
		status model.Status
	}{
		{"ts-closed1", "First closed task", model.StatusDone},
		{"ts-closed2", "Second closed task", model.StatusCanceled},
		{"ts-open1", "Still open task", model.StatusOpen},
	}

	for _, tc := range items {
		createTestItem(t, database, tc.id, tc.title, withStatus(tc.status))
	}

	// Set closed_at for closed items via direct SQL (simulating what CompleteItem does)
	for _, tc := range items {
		if tc.status == model.StatusDone || tc.status == model.StatusCanceled {
			closedAt := now.Add(-time.Hour) // closed 1 hour ago
			_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedAt.UTC().Format("2006-01-02 15:04:05"), tc.id)
			if err != nil {
				t.Fatalf("failed to set closed_at: %v", err)
			}
		}
	}

	// Act: Run the closed command
	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should show closed tasks
	if !strings.Contains(output, "ts-closed1") {
		t.Errorf("expected output to contain ts-closed1, got: %s", output)
	}
	if !strings.Contains(output, "ts-closed2") {
		t.Errorf("expected output to contain ts-closed2, got: %s", output)
	}
	if !strings.Contains(output, "First closed task") {
		t.Errorf("expected output to contain task title, got: %s", output)
	}

	// Should NOT show open task
	if strings.Contains(output, "ts-open1") {
		t.Errorf("output should not contain open task ts-open1, got: %s", output)
	}
}

// TestClosedCmd_WithLimit verifies --limit flag restricts the number of results.
func TestClosedCmd_WithLimit(t *testing.T) {
	// Arrange: Create more closed tasks than the limit
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	for i := 0; i < 10; i++ {
		id := "ts-limit" + string(rune('0'+i))
		createTestItem(t, database, id, "Closed task "+string(rune('0'+i)), withStatus(model.StatusDone))
		closedAt := now.Add(-time.Duration(i) * time.Hour) // Each closed at different times
		_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedAt.UTC().Format("2006-01-02 15:04:05"), id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act: Run with --limit 3
	flagClosedLimit = 3

	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Count how many task IDs appear in output (ts-limit prefix)
	count := strings.Count(output, "ts-limit")
	if count > 3 {
		t.Errorf("expected at most 3 results with --limit 3, got %d tasks in output: %s", count, output)
	}
}

// TestClosedCmd_WithSince verifies --since flag filters by time.
func TestClosedCmd_WithSince(t *testing.T) {
	// Arrange: Create tasks closed at different times
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	items := []struct {
		id       string
		title    string
		closedAt time.Duration // offset from now
	}{
		{"ts-recent", "Recently closed", -1 * time.Hour},  // 1 hour ago
		{"ts-old", "Old closed task", -48 * time.Hour},    // 2 days ago
		{"ts-veryold", "Very old task", -200 * time.Hour}, // 8+ days ago
	}

	for _, tc := range items {
		createTestItem(t, database, tc.id, tc.title, withStatus(model.StatusDone))
		closedAt := now.Add(tc.closedAt)
		_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedAt.UTC().Format("2006-01-02 15:04:05"), tc.id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act: Run with --since 24h (should only show tasks closed in last 24 hours)
	flagClosedSince = "24h"

	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should show recent task
	if !strings.Contains(output, "ts-recent") {
		t.Errorf("expected output to contain ts-recent (closed 1h ago), got: %s", output)
	}

	// Should NOT show old tasks
	if strings.Contains(output, "ts-old") {
		t.Errorf("output should not contain ts-old (closed 48h ago) with --since 24h, got: %s", output)
	}
	if strings.Contains(output, "ts-veryold") {
		t.Errorf("output should not contain ts-veryold (closed 8d ago) with --since 24h, got: %s", output)
	}
}

// TestClosedCmd_WithSinceDays verifies --since with day syntax (e.g., "7d").
func TestClosedCmd_WithSinceDays(t *testing.T) {
	// Arrange: Create tasks closed at different times
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	items := []struct {
		id       string
		title    string
		closedAt time.Duration
	}{
		{"ts-day1", "Closed yesterday", -24 * time.Hour},
		{"ts-day5", "Closed 5 days ago", -120 * time.Hour},
		{"ts-day10", "Closed 10 days ago", -240 * time.Hour},
	}

	for _, tc := range items {
		createTestItem(t, database, tc.id, tc.title, withStatus(model.StatusDone))
		closedAt := now.Add(tc.closedAt)
		_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedAt.UTC().Format("2006-01-02 15:04:05"), tc.id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act: Run with --since 7d
	flagClosedSince = "7d"

	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should show tasks closed within 7 days
	if !strings.Contains(output, "ts-day1") {
		t.Errorf("expected output to contain ts-day1 (closed 1 day ago), got: %s", output)
	}
	if !strings.Contains(output, "ts-day5") {
		t.Errorf("expected output to contain ts-day5 (closed 5 days ago), got: %s", output)
	}

	// Should NOT show task closed 10 days ago
	if strings.Contains(output, "ts-day10") {
		t.Errorf("output should not contain ts-day10 (closed 10 days ago) with --since 7d, got: %s", output)
	}
}

// TestClosedCmd_EmptyResult verifies empty output message when no tasks are closed.
func TestClosedCmd_EmptyResult(t *testing.T) {
	// Arrange: Create only open tasks
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)
	createTestItem(t, database, "ts-open1", "Open task 1", withStatus(model.StatusOpen))
	createTestItem(t, database, "ts-open2", "Open task 2", withStatus(model.StatusInProgress))

	// Act
	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should indicate no closed tasks
	if !strings.Contains(strings.ToLower(output), "no") {
		t.Errorf("expected output to indicate no closed tasks, got: %s", output)
	}
}

// TestClosedCmd_ShowsStatusAndClosedAt verifies output includes status and closed timestamp.
func TestClosedCmd_ShowsStatusAndClosedAt(t *testing.T) {
	// Arrange: Create a done and a canceled task
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	createTestItem(t, database, "ts-done1", "Completed task", withStatus(model.StatusDone))
	createTestItem(t, database, "ts-cancel1", "Canceled task", withStatus(model.StatusCanceled))

	// Set closed_at
	for _, id := range []string{"ts-done1", "ts-cancel1"} {
		_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", now.UTC().Format("2006-01-02 15:04:05"), id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act
	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should show status for each task type
	if !strings.Contains(output, "done") {
		t.Errorf("expected output to show 'done' status, got: %s", output)
	}
	if !strings.Contains(output, "canceled") {
		t.Errorf("expected output to show 'canceled' status, got: %s", output)
	}
}

// TestClosedCmd_SortedByClosedAtDesc verifies results are sorted by closed_at descending.
func TestClosedCmd_SortedByClosedAtDesc(t *testing.T) {
	// Arrange: Create tasks closed at different times
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	items := []struct {
		id       string
		title    string
		closedAt time.Duration
	}{
		{"ts-first", "First (most recent)", -1 * time.Hour}, // closed 1 hour ago
		{"ts-second", "Second", -5 * time.Hour},             // closed 5 hours ago
		{"ts-third", "Third (oldest)", -10 * time.Hour},     // closed 10 hours ago
	}

	// Create in random order
	createTestItem(t, database, items[1].id, items[1].title, withStatus(model.StatusDone))
	createTestItem(t, database, items[2].id, items[2].title, withStatus(model.StatusDone))
	createTestItem(t, database, items[0].id, items[0].title, withStatus(model.StatusDone))

	for _, tc := range items {
		closedAt := now.Add(tc.closedAt)
		_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedAt.UTC().Format("2006-01-02 15:04:05"), tc.id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act
	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Verify order: ts-first should appear before ts-second, which should appear before ts-third
	firstIdx := strings.Index(output, "ts-first")
	secondIdx := strings.Index(output, "ts-second")
	thirdIdx := strings.Index(output, "ts-third")

	if firstIdx == -1 || secondIdx == -1 || thirdIdx == -1 {
		t.Fatalf("not all tasks found in output: %s", output)
	}

	if firstIdx > secondIdx {
		t.Errorf("ts-first should appear before ts-second (most recent first), got firstIdx=%d, secondIdx=%d", firstIdx, secondIdx)
	}
	if secondIdx > thirdIdx {
		t.Errorf("ts-second should appear before ts-third, got secondIdx=%d, thirdIdx=%d", secondIdx, thirdIdx)
	}
}

// TestClosedCmd_UsesDBGetRecentlyClosed verifies the command uses db.GetRecentlyClosed.
func TestClosedCmd_UsesDBGetRecentlyClosed(t *testing.T) {
	// Arrange: Use db.GetRecentlyClosed directly to verify command behavior matches
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	createTestItem(t, database, "ts-verify", "Verify task", withStatus(model.StatusDone))
	_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", now.UTC().Format("2006-01-02 15:04:05"), "ts-verify")
	if err != nil {
		t.Fatalf("failed to set closed_at: %v", err)
	}

	// Verify db.GetRecentlyClosed returns our task
	items, err := database.GetRecentlyClosed(10, time.Time{})
	if err != nil {
		t.Fatalf("GetRecentlyClosed failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item from GetRecentlyClosed, got %d", len(items))
	}
	if items[0].ID != "ts-verify" {
		t.Errorf("expected item ID ts-verify, got %s", items[0].ID)
	}

	// Act: Run command and verify it shows the same task
	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}
	if !strings.Contains(output, "ts-verify") {
		t.Errorf("expected output to contain ts-verify, got: %s", output)
	}
}

// TestClosedCmd_DefaultsSince7Days verifies default --since is 7 days when not specified.
func TestClosedCmd_DefaultsSince7Days(t *testing.T) {
	// Arrange: Create a task closed 10 days ago and one closed 3 days ago
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	createTestItem(t, database, "ts-within7d", "Within 7 days", withStatus(model.StatusDone))
	createTestItem(t, database, "ts-beyond7d", "Beyond 7 days", withStatus(model.StatusDone))

	// ts-within7d closed 3 days ago
	closedWithin := now.Add(-72 * time.Hour)
	_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedWithin.UTC().Format("2006-01-02 15:04:05"), "ts-within7d")
	if err != nil {
		t.Fatalf("failed to set closed_at: %v", err)
	}

	// ts-beyond7d closed 10 days ago
	closedBeyond := now.Add(-240 * time.Hour)
	_, err = database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", closedBeyond.UTC().Format("2006-01-02 15:04:05"), "ts-beyond7d")
	if err != nil {
		t.Fatalf("failed to set closed_at: %v", err)
	}

	// Act: Run without --since (should default to 7 days)
	flagClosedSince = ""

	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should show task within 7 days
	if !strings.Contains(output, "ts-within7d") {
		t.Errorf("expected output to contain ts-within7d (closed 3 days ago), got: %s", output)
	}

	// Should NOT show task beyond 7 days (default filter)
	if strings.Contains(output, "ts-beyond7d") {
		t.Errorf("output should not contain ts-beyond7d (closed 10 days ago) with default 7d filter, got: %s", output)
	}
}

// TestClosedCmd_WithStatusFilter verifies --status flag filters by done/canceled.
func TestClosedCmd_WithStatusFilter(t *testing.T) {
	// Arrange: Create both done and canceled tasks
	database := setupClosedTestDB(t)
	resetClosedCmdFlags()
	t.Cleanup(resetClosedCmdFlags)

	now := time.Now()
	createTestItem(t, database, "ts-done1", "Done task 1", withStatus(model.StatusDone))
	createTestItem(t, database, "ts-done2", "Done task 2", withStatus(model.StatusDone))
	createTestItem(t, database, "ts-cancel1", "Canceled task", withStatus(model.StatusCanceled))

	for _, id := range []string{"ts-done1", "ts-done2", "ts-cancel1"} {
		_, err := database.Exec("UPDATE items SET closed_at = ? WHERE id = ?", now.UTC().Format("2006-01-02 15:04:05"), id)
		if err != nil {
			t.Fatalf("failed to set closed_at: %v", err)
		}
	}

	// Act: Run with --status done
	flagClosedStatus = "done"

	var runErr error
	output := captureOutput(func() {
		runErr = closedCmd.RunE(closedCmd, []string{})
	})

	// Assert
	if runErr != nil {
		t.Fatalf("closedCmd failed: %v", runErr)
	}

	// Should show done tasks
	if !strings.Contains(output, "ts-done1") {
		t.Errorf("expected output to contain ts-done1, got: %s", output)
	}
	if !strings.Contains(output, "ts-done2") {
		t.Errorf("expected output to contain ts-done2, got: %s", output)
	}

	// Should NOT show canceled task when filtering for done only
	if strings.Contains(output, "ts-cancel1") {
		t.Errorf("output should not contain ts-cancel1 when --status done, got: %s", output)
	}
}
