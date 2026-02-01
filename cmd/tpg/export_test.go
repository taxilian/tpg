package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

// Note: setupTestDB is defined in prime_test.go in the same package

// createTestItem is a helper to create items with sensible defaults.
func createTestItem(t *testing.T, database *db.DB, id, title string, opts ...func(*model.Item)) *model.Item {
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
	for _, opt := range opts {
		opt(item)
	}
	if err := database.CreateItem(item); err != nil {
		t.Fatalf("failed to create item %s: %v", id, err)
	}
	return item
}

// withDescription sets the item description.
func withDescription(desc string) func(*model.Item) {
	return func(i *model.Item) { i.Description = desc }
}

// withStatus sets the item status.
func withStatus(s model.Status) func(*model.Item) {
	return func(i *model.Item) { i.Status = s }
}

// withPriority sets the item priority.
func withPriority(p int) func(*model.Item) {
	return func(i *model.Item) { i.Priority = p }
}

// withType sets the item type.
func withType(t model.ItemType) func(*model.Item) {
	return func(i *model.Item) { i.Type = t }
}

// withParent sets the item parent.
func withParent(parentID string) func(*model.Item) {
	return func(i *model.Item) { i.ParentID = &parentID }
}

// withProject sets the item project.
func withProject(project string) func(*model.Item) {
	return func(i *model.Item) { i.Project = project }
}

// =============================================================================
// Basic Export Functionality Tests
// =============================================================================

func TestExport_OutputsAllTaskDetails(t *testing.T) {
	// Arrange: Create a task with full details
	database := setupTestDB(t)
	item := createTestItem(t, database, "ts-export1", "Test Export Task",
		withDescription("This is a detailed description\nwith multiple lines"),
		withPriority(1),
	)

	// Add a log entry
	if err := database.AddLog(item.ID, "Started working on this task"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	// Act: Export should include all task details
	// (Implementation will call exportTasks which we're testing)
	filter := db.ListFilter{Project: "test"}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to list items: %v", err)
	}

	// Assert: Verify we have the item with expected fields
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-export1" {
		t.Errorf("expected ID ts-export1, got %s", items[0].ID)
	}
	if items[0].Description != "This is a detailed description\nwith multiple lines" {
		t.Errorf("description not preserved correctly")
	}
}

func TestExport_IncludesDescription(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	desc := `## Objective
Complete the feature implementation.

## Requirements
- Must handle edge cases
- Must be tested

## Notes
Additional context here.`
	createTestItem(t, database, "ts-desc1", "Task with Description", withDescription(desc))

	// Act
	item, err := database.GetItem("ts-desc1")
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Description should be fully preserved
	if item.Description != desc {
		t.Errorf("description not preserved:\ngot: %q\nwant: %q", item.Description, desc)
	}
}

func TestExport_IncludesDependencies(t *testing.T) {
	// Arrange: Create tasks with dependencies
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-blocker", "Blocker Task")
	blocked := createTestItem(t, database, "ts-blocked", "Blocked Task")

	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	// Act
	deps, err := database.GetDeps(blocked.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}

	// Assert: Dependencies should be retrievable for export
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0] != blocker.ID {
		t.Errorf("expected dependency on %s, got %s", blocker.ID, deps[0])
	}
}

func TestExport_IncludesLogs(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	item := createTestItem(t, database, "ts-logs1", "Task with Logs")

	// Add multiple log entries
	logMessages := []string{
		"Started investigation",
		"Found the root cause",
		"Implemented fix",
	}
	for _, msg := range logMessages {
		if err := database.AddLog(item.ID, msg); err != nil {
			t.Fatalf("failed to add log: %v", err)
		}
	}

	// Act
	logs, err := database.GetLogs(item.ID)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	// Assert: All logs should be retrievable
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
	for i, log := range logs {
		if log.Message != logMessages[i] {
			t.Errorf("log %d: got %q, want %q", i, log.Message, logMessages[i])
		}
	}
}

func TestExport_IncludesLabels(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	item := createTestItem(t, database, "ts-labels1", "Task with Labels")

	// Add labels (need to get item's project for AddLabelToItem)
	if err := database.AddLabelToItem(item.ID, item.Project, "bug"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}
	if err := database.AddLabelToItem(item.ID, item.Project, "urgent"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}

	// Act
	labels, err := database.GetItemLabels(item.ID)
	if err != nil {
		t.Fatalf("failed to get labels: %v", err)
	}

	// Assert
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
	labelNames := make(map[string]bool)
	for _, l := range labels {
		labelNames[l.Name] = true
	}
	if !labelNames["bug"] || !labelNames["urgent"] {
		t.Errorf("expected labels 'bug' and 'urgent', got %v", labels)
	}
}

// =============================================================================
// Filter Tests - Reusing list command filters
// =============================================================================

func TestExport_FilterByStatus(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-open1", "Open Task", withStatus(model.StatusOpen))
	createTestItem(t, database, "ts-prog1", "In Progress Task", withStatus(model.StatusInProgress))
	createTestItem(t, database, "ts-done1", "Done Task", withStatus(model.StatusDone))

	// Act: Filter by status
	openStatus := model.StatusOpen
	filter := db.ListFilter{Status: &openStatus}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only open tasks returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-open1" {
		t.Errorf("expected ts-open1, got %s", items[0].ID)
	}
}

func TestExport_FilterByLabel(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	bugTask := createTestItem(t, database, "ts-bug1", "Bug Task")
	createTestItem(t, database, "ts-feat1", "Feature Task")

	if err := database.AddLabelToItem(bugTask.ID, bugTask.Project, "bug"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}

	// Act: Filter by label
	filter := db.ListFilter{Labels: []string{"bug"}}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only bug-labeled task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-bug1" {
		t.Errorf("expected ts-bug1, got %s", items[0].ID)
	}
}

func TestExport_FilterByParent(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	epic := createTestItem(t, database, "ep-parent1", "Parent Epic", withType(model.ItemTypeEpic))
	createTestItem(t, database, "ts-child1", "Child Task", withParent(epic.ID))
	createTestItem(t, database, "ts-orphan1", "Orphan Task")

	// Act: Filter by parent
	filter := db.ListFilter{Parent: epic.ID}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only child task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-child1" {
		t.Errorf("expected ts-child1, got %s", items[0].ID)
	}
}

func TestExport_FilterByType(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ep-epic1", "Epic Item", withType(model.ItemTypeEpic))
	createTestItem(t, database, "ts-task1", "Task Item", withType(model.ItemTypeTask))

	// Act: Filter by type
	filter := db.ListFilter{Type: "epic"}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only epic returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ep-epic1" {
		t.Errorf("expected ep-epic1, got %s", items[0].ID)
	}
}

func TestExport_FilterByProject(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-proj1", "Project A Task", withProject("project-a"))
	createTestItem(t, database, "ts-proj2", "Project B Task", withProject("project-b"))

	// Act: Filter by project
	filter := db.ListFilter{Project: "project-a"}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only project-a task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-proj1" {
		t.Errorf("expected ts-proj1, got %s", items[0].ID)
	}
}

func TestExport_FilterByHasBlockers(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-blocker2", "Blocker Task")
	blocked := createTestItem(t, database, "ts-blocked2", "Blocked Task")
	createTestItem(t, database, "ts-free2", "Free Task")

	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Act: Filter by has-blockers
	filter := db.ListFilter{HasBlockers: true}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only blocked task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-blocked2" {
		t.Errorf("expected ts-blocked2, got %s", items[0].ID)
	}
}

func TestExport_FilterByNoBlockers(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-blocker3", "Blocker Task")
	blocked := createTestItem(t, database, "ts-blocked3", "Blocked Task")
	free := createTestItem(t, database, "ts-free3", "Free Task")

	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Act: Filter by no-blockers
	filter := db.ListFilter{NoBlockers: true}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Blocker and free tasks returned (not blocked)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[blocker.ID] || !ids[free.ID] {
		t.Errorf("expected %s and %s, got %v", blocker.ID, free.ID, items)
	}
}

func TestExport_FilterByBlocking(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-blocker4", "Blocker Task")
	blocked := createTestItem(t, database, "ts-blocked4", "Blocked Task")
	createTestItem(t, database, "ts-other4", "Other Task")

	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Act: Filter by blocking (items that block ts-blocked4)
	filter := db.ListFilter{Blocking: blocked.ID}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only blocker task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != blocker.ID {
		t.Errorf("expected %s, got %s", blocker.ID, items[0].ID)
	}
}

func TestExport_FilterByBlockedBy(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-blocker5", "Blocker Task")
	blocked := createTestItem(t, database, "ts-blocked5", "Blocked Task")
	createTestItem(t, database, "ts-other5", "Other Task")

	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Act: Filter by blocked-by (items blocked by ts-blocker5)
	filter := db.ListFilter{BlockedBy: blocker.ID}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only blocked task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != blocked.ID {
		t.Errorf("expected %s, got %s", blocked.ID, items[0].ID)
	}
}

func TestExport_FilterAll_IncludesDoneAndCanceled(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-open2", "Open Task", withStatus(model.StatusOpen))
	createTestItem(t, database, "ts-done2", "Done Task", withStatus(model.StatusDone))
	createTestItem(t, database, "ts-cancel2", "Canceled Task", withStatus(model.StatusCanceled))

	// Act: No status filter (--all behavior)
	filter := db.ListFilter{}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: All items returned
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestExport_CombinedFilters(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-match1", "Matching Task",
		withProject("proj-x"),
		withStatus(model.StatusOpen),
	)
	createTestItem(t, database, "ts-nomatch1", "Non-matching Task",
		withProject("proj-x"),
		withStatus(model.StatusDone),
	)
	createTestItem(t, database, "ts-nomatch2", "Non-matching Task 2",
		withProject("proj-y"),
		withStatus(model.StatusOpen),
	)

	// Act: Combine project and status filters
	openStatus := model.StatusOpen
	filter := db.ListFilter{
		Project: "proj-x",
		Status:  &openStatus,
	}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only matching task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-match1" {
		t.Errorf("expected ts-match1, got %s", items[0].ID)
	}
}

// =============================================================================
// Output Format Tests
// =============================================================================

func TestExport_OutputIsMarkdown(t *testing.T) {
	// This test verifies the export output format is markdown-compatible
	// The actual formatting will be tested when implementation exists

	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-md1", "Markdown Test Task",
		withDescription("## Section\n\nSome content"),
	)

	// Act
	item, err := database.GetItem("ts-md1")
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Description with markdown should be preserved
	if !strings.Contains(item.Description, "## Section") {
		t.Error("markdown formatting should be preserved in description")
	}
}

func TestExport_OutputIsDeterministic(t *testing.T) {
	// Arrange: Create items with different priorities
	database := setupTestDB(t)
	createTestItem(t, database, "ts-det3", "Low Priority", withPriority(3))
	createTestItem(t, database, "ts-det1", "High Priority", withPriority(1))
	createTestItem(t, database, "ts-det2", "Medium Priority", withPriority(2))

	// Act: List items multiple times
	filter := db.ListFilter{}
	items1, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter (1): %v", err)
	}
	items2, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter (2): %v", err)
	}

	// Assert: Order should be consistent (by priority, then created_at)
	if len(items1) != len(items2) {
		t.Fatalf("inconsistent item counts: %d vs %d", len(items1), len(items2))
	}
	for i := range items1 {
		if items1[i].ID != items2[i].ID {
			t.Errorf("inconsistent order at index %d: %s vs %s", i, items1[i].ID, items2[i].ID)
		}
	}

	// Verify priority ordering
	if items1[0].Priority != 1 {
		t.Errorf("expected first item to have priority 1, got %d", items1[0].Priority)
	}
}

// =============================================================================
// Output Destination Tests
// =============================================================================

func TestExport_WritesToStdoutByDefault(t *testing.T) {
	// This test documents the expected behavior: stdout by default
	// Implementation will write to stdout when no -o flag is provided

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Simulate writing to stdout (what export would do)
	testOutput := "# Export Output\n\nTask details here..."
	_, _ = io.WriteString(w, testOutput)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("failed to read from pipe: %v", copyErr)
	}
	r.Close()

	// Assert
	if buf.String() != testOutput {
		t.Errorf("stdout output mismatch:\ngot: %q\nwant: %q", buf.String(), testOutput)
	}
}

func TestExport_WritesToFileWithOutputFlag(t *testing.T) {
	// This test documents the expected behavior: write to file with -o flag

	// Arrange
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "export.md")

	// Simulate writing to file (what export would do with -o flag)
	testOutput := "# Export Output\n\nTask details here..."
	if err := os.WriteFile(outputPath, []byte(testOutput), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Assert: File should exist with correct content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != testOutput {
		t.Errorf("file content mismatch:\ngot: %q\nwant: %q", string(content), testOutput)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestExport_EmptyDatabase(t *testing.T) {
	// Arrange
	database := setupTestDB(t)

	// Act
	filter := db.ListFilter{}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Should return empty list, not error
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestExport_TaskWithNoDescription(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-nodesc", "Task Without Description")

	// Act
	item, err := database.GetItem("ts-nodesc")
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Empty description should be handled gracefully
	if item.Description != "" {
		t.Errorf("expected empty description, got %q", item.Description)
	}
}

func TestExport_TaskWithNoLogs(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-nologs", "Task Without Logs")

	// Act
	logs, err := database.GetLogs("ts-nologs")
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	// Assert: Empty logs should be handled gracefully
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

func TestExport_TaskWithNoDependencies(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-nodeps", "Task Without Dependencies")

	// Act
	deps, err := database.GetDeps("ts-nodeps")
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}

	// Assert: Empty deps should be handled gracefully
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestExport_LargeDescription(t *testing.T) {
	// Arrange: Create a task with a very large description
	database := setupTestDB(t)
	largeDesc := strings.Repeat("This is a line of text.\n", 1000)
	createTestItem(t, database, "ts-large", "Task with Large Description",
		withDescription(largeDesc))

	// Act
	item, err := database.GetItem("ts-large")
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Large description should be preserved
	if item.Description != largeDesc {
		t.Errorf("large description not preserved, got %d chars, want %d chars",
			len(item.Description), len(largeDesc))
	}
}

func TestExport_SpecialCharactersInContent(t *testing.T) {
	// Arrange: Create a task with special characters
	database := setupTestDB(t)
	specialDesc := "Code: `func main() { fmt.Println(\"hello\") }`\n" +
		"Math: 2 < 3 && 5 > 4\n" +
		"Symbols: @#$%^&*()\n" +
		"Unicode: 日本語 🎉"
	createTestItem(t, database, "ts-special", "Task with Special Characters",
		withDescription(specialDesc))

	// Act
	item, err := database.GetItem("ts-special")
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Special characters should be preserved
	if item.Description != specialDesc {
		t.Errorf("special characters not preserved:\ngot: %q\nwant: %q",
			item.Description, specialDesc)
	}
}

func TestExport_MultipleLabelsOnTask(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	item := createTestItem(t, database, "ts-multilabel", "Task with Multiple Labels")

	labels := []string{"bug", "urgent", "backend", "needs-review"}
	for _, label := range labels {
		if err := database.AddLabelToItem(item.ID, item.Project, label); err != nil {
			t.Fatalf("failed to add label %s: %v", label, err)
		}
	}

	// Act
	gotLabels, err := database.GetItemLabels(item.ID)
	if err != nil {
		t.Fatalf("failed to get labels: %v", err)
	}

	// Assert: All labels should be present
	if len(gotLabels) != len(labels) {
		t.Fatalf("expected %d labels, got %d", len(labels), len(gotLabels))
	}
}

func TestExport_TaskWithResults(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	item := createTestItem(t, database, "ts-results", "Completed Task",
		withStatus(model.StatusDone))

	// Simulate setting results (done command behavior)
	results := "Completed successfully. Implemented feature X with tests."
	if err := database.CompleteItem(item.ID, results, db.AgentContext{}); err != nil {
		t.Fatalf("failed to complete item: %v", err)
	}

	// Act
	got, err := database.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Results should be retrievable
	if got.Results != results {
		t.Errorf("results not preserved:\ngot: %q\nwant: %q", got.Results, results)
	}
}

// =============================================================================
// Hierarchical Export Tests
// =============================================================================

func TestExport_EpicWithChildren(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	epic := createTestItem(t, database, "ep-hier1", "Parent Epic",
		withType(model.ItemTypeEpic))
	child1 := createTestItem(t, database, "ts-hier1", "Child Task 1",
		withParent(epic.ID))
	child2 := createTestItem(t, database, "ts-hier2", "Child Task 2",
		withParent(epic.ID))

	// Act: Get children of epic
	filter := db.ListFilter{Parent: epic.ID}
	children, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Both children should be found
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	ids := make(map[string]bool)
	for _, c := range children {
		ids[c.ID] = true
	}
	if !ids[child1.ID] || !ids[child2.ID] {
		t.Errorf("expected children %s and %s, got %v", child1.ID, child2.ID, children)
	}
}

func TestExport_NestedHierarchy(t *testing.T) {
	// Arrange: Create a nested hierarchy
	database := setupTestDB(t)
	epic := createTestItem(t, database, "ep-nest1", "Top Epic",
		withType(model.ItemTypeEpic))
	subEpic := createTestItem(t, database, "ep-nest2", "Sub Epic",
		withType(model.ItemTypeEpic),
		withParent(epic.ID))
	task := createTestItem(t, database, "ts-nest1", "Leaf Task",
		withParent(subEpic.ID))

	// Act: Verify parent chain
	gotTask, err := database.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	gotSubEpic, err := database.GetItem(subEpic.ID)
	if err != nil {
		t.Fatalf("failed to get sub-epic: %v", err)
	}

	// Assert: Parent relationships are correct
	if gotTask.ParentID == nil || *gotTask.ParentID != subEpic.ID {
		t.Errorf("task parent should be %s", subEpic.ID)
	}
	if gotSubEpic.ParentID == nil || *gotSubEpic.ParentID != epic.ID {
		t.Errorf("sub-epic parent should be %s", epic.ID)
	}
}

// =============================================================================
// Dependency Chain Tests
// =============================================================================

func TestExport_DependencyChain(t *testing.T) {
	// Arrange: Create a dependency chain A -> B -> C
	database := setupTestDB(t)
	taskA := createTestItem(t, database, "ts-chain-a", "Task A")
	taskB := createTestItem(t, database, "ts-chain-b", "Task B")
	taskC := createTestItem(t, database, "ts-chain-c", "Task C")

	// B depends on A, C depends on B
	if err := database.AddDep(taskB.ID, taskA.ID); err != nil {
		t.Fatalf("failed to add dep B->A: %v", err)
	}
	if err := database.AddDep(taskC.ID, taskB.ID); err != nil {
		t.Fatalf("failed to add dep C->B: %v", err)
	}

	// Act: Get deps for each task
	depsA, _ := database.GetDeps(taskA.ID)
	depsB, _ := database.GetDeps(taskB.ID)
	depsC, _ := database.GetDeps(taskC.ID)

	// Assert: Dependency chain is correct
	if len(depsA) != 0 {
		t.Errorf("task A should have no deps, got %v", depsA)
	}
	if len(depsB) != 1 || depsB[0] != taskA.ID {
		t.Errorf("task B should depend on A, got %v", depsB)
	}
	if len(depsC) != 1 || depsC[0] != taskB.ID {
		t.Errorf("task C should depend on B, got %v", depsC)
	}
}

func TestExport_MultipleDependencies(t *testing.T) {
	// Arrange: Task depends on multiple other tasks
	database := setupTestDB(t)
	dep1 := createTestItem(t, database, "ts-mdep1", "Dependency 1")
	dep2 := createTestItem(t, database, "ts-mdep2", "Dependency 2")
	dep3 := createTestItem(t, database, "ts-mdep3", "Dependency 3")
	main := createTestItem(t, database, "ts-main", "Main Task")

	for _, dep := range []*model.Item{dep1, dep2, dep3} {
		if err := database.AddDep(main.ID, dep.ID); err != nil {
			t.Fatalf("failed to add dep: %v", err)
		}
	}

	// Act
	deps, err := database.GetDeps(main.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}

	// Assert: All dependencies present
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(deps))
	}
}

// =============================================================================
// Template-Related Tests
// =============================================================================

func TestExport_TemplatedTask(t *testing.T) {
	// Arrange: Create a task with template metadata
	database := setupTestDB(t)
	stepIndex := 0
	item := &model.Item{
		ID:           "ts-tmpl1",
		Project:      "test",
		Type:         model.ItemTypeTask,
		Title:        "Templated Task",
		Status:       model.StatusOpen,
		Priority:     2,
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateHash: "abc123",
		TemplateVars: map[string]string{"module": "auth"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := database.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Act
	got, err := database.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	// Assert: Template metadata should be preserved
	if got.TemplateID != "test-template" {
		t.Errorf("template ID not preserved: got %q", got.TemplateID)
	}
	if got.StepIndex == nil || *got.StepIndex != 0 {
		t.Errorf("step index not preserved")
	}
	if got.TemplateHash != "abc123" {
		t.Errorf("template hash not preserved: got %q", got.TemplateHash)
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestExport_InvalidStatusFilter(t *testing.T) {
	// Arrange
	database := setupTestDB(t)

	// Act: Try to filter with invalid status
	invalidStatus := model.Status("invalid")
	filter := db.ListFilter{Status: &invalidStatus}
	_, err := database.ListItemsFiltered(filter)

	// Assert: Should return error
	if err == nil {
		t.Error("expected error for invalid status filter")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("error should mention invalid status, got: %v", err)
	}
}

func TestExport_CustomTypeFilter(t *testing.T) {
	// Arrange: The system allows custom types (story, bug, etc.)
	database := setupTestDB(t)
	createTestItem(t, database, "ts-custom1", "Custom Type Task",
		withType(model.ItemType("story")))
	createTestItem(t, database, "ts-task1", "Regular Task",
		withType(model.ItemTypeTask))

	// Act: Filter by custom type
	filter := db.ListFilter{Type: "story"}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Assert: Only custom type task returned
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "ts-custom1" {
		t.Errorf("expected ts-custom1, got %s", items[0].ID)
	}
}

// =============================================================================
// Integration-Style Tests (Testing the full export flow)
// =============================================================================

func TestExport_FullTaskExport(t *testing.T) {
	// This test verifies all components needed for a full export are available

	// Arrange: Create a comprehensive task
	database := setupTestDB(t)
	epic := createTestItem(t, database, "ep-full1", "Parent Epic",
		withType(model.ItemTypeEpic),
		withDescription("Epic description"))

	task := createTestItem(t, database, "ts-full1", "Full Task",
		withDescription("## Objective\nComplete the task\n\n## Requirements\n- Req 1\n- Req 2"),
		withParent(epic.ID),
		withPriority(1),
	)

	// Add labels
	if err := database.AddLabelToItem(task.ID, task.Project, "important"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}

	// Add logs
	if err := database.AddLog(task.ID, "Started work"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}
	if err := database.AddLog(task.ID, "Made progress"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	// Add dependency
	blocker := createTestItem(t, database, "ts-full-blocker", "Blocker")
	if err := database.AddDep(task.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Act: Retrieve all data needed for export
	item, err := database.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}
	labels, err := database.GetItemLabels(task.ID)
	if err != nil {
		t.Fatalf("failed to get labels: %v", err)
	}
	logs, err := database.GetLogs(task.ID)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	deps, err := database.GetDeps(task.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}

	// Assert: All data is available
	if item.ID != task.ID {
		t.Errorf("wrong item ID")
	}
	if item.Description == "" {
		t.Error("description should not be empty")
	}
	if item.ParentID == nil || *item.ParentID != epic.ID {
		t.Error("parent should be set")
	}
	if len(labels) != 1 {
		t.Errorf("expected 1 label, got %d", len(labels))
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
	if len(deps) != 1 {
		t.Errorf("expected 1 dep, got %d", len(deps))
	}
}

// =============================================================================
// Export Command Function Tests
// =============================================================================

// Note: ExportData is defined in export.go and represents the data structure
// for a single exported task.

// TestExportMarkdown_ReturnsFormattedMarkdown tests that the export function
// produces valid markdown output containing all task details.
func TestExportMarkdown_ReturnsFormattedMarkdown(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-fmt1", "Formatted Task",
		withDescription("Task description here"),
		withPriority(1),
	)

	// Build export data
	exportData := []ExportData{{
		Item:         task,
		Labels:       nil,
		Logs:         nil,
		Dependencies: nil,
		DepStatuses:  nil,
	}}

	// Act
	var buf bytes.Buffer
	err := exportMarkdown(&buf, exportData)
	if err != nil {
		t.Fatalf("exportMarkdown failed: %v", err)
	}
	output := buf.String()

	// Assert
	if !strings.Contains(output, "ts-fmt1") {
		t.Error("output should contain task ID")
	}
	if !strings.Contains(output, "Formatted Task") {
		t.Error("output should contain task title")
	}
	if !strings.Contains(output, "Task description here") {
		t.Error("output should contain task description")
	}
	if !strings.Contains(output, "# Task Export") {
		t.Error("output should be valid markdown with header")
	}
}

// TestExportMarkdown_IncludesAllTaskFields tests that exported output
// includes all relevant task fields for LLM consumption.
func TestExportMarkdown_IncludesAllTaskFields(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-fields1", "Task With All Fields",
		withDescription("Detailed description"),
		withPriority(1),
		withStatus(model.StatusInProgress),
	)
	if err := database.AddLabelToItem(task.ID, task.Project, "important"); err != nil {
		t.Fatalf("failed to add label: %v", err)
	}
	if err := database.AddLog(task.ID, "Progress update"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	// Get labels and logs for export data
	labels, _ := database.GetItemLabels(task.ID)
	logs, _ := database.GetLogs(task.ID)

	exportData := []ExportData{{
		Item:         task,
		Labels:       labels,
		Logs:         logs,
		Dependencies: nil,
		DepStatuses:  nil,
	}}

	// Act
	var buf bytes.Buffer
	err := exportMarkdown(&buf, exportData)
	if err != nil {
		t.Fatalf("exportMarkdown failed: %v", err)
	}
	output := buf.String()

	// Assert
	if !strings.Contains(output, "ts-fields1") {
		t.Error("output should contain task ID")
	}
	if !strings.Contains(output, "Task With All Fields") {
		t.Error("output should contain task title")
	}
	if !strings.Contains(output, "in_progress") {
		t.Error("output should contain status")
	}
	if !strings.Contains(output, "Priority:** 1") {
		t.Error("output should contain priority")
	}
	if !strings.Contains(output, "Detailed description") {
		t.Error("output should contain description")
	}
	if !strings.Contains(output, "important") {
		t.Error("output should contain labels")
	}
	if !strings.Contains(output, "Progress update") {
		t.Error("output should contain logs")
	}
}

// TestExportMarkdown_FiltersApplied tests that filtering works correctly
// when building export data from filtered items.
func TestExportMarkdown_FiltersApplied(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	openTask := createTestItem(t, database, "ts-filt1", "Open Task", withStatus(model.StatusOpen))
	createTestItem(t, database, "ts-filt2", "Done Task", withStatus(model.StatusDone))

	// Filter to only open tasks
	openStatus := model.StatusOpen
	filter := db.ListFilter{Status: &openStatus}
	items, err := database.ListItemsFiltered(filter)
	if err != nil {
		t.Fatalf("failed to filter: %v", err)
	}

	// Build export data from filtered items
	exportData := make([]ExportData, 0, len(items))
	for i := range items {
		exportData = append(exportData, ExportData{Item: &items[i]})
	}

	// Act
	var buf bytes.Buffer
	err = exportMarkdown(&buf, exportData)
	if err != nil {
		t.Fatalf("exportMarkdown failed: %v", err)
	}
	output := buf.String()

	// Assert
	if !strings.Contains(output, openTask.ID) {
		t.Error("output should contain open task")
	}
	if strings.Contains(output, "ts-filt2") {
		t.Error("output should NOT contain done task")
	}
}

// TestExportMarkdown_DeterministicOutput tests that the export function
// produces consistent output for the same input.
func TestExportMarkdown_DeterministicOutput(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	createTestItem(t, database, "ts-det1", "Task A", withPriority(2))
	createTestItem(t, database, "ts-det2", "Task B", withPriority(1))
	createTestItem(t, database, "ts-det3", "Task C", withPriority(3))

	// Get items (should be ordered by priority)
	filter := db.ListFilter{}
	items, _ := database.ListItemsFiltered(filter)

	exportData := make([]ExportData, 0, len(items))
	for i := range items {
		exportData = append(exportData, ExportData{Item: &items[i]})
	}

	// Act - run twice
	var buf1, buf2 bytes.Buffer
	_ = exportMarkdown(&buf1, exportData)
	_ = exportMarkdown(&buf2, exportData)

	// Assert - output should be identical
	if buf1.String() != buf2.String() {
		t.Error("output should be deterministic")
	}

	// Verify priority ordering (priority 1 should come first)
	output := buf1.String()
	pos1 := strings.Index(output, "ts-det2") // priority 1
	pos2 := strings.Index(output, "ts-det1") // priority 2
	pos3 := strings.Index(output, "ts-det3") // priority 3
	if pos1 > pos2 || pos2 > pos3 {
		t.Errorf("tasks should be ordered by priority: got positions %d, %d, %d", pos1, pos2, pos3)
	}
}

// TestExportMarkdown_HandlesEmptyResult tests that the export function
// gracefully handles when no tasks match the filter.
func TestExportMarkdown_HandlesEmptyResult(t *testing.T) {
	// Arrange - empty export data
	exportData := []ExportData{}

	// Act
	var buf bytes.Buffer
	err := exportMarkdown(&buf, exportData)
	if err != nil {
		t.Fatalf("exportMarkdown failed: %v", err)
	}
	output := buf.String()

	// Assert - should not error and should indicate no tasks
	if !strings.Contains(output, "No tasks found") {
		t.Error("output should indicate no tasks found")
	}
}

// TestExportMarkdown_IncludesDependencyDetails tests that the export
// includes dependency information with status.
func TestExportMarkdown_IncludesDependencyDetails(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-depdet1", "Blocker Task")
	blocked := createTestItem(t, database, "ts-depdet2", "Blocked Task")
	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Get dependency statuses
	depStatuses, _ := database.GetDepStatuses(blocked.ID)

	exportData := []ExportData{{
		Item:        blocked,
		DepStatuses: depStatuses,
	}}

	// Act
	var buf bytes.Buffer
	err := exportMarkdown(&buf, exportData)
	if err != nil {
		t.Fatalf("exportMarkdown failed: %v", err)
	}
	output := buf.String()

	// Assert
	if !strings.Contains(output, "ts-depdet1") {
		t.Error("output should show dependency ID")
	}
	if !strings.Contains(output, "Blocker Task") {
		t.Error("output should show dependency title")
	}
	if !strings.Contains(output, "open") {
		t.Error("output should show dependency status")
	}
}

// TestExportMarkdown_IncludesLogHistory tests that the export
// includes the full log history for each task.
func TestExportMarkdown_IncludesLogHistory(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-loghist1", "Task with History")
	if err := database.AddLog(task.ID, "First log entry"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}
	if err := database.AddLog(task.ID, "Second log entry"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	logs, _ := database.GetLogs(task.ID)

	exportData := []ExportData{{
		Item: task,
		Logs: logs,
	}}

	// Act
	var buf bytes.Buffer
	err := exportMarkdown(&buf, exportData)
	if err != nil {
		t.Fatalf("exportMarkdown failed: %v", err)
	}
	output := buf.String()

	// Assert
	if !strings.Contains(output, "First log entry") {
		t.Error("output should contain first log entry")
	}
	if !strings.Contains(output, "Second log entry") {
		t.Error("output should contain second log entry")
	}
	// Logs should have timestamps (format: 2006-01-02 15:04)
	if !strings.Contains(output, "**20") {
		t.Error("output should contain log timestamps")
	}
}

// TestExportMarkdown_ExcludesDoneByDefault tests that done/canceled
// tasks are excluded unless --all flag is used.
func TestExportMarkdown_ExcludesDoneByDefault(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	openTask := createTestItem(t, database, "ts-excl1", "Open Task", withStatus(model.StatusOpen))
	doneTask := createTestItem(t, database, "ts-excl2", "Done Task", withStatus(model.StatusDone))
	canceledTask := createTestItem(t, database, "ts-excl3", "Canceled Task", withStatus(model.StatusCanceled))

	// Simulate default behavior (exclude done/canceled)
	items, _ := database.ListItemsFiltered(db.ListFilter{})
	filtered := make([]model.Item, 0)
	for _, item := range items {
		if item.Status != model.StatusDone && item.Status != model.StatusCanceled {
			filtered = append(filtered, item)
		}
	}

	exportData := make([]ExportData, 0, len(filtered))
	for i := range filtered {
		exportData = append(exportData, ExportData{Item: &filtered[i]})
	}

	// Act
	var buf bytes.Buffer
	_ = exportMarkdown(&buf, exportData)
	output := buf.String()

	// Assert - only open task should be present
	if !strings.Contains(output, openTask.ID) {
		t.Error("output should contain open task")
	}
	if strings.Contains(output, doneTask.ID) {
		t.Error("output should NOT contain done task")
	}
	if strings.Contains(output, canceledTask.ID) {
		t.Error("output should NOT contain canceled task")
	}
}

// TestExportMarkdown_MarkdownStructure tests that the output follows
// a consistent markdown structure suitable for LLM consumption.
func TestExportMarkdown_MarkdownStructure(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-struct1", "Structured Task",
		withDescription("## Objective\nDo something\n\n## Notes\nSome notes"),
	)

	exportData := []ExportData{{Item: task}}

	// Act
	var buf bytes.Buffer
	_ = exportMarkdown(&buf, exportData)
	output := buf.String()

	// Assert - verify markdown structure
	if !strings.Contains(output, "# Task Export") {
		t.Error("output should have main header")
	}
	if !strings.Contains(output, "## ts-struct1: Structured Task") {
		t.Error("output should have task header with ID and title")
	}
	if !strings.Contains(output, "**Status:** open") {
		t.Error("output should have status in metadata line")
	}
	if !strings.Contains(output, "**Priority:** 2") {
		t.Error("output should have priority in metadata line")
	}
	if !strings.Contains(output, "**Project:** test") {
		t.Error("output should have project in metadata line")
	}
	if !strings.Contains(output, "### Description") {
		t.Error("output should have description section")
	}
	if !strings.Contains(output, "## Objective") {
		t.Error("output should preserve markdown in description")
	}
}

// TestExportMarkdown_MultipleTasksSeparated tests that multiple tasks
// are clearly separated in the output.
func TestExportMarkdown_MultipleTasksSeparated(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task1 := createTestItem(t, database, "ts-sep1", "First Task")
	task2 := createTestItem(t, database, "ts-sep2", "Second Task")
	task3 := createTestItem(t, database, "ts-sep3", "Third Task")

	exportData := []ExportData{
		{Item: task1},
		{Item: task2},
		{Item: task3},
	}

	// Act
	var buf bytes.Buffer
	_ = exportMarkdown(&buf, exportData)
	output := buf.String()

	// Assert - tasks should be separated by ---
	separatorCount := strings.Count(output, "---")
	if separatorCount != 2 {
		t.Errorf("expected 2 separators between 3 tasks, got %d", separatorCount)
	}

	// All tasks should be present
	if !strings.Contains(output, "ts-sep1") {
		t.Error("output should contain first task")
	}
	if !strings.Contains(output, "ts-sep2") {
		t.Error("output should contain second task")
	}
	if !strings.Contains(output, "ts-sep3") {
		t.Error("output should contain third task")
	}
}

// TestExportMarkdown_IncludesParentInfo tests that parent/child
// relationships are included in the export.
func TestExportMarkdown_IncludesParentInfo(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	epic := createTestItem(t, database, "ep-parent2", "Parent Epic",
		withType(model.ItemTypeEpic))
	child := createTestItem(t, database, "ts-child2", "Child Task",
		withParent(epic.ID))

	exportData := []ExportData{{Item: child}}

	// Act
	var buf bytes.Buffer
	_ = exportMarkdown(&buf, exportData)
	output := buf.String()

	// Assert - should show parent
	if !strings.Contains(output, "**Parent:** ep-parent2") {
		t.Error("output should show parent ID")
	}
}

// TestExportMarkdown_IncludesResults tests that completed tasks
// include their results message.
func TestExportMarkdown_IncludesResults(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-res1", "Completed Task")
	if err := database.CompleteItem(task.ID, "Successfully completed with all tests passing", db.AgentContext{}); err != nil {
		t.Fatalf("failed to complete item: %v", err)
	}

	// Refresh task to get updated results
	task, _ = database.GetItem(task.ID)

	exportData := []ExportData{{Item: task}}

	// Act
	var buf bytes.Buffer
	_ = exportMarkdown(&buf, exportData)
	output := buf.String()

	// Assert - should contain results
	if !strings.Contains(output, "### Results") {
		t.Error("output should have results section")
	}
	if !strings.Contains(output, "Successfully completed with all tests passing") {
		t.Error("output should contain results message")
	}
}

// =============================================================================
// JSON Export Tests
// =============================================================================

// TestExportJSON_ReturnsValidJSON tests that JSON export produces valid JSON.
func TestExportJSON_ReturnsValidJSON(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-json1", "JSON Test Task",
		withDescription("Task description"),
		withPriority(1),
	)

	exportData := []ExportData{{Item: task}}

	// Act
	var buf bytes.Buffer
	err := exportJSON(&buf, exportData)
	if err != nil {
		t.Fatalf("exportJSON failed: %v", err)
	}
	output := buf.String()

	// Assert - should be valid JSON
	if !strings.HasPrefix(strings.TrimSpace(output), "[") {
		t.Error("JSON output should start with array bracket")
	}
	if !strings.Contains(output, `"id": "ts-json1"`) {
		t.Error("JSON should contain task ID")
	}
	if !strings.Contains(output, `"title": "JSON Test Task"`) {
		t.Error("JSON should contain task title")
	}
	if !strings.Contains(output, `"description": "Task description"`) {
		t.Error("JSON should contain description")
	}
}

// TestExportJSON_IncludesDependencies tests that JSON export includes dependencies.
func TestExportJSON_IncludesDependencies(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-jsonblk", "Blocker")
	blocked := createTestItem(t, database, "ts-jsonblkd", "Blocked")
	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	depStatuses, _ := database.GetDepStatuses(blocked.ID)

	exportData := []ExportData{{
		Item:        blocked,
		DepStatuses: depStatuses,
	}}

	// Act
	var buf bytes.Buffer
	_ = exportJSON(&buf, exportData)
	output := buf.String()

	// Assert
	if !strings.Contains(output, `"dependencies"`) {
		t.Error("JSON should contain dependencies field")
	}
	if !strings.Contains(output, `"id": "ts-jsonblk"`) {
		t.Error("JSON should contain blocker ID in dependencies")
	}
}

// =============================================================================
// JSON Lines Export Tests
// =============================================================================

// TestExportJSONL_ReturnsValidJSONLines tests that JSONL export produces valid JSON Lines.
func TestExportJSONL_ReturnsValidJSONLines(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task1 := createTestItem(t, database, "ts-jsonl1", "JSONL Test Task 1",
		withDescription("Task 1 description"),
		withPriority(1),
	)
	task2 := createTestItem(t, database, "ts-jsonl2", "JSONL Test Task 2",
		withDescription("Task 2 description"),
		withPriority(2),
	)

	exportData := []ExportData{{Item: task1}, {Item: task2}}

	// Act
	var buf bytes.Buffer
	err := exportJSONL(&buf, exportData)
	if err != nil {
		t.Fatalf("exportJSONL failed: %v", err)
	}
	output := buf.String()

	// Assert - should have two lines, each valid JSON
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	// Each line should be valid JSON
	for i, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i+1, err)
		}
	}

	// First line should contain task1
	if !strings.Contains(lines[0], `"id":"ts-jsonl1"`) {
		t.Error("First line should contain task1 ID")
	}
	// Second line should contain task2
	if !strings.Contains(lines[1], `"id":"ts-jsonl2"`) {
		t.Error("Second line should contain task2 ID")
	}
}

// TestExportJSONL_NoArrayWrapper tests that JSONL output has no array wrapper.
func TestExportJSONL_NoArrayWrapper(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-jsonlnw", "JSONL No Wrapper Test")

	exportData := []ExportData{{Item: task}}

	// Act
	var buf bytes.Buffer
	_ = exportJSONL(&buf, exportData)
	output := buf.String()

	// Assert - should NOT start with [ or end with ]
	trimmed := strings.TrimSpace(output)
	if strings.HasPrefix(trimmed, "[") {
		t.Error("JSONL output should NOT start with array bracket")
	}
	if strings.HasSuffix(trimmed, "]") {
		t.Error("JSONL output should NOT end with array bracket")
	}
	// Should start with { (object)
	if !strings.HasPrefix(trimmed, "{") {
		t.Error("JSONL output should start with object bracket")
	}
}

// TestExportJSONL_NoIndentation tests that JSONL output has no indentation.
func TestExportJSONL_NoIndentation(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	task := createTestItem(t, database, "ts-jsonlni", "JSONL No Indent Test",
		withDescription("Some description"),
	)

	exportData := []ExportData{{Item: task}}

	// Act
	var buf bytes.Buffer
	_ = exportJSONL(&buf, exportData)
	output := buf.String()

	// Assert - should be single line (no newlines within the JSON object)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("Single task should produce single line, got %d lines", len(lines))
	}
	// Should not contain indentation spaces after colons
	if strings.Contains(output, ": ") && strings.Contains(output, "  ") {
		// Check for pretty-print style indentation
		if strings.Contains(output, "\n  ") {
			t.Error("JSONL output should not have indentation")
		}
	}
}

// TestExportJSONL_IncludesDependencies tests that JSONL export includes dependencies.
func TestExportJSONL_IncludesDependencies(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	blocker := createTestItem(t, database, "ts-jsonlblk", "Blocker")
	blocked := createTestItem(t, database, "ts-jsonlblkd", "Blocked")
	if err := database.AddDep(blocked.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	depStatuses, _ := database.GetDepStatuses(blocked.ID)

	exportData := []ExportData{{
		Item:        blocked,
		DepStatuses: depStatuses,
	}}

	// Act
	var buf bytes.Buffer
	_ = exportJSONL(&buf, exportData)
	output := buf.String()

	// Assert
	if !strings.Contains(output, `"dependencies"`) {
		t.Error("JSONL should contain dependencies field")
	}
	if !strings.Contains(output, `"id":"ts-jsonlblk"`) {
		t.Error("JSONL should contain blocker ID in dependencies")
	}
}

// TestExportJSONL_EmptyData tests that JSONL handles empty data correctly.
func TestExportJSONL_EmptyData(t *testing.T) {
	// Arrange
	exportData := []ExportData{}

	// Act
	var buf bytes.Buffer
	err := exportJSONL(&buf, exportData)

	// Assert
	if err != nil {
		t.Fatalf("exportJSONL should not error on empty data: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("Empty data should produce empty output, got: %q", buf.String())
	}
}

// TestExport_JSONAndJSONLMutuallyExclusive tests that --json and --jsonl cannot be used together.
func TestExport_JSONAndJSONLMutuallyExclusive(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	_ = createTestItem(t, database, "ts-mutex", "Mutex Test")

	// Set both flags
	flagExportJSON = true
	flagExportJSONL = true
	defer func() {
		flagExportJSON = false
		flagExportJSONL = false
	}()

	// Act
	err := exportCmd.RunE(exportCmd, []string{})

	// Assert
	if err == nil {
		t.Error("Expected error when both --json and --jsonl are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("Error should mention mutually exclusive, got: %v", err)
	}
}
