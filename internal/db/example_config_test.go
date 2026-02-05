package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleConfig_FromTask(t *testing.T) {
	// This test verifies config loading with custom_prefixes field.
	// Custom prefixes are no longer used but config should still load without error.
	dir := t.TempDir()
	tpgDir := filepath.Join(dir, ".tpg")
	if err := os.MkdirAll(tpgDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg dir: %v", err)
	}

	configJSON := `{
  "prefixes": {
    "task": "ts",
    "epic": "ep"
  },
  "custom_prefixes": {
    "story": "st",
    "bug": "bg"
  },
  "default_project": "testproj"
}`

	configPath := filepath.Join(tpgDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify standard prefixes loaded correctly
	if config.Prefixes.Task != "ts" {
		t.Errorf("Expected task prefix 'ts', got %q", config.Prefixes.Task)
	}

	if config.Prefixes.Epic != "ep" {
		t.Errorf("Expected epic prefix 'ep', got %q", config.Prefixes.Epic)
	}

	// Note: The custom_prefixes field in JSON is silently ignored.
	// This ensures backward compatibility with existing config files.

	t.Log("✅ Config with custom_prefixes field loads correctly (custom prefixes ignored)")
}
