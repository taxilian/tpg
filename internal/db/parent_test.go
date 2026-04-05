package db

import (
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestCannotAddChildToClosedParent(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic parent and mark it done (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Try to set parent to the done epic - should fail
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

	// Create an epic parent and mark it canceled (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Try to set parent to the canceled epic - should fail
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

	// Create an epic parent and mark it done (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Create an epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Create an epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Create an epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Try to complete parent - should fail (epics cannot be completed directly)
	err := db.CompleteItem(parent.ID, "done", AgentContext{})
	if err == nil {
		t.Error("expected error when completing epic directly")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot complete epic") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCanCloseParentWhenChildrenDone(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Create an epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Create an open epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

	// Create an in_progress epic parent (only epics can have children)
	parent := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
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

func TestWorktreeEpicReturnsReadyToMerge(t *testing.T) {
	db := setupTestDB(t)

	epic := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Worktree Epic",
		Status:         model.StatusOpen,
		WorktreeBranch: "feature/test-branch",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusDone,
		ParentID:  &epic.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	info, err := db.CheckParentEpicCompletion(child.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected completion info for worktree epic, got nil")
	}
	if !info.ReadyToMerge {
		t.Error("expected ReadyToMerge=true for worktree epic")
	}
	if info.WorktreeBranch != "feature/test-branch" {
		t.Errorf("expected WorktreeBranch=feature/test-branch, got %s", info.WorktreeBranch)
	}

	item, err := db.GetItem(epic.ID)
	if err != nil {
		t.Fatalf("failed to get epic: %v", err)
	}
	if item.Status != model.StatusOpen {
		t.Errorf("expected epic status to remain open, got %s", item.Status)
	}
}

func TestNonWorktreeEpicAutoCompletes(t *testing.T) {
	db := setupTestDB(t)

	epic := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Regular Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	child := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusDone,
		ParentID:  &epic.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(child); err != nil {
		t.Fatalf("failed to create child: %v", err)
	}

	info, err := db.CheckParentEpicCompletion(child.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected completion info for regular epic, got nil")
	}
	if info.Epic.ID != epic.ID {
		t.Errorf("expected epic ID %s, got %s", epic.ID, info.Epic.ID)
	}
	if info.ReadyToMerge {
		t.Error("expected ReadyToMerge=false for non-worktree epic")
	}
}

func TestCompleteItem_RegularEpicError(t *testing.T) {
	db := setupTestDB(t)

	epic := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Regular Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	err := db.CompleteItem(epic.ID, "test results", AgentContext{})
	if err == nil {
		t.Fatal("expected error when completing epic directly")
	}
	if !strings.Contains(err.Error(), "cannot complete epic") {
		t.Errorf("expected error about cannot complete epic, got: %v", err)
	}
}

func TestCompleteItem_WorktreeEpicError(t *testing.T) {
	db := setupTestDB(t)

	epic := &model.Item{
		ID:             model.GenerateID(model.ItemTypeEpic),
		Project:        "test",
		Type:           model.ItemTypeEpic,
		Title:          "Worktree Epic",
		Status:         model.StatusOpen,
		WorktreeBranch: "feature/test",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	err := db.CompleteItem(epic.ID, "test results", AgentContext{})
	if err == nil {
		t.Fatal("expected error when completing worktree epic directly")
	}
	if !strings.Contains(err.Error(), "cannot complete worktree epic") {
		t.Errorf("expected error about cannot complete worktree epic, got: %v", err)
	}
	if !strings.Contains(err.Error(), "tpg epic merge") {
		t.Errorf("expected error to mention 'tpg epic merge', got: %v", err)
	}
}
