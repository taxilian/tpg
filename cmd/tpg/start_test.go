package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

func setupCommandDB(t *testing.T) *db.DB {
	t.Helper()
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "test.db")
	t.Setenv("TPG_DB", path)
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestStart_InProgressWithoutResumeFails(t *testing.T) {
	database := setupCommandDB(t)
	item := createTestItem(t, database, "ts-inprog", "In progress task")

	agentCtx := db.AgentContext{ID: "agent-a"}
	if err := database.UpdateStatus(item.ID, model.StatusInProgress, agentCtx, false); err != nil {
		t.Fatalf("failed to set in_progress: %v", err)
	}

	flagResume = false
	t.Cleanup(func() { flagResume = false })

	err := startCmd.RunE(startCmd, []string{item.ID})
	if err == nil {
		t.Fatal("expected error when starting in-progress task without --resume")
	}

	msg := err.Error()
	if !strings.Contains(msg, "already in progress") {
		t.Fatalf("expected in-progress error, got %q", msg)
	}
	if !strings.Contains(msg, "agent-a") {
		t.Fatalf("expected agent id in error, got %q", msg)
	}
	if !strings.Contains(msg, "--resume") {
		t.Fatalf("expected --resume hint in error, got %q", msg)
	}
}

func TestStart_InProgressWithResumeSucceeds(t *testing.T) {
	database := setupCommandDB(t)
	item := createTestItem(t, database, "ts-resume", "Resume task")

	agentCtx := db.AgentContext{ID: "agent-a"}
	if err := database.UpdateStatus(item.ID, model.StatusInProgress, agentCtx, false); err != nil {
		t.Fatalf("failed to set in_progress: %v", err)
	}

	flagResume = true
	t.Cleanup(func() { flagResume = false })

	var runErr error
	output := captureOutput(func() {
		runErr = startCmd.RunE(startCmd, []string{item.ID})
	})
	if runErr != nil {
		t.Fatalf("expected resume to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "Resuming ts-resume") {
		t.Fatalf("expected resume output, got %q", output)
	}

	updated, err := database.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to fetch updated item: %v", err)
	}
	if updated.Status != model.StatusInProgress {
		t.Fatalf("expected status in_progress, got %s", updated.Status)
	}

	logs, err := database.GetLogs(item.ID)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("expected a log entry for resume")
	}
	if !strings.Contains(logs[len(logs)-1].Message, "Resumed") {
		t.Fatalf("expected resume log entry, got %q", logs[len(logs)-1].Message)
	}
}

func TestStart_OpenSucceeds(t *testing.T) {
	database := setupCommandDB(t)
	item := createTestItem(t, database, "ts-open", "Open task", func(i *model.Item) {
		i.Status = model.StatusOpen
		i.UpdatedAt = time.Now()
	})

	flagResume = false
	t.Cleanup(func() { flagResume = false })

	var runErr error
	output := captureOutput(func() {
		runErr = startCmd.RunE(startCmd, []string{item.ID})
	})
	if runErr != nil {
		t.Fatalf("expected start to succeed, got %v", runErr)
	}
	if !strings.Contains(output, "Started ts-open") {
		t.Fatalf("expected start output, got %q", output)
	}

	updated, err := database.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to fetch updated item: %v", err)
	}
	if updated.Status != model.StatusInProgress {
		t.Fatalf("expected status in_progress, got %s", updated.Status)
	}
}
