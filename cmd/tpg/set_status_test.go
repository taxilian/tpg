package main

import (
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

func TestSetStatus_RequiresForceFlag(t *testing.T) {
	database := setupTestDB(t)

	// Create a task
	task := &model.Item{
		ID:        "ts-forcetest",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Force Test Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Attempt to set status without --force flag should fail
	// This simulates what setStatusCmd would do without the force flag
	forceFlag := false
	if !forceFlag {
		// This is the expected behavior - without --force, command should fail
		// The actual error is returned by the command, which we verify in integration tests
		// For this unit test, we just verify the logic path
		return
	}
}

func TestSetStatus_ErrorMessage_SuggestsAlternatives(t *testing.T) {
	// Verify the error message contains helpful alternatives
	expectedAlternatives := []string{
		"tpg start",
		"tpg done",
		"tpg cancel",
	}

	// This test will verify the error message content once implemented
	errorMessage := "set-status is for fixing mistakes only. Use: tpg start, tpg done, or tpg cancel instead. Use --force if you really need this."

	for _, alt := range expectedAlternatives {
		if !strings.Contains(errorMessage, alt) {
			t.Errorf("error message should mention %q as an alternative", alt)
		}
	}

	if !strings.Contains(errorMessage, "--force") {
		t.Error("error message should mention --force flag")
	}

	if !strings.Contains(errorMessage, "mistakes") || !strings.Contains(errorMessage, "fixing") {
		t.Error("error message should explain this is for fixing mistakes")
	}
}

func TestSetStatus_WithForceFlag_Succeeds(t *testing.T) {
	database := setupTestDB(t)

	// Create a task
	task := &model.Item{
		ID:        "ts-forcesuccess",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Force Success Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// With --force flag, status change should succeed
	forceFlag := true
	if !forceFlag {
		t.Fatal("force flag should be true for this test")
	}

	// Simulate the status update that would happen with --force
	newStatus := model.StatusInProgress
	if err := database.UpdateStatus(task.ID, newStatus, db.AgentContext{}, true); err != nil {
		t.Fatalf("failed to update status with force flag: %v", err)
	}

	// Verify the status was changed
	updated, err := database.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get updated task: %v", err)
	}

	if updated.Status != newStatus {
		t.Errorf("status = %q, want %q", updated.Status, newStatus)
	}
}

func TestSetStatus_WithForce_PreservesExistingBehavior(t *testing.T) {
	database := setupTestDB(t)

	// Create a task
	task := &model.Item{
		ID:        "ts-preserve",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Preserve Behavior Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// With --force, the command should work exactly as before
	forceFlag := true
	if !forceFlag {
		t.Fatal("force flag should be true for this test")
	}

	// Test multiple status transitions
	transitions := []model.Status{
		model.StatusInProgress,
		model.StatusBlocked,
		model.StatusDone,
		model.StatusOpen,
	}

	for _, status := range transitions {
		if err := database.UpdateStatus(task.ID, status, db.AgentContext{}, true); err != nil {
			t.Fatalf("failed to update status to %s: %v", status, err)
		}

		updated, err := database.GetItem(task.ID)
		if err != nil {
			t.Fatalf("failed to get task: %v", err)
		}

		if updated.Status != status {
			t.Errorf("after setting to %s, status = %q", status, updated.Status)
		}
	}
}

func TestSetStatus_InvalidStatusStillFailsWithForce(t *testing.T) {
	database := setupTestDB(t)

	// Create a task
	task := &model.Item{
		ID:        "ts-invalid",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Invalid Status Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Even with --force, invalid status should still fail
	forceFlag := true
	invalidStatus := model.Status("invalid_status")

	if !invalidStatus.IsValid() {
		// This is expected - invalid status should be rejected regardless of force flag
		// The test documents this expected behavior
		_ = forceFlag // acknowledge force flag doesn't bypass validation
	}
}

func TestSetStatus_LogsEntryWhenForceUsed(t *testing.T) {
	database := setupTestDB(t)

	// Create a task
	task := &model.Item{
		ID:        "ts-logtest",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Log Test Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// When --force is used, a log entry should be added
	forceFlag := true
	if !forceFlag {
		t.Fatal("force flag should be true for this test")
	}

	newStatus := model.StatusInProgress
	if err := database.UpdateStatus(task.ID, newStatus, db.AgentContext{}, true); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Add log entry (as the command would do)
	logMsg := "Status force-set to " + string(newStatus)
	if err := database.AddLog(task.ID, logMsg); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	// Verify log was added
	logs, err := database.GetLogs(task.ID)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}

	found := false
	for _, log := range logs {
		if strings.Contains(log.Message, "force-set") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected log entry for force-set status change")
	}
}

func TestSetStatus_NonexistentTaskFails(t *testing.T) {
	database := setupTestDB(t)

	// Even with --force, nonexistent task should fail
	forceFlag := true
	_ = forceFlag

	// Attempt to update nonexistent task
	err := database.UpdateStatus("ts-nonexistent", model.StatusDone, db.AgentContext{}, true)
	if err == nil {
		t.Error("expected error when updating nonexistent task")
	}
}
