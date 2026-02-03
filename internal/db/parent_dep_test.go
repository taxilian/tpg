package db

import (
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

// TestParentDependencyInheritance verifies whether child tasks inherit parent dependencies
func TestParentDependencyInheritance(t *testing.T) {
	db := setupTestDB(t)

	// Create epic1 (base epic)
	epic1 := createTestEpic(t, db, "Base Epic", "test")

	// Create epic2 that depends on epic1
	epic2 := createTestEpic(t, db, "Dependent Epic", "test")
	if err := db.AddDep(epic2.ID, epic1.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Create a task under epic2
	task := createTestItemWithProject(t, db, "Task under epic2", "test", model.StatusOpen, 2)
	if err := db.SetParent(task.ID, epic2.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Check if task is ready
	// If parent deps are inherited: task should NOT be ready (epic2 is blocked by epic1)
	// If parent deps are NOT inherited: task SHOULD be ready (task has no direct deps)
	ready, err := db.ReadyItems("test")
	if err != nil {
		t.Fatalf("failed to get ready items: %v", err)
	}

	// Find our task in the ready list
	taskIsReady := false
	for _, item := range ready {
		if item.ID == task.ID {
			taskIsReady = true
			break
		}
	}

	if taskIsReady {
		t.Logf("Task %s IS ready (parent dependencies are NOT inherited)", task.ID)
		t.Logf("This means child tasks can be worked on even when parent epic is blocked")
	} else {
		t.Logf("Task %s is NOT ready (parent dependencies ARE inherited)", task.ID)
		t.Logf("This means child tasks are blocked when parent epic is blocked")
	}
}
