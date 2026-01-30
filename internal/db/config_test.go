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

func TestInitProject_CustomPrefixes(t *testing.T) {
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

func TestUpdatePrefixes_UpdatesTaskPrefix(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Initialize with defaults
	_, err := InitProject("", "")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	// Update only task prefix
	if err := UpdatePrefixes("newtask", ""); err != nil {
		t.Fatalf("UpdatePrefixes() error = %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != "newtask" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "newtask")
	}
	// Epic should remain default
	if config.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, DefaultEpicPrefix)
	}
}

func TestUpdatePrefixes_UpdatesEpicPrefix(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Initialize with defaults
	_, err := InitProject("", "")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	// Update only epic prefix
	if err := UpdatePrefixes("", "newepic"); err != nil {
		t.Fatalf("UpdatePrefixes() error = %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Task should remain default
	if config.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, DefaultTaskPrefix)
	}
	if config.Prefixes.Epic != "newepic" {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, "newepic")
	}
}

func TestUpdatePrefixes_UpdatesBoth(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Initialize with defaults
	_, err := InitProject("", "")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	// Update both prefixes
	if err := UpdatePrefixes("bug", "feature"); err != nil {
		t.Fatalf("UpdatePrefixes() error = %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != "bug" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "bug")
	}
	if config.Prefixes.Epic != "feature" {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, "feature")
	}
}

func TestUpdatePrefixes_NormalizesTrailingDash(t *testing.T) {
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Initialize with defaults
	_, err := InitProject("", "")
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}

	// Update with trailing dashes
	if err := UpdatePrefixes("issue-", "milestone-"); err != nil {
		t.Fatalf("UpdatePrefixes() error = %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Prefixes.Task != "issue" {
		t.Errorf("Task prefix = %q, want %q", config.Prefixes.Task, "issue")
	}
	if config.Prefixes.Epic != "milestone" {
		t.Errorf("Epic prefix = %q, want %q", config.Prefixes.Epic, "milestone")
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
// LoadMergedConfig Tests
// ============================================================================

// TestLoadMergedConfig_SingleLocation tests loading from a single config location
func TestLoadMergedConfig_SingleLocation(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	config := &Config{
		Prefixes: PrefixConfig{
			Task: "custom",
			Epic: "epc",
		},
		DefaultProject: "myproject",
		IDLength:       5,
	}
	writeConfig(t, tpgDir, config)

	// Act
	merged, err := LoadMergedConfig()

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfig() error = %v", err)
	}
	if merged.Prefixes.Task != "custom" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "custom")
	}
	if merged.Prefixes.Epic != "epc" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "epc")
	}
	if merged.DefaultProject != "myproject" {
		t.Errorf("DefaultProject = %q, want %q", merged.DefaultProject, "myproject")
	}
	if merged.IDLength != 5 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 5)
	}
}

// TestLoadMergedConfig_NoConfigs_ReturnsDefaults tests that defaults are applied when no configs exist
func TestLoadMergedConfig_NoConfigs_ReturnsDefaults(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Act
	merged, err := LoadMergedConfig()

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfig() error = %v", err)
	}
	if merged.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, DefaultTaskPrefix)
	}
	if merged.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, DefaultEpicPrefix)
	}
	if merged.IDLength != 3 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 3)
	}
}

// TestLoadMergedConfig_MultipleLocations_Override tests that later configs override earlier ones
func TestLoadMergedConfig_MultipleLocations_Override(t *testing.T) {
	// Arrange: Create system, user, and worktree configs with different values
	systemDir := t.TempDir()
	userDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	setupTpgDir(t, userDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: base values
	systemConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "sys",
			Epic: "sysepic",
		},
		DefaultProject: "system-project",
		IDLength:       4,
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// User config: overrides some values
	userConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "usr", // Override system
			// Epic not set, should inherit from system
		},
		DefaultProject: "user-project", // Override system
		// IDLength not set, should inherit from system
	}
	writeConfig(t, filepath.Join(userDir, ".tpg"), userConfig)

	// Worktree config: overrides some values
	worktreeConfig := &Config{
		Prefixes: PrefixConfig{
			// Task not set, should inherit from user
			Epic: "wtepic", // Override user/system
		},
		// DefaultProject not set, should inherit from user
		IDLength: 6, // Override system
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act: Load merged config with search paths
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(userDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}
	// Task: worktree (not set) -> user ("usr") -> system ("sys") = "usr"
	if merged.Prefixes.Task != "usr" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "usr")
	}
	// Epic: worktree ("wtepic") overrides user (not set) and system ("sysepic")
	if merged.Prefixes.Epic != "wtepic" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "wtepic")
	}
	// DefaultProject: worktree (not set) -> user ("user-project") = "user-project"
	if merged.DefaultProject != "user-project" {
		t.Errorf("DefaultProject = %q, want %q", merged.DefaultProject, "user-project")
	}
	// IDLength: worktree (6) overrides system (4)
	if merged.IDLength != 6 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 6)
	}
}

// TestLoadMergedConfig_MissingFiles_Graceful tests that missing configs are handled gracefully
func TestLoadMergedConfig_MissingFiles_Graceful(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Only create one config, leave others missing
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "only",
		},
	}
	writeConfig(t, tpgDir, config)

	// Act: Try to load from multiple paths where some don't exist
	merged, err := LoadMergedConfigWithPaths(
		"/nonexistent/system/config.json",
		"/nonexistent/user/config.json",
		filepath.Join(tpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() should handle missing files gracefully, got error: %v", err)
	}
	if merged.Prefixes.Task != "only" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "only")
	}
	// Other fields should have defaults
	if merged.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want default %q", merged.Prefixes.Epic, DefaultEpicPrefix)
	}
}

// TestLoadMergedConfig_AllMissing_ReturnsDefaults tests that all missing configs return defaults
func TestLoadMergedConfig_AllMissing_ReturnsDefaults(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Act: Load from paths that don't exist
	merged, err := LoadMergedConfigWithPaths(
		"/nonexistent/1/config.json",
		"/nonexistent/2/config.json",
		"/nonexistent/3/config.json",
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}
	if merged.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want default %q", merged.Prefixes.Task, DefaultTaskPrefix)
	}
	if merged.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want default %q", merged.Prefixes.Epic, DefaultEpicPrefix)
	}
}

// TestLoadMergedConfig_InvalidJSON_ReturnsError tests that invalid JSON is handled properly
func TestLoadMergedConfig_InvalidJSON_ReturnsError(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write invalid JSON
	configPath := filepath.Join(tpgDir, ConfigFile)
	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	// Act
	_, err := LoadMergedConfig()

	// Assert
	if err == nil {
		t.Error("LoadMergedConfig() expected error for invalid JSON, got nil")
	}
}

// TestLoadMergedConfig_InvalidJSONInChain_ReturnsError tests that invalid JSON anywhere in the chain fails
func TestLoadMergedConfig_InvalidJSONInChain_ReturnsError(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write valid config
	validConfig := &Config{Prefixes: PrefixConfig{Task: "valid"}}
	writeConfig(t, tpgDir, validConfig)

	// Write invalid config in another location
	invalidDir := t.TempDir()
	invalidTpgDir := filepath.Join(invalidDir, ".tpg")
	os.MkdirAll(invalidTpgDir, 0755)
	invalidConfigPath := filepath.Join(invalidTpgDir, ConfigFile)
	os.WriteFile(invalidConfigPath, []byte("{not valid}"), 0644)

	// Act
	_, err := LoadMergedConfigWithPaths(
		invalidConfigPath,
		filepath.Join(tpgDir, ConfigFile),
	)

	// Assert
	if err == nil {
		t.Error("LoadMergedConfigWithPaths() expected error when any config is invalid")
	}
}

// TestLoadMergedConfig_EmptyConfigFile tests handling of empty config files
func TestLoadMergedConfig_EmptyConfigFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// Write empty JSON object
	configPath := filepath.Join(tpgDir, ConfigFile)
	os.WriteFile(configPath, []byte("{}"), 0644)

	// Act
	merged, err := LoadMergedConfig()

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfig() error = %v", err)
	}
	// Should apply defaults for empty fields
	if merged.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want default %q", merged.Prefixes.Task, DefaultTaskPrefix)
	}
	if merged.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want default %q", merged.Prefixes.Epic, DefaultEpicPrefix)
	}
}

// TestLoadMergedConfig_PartialOverride tests selective field overriding
func TestLoadMergedConfig_PartialOverride(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	tpgDir := setupTpgDir(t, dir)
	chdir(t, dir)

	// System: all fields set
	systemDir := t.TempDir()
	setupTpgDir(t, systemDir)
	systemConfig := &Config{
		Prefixes:       PrefixConfig{Task: "sys", Epic: "sysepic"},
		DefaultProject: "sysproj",
		IDLength:       4,
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// User: only overrides IDLength
	userDir := t.TempDir()
	setupTpgDir(t, userDir)
	userConfig := &Config{
		IDLength: 8,
	}
	writeConfig(t, filepath.Join(userDir, ".tpg"), userConfig)

	// Worktree: only overrides Task
	worktreeConfig := &Config{
		Prefixes: PrefixConfig{Task: "work"},
	}
	writeConfig(t, tpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(userDir, ".tpg", ConfigFile),
		filepath.Join(tpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}
	// Task from worktree
	if merged.Prefixes.Task != "work" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "work")
	}
	// Epic from system (not overridden)
	if merged.Prefixes.Epic != "sysepic" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "sysepic")
	}
	// DefaultProject from system (not overridden)
	if merged.DefaultProject != "sysproj" {
		t.Errorf("DefaultProject = %q, want %q", merged.DefaultProject, "sysproj")
	}
	// IDLength from user (overrides system, not overridden by worktree)
	if merged.IDLength != 8 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 8)
	}
}

// TestLoadMergedConfig_WorktreeLocalAndRoot tests worktree-local vs worktree-root config merging
func TestLoadMergedConfig_WorktreeLocalAndRoot(t *testing.T) {
	// Arrange: Simulate a worktree with local .tpg and root .tpg
	rootDir := t.TempDir()
	rootTpgDir := setupTpgDir(t, rootDir)

	// Create a subdirectory that also has .tpg (worktree-local)
	subDir := filepath.Join(rootDir, "subdir")
	localTpgDir := setupTpgDir(t, subDir)
	chdir(t, subDir)

	// Root config: base settings
	rootConfig := &Config{
		Prefixes:       PrefixConfig{Task: "root", Epic: "roote"},
		DefaultProject: "rootproj",
		IDLength:       5,
	}
	writeConfig(t, rootTpgDir, rootConfig)

	// Local config: overrides some settings
	localConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "local", // Override root
			// Epic not set, should inherit from root
		},
		// DefaultProject not set, should inherit from root
		IDLength: 3, // Override root
	}
	writeConfig(t, localTpgDir, localConfig)

	// Act: Load with local first (higher priority), then root
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(rootTpgDir, ConfigFile),
		filepath.Join(localTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}
	// Task from local (later in chain)
	if merged.Prefixes.Task != "local" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "local")
	}
	// Epic from root (not overridden by local)
	if merged.Prefixes.Epic != "roote" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "roote")
	}
	// DefaultProject from root (not overridden by local)
	if merged.DefaultProject != "rootproj" {
		t.Errorf("DefaultProject = %q, want %q", merged.DefaultProject, "rootproj")
	}
	// IDLength from local (overrides root)
	if merged.IDLength != 3 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 3)
	}
}

// ============================================================================
// LoadMergedConfig Tests - Custom Prefixes Merging
// ============================================================================

// TestLoadMergedConfig_CustomPrefixes_Merge tests that custom prefixes are properly merged
func TestLoadMergedConfig_CustomPrefixes_Merge(t *testing.T) {
	// Arrange: Create configs with different custom prefixes
	systemDir := t.TempDir()
	userDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	setupTpgDir(t, userDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: base custom prefixes
	systemConfig := &Config{
		Prefixes: PrefixConfig{Task: "sys", Epic: "sysepic"},
		CustomPrefixes: map[string]string{
			"story": "st",
			"bug":   "bg",
		},
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// User config: adds new custom prefix, overrides one
	userConfig := &Config{
		CustomPrefixes: map[string]string{
			"bug":     "bugfix", // Override system
			"feature": "ft",     // New entry
		},
	}
	writeConfig(t, filepath.Join(userDir, ".tpg"), userConfig)

	// Worktree config: adds another custom prefix
	worktreeConfig := &Config{
		CustomPrefixes: map[string]string{
			"issue": "iss", // New entry
		},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(userDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// Verify custom prefixes merged correctly
	if merged.CustomPrefixes == nil {
		t.Fatal("CustomPrefixes should not be nil")
	}

	// story from system (not overridden)
	if merged.CustomPrefixes["story"] != "st" {
		t.Errorf("story prefix = %q, want %q", merged.CustomPrefixes["story"], "st")
	}

	// bug from user (overrides system)
	if merged.CustomPrefixes["bug"] != "bugfix" {
		t.Errorf("bug prefix = %q, want %q", merged.CustomPrefixes["bug"], "bugfix")
	}

	// feature from user (new)
	if merged.CustomPrefixes["feature"] != "ft" {
		t.Errorf("feature prefix = %q, want %q", merged.CustomPrefixes["feature"], "ft")
	}

	// issue from worktree (new)
	if merged.CustomPrefixes["issue"] != "iss" {
		t.Errorf("issue prefix = %q, want %q", merged.CustomPrefixes["issue"], "iss")
	}
}

// TestLoadMergedConfig_CustomPrefixes_EmptyMap tests merging with empty custom_prefixes
func TestLoadMergedConfig_CustomPrefixes_EmptyMap(t *testing.T) {
	// Arrange
	systemDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: has custom prefixes
	systemConfig := &Config{
		CustomPrefixes: map[string]string{
			"story": "st",
		},
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// Worktree config: empty custom_prefixes map (should not clear system values)
	worktreeConfig := &Config{
		CustomPrefixes: map[string]string{},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// story should still be present from system
	if merged.CustomPrefixes["story"] != "st" {
		t.Errorf("story prefix = %q, want %q", merged.CustomPrefixes["story"], "st")
	}
}

// TestLoadMergedConfig_CustomPrefixes_NilMap tests merging with nil custom_prefixes
func TestLoadMergedConfig_CustomPrefixes_NilMap(t *testing.T) {
	// Arrange
	systemDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: has custom prefixes
	systemConfig := &Config{
		CustomPrefixes: map[string]string{
			"story": "st",
		},
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// Worktree config: no custom_prefixes field (nil)
	worktreeConfig := &Config{
		Prefixes: PrefixConfig{Task: "wt"},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// story should still be present from system
	if merged.CustomPrefixes["story"] != "st" {
		t.Errorf("story prefix = %q, want %q", merged.CustomPrefixes["story"], "st")
	}
}

// TestLoadMergedConfig_CustomPrefixes_OverrideToEmpty tests overriding with empty string value
func TestLoadMergedConfig_CustomPrefixes_OverrideToEmpty(t *testing.T) {
	// Arrange
	systemDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: has custom prefixes
	systemConfig := &Config{
		CustomPrefixes: map[string]string{
			"story": "st",
		},
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// Worktree config: overrides with empty string
	worktreeConfig := &Config{
		CustomPrefixes: map[string]string{
			"story": "", // Override with empty
		},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// story should be empty after override
	if merged.CustomPrefixes["story"] != "" {
		t.Errorf("story prefix = %q, want empty string", merged.CustomPrefixes["story"])
	}
}

// ============================================================================
// LoadMergedConfig Tests - Environment Variable
// ============================================================================

// TestLoadMergedConfig_SystemConfigEnvVar tests TPG_SYSTEM_CONFIG environment variable
func TestLoadMergedConfig_SystemConfigEnvVar(t *testing.T) {
	// Arrange
	systemDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// Create system config at custom location
	systemConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "sys",
			Epic: "sysepic",
		},
		DefaultProject: "system-project",
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// Create worktree config
	worktreeConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "work",
		},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Set environment variable
	systemConfigPath := filepath.Join(systemDir, ".tpg", ConfigFile)
	t.Setenv("TPG_SYSTEM_CONFIG", systemConfigPath)

	// Act
	merged, err := LoadMergedConfig()

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfig() error = %v", err)
	}

	// Task from worktree (highest priority)
	if merged.Prefixes.Task != "work" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "work")
	}

	// Epic from system (not overridden)
	if merged.Prefixes.Epic != "sysepic" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "sysepic")
	}
}

// ============================================================================
// LoadMergedConfig Tests - Complex Scenarios
// ============================================================================

// TestLoadMergedConfig_CompleteHierarchy tests a complete 3-level hierarchy
func TestLoadMergedConfig_CompleteHierarchy(t *testing.T) {
	// Arrange: System -> User -> Worktree
	systemDir := t.TempDir()
	userDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	setupTpgDir(t, userDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System: base configuration
	systemConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "sys-task",
			Epic: "sys-epic",
		},
		DefaultProject: "system",
		IDLength:       4,
		CustomPrefixes: map[string]string{
			"story": "sys-story",
			"bug":   "sys-bug",
		},
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// User: overrides some values
	userConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "user-task", // Override system
			// Epic not set
		},
		DefaultProject: "user", // Override system
		// IDLength not set
		CustomPrefixes: map[string]string{
			"bug":     "user-bug",  // Override system
			"feature": "user-feat", // New
		},
	}
	writeConfig(t, filepath.Join(userDir, ".tpg"), userConfig)

	// Worktree: overrides some values
	worktreeConfig := &Config{
		Prefixes: PrefixConfig{
			// Task not set
			Epic: "work-epic", // Override
		},
		// DefaultProject not set
		IDLength: 6, // Override
		CustomPrefixes: map[string]string{
			"issue": "work-issue", // New
		},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(userDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// Task: worktree (not set) -> user ("user-task") -> system ("sys-task") = "user-task"
	if merged.Prefixes.Task != "user-task" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "user-task")
	}

	// Epic: worktree ("work-epic") -> user (not set) -> system ("sys-epic") = "work-epic"
	if merged.Prefixes.Epic != "work-epic" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "work-epic")
	}

	// DefaultProject: worktree (not set) -> user ("user") -> system ("system") = "user"
	if merged.DefaultProject != "user" {
		t.Errorf("DefaultProject = %q, want %q", merged.DefaultProject, "user")
	}

	// IDLength: worktree (6) -> user (not set) -> system (4) = 6
	if merged.IDLength != 6 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 6)
	}

	// Custom prefixes merged
	if merged.CustomPrefixes["story"] != "sys-story" {
		t.Errorf("story prefix = %q, want %q", merged.CustomPrefixes["story"], "sys-story")
	}
	if merged.CustomPrefixes["bug"] != "user-bug" {
		t.Errorf("bug prefix = %q, want %q", merged.CustomPrefixes["bug"], "user-bug")
	}
	if merged.CustomPrefixes["feature"] != "user-feat" {
		t.Errorf("feature prefix = %q, want %q", merged.CustomPrefixes["feature"], "user-feat")
	}
	if merged.CustomPrefixes["issue"] != "work-issue" {
		t.Errorf("issue prefix = %q, want %q", merged.CustomPrefixes["issue"], "work-issue")
	}
}

// TestLoadMergedConfig_SingleFileOnly tests loading from just one file
func TestLoadMergedConfig_SingleFileOnly(t *testing.T) {
	// Arrange
	worktreeDir := t.TempDir()
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	config := &Config{
		Prefixes: PrefixConfig{
			Task: "only-task",
			Epic: "only-epic",
		},
		DefaultProject: "only-project",
		IDLength:       5,
		CustomPrefixes: map[string]string{
			"story": "only-story",
		},
	}
	writeConfig(t, worktreeTpgDir, config)

	// Act: Load from single path
	merged, err := LoadMergedConfigWithPaths(filepath.Join(worktreeTpgDir, ConfigFile))

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	if merged.Prefixes.Task != "only-task" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "only-task")
	}
	if merged.Prefixes.Epic != "only-epic" {
		t.Errorf("Epic prefix = %q, want %q", merged.Prefixes.Epic, "only-epic")
	}
	if merged.DefaultProject != "only-project" {
		t.Errorf("DefaultProject = %q, want %q", merged.DefaultProject, "only-project")
	}
	if merged.IDLength != 5 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 5)
	}
	if merged.CustomPrefixes["story"] != "only-story" {
		t.Errorf("story prefix = %q, want %q", merged.CustomPrefixes["story"], "only-story")
	}
}

// TestLoadMergedConfig_NoPaths_ReturnsDefaults tests that no paths returns defaults
func TestLoadMergedConfig_NoPaths_ReturnsDefaults(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	setupTpgDir(t, dir)
	chdir(t, dir)

	// Act: Load with no paths
	merged, err := LoadMergedConfigWithPaths()

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// Should have defaults
	if merged.Prefixes.Task != DefaultTaskPrefix {
		t.Errorf("Task prefix = %q, want default %q", merged.Prefixes.Task, DefaultTaskPrefix)
	}
	if merged.Prefixes.Epic != DefaultEpicPrefix {
		t.Errorf("Epic prefix = %q, want default %q", merged.Prefixes.Epic, DefaultEpicPrefix)
	}
	if merged.IDLength != 3 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 3)
	}
}

// TestLoadMergedConfig_PrefixOverride tests that later configs override earlier ones
func TestLoadMergedConfig_PrefixOverride(t *testing.T) {
	// Arrange
	systemDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: base prefix
	systemConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "sys-task",
		},
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// Worktree config: overrides prefix
	worktreeConfig := &Config{
		Prefixes: PrefixConfig{
			Task: "work-task",
		},
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// Worktree task should override system
	if merged.Prefixes.Task != "work-task" {
		t.Errorf("Task prefix = %q, want %q", merged.Prefixes.Task, "work-task")
	}
}

// TestLoadMergedConfig_IDLengthZeroNotOverride tests that IDLength 0 doesn't override
func TestLoadMergedConfig_IDLengthZeroNotOverride(t *testing.T) {
	// Arrange
	systemDir := t.TempDir()
	worktreeDir := t.TempDir()
	setupTpgDir(t, systemDir)
	worktreeTpgDir := setupTpgDir(t, worktreeDir)
	chdir(t, worktreeDir)

	// System config: has IDLength
	systemConfig := &Config{
		IDLength: 8,
	}
	writeConfig(t, filepath.Join(systemDir, ".tpg"), systemConfig)

	// Worktree config: IDLength 0 (should not override)
	worktreeConfig := &Config{
		IDLength: 0,
	}
	writeConfig(t, worktreeTpgDir, worktreeConfig)

	// Act
	merged, err := LoadMergedConfigWithPaths(
		filepath.Join(systemDir, ".tpg", ConfigFile),
		filepath.Join(worktreeTpgDir, ConfigFile),
	)

	// Assert
	if err != nil {
		t.Fatalf("LoadMergedConfigWithPaths() error = %v", err)
	}

	// IDLength should be from system (0 doesn't override)
	if merged.IDLength != 8 {
		t.Errorf("IDLength = %d, want %d", merged.IDLength, 8)
	}
}
