package db

import (
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestCancelBlockedByDependents(t *testing.T) {
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

	if err := database.UpdateStatus(blocker.ID, model.StatusCanceled, AgentContext{}, false); err == nil {
		t.Fatal("expected error when canceling item with dependents")
	}
}

func TestCancelForceAllowsDependents(t *testing.T) {
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

	if err := database.UpdateStatus(blocker.ID, model.StatusCanceled, AgentContext{}, true); err != nil {
		t.Fatalf("expected force cancel to succeed: %v", err)
	}
}
