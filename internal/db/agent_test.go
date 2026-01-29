package db

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func TestGetAgentContext(t *testing.T) {
	// Save original env vars
	origID := os.Getenv("AGENT_ID")
	origType := os.Getenv("AGENT_TYPE")
	defer func() {
		os.Setenv("AGENT_ID", origID)
		os.Setenv("AGENT_TYPE", origType)
	}()

	tests := []struct {
		name       string
		agentID    string
		agentType  string
		wantActive bool
	}{
		{
			name:       "both set",
			agentID:    "agent-123",
			agentType:  "general",
			wantActive: true,
		},
		{
			name:       "only id set",
			agentID:    "agent-456",
			agentType:  "",
			wantActive: true,
		},
		{
			name:       "neither set",
			agentID:    "",
			agentType:  "",
			wantActive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AGENT_ID", tt.agentID)
			os.Setenv("AGENT_TYPE", tt.agentType)

			ctx := GetAgentContext()

			if ctx.ID != tt.agentID {
				t.Errorf("ID = %q, want %q", ctx.ID, tt.agentID)
			}
			if ctx.Type != tt.agentType {
				t.Errorf("Type = %q, want %q", ctx.Type, tt.agentType)
			}
			if ctx.IsActive() != tt.wantActive {
				t.Errorf("IsActive() = %v, want %v", ctx.IsActive(), tt.wantActive)
			}
		})
	}
}

func TestAgentContext_IsSubagent(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		want      bool
	}{
		{
			name:      "explore subagent",
			agentType: "explore",
			want:      false, // IsSubagent checks != "general", but empty also returns false
		},
		{
			name:      "general agent",
			agentType: "general",
			want:      false,
		},
		{
			name:      "empty type",
			agentType: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := AgentContext{Type: tt.agentType}
			if got := ctx.IsSubagent(); got != tt.want {
				t.Errorf("IsSubagent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateStatus_SetsAgentID(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Test task", "test", model.StatusOpen, 2)

	agentCtx := AgentContext{
		ID:   "agent-123",
		Type: "general",
	}

	// Start task - should set agent_id
	if err := db.UpdateStatus(item.ID, model.StatusInProgress, agentCtx); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if got.AgentID == nil {
		t.Fatal("AgentID should be set")
	}
	if *got.AgentID != "agent-123" {
		t.Errorf("AgentID = %q, want %q", *got.AgentID, "agent-123")
	}
	if got.AgentLastActive == nil {
		t.Error("AgentLastActive should be set")
	}
}

func TestUpdateStatus_ClearsAgentID_OnDone(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Test task", "test", model.StatusInProgress, 2)

	// Set agent first
	agentCtx := AgentContext{ID: "agent-123"}
	db.UpdateStatus(item.ID, model.StatusInProgress, agentCtx)

	// Mark done - should clear agent_id
	if err := db.UpdateStatus(item.ID, model.StatusDone, agentCtx); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if got.AgentID != nil {
		t.Errorf("AgentID should be cleared, got %q", *got.AgentID)
	}
}

func TestUpdateStatus_ClearsAgentID_OnBlocked(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Test task", "test", model.StatusInProgress, 2)

	agentCtx := AgentContext{ID: "agent-123"}
	db.UpdateStatus(item.ID, model.StatusInProgress, agentCtx)

	// Block - should clear agent_id
	if err := db.UpdateStatus(item.ID, model.StatusBlocked, agentCtx); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if got.AgentID != nil {
		t.Errorf("AgentID should be cleared, got %q", *got.AgentID)
	}
}

func TestUpdateStatus_ClearsAgentID_OnCanceled(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Test task", "test", model.StatusInProgress, 2)

	agentCtx := AgentContext{ID: "agent-123"}
	db.UpdateStatus(item.ID, model.StatusInProgress, agentCtx)

	// Cancel - should clear agent_id
	if err := db.UpdateStatus(item.ID, model.StatusCanceled, agentCtx); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if got.AgentID != nil {
		t.Errorf("AgentID should be cleared, got %q", *got.AgentID)
	}
}

func TestUpdateStatus_NoAgentContext(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Test task", "test", model.StatusOpen, 2)

	// Update without agent context
	emptyCtx := AgentContext{}
	if err := db.UpdateStatus(item.ID, model.StatusInProgress, emptyCtx); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if got.AgentID != nil {
		t.Errorf("AgentID should be nil when no agent context, got %q", *got.AgentID)
	}
}

func TestCompleteItem_ClearsAgentID(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Test task", "test", model.StatusInProgress, 2)

	// Set agent first
	agentCtx := AgentContext{ID: "agent-123"}
	db.UpdateStatus(item.ID, model.StatusInProgress, agentCtx)

	// Complete - should clear agent_id
	if err := db.CompleteItem(item.ID, "Done", agentCtx); err != nil {
		t.Fatalf("CompleteItem failed: %v", err)
	}

	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("GetItem failed: %v", err)
	}

	if got.AgentID != nil {
		t.Errorf("AgentID should be cleared, got %q", *got.AgentID)
	}
}

func TestProjectStatusFiltered_SeparatesAgentWork(t *testing.T) {
	db := setupTestDB(t)

	// Create tasks
	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusInProgress, 2)
	task2 := createTestItemWithProject(t, db, "Task 2", "test", model.StatusInProgress, 2)
	task3 := createTestItemWithProject(t, db, "Task 3", "test", model.StatusInProgress, 2)

	// Assign to different agents
	agent1 := AgentContext{ID: "agent-1"}
	agent2 := AgentContext{ID: "agent-2"}

	db.UpdateStatus(task1.ID, model.StatusInProgress, agent1)
	db.UpdateStatus(task2.ID, model.StatusInProgress, agent1)
	db.UpdateStatus(task3.ID, model.StatusInProgress, agent2)

	// Query as agent-1
	report, err := db.ProjectStatusFiltered("test", nil, "agent-1")
	if err != nil {
		t.Fatalf("ProjectStatusFiltered failed: %v", err)
	}

	// Should see 2 own tasks, 1 other agent task
	if len(report.MyInProgItems) != 2 {
		t.Errorf("MyInProgItems = %d, want 2", len(report.MyInProgItems))
	}
	if report.OtherInProgCount != 1 {
		t.Errorf("OtherInProgCount = %d, want 1", report.OtherInProgCount)
	}

	// Check that InProgress count includes both
	if report.InProgress != 3 {
		t.Errorf("InProgress = %d, want 3", report.InProgress)
	}
}

func TestProjectStatusFiltered_NoAgentID(t *testing.T) {
	db := setupTestDB(t)

	task1 := createTestItemWithProject(t, db, "Task 1", "test", model.StatusInProgress, 2)
	_ = createTestItemWithProject(t, db, "Task 2", "test", model.StatusInProgress, 2)

	agent1 := AgentContext{ID: "agent-1"}
	db.UpdateStatus(task1.ID, model.StatusInProgress, agent1)
	// task2 has no agent

	// Query without agent ID
	report, err := db.ProjectStatusFiltered("test", nil, "")
	if err != nil {
		t.Fatalf("ProjectStatusFiltered failed: %v", err)
	}

	// Should show all in-progress as InProgItems, none as "mine"
	if len(report.MyInProgItems) != 0 {
		t.Errorf("MyInProgItems should be empty when no agent ID provided")
	}
	if len(report.InProgItems) != 2 {
		t.Errorf("InProgItems = %d, want 2", len(report.InProgItems))
	}
}

func TestRecordAgentProjectAccess(t *testing.T) {
	db := setupTestDB(t)

	agentID := "agent-123"
	project := "testproject"

	// Record access
	if err := db.RecordAgentProjectAccess(agentID, project); err != nil {
		t.Fatalf("RecordAgentProjectAccess failed: %v", err)
	}

	// Verify recorded
	lastProject, err := db.GetAgentLastProject(agentID)
	if err != nil {
		t.Fatalf("GetAgentLastProject failed: %v", err)
	}

	if lastProject != project {
		t.Errorf("lastProject = %q, want %q", lastProject, project)
	}
}

func TestRecordAgentProjectAccess_UpdatesExisting(t *testing.T) {
	db := setupTestDB(t)

	agentID := "agent-123"
	project := "myproject"

	// Record first access
	db.RecordAgentProjectAccess(agentID, project)

	// Get timestamp
	var firstTime string
	db.QueryRow("SELECT last_active FROM agent_sessions WHERE agent_id = ? AND project = ?", agentID, project).Scan(&firstTime)

	time.Sleep(100 * time.Millisecond) // Ensure different timestamps

	// Record second access to SAME project - should update timestamp
	db.RecordAgentProjectAccess(agentID, project)

	// Get updated timestamp
	var secondTime string
	db.QueryRow("SELECT last_active FROM agent_sessions WHERE agent_id = ? AND project = ?", agentID, project).Scan(&secondTime)

	if firstTime == secondTime {
		t.Error("Timestamp should be updated on repeat access")
	}

	// Should still return the project
	lastProject, err := db.GetAgentLastProject(agentID)
	if err != nil {
		t.Fatalf("GetAgentLastProject failed: %v", err)
	}

	if lastProject != project {
		t.Errorf("lastProject = %q, want %q", lastProject, project)
	}
}

func TestGetAgentLastProject_NoHistory(t *testing.T) {
	db := setupTestDB(t)

	lastProject, err := db.GetAgentLastProject("nonexistent-agent")
	if err != nil {
		t.Fatalf("GetAgentLastProject failed: %v", err)
	}

	if lastProject != "" {
		t.Errorf("lastProject should be empty for unknown agent, got %q", lastProject)
	}
}

func TestCleanupOldAgentSessions(t *testing.T) {
	db := setupTestDB(t)

	agentID := "agent-123"

	// Record 25 accesses (more than the 20 limit) to different projects
	// Use explicit timestamps to ensure ordering
	for i := 0; i < 25; i++ {
		project := fmt.Sprintf("project-%02d", i)
		// Insert with explicit timestamp to avoid CURRENT_TIMESTAMP collision
		timestamp := time.Now().Add(time.Duration(i) * time.Second)
		_, err := db.Exec(`
			INSERT INTO agent_sessions (agent_id, project, last_active)
			VALUES (?, ?, ?)
		`, agentID, project, timestamp)
		if err != nil {
			t.Fatalf("Failed to insert session: %v", err)
		}
	}

	// Cleanup
	if err := db.CleanupOldAgentSessions(); err != nil {
		t.Fatalf("CleanupOldAgentSessions failed: %v", err)
	}

	// Check count - should be 20 or less
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM agent_sessions WHERE agent_id = ?", agentID).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if count > 20 {
		t.Errorf("agent_sessions count = %d, want <= 20", count)
	}

	// Most recent should still be there (project-24, index 24)
	lastProject, _ := db.GetAgentLastProject(agentID)
	expectedLast := "project-24"
	if lastProject != expectedLast {
		t.Errorf("Most recent project = %q, want %q", lastProject, expectedLast)
	}
}

func TestAgentTakeover(t *testing.T) {
	db := setupTestDB(t)

	item := createTestItemWithProject(t, db, "Shared task", "test", model.StatusInProgress, 2)

	// Agent 1 starts task
	agent1 := AgentContext{ID: "agent-1"}
	db.UpdateStatus(item.ID, model.StatusInProgress, agent1)

	// Agent 2 takes over (silent takeover - no error)
	agent2 := AgentContext{ID: "agent-2"}
	err := db.UpdateStatus(item.ID, model.StatusInProgress, agent2)
	if err != nil {
		t.Errorf("Agent takeover should not error, got: %v", err)
	}

	// Verify agent 2 now owns it
	got, _ := db.GetItem(item.ID)
	if got.AgentID == nil || *got.AgentID != "agent-2" {
		t.Errorf("Task should be assigned to agent-2, got %v", got.AgentID)
	}
}
func TestRecordAgentProjectAccess_MultipleProjects(t *testing.T) {
	db := setupTestDB(t)

	agentID := "agent-123"

	// Record accesses to multiple projects
	db.RecordAgentProjectAccess(agentID, "project1")
	time.Sleep(50 * time.Millisecond)
	db.RecordAgentProjectAccess(agentID, "project2")
	time.Sleep(50 * time.Millisecond)
	db.RecordAgentProjectAccess(agentID, "project3")

	// Should return most recent
	lastProject, err := db.GetAgentLastProject(agentID)
	if err != nil {
		t.Fatalf("GetAgentLastProject failed: %v", err)
	}

	if lastProject != "project3" {
		t.Errorf("lastProject = %q, want %q", lastProject, "project3")
	}
}
