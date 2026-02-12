package db

import (
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestDeleteItemBlockedByDependents(t *testing.T) {
	database := setupTestDB(t)
	now := time.Now()

	blocker := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Blocker",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateItem(blocker); err != nil {
		t.Fatalf("failed to create blocker: %v", err)
	}

	dependent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Dependent",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateItem(dependent); err != nil {
		t.Fatalf("failed to create dependent: %v", err)
	}

	if err := database.AddDep(dependent.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	if err := database.DeleteItem(blocker.ID, false, false); err == nil {
		t.Fatal("expected error when deleting item with dependents")
	}
}

func TestDeleteItemForceRemovesDependencies(t *testing.T) {
	database := setupTestDB(t)
	now := time.Now()

	blocker := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Blocker",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateItem(blocker); err != nil {
		t.Fatalf("failed to create blocker: %v", err)
	}

	dependent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Dependent",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateItem(dependent); err != nil {
		t.Fatalf("failed to create dependent: %v", err)
	}

	if err := database.AddDep(dependent.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dependency: %v", err)
	}

	if err := database.DeleteItem(blocker.ID, true, false); err != nil {
		t.Fatalf("expected force delete to succeed: %v", err)
	}

	deps, err := database.GetDeps(dependent.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected dependencies to be removed, got %d", len(deps))
	}
}

func TestDeleteItemWithChildrenBlocksWithoutForce(t *testing.T) {
	database := setupTestDB(t)
	now := time.Now()

	// Create parent epic
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child task
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
		ParentID:  &parent.ID,
	}
	if err := database.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Try to delete parent without force - should fail
	if err := database.DeleteItem(parent.ID, false, false); err == nil {
		t.Fatal("expected error when deleting epic with children without force")
	}

	// Verify both still exist
	if _, err := database.GetItem(parent.ID); err != nil {
		t.Fatalf("parent should still exist: %v", err)
	}
	if _, err := database.GetItem(child.ID); err != nil {
		t.Fatalf("child should still exist: %v", err)
	}
}

func TestDeleteItemForceDeletesChildrenRecursively(t *testing.T) {
	database := setupTestDB(t)
	now := time.Now()

	// Create parent epic
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := database.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create child task
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
		ParentID:  &parent.ID,
	}
	if err := database.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Create another child (tasks can't have children, so both are direct children of epic)
	child2 := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task 2",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
		ParentID:  &parent.ID,
	}
	if err := database.CreateItem(child2); err != nil {
		t.Fatalf("failed to create child2: %v", err)
	}

	// Delete parent with force - should delete all descendants
	if err := database.DeleteItem(parent.ID, true, true); err != nil {
		t.Fatalf("expected force delete to succeed: %v", err)
	}

	// Verify all are deleted
	if _, err := database.GetItem(parent.ID); err == nil {
		t.Fatal("parent should be deleted")
	}
	if _, err := database.GetItem(child.ID); err == nil {
		t.Fatal("child should be deleted")
	}
	if _, err := database.GetItem(child2.ID); err == nil {
		t.Fatal("child2 should be deleted")
	}
}
