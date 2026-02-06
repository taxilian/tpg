package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hasPathSuffix checks if a path ends with the given suffix components
func hasPathSuffix(path, suffix string) bool {
	return strings.HasSuffix(path, suffix)
}

// Helper to change working directory and restore it on cleanup
func chdir(t *testing.T, dir string) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
}

// Helper to create .tpg directory structure
func setupTpgDir(t *testing.T, dir string) string {
	t.Helper()
	tpgDir := filepath.Join(dir, DataDir)
	if err := os.MkdirAll(tpgDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg dir: %v", err)
	}
	return tpgDir
}

// Helper to write config.json
func writeConfig(t *testing.T, tpgDir string, config *Config) {
	t.Helper()
	// Create the .tpg directory if it doesn't exist
	if err := os.MkdirAll(tpgDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg dir: %v", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	configPath := filepath.Join(tpgDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
}

func TestLoadConfig_DefaultsWhenNoConfigExists(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, DefaultTaskPrefix)
	}
	if config.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, DefaultEpicPrefix)
	}
	// DefaultProject should be derived from temp directory name
	if config.DefaultProject == "" {
		t.Error("DefaultProject should not be empty")
	}
}

func TestLoadConfig_LoadsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write a custom config
	existingConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "tsk",
			Epic: "epc",
		},
		DefaultProject: "myproject",
	}
	writeConfig(t, tpgDir, existingConfig)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != "tsk" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "tsk")
	}
	if config.Prefixes.Epic != "epc" {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, "epc")
	}
	if config.DefaultProject != "myproject" {
		t.Errorf("DefaultProject = %q, want %q", config.DefaultProject, "myproject")
	}
}

func TestLoadConfig_AppliesDefaultsToPartialConfig(t *testing.T) {
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write partial config (only task prefix)
	partialConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "custom",
		},
	}
	writeConfig(t, tpgDir, partialConfig)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != "custom" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "custom")
	}
	// Epic should get default since it was empty
	if config.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, DefaultEpicPrefix)
	}
	// DefaultProject should be derived from directory
	if config.DefaultProject == "" {
		t.Error("DefaultProject should not be empty")
	}
}

func TestLoadConfig_NoTpgDirectory(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	// No .tpg directory created

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error when .tpg directory does not exist")
	}
}

func TestSaveConfig_WriteAndReload(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Save a custom config
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "mytask",
			Epic: "myepic",
		},
		DefaultProject: "testproj",
	}

	if err := SaveConfig(config); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(dir, DataDir, ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Reload and verify
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Prefixes.Task != "mytask" {
		t.Errorf("Task prefix = %q, want %q", loaded.Prefixes.Task, "mytask")
	}
	if loaded.Prefixes.Epic != "myepic" {
		t.Errorf("Epic prefix = %q, want %q", loaded.Prefixes.Epic, "myepic")
	}
	if loaded.DefaultProject != "testproj" {
		t.Errorf("DefaultProject = %q, want %q", loaded.DefaultProject, "testproj")
	}
}

func TestSaveConfig_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Save config with empty fields
	config := &Config{}

	if err := SaveConfig(config); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Reload and verify defaults were applied
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want %q", loaded.Prefixes.Task, DefaultTaskPrefix)
	}
	if loaded.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want %q", loaded.Prefixes.Epic, DefaultEpicPrefix)
	}
}

func TestInitProject_CreatesDirectoryAndConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	dbPath, err := InitProject("", "")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	// Verify .tpg directory was created
	tpgDir := filepath.Join(dir, DataDir)
	info, err := os.Stat(tpgDir)
	if os.IsNotExist(err) {
		t.Fatal(".tpg directory was not created")
	}
	if !info.IsDir() {
		t.Fatal(".tpg should be a directory")
	}

	// Verify config.json was created
	configPath := filepath.Join(tpgDir, ConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.json was not created")
	}

	// Verify returned db path ends correctly (avoid /private symlink issues on macOS)
	expectedSuffix := filepath.Join(DataDir, DBFile)
	if !filepath.IsAbs(dbPath) || !hasPathSuffix(dbPath, expectedSuffix) {
		t.Errorf("dbPath = %q, want to end with %q", dbPath, expectedSuffix)
	}

	// Load and verify defaults
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if config.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, DefaultTaskPrefix)
	}
	if config.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, DefaultEpicPrefix)
	}
}

func TestInitProject_CustomTaskEpicPrefixes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := InitProject("ticket", "story")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != "ticket" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "ticket")
	}
	if config.Prefixes.Epic != "story" {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, "story")
	}
}

func TestInitProject_NormalizesTrailingDash(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := InitProject("task-", "epic-")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Trailing dashes should be removed
	if config.Prefixes.Task != "task" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "task")
	}
	if config.Prefixes.Epic != "epic" {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, "epic")
	}
}

func TestDefaultProject_DerivedFromDirectory(t *testing.T) {
	// Create a directory with a known name
	parentDir := t.TempDir()
	projectDir := filepath.Join(parentDir, "my-awesome-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	tpgDir := filepath.Join(projectDir, DataDir)
	if err := os.MkdirAll(tpgDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg dir: %v", err)
	}

	chdir(t, projectDir)

	project, err := DefaultProject()
	if err != nil {
		t.Fatalf("DefaultProject() error = %v", err)
	}

	if project != "my-awesome-project" {
		t.Errorf("DefaultProject() = %q, want %q", project, "my-awesome-project")
	}
}

func TestDefaultProject_UsesConfigIfSet(t *testing.T) {
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write config with explicit project name
	config := &Config{
		DefaultProject: "explicit-project-name",
	}
	writeConfig(t, tpgDir, config)

	project, err := DefaultProject()
	if err != nil {
		t.Fatalf("DefaultProject() error = %v", err)
	}

	if project != "explicit-project-name" {
		t.Errorf("DefaultProject() = %q, want %q", project, "explicit-project-name")
	}
}

func TestNormalizePrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"task", "task"},
		{"task-", "task"},
		{"  task  ", "task"},
		{"  task-  ", "task"},
		{"task--", "task-"}, // Only removes one trailing dash
		{"", ""},
		{"-", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizePrefix(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultProjectName(t *testing.T) {
	tests := []struct {
		name     string
		dataDir  string
		expected string
	}{
		{
			name:     "normal directory",
			dataDir:  "/home/user/myproject/.tpg",
			expected: "myproject",
		},
		{
			name:     "nested directory",
			dataDir:  "/home/user/code/awesome-app/.tpg",
			expected: "awesome-app",
		},
		{
			name:     "root with .tpg",
			dataDir:  "/.tpg",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := defaultProjectName(tt.dataDir)
			if result != tt.expected {
				t.Errorf("defaultProjectName(%q) = %q, want %q", tt.dataDir, result, tt.expected)
			}
		})
	}
}

func TestLoadConfig_FromSubdirectory(t *testing.T) {
	// Test that LoadConfig works when in a subdirectory of the project
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)

	// Write a config at the root level
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "rootconfig",
		},
	}
	writeConfig(t, tpgDir, config)

	// Create a subdirectory and change to it
	subDir := filepath.Join(dir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	chdir(t, subDir)

	// Should find the config in parent directory
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Prefixes.Task != "rootconfig" {
		t.Errorf("Task prefix = %q, want %q", loaded.Prefixes.Task, "rootconfig")
	}
}

// ============================================================================
// WorktreeConfig Tests
// ============================================================================

func TestWorktreeConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Load config without worktree section
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify defaults
	if config.Worktree.BranchPrefix != "feature" {
		t.Errorf("BranchPrefix = %q, want %q", config.Worktree.BranchPrefix, "feature")
	}
	if config.Worktree.Root != ".worktrees" {
		t.Errorf("Root = %q, want %q", config.Worktree.Root, ".worktrees")
	}
	if !config.Worktree.RequireEpicIDEnabled() {
		t.Errorf("RequireEpicIDEnabled() = false, want true")
	}
}

func TestWorktreeConfig_LoadsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write config with worktree settings
	requireFalse := false
	existingConfig := &Config{
		Worktree: WorktreeConfig{
			BranchPrefix:  "wip",
			RequireEpicID: &requireFalse,
			Root:          "worktrees",
		},
	}
	writeConfig(t, tpgDir, existingConfig)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Worktree.BranchPrefix != "wip" {
		t.Errorf("BranchPrefix = %q, want %q", config.Worktree.BranchPrefix, "wip")
	}
	if config.Worktree.Root != "worktrees" {
		t.Errorf("Root = %q, want %q", config.Worktree.Root, "worktrees")
	}
	if config.Worktree.RequireEpicIDEnabled() {
		t.Errorf("RequireEpicIDEnabled() = true, want false")
	}
}

func TestWorktreeConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Save config with worktree settings
	requireTrue := true
	config := &Config{
		Worktree: WorktreeConfig{
			BranchPrefix:  "dev",
			RequireEpicID: &requireTrue,
			Root:          "wt",
		},
	}

	if err := SaveConfig(config); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Reload and verify
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Worktree.BranchPrefix != "dev" {
		t.Errorf("BranchPrefix = %q, want %q", loaded.Worktree.BranchPrefix, "dev")
	}
	if !loaded.Worktree.RequireEpicIDEnabled() {
		t.Errorf("RequireEpicIDEnabled() = false, want true")
	}
	if loaded.Worktree.Root != "wt" {
		t.Errorf("Root = %q, want %q", loaded.Worktree.Root, "wt")
	}
}
