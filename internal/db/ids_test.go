package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

// setupTestProject creates a temp dir with .tpg directory and optional config.
// It changes the working directory to the temp dir.
// Returns cleanup function to restore original directory.
func setupTestProject(t *testing.T, config *Config) func() {
	t.Helper()

	dir := t.TempDir()
	tpgDir := filepath.Join(dir, ".tpg")
	if err := os.MkdirAll(tpgDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg dir: %v", err)
	}

	if config != nil {
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}
		configPath := filepath.Join(tpgDir, "config.json")
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	return func() {
		_ = os.Chdir(oldWd)
	}
}

// idFormatRegexp matches IDs in the format "prefix-Nalphanumchars"
var idFormatRegexp = regexp.MustCompile(fmt.Sprintf(`^[a-zA-Z]+-[0-9a-z]{%d}$`, model.DefaultIDLength))

func TestGenerateItemIDStatic_UsesConfiguredTaskPrefix(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "task",
			Epic: "epic",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	id, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic failed: %v", err)
	}

	if !regexp.MustCompile(`^task-[0-9a-z]{3}$`).MatchString(id) {
		t.Errorf("expected ID to match 'task-XXX', got %q", id)
	}
}

func TestGenerateItemIDStatic_UsesConfiguredEpicPrefix(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "task",
			Epic: "epic",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	id, err := GenerateItemIDStatic(model.ItemTypeEpic)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic failed: %v", err)
	}

	if !regexp.MustCompile(`^epic-[0-9a-z]{3}$`).MatchString(id) {
		t.Errorf("expected ID to match 'epic-XXX', got %q", id)
	}
}

func TestGenerateItemIDStatic_UsesDefaultsWhenNoConfig(t *testing.T) {
	// Setup project without config file (nil config)
	cleanup := setupTestProject(t, nil)
	defer cleanup()

	taskID, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic for task failed: %v", err)
	}

	epicID, err := GenerateItemIDStatic(model.ItemTypeEpic)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic for epic failed: %v", err)
	}

	// Should use default prefixes: ts for task, ep for epic
	if !regexp.MustCompile(`^ts-[0-9a-z]{3}$`).MatchString(taskID) {
		t.Errorf("expected task ID to match 'ts-XXX', got %q", taskID)
	}

	if !regexp.MustCompile(`^ep-[0-9a-z]{3}$`).MatchString(epicID) {
		t.Errorf("expected epic ID to match 'ep-XXX', got %q", epicID)
	}
}

func TestGenerateItemIDStatic_UsesCustomPrefixesFromInitProject(t *testing.T) {
	dir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Initialize project with custom prefixes
	_, err = InitProject("myTask", "myEpic")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	taskID, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic for task failed: %v", err)
	}

	epicID, err := GenerateItemIDStatic(model.ItemTypeEpic)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic for epic failed: %v", err)
	}

	if !regexp.MustCompile(`^myTask-[0-9a-z]{3}$`).MatchString(taskID) {
		t.Errorf("expected task ID to match 'myTask-XXX', got %q", taskID)
	}

	if !regexp.MustCompile(`^myEpic-[0-9a-z]{3}$`).MatchString(epicID) {
		t.Errorf("expected epic ID to match 'myEpic-XXX', got %q", epicID)
	}
}

func TestGenerateItemIDStatic_CorrectFormat(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "t",
			Epic: "e",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	tests := []struct {
		name     string
		itemType model.ItemType
		prefix   string
	}{
		{"task ID format", model.ItemTypeTask, "t"},
		{"epic ID format", model.ItemTypeEpic, "e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := GenerateItemIDStatic(tt.itemType)
			if err != nil {
				t.Fatalf("GenerateItemIDStatic failed: %v", err)
			}

			// Check general format: prefix-Nalphanumchars
			if !idFormatRegexp.MatchString(id) {
				t.Errorf("ID %q does not match expected format 'prefix-XXX'", id)
			}

			// Check specific prefix
			expectedPattern := regexp.MustCompile(`^` + tt.prefix + `-[0-9a-z]{3}$`)
			if !expectedPattern.MatchString(id) {
				t.Errorf("expected ID to start with %q-, got %q", tt.prefix, id)
			}

			// Verify correct number of alphanumeric chars after the dash
			parts := regexp.MustCompile(`-`).Split(id, 2)
			if len(parts) != 2 {
				t.Fatalf("ID %q should have exactly one dash", id)
			}
			if len(parts[1]) != model.DefaultIDLength {
				t.Errorf("expected %d alphanumeric chars after dash, got %d in %q", model.DefaultIDLength, len(parts[1]), id)
			}
		})
	}
}

func TestGenerateItemIDStatic_UniqueIDs(t *testing.T) {
	cleanup := setupTestProject(t, nil)
	defer cleanup()

	ids := make(map[string]bool)
	// With 36^3 = 46656 possible values, keep iterations low to avoid
	// birthday-paradox collisions (~10% at 100, ~0.4% at 20).
	const iterations = 20

	for i := 0; i < iterations; i++ {
		id, err := GenerateItemIDStatic(model.ItemTypeTask)
		if err != nil {
			t.Fatalf("GenerateItemIDStatic failed on iteration %d: %v", i, err)
		}

		if ids[id] {
			t.Errorf("duplicate ID generated: %q", id)
		}
		ids[id] = true
	}
}

func TestGenerateItemIDStatic_NormalizesTrailingDash(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "task-", // trailing dash should be normalized
			Epic: "epic-",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	taskID, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic failed: %v", err)
	}

	// Should NOT have double dash (task--xxx)
	if regexp.MustCompile(`--`).MatchString(taskID) {
		t.Errorf("ID should not have double dash, got %q", taskID)
	}

	// Should match normalized format
	if !regexp.MustCompile(`^task-[0-9a-z]{3}$`).MatchString(taskID) {
		t.Errorf("expected ID to match 'task-XXX', got %q", taskID)
	}
}

func TestGenerateItemIDStatic_PartialConfig(t *testing.T) {
	// Config with only task prefix, epic should default
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "custom",
			// Epic not set - should default to "ep"
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	taskID, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic for task failed: %v", err)
	}

	epicID, err := GenerateItemIDStatic(model.ItemTypeEpic)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic for epic failed: %v", err)
	}

	if !regexp.MustCompile(`^custom-[0-9a-z]{3}$`).MatchString(taskID) {
		t.Errorf("expected task ID to match 'custom-XXX', got %q", taskID)
	}

	if !regexp.MustCompile(`^ep-[0-9a-z]{3}$`).MatchString(epicID) {
		t.Errorf("expected epic ID to default to 'ep-XXX', got %q", epicID)
	}
}

func TestGenerateItemIDStatic_ErrorWithoutTpgDir(t *testing.T) {
	dir := t.TempDir()

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// No .tpg directory exists
	_, err = GenerateItemIDStatic(model.ItemTypeTask)
	if err == nil {
		t.Error("expected error when .tpg directory does not exist")
	}
}

func TestGenerateItemIDStatic_CustomIDLength(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		IDLength: 5,
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	id, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic failed: %v", err)
	}

	if !regexp.MustCompile(`^ts-[0-9a-z]{5}$`).MatchString(id) {
		t.Errorf("expected ID to match 'ts-XXXXX' (5 chars), got %q", id)
	}
}
