package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Should create parent directories
	if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}

func TestDefaultPath(t *testing.T) {
	// Create a temp directory with .tpg subdirectory
	dir := t.TempDir()
	tpgDir := filepath.Join(dir, ".tpg")
	if err := os.MkdirAll(tpgDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg dir: %v", err)
	}

	// Change to temp dir so findDataDir can find .tpg
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("failed to get default path: %v", err)
	}

	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}

	if !contains(path, ".tpg/tpg.db") {
		t.Errorf("expected path to contain .tpg/tpg.db, got %q", path)
	}
}

func TestDefaultPath_EnvVar(t *testing.T) {
	customPath := "/custom/path/to/db.db"
	t.Setenv("TPG_DB", customPath)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("failed to get default path: %v", err)
	}

	if path != customPath {
		t.Errorf("expected path %q, got %q", customPath, path)
	}
}

func TestDefaultPath_NotFound(t *testing.T) {
	// Create a temp directory without .tpg
	dir := t.TempDir()

	// Change to temp dir
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	// Clear any env var
	t.Setenv("TPG_DB", "")

	_, err := DefaultPath()
	if err == nil {
		t.Fatal("expected error when .tpg not found")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s[1:], substr))
}

func TestCreateItem(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test task",
		Status:    model.StatusOpen,
		Priority:  2,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Verify it was created
	got, err := db.GetItem(item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if got.Title != item.Title {
		t.Errorf("title = %q, want %q", got.Title, item.Title)
	}
	if got.Project != item.Project {
		t.Errorf("project = %q, want %q", got.Project, item.Project)
	}
}

func TestCreateItem_InvalidType(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:      "ts-123456",
		Project: "test",
		Type:    model.ItemType(""), // empty type is invalid
		Title:   "Test",
		Status:  model.StatusOpen,
	}

	err := db.CreateItem(item)
	if err == nil {
		t.Error("expected error for invalid (empty) type")
	}
}

func TestCreateItem_InvalidStatus(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:      "ts-123456",
		Project: "test",
		Type:    model.ItemTypeTask,
		Title:   "Test",
		Status:  model.Status("invalid"),
	}

	err := db.CreateItem(item)
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestGetItem_NotFound(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.GetItem("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestUpdateStatus(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := db.UpdateStatus(item.ID, model.StatusInProgress, AgentContext{}, false); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	got, _ := db.GetItem(item.ID)
	if got.Status != model.StatusInProgress {
		t.Errorf("status = %q, want %q", got.Status, model.StatusInProgress)
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := db.UpdateStatus("nonexistent", model.StatusDone, AgentContext{}, false)
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestUpdateStatus_InvalidStatus(t *testing.T) {
	db := setupTestDB(t)

	err := db.UpdateStatus("ts-123456", model.Status("invalid"), AgentContext{}, false)
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestAppendDescription(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:          model.GenerateID(model.ItemTypeTask),
		Project:     "test",
		Type:        model.ItemTypeTask,
		Title:       "Test",
		Description: "Initial",
		Status:      model.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := db.AppendDescription(item.ID, "Appended text"); err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	got, _ := db.GetItem(item.ID)
	if got.Description == "Initial" {
		t.Error("description was not appended")
	}
}

func TestSetParent(t *testing.T) {
	db := setupTestDB(t)

	epic := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Test Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	task := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if err := db.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	got, _ := db.GetItem(task.ID)
	if got.ParentID == nil {
		t.Fatal("expected parent ID to be set")
	}
	if *got.ParentID != epic.ID {
		t.Errorf("parent = %q, want %q", *got.ParentID, epic.ID)
	}
}

func TestSetParent_NonEpicParent(t *testing.T) {
	db := setupTestDB(t)

	task1 := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Task 1",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task1); err != nil {
		t.Fatalf("failed to create task1: %v", err)
	}

	task2 := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Task 2",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task2); err != nil {
		t.Fatalf("failed to create task2: %v", err)
	}

	// Non-epics can now be parents (arbitrary hierarchies allowed)
	err := db.SetParent(task2.ID, task1.ID)
	if err != nil {
		t.Errorf("unexpected error when setting non-epic as parent: %v", err)
	}

	// Verify the parent was set
	got, err := db.GetItem(task2.ID)
	if err != nil {
		t.Fatalf("failed to get task2: %v", err)
	}
	if got.ParentID == nil {
		t.Error("expected parent to be set")
	} else if *got.ParentID != task1.ID {
		t.Errorf("parent = %q, want %q", *got.ParentID, task1.ID)
	}
}

func TestSetDescription(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:          model.GenerateID(model.ItemTypeTask),
		Project:     "test",
		Type:        model.ItemTypeTask,
		Title:       "Test",
		Description: "Original description",
		Status:      model.StatusOpen,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := db.SetDescription(item.ID, "New description"); err != nil {
		t.Fatalf("failed to set description: %v", err)
	}

	got, _ := db.GetItem(item.ID)
	if got.Description != "New description" {
		t.Errorf("description = %q, want %q", got.Description, "New description")
	}
}

func TestSetDescription_EmptyToContent(t *testing.T) {
	db := setupTestDB(t)

	item := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := db.SetDescription(item.ID, "Added description"); err != nil {
		t.Fatalf("failed to set description: %v", err)
	}

	got, _ := db.GetItem(item.ID)
	if got.Description != "Added description" {
		t.Errorf("description = %q, want %q", got.Description, "Added description")
	}
}

func TestSetDescription_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := db.SetDescription("nonexistent", "text")
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestSetParent_NotFound(t *testing.T) {
	db := setupTestDB(t)

	epic := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	// Nonexistent task
	err := db.SetParent("nonexistent", epic.ID)
	if err == nil {
		t.Error("expected error for nonexistent task")
	}

	// Nonexistent parent
	task := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	err = db.SetParent(task.ID, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent parent")
	}
}

func TestSetSharedContext(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic
	epic := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Test Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	// Set context on epic - should succeed
	err := db.SetSharedContext(epic.ID, "shared context for all tasks")
	if err != nil {
		t.Errorf("SetSharedContext on epic failed: %v", err)
	}

	// Verify it was set
	got, err := db.GetItem(epic.ID)
	if err != nil {
		t.Fatalf("failed to get epic: %v", err)
	}
	if got.SharedContext != "shared context for all tasks" {
		t.Errorf("SharedContext = %q, want %q", got.SharedContext, "shared context for all tasks")
	}
}

func TestSetSharedContext_NonEpic(t *testing.T) {
	db := setupTestDB(t)

	// Create a task (not an epic)
	task := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Set context on task - should fail
	err := db.SetSharedContext(task.ID, "context")
	if err == nil {
		t.Error("expected error when setting shared context on non-epic")
	}
	if !strings.Contains(err.Error(), "only be set on epics") {
		t.Errorf("error message should mention epics: %v", err)
	}
}

func TestSetSharedContext_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := db.SetSharedContext("nonexistent", "context")
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestSetClosingInstructions(t *testing.T) {
	db := setupTestDB(t)

	// Create an epic
	epic := &model.Item{
		ID:        model.GenerateID(model.ItemTypeEpic),
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Test Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	// Set instructions on epic - should succeed
	err := db.SetClosingInstructions(epic.ID, "run tests before merging")
	if err != nil {
		t.Errorf("SetClosingInstructions on epic failed: %v", err)
	}

	// Verify it was set
	got, err := db.GetItem(epic.ID)
	if err != nil {
		t.Fatalf("failed to get epic: %v", err)
	}
	if got.ClosingInstructions != "run tests before merging" {
		t.Errorf("ClosingInstructions = %q, want %q", got.ClosingInstructions, "run tests before merging")
	}
}

func TestSetClosingInstructions_NonEpic(t *testing.T) {
	db := setupTestDB(t)

	// Create a task (not an epic)
	task := &model.Item{
		ID:        model.GenerateID(model.ItemTypeTask),
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Test Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Set instructions on task - should fail
	err := db.SetClosingInstructions(task.ID, "instructions")
	if err == nil {
		t.Error("expected error when setting closing instructions on non-epic")
	}
	if !strings.Contains(err.Error(), "only be set on epics") {
		t.Errorf("error message should mention epics: %v", err)
	}
}

func TestSetClosingInstructions_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := db.SetClosingInstructions("nonexistent", "instructions")
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestSchemaVersion(t *testing.T) {
	// Verify SchemaVersion is set to 6
	if SchemaVersion != 6 {
		t.Errorf("SchemaVersion = %d, want 6", SchemaVersion)
	}
}

func TestMigrationV4_WorktreeColumns(t *testing.T) {
	db := setupTestDB(t)

	// Verify worktree columns exist by querying them
	var worktreeBranch, worktreeBase sql.NullString
	err := db.QueryRow("SELECT worktree_branch, worktree_base FROM items LIMIT 1").Scan(&worktreeBranch, &worktreeBase)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("failed to query worktree columns: %v", err)
	}
	// Success - columns exist (result may be empty since no items, but no error = columns exist)
}

func TestMigrationV4_ExistingDataPreserved(t *testing.T) {
	// Create a v3 database and migrate it to v4
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Create schema as if it were v3 (without worktree columns)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'open',
			priority INTEGER DEFAULT 2,
			parent_id TEXT,
			agent_id TEXT,
			agent_last_active DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			template_id TEXT,
			step_index INTEGER,
			variables TEXT,
			template_hash TEXT,
			results TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to create v3 schema: %v", err)
	}

	// Set version to 3
	if err := db.setSchemaVersion(3); err != nil {
		t.Fatalf("failed to set schema version to 3: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES ('ts-test123', 'test', 'task', 'Test Item', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	db.Close()

	// Reopen and migrate
	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Verify version is now 6
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 6 {
		t.Errorf("schema version = %d, want 6", version)
	}

	// Verify existing data is preserved
	var title string
	err = db.QueryRow("SELECT title FROM items WHERE id = 'ts-test123'").Scan(&title)
	if err != nil {
		t.Fatalf("failed to query existing item: %v", err)
	}
	if title != "Test Item" {
		t.Errorf("title = %q, want 'Test Item'", title)
	}

	// Verify new columns are NULL by default
	var branch, base sql.NullString
	err = db.QueryRow("SELECT worktree_branch, worktree_base FROM items WHERE id = 'ts-test123'").Scan(&branch, &base)
	if err != nil {
		t.Fatalf("failed to query worktree columns: %v", err)
	}
	if branch.Valid {
		t.Error("expected worktree_branch to be NULL")
	}
	if base.Valid {
		t.Error("expected worktree_base to be NULL")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"task", "task"},
		{"User Story", "user-story"},
		{"Bug Fix", "bug-fix"},
		{"user story", "user-story"},
		{"  spaced  ", "spaced"},
		{"UPPERCASE", "uppercase"},
		{"with123numbers", "with123numbers"},
		{"special!@#chars", "special-chars"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"", ""},
		{"---", ""},
		{"a", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMigrationV6_ConvertsLegacyTypes(t *testing.T) {
	// Create a v5 database with legacy item types
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Initialize with base schema
	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Reset to v5 to test migration
	if err := db.setSchemaVersion(5); err != nil {
		t.Fatalf("failed to set schema version to 5: %v", err)
	}

	// Insert items with legacy types (bypass validation by using raw SQL)
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES
			('ts-legacy1', 'test', 'story', 'A story item', 'open'),
			('ts-legacy2', 'test', 'user story', 'A user story', 'open'),
			('ts-task1', 'test', 'task', 'A regular task', 'open'),
			('ep-epic1', 'test', 'epic', 'An epic', 'open'),
			('ts-legacy3', 'test', 'Bug Fix', 'A bug fix', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	db.Close()

	// Reopen and run migration
	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Verify version is now 6
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 6 {
		t.Errorf("schema version = %d, want 6", version)
	}

	// Verify legacy types converted to task
	tests := []struct {
		id          string
		wantType    string
		wantLabels  []string
		description string
	}{
		{"ts-legacy1", "task", []string{"story"}, "story should become task + story label"},
		{"ts-legacy2", "task", []string{"user-story"}, "user story should become task + user-story label"},
		{"ts-task1", "task", nil, "task type should remain unchanged"},
		{"ep-epic1", "epic", nil, "epic type should remain unchanged"},
		{"ts-legacy3", "task", []string{"bug-fix"}, "Bug Fix should become task + bug-fix label"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var itemType string
			err := db.QueryRow("SELECT type FROM items WHERE id = ?", tt.id).Scan(&itemType)
			if err != nil {
				t.Fatalf("failed to query item %s: %v", tt.id, err)
			}
			if itemType != tt.wantType {
				t.Errorf("item %s type = %q, want %q", tt.id, itemType, tt.wantType)
			}

			// Check labels
			labels, err := db.GetItemLabels(tt.id)
			if err != nil {
				t.Fatalf("failed to get labels for %s: %v", tt.id, err)
			}

			if tt.wantLabels == nil {
				if len(labels) > 0 {
					t.Errorf("item %s should have no labels, got %v", tt.id, labels)
				}
			} else {
				if len(labels) != len(tt.wantLabels) {
					t.Errorf("item %s has %d labels, want %d", tt.id, len(labels), len(tt.wantLabels))
				}
				for i, want := range tt.wantLabels {
					if i < len(labels) && labels[i].Name != want {
						t.Errorf("item %s label[%d] = %q, want %q", tt.id, i, labels[i].Name, want)
					}
				}
			}
		})
	}
}

func TestMigrationV6_IdempotentLabels(t *testing.T) {
	// Multiple items with same legacy type should share the same label
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	if err := db.setSchemaVersion(5); err != nil {
		t.Fatalf("failed to set schema version to 5: %v", err)
	}

	// Insert multiple items with same legacy type
	_, err = db.Exec(`
		INSERT INTO items (id, project, type, title, status)
		VALUES
			('ts-bug1', 'test', 'bug', 'Bug 1', 'open'),
			('ts-bug2', 'test', 'bug', 'Bug 2', 'open'),
			('ts-bug3', 'test', 'bug', 'Bug 3', 'open')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	db.Close()

	db, err = Open(path)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Verify there's only one 'bug' label
	labels, err := db.ListLabels("test")
	if err != nil {
		t.Fatalf("failed to list labels: %v", err)
	}

	bugCount := 0
	for _, l := range labels {
		if l.Name == "bug" {
			bugCount++
		}
	}
	if bugCount != 1 {
		t.Errorf("expected exactly 1 'bug' label, got %d", bugCount)
	}

	// Verify all three items have the bug label
	for _, id := range []string{"ts-bug1", "ts-bug2", "ts-bug3"} {
		itemLabels, err := db.GetItemLabels(id)
		if err != nil {
			t.Fatalf("failed to get labels for %s: %v", id, err)
		}
		if len(itemLabels) != 1 || itemLabels[0].Name != "bug" {
			t.Errorf("item %s should have 'bug' label, got %v", id, itemLabels)
		}
	}
}

func TestMigrationV6_EmptyDatabase(t *testing.T) {
	// Migration should work on empty database (no items to migrate)
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// Verify version is 6
	version, err := db.getSchemaVersion()
	if err != nil {
		t.Fatalf("failed to get schema version: %v", err)
	}
	if version != 6 {
		t.Errorf("schema version = %d, want 6", version)
	}
}
