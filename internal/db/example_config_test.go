package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleConfig_FromTask(t *testing.T) {
	// This test uses the exact example config from the task
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

	// Verify custom prefixes loaded
	if config.CustomPrefixes == nil {
		t.Fatal("CustomPrefixes should not be nil")
	}

	if config.CustomPrefixes["story"] != "st" {
		t.Errorf("Expected story prefix 'st', got %q", config.CustomPrefixes["story"])
	}

	if config.CustomPrefixes["bug"] != "bg" {
		t.Errorf("Expected bug prefix 'bg', got %q", config.CustomPrefixes["bug"])
	}

	// Verify GetPrefixForType works with custom prefixes
	if prefix := config.GetPrefixForType("story"); prefix != "st" {
		t.Errorf("Expected GetPrefixForType('story') = 'st', got %q", prefix)
	}

	if prefix := config.GetPrefixForType("bug"); prefix != "bg" {
		t.Errorf("Expected GetPrefixForType('bug') = 'bg', got %q", prefix)
	}

	// Verify standard types still work
	if prefix := config.GetPrefixForType("task"); prefix != "ts" {
		t.Errorf("Expected GetPrefixForType('task') = 'ts', got %q", prefix)
	}

	if prefix := config.GetPrefixForType("epic"); prefix != "ep" {
		t.Errorf("Expected GetPrefixForType('epic') = 'ep', got %q", prefix)
	}

	t.Log("✅ Example config from task works correctly!")
}
