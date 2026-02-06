package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

func setupAddCommandTest(t *testing.T) *db.DB {
	t.Helper()

	workDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
	})

	if _, err := db.InitProject("", ""); err != nil {
		t.Fatalf("failed to init project: %v", err)
	}

	dbPath := filepath.Join(workDir, ".tpg", "tpg.db")
	t.Setenv("TPG_DB", dbPath)

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.Init(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}

func resetAddCmdFlags() {
	flagTemplateID = ""
	flagTemplateVars = nil
	flagTemplateVarsYAML = false
	flagEpic = false
	flagType = ""
	flagPrefix = ""
	flagDescription = ""
	flagPriority = 0
	flagParent = ""
	flagBlocks = ""
	flagAfter = ""
	flagAddLabels = nil
	flagDryRun = false
	flagWorktree = false
	flagWorktreeBranch = ""
	flagWorktreeBase = ""
	flagWorktreeAllow = false
	flagProject = ""
}

func captureStdoutAndStderr(f func()) (string, string) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	os.Stdout = stdoutW
	os.Stderr = stderrW
	f()
	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return stdoutBuf.String(), stderrBuf.String()
}

func captureCombinedOutput(f func()) string {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	os.Stdout = w
	os.Stderr = w
	f()
	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func boolPtr(value bool) *bool {
	return &value
}

func TestAddCmd_ParentFlag(t *testing.T) {
	database := setupTestDB(t)

	// Create an epic to be the parent
	epic := &model.Item{
		ID:        "ep-parent",
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Parent Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	// Create a task with --parent flag
	task := &model.Item{
		ID:        "ts-child1",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Simulate what addCmd does with --parent flag
	if err := database.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}

	// Verify the parent was set
	got, err := database.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if got.ParentID == nil {
		t.Fatal("expected parent to be set")
	}
	if *got.ParentID != epic.ID {
		t.Errorf("parent = %q, want %q", *got.ParentID, epic.ID)
	}
}

func TestAddCmd_BlocksFlag(t *testing.T) {
	database := setupTestDB(t)

	// Create a task that will be blocked
	blockedTask := &model.Item{
		ID:        "ts-blocked",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Blocked Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(blockedTask); err != nil {
		t.Fatalf("failed to create blocked task: %v", err)
	}

	// Create a blocker task with --blocks flag
	blocker := &model.Item{
		ID:        "ts-blocker",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Blocker Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(blocker); err != nil {
		t.Fatalf("failed to create blocker: %v", err)
	}

	// Simulate what addCmd does with --blocks flag
	// blockedTask depends on blocker (blocker blocks blockedTask)
	if err := database.AddDep(blockedTask.ID, blocker.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Verify the dependency was created
	deps, err := database.GetDeps(blockedTask.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0] != blocker.ID {
		t.Errorf("dep = %q, want %q", deps[0], blocker.ID)
	}

	// Verify blocker task shows as unmet dependency
	hasUnmet, err := database.HasUnmetDeps(blockedTask.ID)
	if err != nil {
		t.Fatalf("failed to check unmet deps: %v", err)
	}
	if !hasUnmet {
		t.Error("expected blocked task to have unmet deps")
	}
}

func TestAddCmd_ParentFlag_NonEpicParent(t *testing.T) {
	database := setupTestDB(t)

	// Create a task (not an epic)
	notEpic := &model.Item{
		ID:        "ts-notepic",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Not an Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(notEpic); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create another task
	task := &model.Item{
		ID:        "ts-child2",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Child Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Non-epics can now be parents (arbitrary hierarchies allowed)
	err := database.SetParent(task.ID, notEpic.ID)
	if err != nil {
		t.Errorf("unexpected error when setting non-epic as parent: %v", err)
	}

	// Verify the parent was set
	got, err := database.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if got.ParentID == nil {
		t.Error("expected parent to be set")
	} else if *got.ParentID != notEpic.ID {
		t.Errorf("parent = %q, want %q", *got.ParentID, notEpic.ID)
	}
}

func TestAddCmd_BlocksFlag_NonexistentTarget(t *testing.T) {
	database := setupTestDB(t)

	// Create a blocker task
	blocker := &model.Item{
		ID:        "ts-blocker2",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Blocker",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(blocker); err != nil {
		t.Fatalf("failed to create blocker: %v", err)
	}

	// Try to block a nonexistent task - should fail
	err := database.AddDep("nonexistent", blocker.ID)
	if err == nil {
		t.Error("expected error when blocking nonexistent task")
	}
}

func TestAddCmd_BothFlags(t *testing.T) {
	database := setupTestDB(t)

	// Create an epic
	epic := &model.Item{
		ID:        "ep-combo",
		Project:   "test",
		Type:      model.ItemTypeEpic,
		Title:     "Combo Epic",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(epic); err != nil {
		t.Fatalf("failed to create epic: %v", err)
	}

	// Create a task to be blocked
	blockedTask := &model.Item{
		ID:        "ts-blocked2",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Blocked Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(blockedTask); err != nil {
		t.Fatalf("failed to create blocked task: %v", err)
	}

	// Create a task with both --parent and --blocks
	task := &model.Item{
		ID:        "ts-combo",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Combo Task",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Simulate addCmd with both flags
	if err := database.SetParent(task.ID, epic.ID); err != nil {
		t.Fatalf("failed to set parent: %v", err)
	}
	if err := database.AddDep(blockedTask.ID, task.ID); err != nil {
		t.Fatalf("failed to add dep: %v", err)
	}

	// Verify parent
	got, err := database.GetItem(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != epic.ID {
		t.Error("parent not set correctly")
	}

	// Verify blocking relationship
	deps, err := database.GetDeps(blockedTask.ID)
	if err != nil {
		t.Fatalf("failed to get deps: %v", err)
	}
	if len(deps) != 1 || deps[0] != task.ID {
		t.Error("blocking relationship not set correctly")
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  hello   world  ", 2},
		{"one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen", 15},
		{"one two three four five six seven eight nine ten eleven twelve thirteen fourteen", 14},
		{"This is a short description", 5},
		{"This is a longer description that has more than fifteen words in it to test the threshold properly", 18},
		{"\n\ttabbed\n\nand\nnewlines\n", 3},
	}

	for _, tt := range tests {
		got := countWords(tt.input)
		if got != tt.expected {
			t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestAddCmd_EmptyDescriptionWarnsOnStderr(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	var runErr error
	stdout, stderr := captureStdoutAndStderr(func() {
		runErr = addCmd.RunE(addCmd, []string{"Empty description warning"})
	})
	if runErr != nil {
		t.Fatalf("expected add command to succeed, got %v", runErr)
	}
	if !strings.Contains(stderr, "WARNING: This description is very short") {
		t.Fatalf("expected warning on stderr, got %q", stderr)
	}
	if strings.Contains(stdout, "WARNING: This description is very short") {
		t.Fatalf("expected warning to stay on stderr, got stdout %q", stdout)
	}
}

func TestAddCmd_EmptyDescriptionWarningPrintedBeforeID(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	var runErr error
	output := captureCombinedOutput(func() {
		runErr = addCmd.RunE(addCmd, []string{"Empty description ordering"})
	})
	if runErr != nil {
		t.Fatalf("expected add command to succeed, got %v", runErr)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	warningIndex := -1
	idIndex := -1
	idLine := regexp.MustCompile(`^ts-[a-z0-9]+$`)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if warningIndex == -1 && strings.HasPrefix(trimmed, "WARNING: This description is very short") {
			warningIndex = i
		}
		if idIndex == -1 && idLine.MatchString(trimmed) {
			idIndex = i
		}
	}
	if warningIndex == -1 {
		t.Fatalf("expected warning line in output, got %q", output)
	}
	if idIndex == -1 {
		t.Fatalf("expected item id line in output, got %q", output)
	}
	if warningIndex > idIndex {
		t.Fatalf("expected warning before item id, got warning at %d and id at %d", warningIndex, idIndex)
	}
}

func TestAddCmd_ShortDescriptionWarnsForNonEmptyDescription(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	flagDescription = "Short description"

	var runErr error
	_, stderr := captureStdoutAndStderr(func() {
		runErr = addCmd.RunE(addCmd, []string{"Short description warning"})
	})
	if runErr != nil {
		t.Fatalf("expected add command to succeed, got %v", runErr)
	}
	if !strings.Contains(stderr, "WARNING: This description is very short") {
		t.Fatalf("expected warning for short description, got %q", stderr)
	}
}

func TestAddCmd_ShortDescriptionWarningDisabledByConfig(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	config := &db.Config{
		Warnings: db.WarningsConfig{
			ShortDescription: boolPtr(false),
		},
	}
	if err := db.SaveConfig(config); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	flagDescription = "Short description"

	var runErr error
	_, stderr := captureStdoutAndStderr(func() {
		runErr = addCmd.RunE(addCmd, []string{"Short description warning disabled"})
	})
	if runErr != nil {
		t.Fatalf("expected add command to succeed, got %v", runErr)
	}
	if strings.Contains(stderr, "WARNING: This description is very short") {
		t.Fatalf("expected warning to be disabled, got %q", stderr)
	}
}

func TestAddCmd_TemplateSkipsShortDescriptionWarning(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	templatesDir := filepath.Join(".tpg", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	content := `title: Template Task
description: Template description
steps:
  - id: step1
    title: First step
    description: Do the first thing
`
	if err := os.WriteFile(filepath.Join(templatesDir, "simple-template.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	flagTemplateID = "simple-template"

	var runErr error
	_, stderr := captureStdoutAndStderr(func() {
		runErr = addCmd.RunE(addCmd, []string{"Template-based task"})
	})
	if runErr != nil {
		t.Fatalf("expected add command to succeed, got %v", runErr)
	}
	if strings.Contains(stderr, "WARNING: This description is very short") {
		t.Fatalf("expected template instantiation to skip warnings, got %q", stderr)
	}
}

// Tests for --type flag validation

func TestValidateTypeFlag(t *testing.T) {
	tests := []struct {
		name        string
		typeValue   string
		expectError bool
	}{
		{"empty string is valid", "", false},
		{"task is valid", "task", false},
		{"epic is valid", "epic", false},
		{"story is invalid", "story", true},
		{"bug is invalid", "bug", true},
		{"feature is invalid", "feature", true},
		{"uppercase TASK is invalid", "TASK", true},
		{"unknown is invalid", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTypeFlag(tt.typeValue)
			if tt.expectError && err == nil {
				t.Errorf("validateTypeFlag(%q) = nil, want error", tt.typeValue)
			}
			if !tt.expectError && err != nil {
				t.Errorf("validateTypeFlag(%q) = %v, want nil", tt.typeValue, err)
			}
		})
	}
}

func TestValidateTypeFlag_ErrorMessageContent(t *testing.T) {
	err := validateTypeFlag("story")
	if err == nil {
		t.Fatal("expected error for invalid type")
	}

	errMsg := err.Error()

	// Check the error message contains required parts
	if !strings.Contains(errMsg, `--type must be "task" or "epic"`) {
		t.Errorf("error message should mention valid types, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "--label") {
		t.Errorf("error message should suggest using --label, got: %s", errMsg)
	}
}

func TestAddCmd_TypeValidation_RejectsInvalidType(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	flagType = "story" // Invalid type

	err := addCmd.RunE(addCmd, []string{"Test task"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}

	if !strings.Contains(err.Error(), `--type must be "task" or "epic"`) {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAddCmd_TypeValidation_AcceptsTask(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	flagType = "task"

	var runErr error
	captureStdoutAndStderr(func() {
		runErr = addCmd.RunE(addCmd, []string{"Test task"})
	})

	if runErr != nil {
		t.Fatalf("expected success for --type task, got: %v", runErr)
	}
}

func TestAddCmd_TypeValidation_AcceptsEpic(t *testing.T) {
	setupAddCommandTest(t)
	resetAddCmdFlags()
	t.Cleanup(resetAddCmdFlags)

	flagType = "epic"

	var runErr error
	captureStdoutAndStderr(func() {
		runErr = addCmd.RunE(addCmd, []string{"Test epic"})
	})

	if runErr != nil {
		t.Fatalf("expected success for --type epic, got: %v", runErr)
	}
}

func resetReplaceCmdFlags() {
	flagEpic = false
	flagType = ""
	flagPrefix = ""
	flagDescription = ""
	flagPriority = 0
	flagContext = ""
	flagOnClose = ""
	flagAddLabels = nil
	flagProject = ""
}

func TestReplaceCmd_TypeValidation_RejectsInvalidType(t *testing.T) {
	database := setupAddCommandTest(t)
	resetReplaceCmdFlags()
	t.Cleanup(resetReplaceCmdFlags)

	// Create a task to replace
	task := &model.Item{
		ID:        "ts-toreplace",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Task to replace",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	flagType = "story" // Invalid type

	err := replaceCmd.RunE(replaceCmd, []string{"ts-toreplace", "New title"})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}

	if !strings.Contains(err.Error(), `--type must be "task" or "epic"`) {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReplaceCmd_TypeValidation_AcceptsTask(t *testing.T) {
	database := setupAddCommandTest(t)
	resetReplaceCmdFlags()
	t.Cleanup(resetReplaceCmdFlags)

	// Create a task to replace
	task := &model.Item{
		ID:        "ts-toreplace2",
		Project:   "test",
		Type:      model.ItemTypeTask,
		Title:     "Task to replace",
		Status:    model.StatusOpen,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.CreateItem(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	flagType = "task"

	var runErr error
	captureStdoutAndStderr(func() {
		runErr = replaceCmd.RunE(replaceCmd, []string{"ts-toreplace2", "New title"})
	})

	if runErr != nil {
		t.Fatalf("expected success for --type task, got: %v", runErr)
	}
}

func resetListCmdFlags() {
	flagListAll = false
	flagStatus = ""
	flagListParent = ""
	flagListType = ""
	flagListEpic = ""
	flagBlocking = ""
	flagBlockedBy = ""
	flagHasBlockers = false
	flagNoBlockers = false
	flagIdsOnly = false
	flagListFlat = false
	flagFilterLabels = nil
	flagProject = ""
}

func TestListCmd_TypeValidation_RejectsInvalidType(t *testing.T) {
	setupAddCommandTest(t)
	resetListCmdFlags()
	t.Cleanup(resetListCmdFlags)

	flagListType = "feature" // Invalid type

	err := listCmd.RunE(listCmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid type")
	}

	if !strings.Contains(err.Error(), `--type must be "task" or "epic"`) {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestListCmd_TypeValidation_AcceptsEpic(t *testing.T) {
	setupAddCommandTest(t)
	resetListCmdFlags()
	t.Cleanup(resetListCmdFlags)

	flagListType = "epic"

	var runErr error
	captureStdoutAndStderr(func() {
		runErr = listCmd.RunE(listCmd, []string{})
	})

	if runErr != nil {
		t.Fatalf("expected success for --type epic, got: %v", runErr)
	}
}

func TestListCmd_TypeValidation_AcceptsTask(t *testing.T) {
	setupAddCommandTest(t)
	resetListCmdFlags()
	t.Cleanup(resetListCmdFlags)

	flagListType = "task"

	var runErr error
	captureStdoutAndStderr(func() {
		runErr = listCmd.RunE(listCmd, []string{})
	})

	if runErr != nil {
		t.Fatalf("expected success for --type task, got: %v", runErr)
	}
}
