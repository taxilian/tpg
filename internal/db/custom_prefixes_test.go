package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestProject creates a temporary project directory with config and returns a cleanup function.
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

func TestCustomPrefixes_LoadsFromConfig(t *testing.T) {
	// Create config with custom_prefixes
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		CustomPrefixes: map[string]string{
			"story": "st",
			"bug":   "bg",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	// Load config and verify custom prefixes
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedConfig.CustomPrefixes == nil {
		t.Fatal("CustomPrefixes should not be nil")
	}

	if loadedConfig.CustomPrefixes["story"] != "st" {
		t.Errorf("Expected story prefix 'st', got %q", loadedConfig.CustomPrefixes["story"])
	}

	if loadedConfig.CustomPrefixes["bug"] != "bg" {
		t.Errorf("Expected bug prefix 'bg', got %q", loadedConfig.CustomPrefixes["bug"])
	}
}

func TestCustomPrefixes_GetPrefixForType_StandardTypes(t *testing.T) {
	// Config with both standard and custom prefixes
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "mytask",
			Epic: "myepic",
		},
		CustomPrefixes: map[string]string{
			"story": "st",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Standard types should use Prefixes, not CustomPrefixes
	if prefix := loadedConfig.GetPrefixForType("task"); prefix != "mytask" {
		t.Errorf("Expected task prefix 'mytask', got %q", prefix)
	}

	if prefix := loadedConfig.GetPrefixForType("epic"); prefix != "myepic" {
		t.Errorf("Expected epic prefix 'myepic', got %q", prefix)
	}
}

func TestCustomPrefixes_GetPrefixForType_CustomTypes(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		CustomPrefixes: map[string]string{
			"story":   "st",
			"bug":     "bg",
			"feature": "ft",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Custom types should use CustomPrefixes
	if prefix := loadedConfig.GetPrefixForType("story"); prefix != "st" {
		t.Errorf("Expected story prefix 'st', got %q", prefix)
	}

	if prefix := loadedConfig.GetPrefixForType("bug"); prefix != "bg" {
		t.Errorf("Expected bug prefix 'bg', got %q", prefix)
	}

	if prefix := loadedConfig.GetPrefixForType("feature"); prefix != "ft" {
		t.Errorf("Expected feature prefix 'ft', got %q", prefix)
	}
}

func TestCustomPrefixes_GetPrefixForType_UnknownType(t *testing.T) {
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		CustomPrefixes: map[string]string{
			"story": "st",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Unknown types should return generic "it" prefix
	if prefix := loadedConfig.GetPrefixForType("unknown"); prefix != "it" {
		t.Errorf("Expected unknown type prefix 'it', got %q", prefix)
	}
}

func TestCustomPrefixes_GetPrefixForType_EmptyCustomPrefixes(t *testing.T) {
	// Config without custom_prefixes field
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Standard types should still work
	if prefix := loadedConfig.GetPrefixForType("task"); prefix != "ts" {
		t.Errorf("Expected task prefix 'ts', got %q", prefix)
	}

	// Unknown types should return "it"
	if prefix := loadedConfig.GetPrefixForType("unknown"); prefix != "it" {
		t.Errorf("Expected unknown type prefix 'it', got %q", prefix)
	}
}

func TestCustomPrefixes_JSONRoundTrip(t *testing.T) {
	// Test that custom_prefixes survives JSON marshal/unmarshal
	config := Config{
		Prefixes: PrefixConfig{
			Task: "ts",
			Epic: "ep",
		},
		CustomPrefixes: map[string]string{
			"story": "st",
			"bug":   "bg",
		},
		DefaultProject: "test",
		IDLength:       3,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Unmarshal back
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify custom prefixes survived
	if loaded.CustomPrefixes == nil {
		t.Fatal("CustomPrefixes should not be nil after round-trip")
	}

	if loaded.CustomPrefixes["story"] != "st" {
		t.Errorf("Expected story prefix 'st' after round-trip, got %q", loaded.CustomPrefixes["story"])
	}

	if loaded.CustomPrefixes["bug"] != "bg" {
		t.Errorf("Expected bug prefix 'bg' after round-trip, got %q", loaded.CustomPrefixes["bug"])
	}
}
