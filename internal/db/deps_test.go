package db

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func createTestItem(t *testing.T, db *DB, title string) *model.Item {
	t.Helper()
	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     title,
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}
	return item
}

func TestAddDep(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItem(t, db, "Task 1")
	task2 := createTestItem(t, db, "Task 2")

	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	deps, err := db.GetDeps(task2.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected 1 dep, got %d", len(deps))
	}
	if deps[0] != task1.ID {
		t.Errorf("dep = %q, want %q", deps[0], task1.ID)
	}
}

func TestAddDep_NonexistentItem(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItem(t, db, "Task 1")

	err := db.AddDep(task1.ID, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent dependency")
	}

	err = db.AddDep("nonexistent", task1.ID)
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestAddDep_Duplicate(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItem(t, db, "Task 1")
	task2 := createTestItem(t, db, "Task 2")

	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Adding duplicate should not error (INSERT OR IGNORE)
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Errorf("duplicate dep should not error: %v", err)
	}

	deps, _ := db.GetDeps(task2.ID)
	if len(deps) != 1 {
		t.Errorf("expected 1 dep after duplicate, got %d", len(deps))
	}
}

func TestHasUnmetDeps(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItem(t, db, "Task 1")
	task2 := createTestItem(t, db, "Task 2")

	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// task1 is open, so task2 has unmet deps
	unmet, err := db.HasUnmetDeps(task2.ID)
	if err != nil {
		t.Fatalf("failed to check deps: %v", err)
	}
	if !unmet {
		t.Error("expected unmet deps when dependency is open")
	}

	// Mark task1 as done
	if err := db.UpdateStatus(task1.ID, model.StatusDone, AgentContext{}); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	unmet, _ = db.HasUnmetDeps(task2.ID)
	if unmet {
		t.Error("expected no unmet deps when dependency is done")
	}
}

func TestHasUnmetDeps_NoDeps(t *testing.T) {
	db := setupTestDB(t)

	task := createTestItem(t, db, "Task")

	unmet, err := db.HasUnmetDeps(task.ID)
	if err != nil {
		t.Fatalf("failed to check deps: %v", err)
	}
	if unmet {
		t.Error("expected no unmet deps for task with no dependencies")
	}
}

func TestGetDeps_Empty(t *testing.T) {
	db := setupTestDB(t)

	task := createTestItem(t, db, "Task")

	deps, err := db.GetDeps(task.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestGetAllDeps(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItem(t, db, "Task 1")
	task2 := createTestItem(t, db, "Task 2")
	task3 := createTestItem(t, db, "Task 3")

	// task2 depends on task1, task3 depends on task1
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}
	if err := db.AddDep(task3.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	edges, err := db.GetAllDeps("")
	if err != nil {
		t.Fatalf("failed to get all deps: %v", err)
	}

	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
	}

	// Check edge details
	for _, e := range edges {
		if e.DependsOnID != task1.ID {
			t.Errorf("expected all deps on %s, got %s", task1.ID, e.DependsOnID)
		}
		if e.DependsOnTitle != "Task 1" {
			t.Errorf("expected dep title 'Task 1', got %q", e.DependsOnTitle)
		}
	}
}

func TestGetAllDeps_FilterByProject(t *testing.T) {
	db := setupTestDB(t)

	// Create tasks in "test" project (default from createTestItem)
	task1 := createTestItem(t, db, "Task 1")
	task2 := createTestItem(t, db, "Task 2")
	if err := db.AddDep(task2.ID, task1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Create task in different project
	otherTask := &model.Item{
		ID:      model.GenerateID(model.ItemTypeTask),
		Project: "other",
		Type:    model.ItemTypeTask,
		Title:   "Other Task",
		Status:  model.StatusOpen,
	}
	if err := db.CreateItem(otherTask); err != nil {
		t.Fatalf("failed to create otherTask: %v", err)
	}
	otherTask2 := &model.Item{
		ID:      model.GenerateID(model.ItemTypeTask),
		Project: "other",
		Type:    model.ItemTypeTask,
		Title:   "Other Task 2",
		Status:  model.StatusOpen,
	}
	if err := db.CreateItem(otherTask2); err != nil {
		t.Fatalf("failed to create otherTask2: %v", err)
	}
	if err := db.AddDep(otherTask2.ID, otherTask.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Filter by "test" project
	edges, err := db.GetAllDeps("test")
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge for 'test' project, got %d", len(edges))
	}

	// Filter by "other" project
	edges, err = db.GetAllDeps("other")
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge for 'other' project, got %d", len(edges))
	}
}

func TestAddDep_RevertsInProgressToOpen(t *testing.T) {
	db := setupTestDB(t)

	blocker := createTestItem(t, db, "Blocker task")
	worker := createTestItem(t, db, "Worker task")

	// Start the worker task
	if err := db.UpdateStatus(worker.ID, model.StatusInProgress, AgentContext{}); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Add an unmet dep — blocker is still open
	if err := db.AddDep(worker.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Worker should have been reverted to open
	item, err := db.GetItem(worker.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}
	if item.Status != model.StatusOpen {
		t.Errorf("status = %s, want open", item.Status)
	}
	if item.AgentID != nil {
		t.Errorf("agent_id should be nil after revert, got %v", item.AgentID)
	}

	// Should have a log entry about the revert
	logs, err := db.GetLogs(worker.ID)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	found := false
	for _, l := range logs {
		if l.Message == fmt.Sprintf("Reverted to open: dependency added on %s (not yet done)", blocker.ID) {
			found = true
		}
	}
	if !found {
		t.Error("expected log entry about revert, found none")
	}
}

func TestAddDep_DoesNotRevertIfDepDone(t *testing.T) {
	db := setupTestDB(t)

	blocker := createTestItem(t, db, "Blocker task")
	worker := createTestItem(t, db, "Worker task")

	// Complete the blocker first
	if err := db.UpdateStatus(blocker.ID, model.StatusDone, AgentContext{}); err != nil {
		t.Fatalf("failed to complete blocker: %v", err)
	}

	// Start the worker
	if err := db.UpdateStatus(worker.ID, model.StatusInProgress, AgentContext{}); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}

	// Add a dep on an already-done task — should NOT revert
	if err := db.AddDep(worker.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	item, err := db.GetItem(worker.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}
	if item.Status != model.StatusInProgress {
		t.Errorf("status = %s, want in_progress (dep already done)", item.Status)
	}
}

func TestAddDep_DoesNotRevertOpenTask(t *testing.T) {
	db := setupTestDB(t)

	blocker := createTestItem(t, db, "Blocker task")
	worker := createTestItem(t, db, "Worker task")

	// Worker is open (default), add dep — should stay open, no log
	if err := db.AddDep(worker.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	item, err := db.GetItem(worker.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}
	if item.Status != model.StatusOpen {
		t.Errorf("status = %s, want open", item.Status)
	}

	// Should have no revert log (only for in_progress → open transitions)
	logs, err := db.GetLogs(worker.ID)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs for open task dep add, got %d", len(logs))
	}
}

func TestGetAllDeps_Empty(t *testing.T) {
	db := setupTestDB(t)

	createTestItem(t, db, "Task 1")

	edges, err := db.GetAllDeps("")
	if err != nil {
		t.Fatalf("failed to get all deps: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestAddDep_CircularDependency(t *testing.T) {
	db := setupTestDB(t)

	// Create items
	taskA := createTestItem(t, db, "Task A")
	taskB := createTestItem(t, db, "Task B")
	taskC := createTestItem(t, db, "Task C")

	// Create A -> B dependency
	if err := db.AddDep(taskA.ID, taskB.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Create B -> C dependency
	if err := db.AddDep(taskB.ID, taskC.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Try to create C -> A (would create cycle)
	err := db.AddDep(taskC.ID, taskA.ID)
	if err == nil {
		t.Error("expected error when creating circular dependency")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected error message to mention 'cycle', got: %v", err)
	}
}

func TestAddDep_ParentChildCycle(t *testing.T) {
	db := setupTestDB(t)

	// Create epic and child task
	epic := createTestEpic(t, db, "Epic", "test")
	task := createTestItemWithProject(t, db, "Task", "test", model.StatusOpen, 2)

	// Set parent relationship
	if err := db.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Try to make epic depend on task (parent depends on child)
	err := db.AddDep(epic.ID, task.ID)
	if err == nil {
		t.Error("expected error when creating parent-child dependency")
	}
	if !strings.Contains(err.Error(), "child") && !strings.Contains(err.Error(), "parent") {
		t.Errorf("expected error about parent-child relationship, got: %v", err)
	}
}

func TestAddDep_SelfDependency(t *testing.T) {
	db := setupTestDB(t)

	task := createTestItem(t, db, "Task")

	// Try to make task depend on itself
	err := db.AddDep(task.ID, task.ID)
	if err == nil {
		t.Error("expected error when creating self-dependency")
	}
	if !strings.Contains(err.Error(), "self") {
		t.Errorf("expected error about self-dependency, got: %v", err)
	}
}

func TestFindCircularDeps(t *testing.T) {
	db := setupTestDB(t)

	// Create a simple cycle: A -> B -> C -> A
	taskA := createTestItem(t, db, "Task A")
	taskB := createTestItem(t, db, "Task B")
	taskC := createTestItem(t, db, "Task C")

	// Create dependencies manually to bypass cycle detection
	_, err := db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, taskA.ID, taskB.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}
	_, err = db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, taskB.ID, taskC.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}
	_, err = db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, taskC.ID, taskA.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}

	circularDeps, err := db.FindCircularDeps()
	if err != nil {
		t.Fatalf("failed to find circular deps: %v", err)
	}

	if len(circularDeps) == 0 {
		t.Error("expected to find circular dependencies, found none")
	}

	// Check that we found a cycle containing our tasks
	foundCycle := false
	for _, dep := range circularDeps {
		if len(dep.CyclePath) >= 3 {
			// Check if cycle contains A, B, and C
			hasA, hasB, hasC := false, false, false
			for _, node := range dep.CyclePath {
				if node == taskA.ID {
					hasA = true
				}
				if node == taskB.ID {
					hasB = true
				}
				if node == taskC.ID {
					hasC = true
				}
			}
			if hasA && hasB && hasC {
				foundCycle = true
				break
			}
		}
	}
	if !foundCycle {
		t.Errorf("expected to find cycle containing A, B, and C")
	}
}

func TestFindCircularDeps_NoCycles(t *testing.T) {
	db := setupTestDB(t)

	// Create a chain: A -> B -> C (no cycle)
	taskA := createTestItem(t, db, "Task A")
	taskB := createTestItem(t, db, "Task B")
	taskC := createTestItem(t, db, "Task C")

	// Create dependencies manually
	_, err := db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, taskA.ID, taskB.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}
	_, err = db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, taskB.ID, taskC.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}

	circularDeps, err := db.FindCircularDeps()
	if err != nil {
		t.Fatalf("failed to find circular deps: %v", err)
	}

	if len(circularDeps) != 0 {
		t.Errorf("expected no circular dependencies, found %d", len(circularDeps))
	}
}

func TestFindParentChildCircularDeps(t *testing.T) {
	db := setupTestDB(t)

	// Create epic and child task
	epic := createTestEpic(t, db, "Epic", "test")
	task := createTestItemWithProject(t, db, "Task", "test", model.StatusOpen, 2)

	// Set parent relationship
	if err := db.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create circular dependency: epic depends on task (parent depends on child)
	_, err := db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, epic.ID, task.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}

	parentChildDeps, err := db.FindParentChildCircularDeps()
	if err != nil {
		t.Fatalf("failed to find parent-child circular deps: %v", err)
	}

	if len(parentChildDeps) != 1 {
		t.Errorf("expected 1 parent-child circular dependency, got %d", len(parentChildDeps))
	}

	if len(parentChildDeps) > 0 {
		dep := parentChildDeps[0]
		if dep.ParentID != epic.ID {
			t.Errorf("expected ParentID = %s, got %s", epic.ID, dep.ParentID)
		}
		if dep.ChildID != task.ID {
			t.Errorf("expected ChildID = %s, got %s", task.ID, dep.ChildID)
		}
	}
}

func TestFindParentChildCircularDeps_ChildDependsOnParent(t *testing.T) {
	db := setupTestDB(t)

	// Create epic and child task
	epic := createTestEpic(t, db, "Epic", "test")
	task := createTestItemWithProject(t, db, "Task", "test", model.StatusOpen, 2)

	// Set parent relationship
	if err := db.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create circular dependency: task depends on epic (child depends on parent)
	_, err := db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, task.ID, epic.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}

	parentChildDeps, err := db.FindParentChildCircularDeps()
	if err != nil {
		t.Fatalf("failed to find parent-child circular deps: %v", err)
	}

	if len(parentChildDeps) != 1 {
		t.Errorf("expected 1 parent-child circular dependency, got %d", len(parentChildDeps))
	}

	if len(parentChildDeps) > 0 {
		dep := parentChildDeps[0]
		if dep.ParentID != task.ID {
			t.Errorf("expected ParentID = %s, got %s", task.ID, dep.ParentID)
		}
		if dep.ChildID != epic.ID {
			t.Errorf("expected ChildID = %s, got %s", epic.ID, dep.ChildID)
		}
	}
}

func TestFixAllParentChildCircularDeps(t *testing.T) {
	db := setupTestDB(t)

	// Create epic and child task
	epic := createTestEpic(t, db, "Epic", "test")
	task := createTestItemWithProject(t, db, "Task", "test", model.StatusOpen, 2)

	// Set parent relationship
	if err := db.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Create circular dependency: epic depends on task
	_, err := db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, epic.ID, task.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}

	// Verify the dependency exists
	deps, err := db.GetDeps(epic.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("expected 1 dep before fix, got %d", len(deps))
	}

	// Fix the circular dependency
	fixed, err := db.FixAllParentChildCircularDeps()
	if err != nil {
		t.Fatalf("failed to fix deps: %v", err)
	}

	if fixed != 1 {
		t.Errorf("expected to fix 1 dependency, fixed %d", fixed)
	}

	// Verify the dependency is removed
	deps, err = db.GetDeps(epic.ID)
	if err != nil {
		t.Fatalf("failed to get deps after fix: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps after fix, got %d", len(deps))
	}
}

func TestRemoveCircularDep(t *testing.T) {
	db := setupTestDB(t)

	taskA := createTestItem(t, db, "Task A")
	taskB := createTestItem(t, db, "Task B")

	// Create dependency manually
	_, err := db.Exec(`INSERT INTO deps (item_id, depends_on) VALUES (?, ?)`, taskA.ID, taskB.ID)
	if err != nil {
		t.Fatalf("failed to insert dep: %v", err)
	}

	// Verify it exists
	deps, err := db.GetDeps(taskA.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 1 {
		t.Errorf("expected 1 dep, got %d", len(deps))
	}

	// Remove it
	if err := db.RemoveCircularDep(taskA.ID, taskB.ID); err != nil {
		t.Fatalf("failed to remove dep: %v", err)
	}

	// Verify it's gone
	deps, err = db.GetDeps(taskA.ID)
	if err != nil {
		t.Fatalf("failed to get deps after remove: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps after remove, got %d", len(deps))
	}
}
