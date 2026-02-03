package db

import (
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestCannotAddChildToClosedParent(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent and mark it done
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusDone,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create a child task
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Try to set parent to the done item - should fail
	err := db.SetParent(child.ID, parent.ID)
	if err == nil {
		t.Error("expected error when setting parent to closed item")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot add child to closed parent") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCannotAddChildToClosedParent_Canceled(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent and mark it canceled
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusCanceled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create a child task
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Try to set parent to the canceled item - should fail
	err := db.SetParent(child.ID, parent.ID)
	if err == nil {
		t.Error("expected error when setting parent to canceled item")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot add child to closed parent") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCannotAddChildToClosedParent_CreateItem(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent and mark it done
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusDone,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Try to create a child with the closed parent - should fail
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		ParentID:  &parent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := db.CreateItem(child)
	if err == nil {
		t.Error("expected error when creating item with closed parent")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot add child to closed parent") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCannotCloseParentWithOpenChildren(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create an open child
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		ParentID:  &parent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Try to mark parent done - should fail
	err := db.UpdateStatus(parent.ID, model.StatusDone, AgentContext{}, false)
	if err == nil {
		t.Error("expected error when closing parent with open children")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot close") {
		t.Errorf("unexpected error message: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "open children") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCannotCloseParentWithOpenChildren_Canceled(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create an in_progress child
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusInProgress,
		ParentID:  &parent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Try to cancel parent - should fail
	err := db.UpdateStatus(parent.ID, model.StatusCanceled, AgentContext{}, false)
	if err == nil {
		t.Error("expected error when canceling parent with open children")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot close") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCannotCloseParentWithOpenChildren_CompleteItem(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusInProgress,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create an open child
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		ParentID:  &parent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Try to complete parent - should fail
	err := db.CompleteItem(parent.ID, "done", AgentContext{})
	if err == nil {
		t.Error("expected error when completing parent with open children")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot close") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCanCloseParentWhenChildrenDone(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create a done child
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusDone,
		ParentID:  &parent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Mark parent done - should succeed
	err := db.UpdateStatus(parent.ID, model.StatusDone, AgentContext{}, false)
	if err != nil {
		t.Errorf("unexpected error when closing parent with done children: %v", err)
	}

	// Verify parent is done
	got, _ := db.GetItem(parent.ID)
	if got.Status != model.StatusDone {
		t.Errorf("status = %q, want %q", got.Status, model.StatusDone)
	}
}

func TestCanCloseParentWhenChildrenCanceled(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create a canceled child
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusCanceled,
		ParentID:  &parent.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Mark parent done - should succeed
	err := db.UpdateStatus(parent.ID, model.StatusDone, AgentContext{}, false)
	if err != nil {
		t.Errorf("unexpected error when closing parent with canceled children: %v", err)
	}
}

func TestCanAddChildToOpenParent(t *testing.T) {
	db := setupTestDB(t)

	// Create an open parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create a child task
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Set parent - should succeed
	err := db.SetParent(child.ID, parent.ID)
	if err != nil {
		t.Errorf("unexpected error when setting parent to open item: %v", err)
	}

	// Verify parent was set
	got, _ := db.GetItem(child.ID)
	if got.ParentID == nil {
		t.Error("expected parent to be set")
	} else if *got.ParentID != parent.ID {
		t.Errorf("parent = %q, want %q", *got.ParentID, parent.ID)
	}
}

func TestCanAddChildToInProgressParent(t *testing.T) {
	db := setupTestDB(t)

	// Create an in_progress parent
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusInProgress,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Create a child task
	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	// Set parent - should succeed
	err := db.SetParent(child.ID, parent.ID)
	if err != nil {
		t.Errorf("unexpected error when setting parent to in_progress item: %v", err)
	}
}

func TestCanCloseParentWithNoChildren(t *testing.T) {
	db := setupTestDB(t)

	// Create a parent with no children
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Parent Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(parent); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	// Mark parent done - should succeed
	err := db.UpdateStatus(parent.ID, model.StatusDone, AgentContext{}, false)
	if err != nil {
		t.Errorf("unexpected error when closing parent with no children: %v", err)
	}
}
