package db

import (
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func createTestItemWithProject(t *testing.T, db *DB, title, project string, status model.Status, priority int) *model.Item {
	t.Helper()
	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   project,
		Type:      model.ItemTypeTask,
		Title:     title,
		Status:    status,
		Priority:  priority,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}
	return item
}

func TestListItems(t *testing.T) {
	db := setupTestDB(t)

	createTestItemWithProject(t, db, "Task 1", "proj1", model.StatusOpen, 2)
	createTestItemWithProject(t, db, "Task 2", "proj1", model.StatusDone, 2)
	createTestItemWithProject(t, db, "Task 3", "proj2", model.StatusOpen, 2)

	// List all
	items, err := db.ListItems("", nil)
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Filter by project
	items, _ = db.ListItems("proj1", nil)
	if len(items) != 2 {
		t.Errorf("expected 2 items for proj1, got %d", len(items))
	}

	// Filter by status
	status := model.StatusOpen
	items, _ = db.ListItems("", &status)
	if len(items) != 2 {
		t.Errorf("expected 2 open items, got %d", len(items))
	}

	// Filter by both
	items, _ = db.ListItems("proj1", &status)
	if len(items) != 1 {
		t.Errorf("expected 1 open item in proj1, got %d", len(items))
	}
}

func TestListItems_InvalidStatus(t *testing.T) {
	db := setupTestDB(t)

	status := model.Status("invalid")
	_, err := db.ListItems("", &status)
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestListItems_OrderByPriority(t *testing.T) {
	db := setupTestDB(t)

	createTestItemWithProject(t, db, "Low", "test", model.StatusOpen, 3)
	createTestItemWithProject(t, db, "High", "test", model.StatusOpen, 1)
	createTestItemWithProject(t, db, "Medium", "test", model.StatusOpen, 2)

	items, _ := db.ListItems("test", nil)

	if items[0].Title != "High" {
		t.Errorf("first item should be High priority, got %q", items[0].Title)
	}
	if items[2].Title != "Low" {
		t.Errorf("last item should be Low priority, got %q", items[2].Title)
	}
}

func TestGetDistinctTypes_Empty(t *testing.T) {
	db := setupTestDB(t)

	types, err := db.GetDistinctTypes()
	if err != nil {
		t.Fatalf("failed to get distinct types: %v", err)
	}

	if len(types) != 0 {
		t.Errorf("expected 0 types, got %d", len(types))
	}
}

func TestGetDistinctTypes_ReturnsSortedDistinct(t *testing.T) {
	db := setupTestDB(t)

	items := []*model.Item{
		{
			ID:        model.GenerateID(model.ItemTypeTask),
			Project:   "test",
			Type:      model.ItemTypeTask,
			Title:     "Task",
			Status:    model.StatusOpen,
			Priority:  2,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        model.GenerateID(model.ItemTypeEpic),
			Project:   "test",
			Type:      model.ItemTypeEpic,
			Title:     "Epic",
			Status:    model.StatusOpen,
			Priority:  2,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        model.GenerateID(model.ItemTypeTask),
			Project:   "test",
			Type:      model.ItemTypeTask,
			Title:     "Task 2",
			Status:    model.StatusOpen,
			Priority:  2,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        model.GenerateID(model.ItemTypeEpic),
			Project:   "test",
			Type:      model.ItemTypeEpic,
			Title:     "Epic 2",
			Status:    model.StatusOpen,
			Priority:  2,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, item := range items {
		if err := db.CreateItem(item); err != nil {
			t.Fatalf("failed to create item: %v", err)
		}
	}

	types, err := db.GetDistinctTypes()
	if err != nil {
		t.Fatalf("failed to get distinct types: %v", err)
	}

	want := []model.ItemType{model.ItemTypeEpic, model.ItemTypeTask}
	if len(types) != len(want) {
		t.Fatalf("expected %d types, got %d", len(want), len(types))
	}
	for i, typ := range want {
		if types[i] != typ {
			t.Errorf("type[%d] = %q, want %q", i, types[i], typ)
		}
	}
}

func TestReadyItems(t *testing.T) {
	db := setupTestDB(t)

	// Create tasks with dependencies
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	createTestItemWithProject(t, db, "Task 3", "test", model.StatusInProgress, 2) // not ready (in_progress)

	// task2 depends on task1
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	ready, err := db.ReadyItems("test")
	if err != nil {
		t.Fatalf("failed to get ready: %v", err)
	}

	// Only task1 should be ready (task2 has unmet dep, task3 is in_progress)
	if len(ready) != 1 {
		t.Errorf("expected 1 ready item, got %d", len(ready))
	}
	if len(ready) > 0 && ready[0].ID != task1.ID {
		t.Errorf("ready item = %q, want %q", ready[0].ID, task1.ID)
	}

	// Complete task1, now task2 should be ready
	if err := db.UpdateStatus(task1.ID, model.StatusDone, AgentContext{}, true); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	ready, _ = db.ReadyItems("test")
	if len(ready) != 1 {
		t.Errorf("expected 1 ready item after completing dep, got %d", len(ready))
	}
	if len(ready) > 0 && ready[0].ID != task2.ID {
		t.Errorf("ready item = %q, want %q", ready[0].ID, task2.ID)
	}
}

func TestReadyItems_ProjectFilter(t *testing.T) {
	db := setupTestDB(t)

	createTestItemWithProject(t, db, "Task 1", "proj1", model.StatusOpen, 2)
	createTestItemWithProject(t, db, "Task 2", "proj2", model.StatusOpen, 2)

	ready, _ := db.ReadyItems("proj1")
	if len(ready) != 1 {
		t.Errorf("expected 1 ready item in proj1, got %d", len(ready))
	}

	ready, _ = db.ReadyItems("")
	if len(ready) != 2 {
		t.Errorf("expected 2 ready items total, got %d", len(ready))
	}
}

func TestProjectStatus(t *testing.T) {
	db := setupTestDB(t)

	createTestItemWithProject(t, db, "Open 1", "test", model.StatusOpen, 2)
	createTestItemWithProject(t, db, "Open 2", "test", model.StatusOpen, 1)
	createTestItemWithProject(t, db, "In Progress", "test", model.StatusInProgress, 2)
	createTestItemWithProject(t, db, "Blocked", "test", model.StatusBlocked, 2)
	createTestItemWithProject(t, db, "Done", "test", model.StatusDone, 2)

	report, err := db.ProjectStatus("test")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if report.Open != 2 {
		t.Errorf("open = %d, want 2", report.Open)
	}
	if report.InProgress != 1 {
		t.Errorf("in_progress = %d, want 1", report.InProgress)
	}
	if report.Blocked != 1 {
		t.Errorf("blocked = %d, want 1", report.Blocked)
	}
	if report.Done != 1 {
		t.Errorf("done = %d, want 1", report.Done)
	}
	if report.Ready != 2 {
		t.Errorf("ready = %d, want 2", report.Ready)
	}

	if len(report.InProgItems) != 1 {
		t.Errorf("in_progress items = %d, want 1", len(report.InProgItems))
	}
	if len(report.BlockedItems) != 1 {
		t.Errorf("blocked items = %d, want 1", len(report.BlockedItems))
	}
	if len(report.RecentDone) != 1 {
		t.Errorf("recent done = %d, want 1", len(report.RecentDone))
	}
}

func TestProjectStatus_Empty(t *testing.T) {
	db := setupTestDB(t)

	report, err := db.ProjectStatus("empty")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if report.Open != 0 || report.Done != 0 || report.Ready != 0 {
		t.Error("expected all counts to be 0 for empty project")
	}
}

func createTestEpic(t *testing.T, db *DB, title, project string) *model.Item {
	t.Helper()
	item := &model.Item{
		ID:       model.GenerateID(model.ItemTypeEpic),
		Project:  project,
		Type:     model.ItemTypeEpic,
		Title:    title,
		Status:   model.StatusOpen,
		Priority: 2,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}
	return item
}

func TestListItemsFiltered_Parent(t *testing.T) {
	db := setupTestDB(t)

	epic := createTestEpic(t, db, "Epic 1", "test")
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)

	// Set task1's parent to epic
	if err := db.SetParent(task1.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Filter by parent
	items, err := db.ListItemsFiltered(ListFilter{Parent: epic.ID})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item under epic, got %d", len(items))
	}
	if len(items) > 0 && items[0].ID != task1.ID {
		t.Errorf("expected task1 under epic, got %s", items[0].ID)
	}

	// task2 should not be included
	for _, item := range items {
		if item.ID == task2.ID {
			t.Error("task2 should not be under epic")
		}
	}
}

func TestListItemsFiltered_Type(t *testing.T) {
	db := setupTestDB(t)

	createTestEpic(t, db, "Epic 1", "test")
	createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)

	// Filter by type=epic
	items, err := db.ListItemsFiltered(ListFilter{Type: "epic"})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 epic, got %d", len(items))
	}

	// Filter by type=task
	items, err = db.ListItemsFiltered(ListFilter{Type: "task"})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(items))
	}
}

func TestListItemsFiltered_InvalidType(t *testing.T) {
	db := setupTestDB(t)

	// Create a test item first so we have data to filter
	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test Task",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Empty type filter should not error - it just means "don't filter by type"
	// The filter is only applied when Type is non-empty
	items, err := db.ListItemsFiltered(ListFilter{Type: ""})
	if err != nil {
		t.Errorf("unexpected error for empty type filter: %v", err)
	}
	// Should return all items (no type filter applied)
	if len(items) != 1 {
		t.Errorf("expected 1 item with no type filter, got %d", len(items))
	}
}

func TestListItemsFiltered_Blocking(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusOpen, 2)

	// task2 depends on task1 (task1 blocks task2)
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}
	// task2 also depends on task3 (task3 blocks task2)
	if err := db.AddDep(task2.ID, task3.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Find items that block task2
	items, err := db.ListItemsFiltered(ListFilter{Blocking: task2.ID})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items blocking task2, got %d", len(items))
	}

	// Verify task1 and task3 are in the list
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[task1.ID] || !ids[task3.ID] {
		t.Errorf("expected task1 and task3, got %v", ids)
	}
}

func TestListItemsFiltered_BlockedBy(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusOpen, 2)

	// task2 depends on task1 (task2 is blocked by task1)
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}
	// task3 depends on task1 (task3 is blocked by task1)
	if err := db.AddDep(task3.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Find items blocked by task1
	items, err := db.ListItemsFiltered(ListFilter{BlockedBy: task1.ID})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items blocked by task1, got %d", len(items))
	}

	// Verify task2 and task3 are in the list
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[task2.ID] || !ids[task3.ID] {
		t.Errorf("expected task2 and task3, got %v", ids)
	}
}

func TestListItemsFiltered_HasBlockers(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusOpen, 2)

	// task2 depends on task1
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Find items with unresolved blockers
	items, err := db.ListItemsFiltered(ListFilter{HasBlockers: true})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item with blockers, got %d", len(items))
	}
	if len(items) > 0 && items[0].ID != task2.ID {
		t.Errorf("expected task2 to have blockers, got %s", items[0].ID)
	}

	// Complete task1, now task2 should have no unresolved blockers
	if err := db.UpdateStatus(task1.ID, model.StatusDone, AgentContext{}, true); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	items, err = db.ListItemsFiltered(ListFilter{HasBlockers: true})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items with blockers after completing dep, got %d", len(items))
	}

	// task3 was never returned because it has no deps
	_ = task3
}

func TestListItemsFiltered_NoBlockers(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusOpen, 2)

	// task2 depends on task1
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Find items with no blockers (task1 and task3)
	items, err := db.ListItemsFiltered(ListFilter{NoBlockers: true})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with no blockers, got %d", len(items))
	}

	// Verify task1 and task3 are in the list (not task2)
	ids := map[string]bool{}
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[task1.ID] || !ids[task3.ID] {
		t.Errorf("expected task1 and task3, got %v", ids)
	}
	if ids[task2.ID] {
		t.Error("task2 should not be in the no-blockers list")
	}

	// Complete task1, now task2 should also have no unresolved blockers
	if err := db.UpdateStatus(task1.ID, model.StatusDone, AgentContext{}, true); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	items, err = db.ListItemsFiltered(ListFilter{NoBlockers: true})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items with no blockers after completing dep, got %d", len(items))
	}
}

func TestListItemsFiltered_CombinedFilters(t *testing.T) {
	db := setupTestDB(t)

	epic := createTestEpic(t, db, "Epic", "test")
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusDone, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "other", model.StatusOpen, 2)

	// Set parents
	if err := db.SetParent(task1.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task2.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Filter by parent and status
	status := model.StatusOpen
	items, err := db.ListItemsFiltered(ListFilter{
		Parent: epic.ID,
		Status: &status,
	})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 open item under epic, got %d", len(items))
	}
	if len(items) > 0 && items[0].ID != task1.ID {
		t.Errorf("expected task1, got %s", items[0].ID)
	}

	// Filter by project and type
	items, err = db.ListItemsFiltered(ListFilter{
		Project: "test",
		Type:    "task",
	})
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 tasks in test project, got %d", len(items))
	}

	_ = task3
}

// createTestItemWithTimestamp creates a test item with a specific updated_at time.
func createTestItemWithTimestamp(t *testing.T, db *DB, title, project string, status model.Status, updatedAt time.Time) *model.Item {
	t.Helper()
	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   project,
		Type:      model.ItemTypeTask,
		Title:     title,
		Status:    status,
		Priority:  2,
		CreatedAt: updatedAt,
		UpdatedAt: updatedAt,
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}
	return item
}

func TestStaleItems_ReturnsInProgressOlderThanCutoff(t *testing.T) {
	db := setupTestDB(t)

	cutoff := time.Now()
	oldTime := cutoff.Add(-2 * time.Hour)

	// Create an old in_progress item (should be stale)
	staleItem := createTestItemWithTimestamp(t, db, "Stale Task", "test", model.StatusInProgress, oldTime)

	items, err := db.StaleItems("", cutoff)
	if err != nil {
		t.Fatalf("failed to get stale items: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 stale item, got %d", len(items))
	}
	if len(items) > 0 && items[0].ID != staleItem.ID {
		t.Errorf("expected stale item %s, got %s", staleItem.ID, items[0].ID)
	}
}

func TestStaleItems_DoesNotReturnItemsNewerThanCutoff(t *testing.T) {
	db := setupTestDB(t)

	cutoff := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	// Create a recent in_progress item (should NOT be stale)
	createTestItemWithTimestamp(t, db, "Recent Task", "test", model.StatusInProgress, newTime)

	items, err := db.StaleItems("", cutoff)
	if err != nil {
		t.Fatalf("failed to get stale items: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 stale items, got %d", len(items))
	}
}

func TestStaleItems_DoesNotReturnOtherStatuses(t *testing.T) {
	db := setupTestDB(t)

	cutoff := time.Now()
	oldTime := cutoff.Add(-2 * time.Hour)

	// Create old items with various statuses (none should be returned except in_progress)
	createTestItemWithTimestamp(t, db, "Old Open", "test", model.StatusOpen, oldTime)
	createTestItemWithTimestamp(t, db, "Old Done", "test", model.StatusDone, oldTime)
	createTestItemWithTimestamp(t, db, "Old Blocked", "test", model.StatusBlocked, oldTime)
	createTestItemWithTimestamp(t, db, "Old Canceled", "test", model.StatusCanceled, oldTime)

	items, err := db.StaleItems("", cutoff)
	if err != nil {
		t.Fatalf("failed to get stale items: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 stale items (no in_progress), got %d", len(items))
	}
}

func TestStaleItems_FiltersByProject(t *testing.T) {
	db := setupTestDB(t)

	cutoff := time.Now()
	oldTime := cutoff.Add(-2 * time.Hour)

	// Create stale items in different projects
	staleProj1 := createTestItemWithTimestamp(t, db, "Stale Proj1", "proj1", model.StatusInProgress, oldTime)
	createTestItemWithTimestamp(t, db, "Stale Proj2", "proj2", model.StatusInProgress, oldTime)

	// Filter by proj1
	items, err := db.StaleItems("proj1", cutoff)
	if err != nil {
		t.Fatalf("failed to get stale items: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 stale item in proj1, got %d", len(items))
	}
	if len(items) > 0 && items[0].ID != staleProj1.ID {
		t.Errorf("expected stale item %s, got %s", staleProj1.ID, items[0].ID)
	}

	// No project filter should return both
	items, err = db.StaleItems("", cutoff)
	if err != nil {
		t.Fatalf("failed to get stale items: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 stale items total, got %d", len(items))
	}
}

func TestStaleItems_ReturnsEmptyWhenNoStaleItems(t *testing.T) {
	db := setupTestDB(t)

	cutoff := time.Now().Add(-1 * time.Hour)
	newTime := time.Now()

	// Create items that won't be stale
	createTestItemWithTimestamp(t, db, "Recent InProgress", "test", model.StatusInProgress, newTime)
	createTestItemWithProject(t, db, "Open Task", "test", model.StatusOpen, 2)

	items, err := db.StaleItems("", cutoff)
	if err != nil {
		t.Fatalf("failed to get stale items: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 stale items, got %d", len(items))
	}
}

func TestCompleteItem_SetsStatusToDone(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Task to complete", "test", model.StatusInProgress, 2)

	if err := db.CompleteItem(item.ID, "Completed successfully", AgentContext{}); err != nil {
		t.Fatalf("failed to complete item: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.Status != model.StatusDone {
		t.Errorf("status = %q, want %q", got.Status, model.StatusDone)
	}
}

func TestCompleteItem_StoresResultsMessage(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Task to complete", "test", model.StatusInProgress, 2)
	resultsMsg := "Task completed with following outcomes: feature implemented, tests passing"

	if err := db.CompleteItem(item.ID, resultsMsg, AgentContext{}); err != nil {
		t.Fatalf("failed to complete item: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.Results != resultsMsg {
		t.Errorf("results = %q, want %q", got.Results, resultsMsg)
	}
}

func TestCompleteItem_UpdatesTimestamp(t *testing.T) {
	db := setupTestDB(t)

	oldTime := time.Now().Add(-1 * time.Hour)
	item := createTestItemWithTimestamp(t, db, "Task to complete", "test", model.StatusInProgress, oldTime)

	beforeComplete := time.Now().UTC().Truncate(time.Second)
	if err := db.CompleteItem(item.ID, "Done", AgentContext{}); err != nil {
		t.Fatalf("failed to complete item: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.UpdatedAt.UTC().Truncate(time.Second).Before(beforeComplete) {
		t.Errorf("updated_at should be updated, got %v (before %v)", got.UpdatedAt, beforeComplete)
	}
}

func TestCompleteItem_ReturnsErrorForNonExistentItem(t *testing.T) {
	db := setupTestDB(t)

	err := db.CompleteItem("nonexistent-id", "Some results", AgentContext{})
	if err == nil {
		t.Error("expected error for non-existent item")
	}
}

func TestCompleteItem_ResultsCanBeRetrievedViaGetItem(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Task with results", "test", model.StatusOpen, 2)
	resultsMsg := "Implementation complete:\n- Added feature X\n- Fixed bug Y\n- Updated docs"

	if err := db.CompleteItem(item.ID, resultsMsg, AgentContext{}); err != nil {
		t.Fatalf("failed to complete item: %v", err)
	}

	// Retrieve via GetItem and verify results
	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.Results != resultsMsg {
		t.Errorf("results from GetItem = %q, want %q", got.Results, resultsMsg)
	}
	if got.Status != model.StatusDone {
		t.Errorf("status from GetItem = %q, want %q", got.Status, model.StatusDone)
	}
}

func TestWorktreeFields_RoundTrip(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic with worktree metadata
	epic := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Worktree Epic",
		Status:         model.StatusOpen,
		Priority:       2,
		WorktreeBranch: "feature/epic-work",
		WorktreeBase:   "main",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic with worktree: %v", err)
	}

	// Retrieve via GetItem
	got, err := db.GetItem(epic.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.WorktreeBranch != "feature/epic-work" {
		t.Errorf("WorktreeBranch from GetItem = %q, want %q", got.WorktreeBranch, "feature/epic-work")
	}
	if got.WorktreeBase != "main" {
		t.Errorf("WorktreeBase from GetItem = %q, want %q", got.WorktreeBase, "main")
	}

	// Retrieve via ListItems
	items, err := db.ListItems("test", nil)
	if err != nil {
		t.Fatalf("failed to list items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].WorktreeBranch != "feature/epic-work" {
		t.Errorf("WorktreeBranch from ListItems = %q, want %q", items[0].WorktreeBranch, "feature/epic-work")
	}
	if items[0].WorktreeBase != "main" {
		t.Errorf("WorktreeBase from ListItems = %q, want %q", items[0].WorktreeBase, "main")
	}
}

func TestWorktreeFields_EmptyByDefault(t *testing.T) {
	db := setupTestDB(t)

	// Create a task without worktree metadata
	task := createTestItemWithProject(t, db, "Regular Task", "test", model.StatusOpen, 2)

	// Retrieve via GetItem
	got, err := db.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.WorktreeBranch != "" {
		t.Errorf("WorktreeBranch should be empty by default, got %q", got.WorktreeBranch)
	}
	if got.WorktreeBase != "" {
		t.Errorf("WorktreeBase should be empty by default, got %q", got.WorktreeBase)
	}
}

func TestReadyItemsForEpic(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic
	epic := createTestEpic(t, db, "Epic", "test")

	// Create tasks under the epic
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusInProgress, 2)

	// Set parents
	if err := db.SetParent(task1.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task2.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task3.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// task2 depends on task1
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Get ready items for epic
	ready, err := db.ReadyItemsForEpic(epic.ID)
	if err != nil {
		t.Fatalf("failed to get ready items: %v", err)
	}

	// Only task1 should be ready (task2 has unmet dep, task3 is in_progress)
	if len(ready) != 1 {
		t.Errorf("expected 1 ready item, got %d", len(ready))
	}
	if len(ready) > 0 && ready[0].ID != task1.ID {
		t.Errorf("ready item = %q, want %q", ready[0].ID, task1.ID)
	}
}

func TestReadyItemsWithCounts_Basic(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic
	epic := createTestEpic(t, db, "Epic 1", "test")

	// Create tasks under the epic
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusInProgress, 2)
	task4 := createTestItemWithProject(t, db, "Task 4", "test", model.StatusDone, 2)

	// Set parents
	if err := db.SetParent(task1.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task2.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task3.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task4.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// task2 depends on task1
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Get ready items with counts
	result, err := db.ReadyItemsWithCounts("test", nil)
	if err != nil {
		t.Fatalf("failed to get ready items with counts: %v", err)
	}

	// Count non-epic ready items (epics themselves can be ready)
	readyTaskCount := 0
	for _, item := range result.ReadyItems {
		if item.Type != model.ItemTypeEpic {
			readyTaskCount++
		}
	}

	// Only task1 should be a ready task (task2 blocked, task3 in_progress, task4 done)
	if readyTaskCount != 1 {
		t.Errorf("expected 1 ready task, got %d", readyTaskCount)
	}

	// Should have epic counts for epics with ready tasks under them
	if len(result.EpicCounts) != 1 {
		t.Errorf("expected 1 epic with counts, got %d", len(result.EpicCounts))
	}

	epicCount := result.EpicCounts[epic.ID]
	if epicCount == nil {
		t.Fatalf("expected epic %s in counts", epic.ID)
	}

	// 1 ready out of 3 active (task1, task2, task3 - not task4 which is done)
	if epicCount.ReadyCount != 1 {
		t.Errorf("epicCount.ReadyCount = %d, want 1", epicCount.ReadyCount)
	}
	if epicCount.TotalCount != 3 {
		t.Errorf("epicCount.TotalCount = %d, want 3", epicCount.TotalCount)
	}
	if epicCount.Epic == nil || epicCount.Epic.ID != epic.ID {
		t.Errorf("epicCount.Epic should be the epic item")
	}
}

func TestReadyItemsWithCounts_MultipleEpics(t *testing.T) {
	db := setupTestDB(t)

	// Create two epics
	epic1 := createTestEpic(t, db, "Epic 1", "test")
	epic2 := createTestEpic(t, db, "Epic 2", "test")

	// Create tasks under epic1
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)
	if err := db.SetParent(task1.ID, epic1.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task2.ID, epic1.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create tasks under epic2
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusOpen, 2)
	task4 := createTestItemWithProject(t, db, "Task 4", "test", model.StatusInProgress, 2)
	task5 := createTestItemWithProject(t, db, "Task 5", "test", model.StatusOpen, 2)
	if err := db.SetParent(task3.ID, epic2.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task4.ID, epic2.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := db.SetParent(task5.ID, epic2.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Get ready items with counts
	result, err := db.ReadyItemsWithCounts("test", nil)
	if err != nil {
		t.Fatalf("failed to get ready items with counts: %v", err)
	}

	// Count non-epic ready items
	readyTaskCount := 0
	for _, item := range result.ReadyItems {
		if item.Type != model.ItemTypeEpic {
			readyTaskCount++
		}
	}

	// 4 ready tasks: task1, task2 (epic1), task3, task5 (epic2)
	if readyTaskCount != 4 {
		t.Errorf("expected 4 ready tasks, got %d", readyTaskCount)
	}

	// Should have 2 epics in counts
	if len(result.EpicCounts) != 2 {
		t.Errorf("expected 2 epics with counts, got %d", len(result.EpicCounts))
	}

	// Check epic1 counts
	epic1Count := result.EpicCounts[epic1.ID]
	if epic1Count == nil {
		t.Fatalf("expected epic1 in counts")
	}
	if epic1Count.ReadyCount != 2 {
		t.Errorf("epic1.ReadyCount = %d, want 2", epic1Count.ReadyCount)
	}
	if epic1Count.TotalCount != 2 {
		t.Errorf("epic1.TotalCount = %d, want 2", epic1Count.TotalCount)
	}

	// Check epic2 counts
	epic2Count := result.EpicCounts[epic2.ID]
	if epic2Count == nil {
		t.Fatalf("expected epic2 in counts")
	}
	if epic2Count.ReadyCount != 2 {
		t.Errorf("epic2.ReadyCount = %d, want 2", epic2Count.ReadyCount)
	}
	if epic2Count.TotalCount != 3 {
		t.Errorf("epic2.TotalCount = %d, want 3", epic2Count.TotalCount)
	}
}

func TestReadyItemsWithCounts_TopLevelTasks(t *testing.T) {
	db := setupTestDB(t)

	// Create tasks without parent epic
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusOpen, 2)

	// Get ready items with counts
	result, err := db.ReadyItemsWithCounts("test", nil)
	if err != nil {
		t.Fatalf("failed to get ready items with counts: %v", err)
	}

	// Both tasks should be ready
	if len(result.ReadyItems) != 2 {
		t.Errorf("expected 2 ready items, got %d", len(result.ReadyItems))
	}

	// No epic counts since tasks have no parent epic
	if len(result.EpicCounts) != 0 {
		t.Errorf("expected 0 epic counts for orphan tasks, got %d", len(result.EpicCounts))
	}

	_ = task1
	_ = task2
}

func TestReadyItemsWithCounts_NestedEpics(t *testing.T) {
	db := setupTestDB(t)

	// Create nested epics: parent -> child
	parentEpic := createTestEpic(t, db, "Parent Epic", "test")
	childEpic := createTestEpic(t, db, "Child Epic", "test")
	if err := db.SetParent(childEpic.ID, parentEpic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create task under child epic
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusOpen, 2)
	if err := db.SetParent(task1.ID, childEpic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Get ready items with counts
	result, err := db.ReadyItemsWithCounts("test", nil)
	if err != nil {
		t.Fatalf("failed to get ready items with counts: %v", err)
	}

	// Count non-epic ready items
	readyTaskCount := 0
	for _, item := range result.ReadyItems {
		if item.Type != model.ItemTypeEpic {
			readyTaskCount++
		}
	}

	// task1 should be ready
	if readyTaskCount != 1 {
		t.Errorf("expected 1 ready task, got %d", readyTaskCount)
	}

	// Should have childEpic in counts (immediate parent)
	// Note: parentEpic and childEpic may both be in ReadyItems since they're "open",
	// but only childEpic has a task directly under it
	if len(result.EpicCounts) != 1 {
		t.Errorf("expected 1 epic with counts, got %d", len(result.EpicCounts))
	}

	childCount := result.EpicCounts[childEpic.ID]
	if childCount == nil {
		t.Fatalf("expected childEpic in counts")
	}
	if childCount.ReadyCount != 1 {
		t.Errorf("childEpic.ReadyCount = %d, want 1", childCount.ReadyCount)
	}
}

func TestReadyItemsWithCounts_Empty(t *testing.T) {
	db := setupTestDB(t)

	// No items
	result, err := db.ReadyItemsWithCounts("test", nil)
	if err != nil {
		t.Fatalf("failed to get ready items with counts: %v", err)
	}

	if len(result.ReadyItems) != 0 {
		t.Errorf("expected 0 ready items, got %d", len(result.ReadyItems))
	}
	if len(result.EpicCounts) != 0 {
		t.Errorf("expected 0 epic counts, got %d", len(result.EpicCounts))
	}
}

func TestGetRootEpic(t *testing.T) {
	db := setupTestDB(t)

	// Create nested epics with worktrees
	rootEpic := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Root Epic",
		Status:         model.StatusOpen,
		Priority:       2,
		WorktreeBranch: "feature/root",
		WorktreeBase:   "main",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateItem(rootEpic); err != nil {
		t.Fatalf("failed to create root epic: %v", err)
	}

	// Create child epic with worktree (no parent initially)
	childEpic := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Child Epic",
		Status:         model.StatusOpen,
		Priority:       2,
		WorktreeBranch: "feature/child",
		WorktreeBase:   "feature/root",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateItem(childEpic); err != nil {
		t.Fatalf("failed to create child epic: %v", err)
	}

	// Set child epic's parent to root epic
	if err := db.SetParent(childEpic.ID, rootEpic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	// Update childEpic struct to reflect the parent
	childEpic.ParentID = &rootEpic.ID

	// Create task under child epic
	task := createTestItemWithProject(t, db, "Task", "test", model.StatusOpen, 2)
	if err := db.SetParent(task.ID, childEpic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Get root epic for task - should find child epic (nearest)
	root, path, err := db.GetRootEpic(task.ID)
	if err != nil {
		t.Fatalf("failed to get root epic: %v", err)
	}
	if root == nil {
		t.Fatal("expected to find root epic, got nil")
	}
	if root.ID != childEpic.ID {
		t.Errorf("root epic = %q, want %q (nearest ancestor)", root.ID, childEpic.ID)
	}

	// Path should include child epic and task
	if len(path) != 2 {
		t.Errorf("expected path length 2, got %d", len(path))
	}

	// Get root epic for child epic itself
	root, path, err = db.GetRootEpic(childEpic.ID)
	if err != nil {
		t.Fatalf("failed to get root epic: %v", err)
	}
	if root == nil {
		t.Fatal("expected to find root epic, got nil")
	}
	if root.ID != childEpic.ID {
		t.Errorf("root epic = %q, want %q", root.ID, childEpic.ID)
	}
	if len(path) != 1 || path[0].ID != childEpic.ID {
		t.Errorf("path should contain only child epic")
	}
}

func TestGetRootEpic_NoWorktree(t *testing.T) {
	db := setupTestDB(t)

	// Create epic without worktree
	epic := createTestEpic(t, db, "No Worktree Epic", "test")

	// Create task under epic
	task := createTestItemWithProject(t, db, "Task", "test", model.StatusOpen, 2)
	if err := db.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Get root epic - should return nil since no ancestor has worktree
	root, path, err := db.GetRootEpic(task.ID)
	if err != nil {
		t.Fatalf("failed to get root epic: %v", err)
	}
	if root != nil {
		t.Errorf("expected nil root epic, got %q", root.ID)
	}
	if path != nil {
		t.Error("expected nil path")
	}
}

func TestFindEpicsNeedingWorktreeSetup(t *testing.T) {
	db := setupTestDB(t)

	// Create epics with worktree metadata
	epic1 := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Epic 1",
		Status:         model.StatusOpen,
		Priority:       2,
		WorktreeBranch: "feature/epic1",
		WorktreeBase:   "main",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateItem(epic1); err != nil {
		t.Fatalf("failed to create epic1: %v", err)
	}

	epic2 := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Epic 2",
		Status:         model.StatusOpen,
		Priority:       2,
		WorktreeBranch: "feature/epic2",
		WorktreeBase:   "main",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateItem(epic2); err != nil {
		t.Fatalf("failed to create epic2: %v", err)
	}

	// Create epic without worktree
	epic3 := createTestEpic(t, db, "Epic 3", "test")
	_ = epic3

	// Simulate that epic1 already has a worktree
	existingBranches := map[string]string{
		"feature/epic1": ".worktrees/epic1",
	}

	// Find epics needing setup
	needingSetup, err := db.FindEpicsNeedingWorktreeSetup(existingBranches)
	if err != nil {
		t.Fatalf("failed to find epics needing setup: %v", err)
	}

	// Only epic2 should need setup
	if len(needingSetup) != 1 {
		t.Errorf("expected 1 epic needing setup, got %d", len(needingSetup))
	}
	if len(needingSetup) > 0 && needingSetup[0].ID != epic2.ID {
		t.Errorf("epic needing setup = %q, want %q", needingSetup[0].ID, epic2.ID)
	}
}
