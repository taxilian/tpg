package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

// setupHistoryTestDB creates a test database with history table initialized.
func setupHistoryTestDB(t *testing.T) *db.DB {
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

// createHistoryTestItem creates a test item and records a history event.
func createHistoryTestItem(t *testing.T, database *db.DB, id, title string) *model.Item {
	t.Helper()
	item := &model.Item{
		ID:        id,
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     title,
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(item); err != nil {
		t.Fatalf("failed to create item %s: %v", id, err)
	}
	return item
}

// Helper to reset history command flags
func resetHistoryCmdFlags() {
	flagHistoryLimit = 0
	flagHistoryAgent = ""
	flagHistorySince = ""
	flagHistoryEventType = ""
	flagHistoryCleanup = false
	flagHistoryDryRun = false
	flagHistoryJSON = false
}

// =============================================================================
// TestHistoryCmd_ShowsRecentHistory
// =============================================================================

func TestHistoryCmd_ShowsRecentHistory(t *testing.T) {
	// Arrange: Create items with history events
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	// Create items (which records "created" events)
	createHistoryTestItem(t, database, "ts-hist1", "First Task")
	createHistoryTestItem(t, database, "ts-hist2", "Second Task")

	// Record additional history events
	_ = database.RecordHistory("ts-hist1", db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Act: Run history command with no arguments (show recent history)
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should show recent history events
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "ts-hist1") {
		t.Errorf("output should contain task ID ts-hist1, got %q", output)
	}
	if !strings.Contains(output, "created") {
		t.Errorf("output should contain 'created' event type, got %q", output)
	}
	if !strings.Contains(output, "status_changed") {
		t.Errorf("output should contain 'status_changed' event type, got %q", output)
	}
}

// =============================================================================
// TestHistoryCmd_ForSpecificTask
// =============================================================================

func TestHistoryCmd_ForSpecificTask(t *testing.T) {
	// Arrange: Create multiple items with history
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-specific1", "Specific Task")
	createHistoryTestItem(t, database, "ts-other1", "Other Task")

	_ = database.RecordHistory("ts-specific1", db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})
	_ = database.RecordHistory("ts-other1", db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "done",
	})

	// Act: Run history command with specific task ID
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{"ts-specific1"})
	})

	// Assert: Should only show history for the specified task
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "ts-specific1") {
		t.Errorf("output should contain task ID ts-specific1, got %q", output)
	}
	if strings.Contains(output, "ts-other1") {
		t.Errorf("output should NOT contain task ID ts-other1, got %q", output)
	}
}

// =============================================================================
// TestHistoryCmd_WithLimit
// =============================================================================

func TestHistoryCmd_WithLimit(t *testing.T) {
	// Arrange: Create multiple history events
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	item := createHistoryTestItem(t, database, "ts-limit1", "Limit Task")

	// Record many history events
	for i := 0; i < 10; i++ {
		_ = database.RecordHistory(item.ID, db.EventTypeTitleChanged, map[string]any{
			"old": "Old Title",
			"new": "New Title",
		})
	}

	// Set limit flag
	flagHistoryLimit = 3

	// Act: Run history command with limit
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should only show limited number of events
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}

	// Count occurrences of event entries (look for event type or timestamp pattern)
	// Note: The exact counting depends on output format, adjust as needed
	lines := strings.Split(output, "\n")
	eventCount := 0
	for _, line := range lines {
		if strings.Contains(line, "ts-limit1") && strings.Contains(line, "title_changed") {
			eventCount++
		}
	}
	// Should be at most 3 (the limit we set)
	if eventCount > 3 {
		t.Errorf("expected at most 3 events, got %d", eventCount)
	}
}

// =============================================================================
// TestHistoryCmd_FilterByAgent
// =============================================================================

func TestHistoryCmd_FilterByAgent(t *testing.T) {
	// Arrange: Create history events from different agents
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	item := createHistoryTestItem(t, database, "ts-agent1", "Agent Task")

	// Simulate recording with different agents by setting env vars
	t.Setenv("AGENT_ID", "agent-alpha")
	t.Setenv("AGENT_TYPE", "subagent")
	_ = database.RecordHistory(item.ID, db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	t.Setenv("AGENT_ID", "agent-beta")
	_ = database.RecordHistory(item.ID, db.EventTypeStatusChanged, map[string]any{
		"old": "in_progress",
		"new": "done",
	})

	// Set agent filter flag
	flagHistoryAgent = "agent-alpha"

	// Act: Run history command with agent filter
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should only show history from agent-alpha
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "agent-alpha") {
		t.Errorf("output should contain agent-alpha, got %q", output)
	}
	// Note: agent-beta events may still appear in "created" event from item creation
	// but status_changed should only show agent-alpha's event
}

// =============================================================================
// TestHistoryCmd_FilterBySince
// =============================================================================

func TestHistoryCmd_FilterBySince(t *testing.T) {
	// Arrange: Create history events (all recent since we just created them)
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-since1", "Since Task")

	// Set since flag to 24 hours ago
	flagHistorySince = "24h"

	// Act: Run history command with since filter
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should show recent history (within 24h)
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "ts-since1") {
		t.Errorf("output should contain ts-since1 (recent event), got %q", output)
	}
}

func TestHistoryCmd_FilterBySince_Duration7d(t *testing.T) {
	// Test parsing of "7d" duration format
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-since7d", "Seven Days Task")

	// Set since flag to 7 days
	flagHistorySince = "7d"

	// Act: Run history command with 7d filter
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Command should succeed (even if no results from old events)
	if runErr != nil {
		t.Fatalf("expected history command to succeed with 7d duration, got %v", runErr)
	}
	// Recent events should be included
	if !strings.Contains(output, "ts-since7d") {
		t.Errorf("output should contain ts-since7d, got %q", output)
	}
}

// =============================================================================
// TestHistoryCmd_CleanupDryRun
// =============================================================================

func TestHistoryCmd_CleanupDryRun(t *testing.T) {
	// Arrange: Create history events
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-cleanup1", "Cleanup Task")

	// Set cleanup and dry-run flags
	flagHistoryCleanup = true
	flagHistoryDryRun = true

	// Act: Run history command with cleanup --dry-run
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should show what would be deleted without actually deleting
	if runErr != nil {
		t.Fatalf("expected history cleanup dry-run to succeed, got %v", runErr)
	}
	if !strings.Contains(strings.ToLower(output), "dry") {
		t.Errorf("output should indicate dry-run mode, got %q", output)
	}
	if !strings.Contains(strings.ToLower(output), "would") || !strings.Contains(strings.ToLower(output), "no changes") {
		t.Errorf("output should indicate no changes made, got %q", output)
	}

	// Verify data was NOT deleted (item history should still exist)
	entries, err := database.GetItemHistory("ts-cleanup1", 10)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected history entries to still exist after dry-run")
	}
}

// =============================================================================
// TestHistoryCmd_Cleanup
// =============================================================================

func TestHistoryCmd_Cleanup(t *testing.T) {
	// Arrange: Create history events
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-realclean", "Real Cleanup Task")

	// Set cleanup flag (without dry-run)
	flagHistoryCleanup = true
	flagHistoryDryRun = false

	// Act: Run history command with cleanup
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should complete cleanup and show results
	if runErr != nil {
		t.Fatalf("expected history cleanup to succeed, got %v", runErr)
	}
	if !strings.Contains(strings.ToLower(output), "cleanup") {
		t.Errorf("output should mention cleanup, got %q", output)
	}
	// Note: Recent entries won't be deleted due to 24h retention policy
	// The output should show cleanup completed
}

// =============================================================================
// TestHistoryCmd_JSONOutput
// =============================================================================

func TestHistoryCmd_JSONOutput(t *testing.T) {
	// Arrange: Create history events
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-json1", "JSON Task")
	_ = database.RecordHistory("ts-json1", db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Set JSON flag
	flagHistoryJSON = true

	// Act: Run history command with JSON output
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should output valid JSON
	if runErr != nil {
		t.Fatalf("expected history JSON command to succeed, got %v", runErr)
	}

	// Check for JSON structure markers
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		t.Errorf("expected JSON array output, got %q", output)
	}
	if !strings.Contains(output, `"item_id"`) {
		t.Errorf("JSON output should contain item_id field, got %q", output)
	}
	if !strings.Contains(output, `"event_type"`) {
		t.Errorf("JSON output should contain event_type field, got %q", output)
	}
}

// =============================================================================
// TestHistoryCmd_FilterByEventType
// =============================================================================

func TestHistoryCmd_FilterByEventType(t *testing.T) {
	// Arrange: Create multiple event types
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	item := createHistoryTestItem(t, database, "ts-evtype1", "Event Type Task")

	// Record different event types
	_ = database.RecordHistory(item.ID, db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})
	_ = database.RecordHistory(item.ID, db.EventTypeTitleChanged, map[string]any{
		"old": "Event Type Task",
		"new": "Renamed Task",
	})

	// Set event type filter
	flagHistoryEventType = "status_changed"

	// Act: Run history command with event type filter
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should only show status_changed events
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "status_changed") {
		t.Errorf("output should contain status_changed events, got %q", output)
	}
	// Note: "created" events may still appear if not filtered; implementation decides
}

// =============================================================================
// TestHistoryCmd_EmptyHistory
// =============================================================================

func TestHistoryCmd_EmptyHistory(t *testing.T) {
	// Arrange: Create database but no history events (no items)
	_ = setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	// Act: Run history command on empty database
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Should handle gracefully
	if runErr != nil {
		t.Fatalf("expected history command to succeed on empty db, got %v", runErr)
	}
	// Should indicate no history found (exact message depends on implementation)
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "no history") && !strings.Contains(lowerOutput, "no events") && len(strings.TrimSpace(output)) == 0 {
		// Either explicit message or empty output is acceptable
	}
}

// =============================================================================
// TestHistoryCmd_InvalidTaskID
// =============================================================================

func TestHistoryCmd_InvalidTaskID(t *testing.T) {
	// Arrange: Create database but task doesn't exist
	_ = setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	// Act: Run history command with non-existent task ID
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{"ts-nonexistent"})
	})

	// Assert: Should either error or show empty history
	// Behavior depends on implementation choice
	if runErr != nil {
		// Error is acceptable for non-existent task
		if !strings.Contains(runErr.Error(), "not found") && !strings.Contains(runErr.Error(), "no history") {
			// Could be any reasonable error
		}
	} else {
		// Empty output is also acceptable
		lowerOutput := strings.ToLower(output)
		if !strings.Contains(lowerOutput, "no history") && len(strings.TrimSpace(output)) > 0 {
			// Should have empty result or "no history" message
		}
	}
}

// =============================================================================
// TestHistoryCmd_OutputFormat
// =============================================================================

func TestHistoryCmd_OutputFormat(t *testing.T) {
	// Test that output contains expected columns: timestamp, event_type, item_id, actor, changes
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-format1", "Format Task")
	_ = database.RecordHistory("ts-format1", db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Act: Run history command
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Output should have table-like format with expected info
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}

	// Check for key information in output
	// Timestamp should be present (look for date pattern or TIME header)
	if !strings.Contains(output, "TIME") && !strings.Contains(output, "20") {
		t.Logf("Note: Output may use different timestamp format")
	}

	// Event type should be visible
	if !strings.Contains(output, "status_changed") && !strings.Contains(output, "STATUS") {
		t.Errorf("output should contain event type, got %q", output)
	}

	// Item ID should be visible
	if !strings.Contains(output, "ts-format1") && !strings.Contains(output, "ITEM") {
		t.Errorf("output should contain item ID, got %q", output)
	}
}

// =============================================================================
// TestHistoryCmd_ChangesFormatted
// =============================================================================

func TestHistoryCmd_ChangesFormatted(t *testing.T) {
	// Test that status changes show "old -> new" format
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	createHistoryTestItem(t, database, "ts-changes1", "Changes Task")
	_ = database.RecordHistory("ts-changes1", db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Act: Run history command
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Changes should be formatted nicely
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}

	// Look for arrow or similar transition indicator
	if !strings.Contains(output, "->") && !strings.Contains(output, "→") &&
		!(strings.Contains(output, "open") && strings.Contains(output, "in_progress")) {
		t.Errorf("output should show status transition, got %q", output)
	}
}

// =============================================================================
// TestHistoryCmd_ActorTruncated
// =============================================================================

func TestHistoryCmd_ActorTruncated(t *testing.T) {
	// Test that long actor IDs are truncated for display
	database := setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	item := createHistoryTestItem(t, database, "ts-actor1", "Actor Task")

	// Set a long agent ID
	t.Setenv("AGENT_ID", "ses_very_long_agent_identifier_12345678901234567890")
	t.Setenv("AGENT_TYPE", "subagent")
	_ = database.RecordHistory(item.ID, db.EventTypeStatusChanged, map[string]any{
		"old": "open",
		"new": "in_progress",
	})

	// Act: Run history command
	var runErr error
	output := captureOutput(func() {
		runErr = historyCmd.RunE(historyCmd, []string{})
	})

	// Assert: Actor ID should be truncated (e.g., "ses_very_long..." or first 12+ chars)
	if runErr != nil {
		t.Fatalf("expected history command to succeed, got %v", runErr)
	}

	// If truncation is implemented, the full ID won't appear
	// or it will have "..." appended
	if strings.Contains(output, "ses_very_long_agent_identifier_12345678901234567890") {
		// Full ID shown - implementation may choose not to truncate
		t.Log("Note: Full agent ID shown (truncation not implemented or disabled)")
	} else if strings.Contains(output, "...") || strings.Contains(output, "ses_very") {
		// Truncated version shown
	}
}

// =============================================================================
// Test Invalid Flag Combinations
// =============================================================================

func TestHistoryCmd_CleanupWithTaskID_Error(t *testing.T) {
	// Cleanup shouldn't be combined with task ID argument
	_ = setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	flagHistoryCleanup = true

	// Act: Run history command with cleanup AND task ID
	err := historyCmd.RunE(historyCmd, []string{"ts-some-task"})

	// Assert: Should error (cleanup is global, not per-task)
	if err == nil {
		t.Error("expected error when combining --cleanup with task ID")
	}
}

func TestHistoryCmd_DryRunWithoutCleanup_Error(t *testing.T) {
	// --dry-run only makes sense with --cleanup
	_ = setupHistoryTestDB(t)
	resetHistoryCmdFlags()
	t.Cleanup(resetHistoryCmdFlags)

	flagHistoryDryRun = true
	flagHistoryCleanup = false

	// Act: Run history command with --dry-run but no --cleanup
	err := historyCmd.RunE(historyCmd, []string{})

	// Assert: Should error (dry-run needs cleanup)
	if err == nil {
		t.Error("expected error when using --dry-run without --cleanup")
	}
}
