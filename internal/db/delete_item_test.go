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

	if err := database.DeleteItem(blocker.ID, false); err == nil {
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

	if err := database.DeleteItem(blocker.ID, true); err != nil {
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
