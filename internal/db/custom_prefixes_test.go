package db

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

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

func TestCustomPrefixes_GenerateID_UsesCustomPrefix(t *testing.T) {
	// Create a minimal test that verifies the config structure works
	// Note: GenerateItemIDStatic only uses task/epic types, not custom types
	// This test verifies the config loads correctly with custom prefixes
	config := &Config{
		Prefixes: PrefixConfig{
			Task: "customtask",
			Epic: "customepic",
		},
		CustomPrefixes: map[string]string{
			"story": "st",
		},
	}
	cleanup := setupTestProject(t, config)
	defer cleanup()

	// Verify standard types use the configured prefixes
	taskID, err := GenerateItemIDStatic(model.ItemTypeTask)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic failed: %v", err)
	}

	if !regexp.MustCompile(`^customtask-[0-9a-z]{3}$`).MatchString(taskID) {
		t.Errorf("Expected task ID to match 'customtask-XXX', got %q", taskID)
	}

	epicID, err := GenerateItemIDStatic(model.ItemTypeEpic)
	if err != nil {
		t.Fatalf("GenerateItemIDStatic failed: %v", err)
	}

	if !regexp.MustCompile(`^customepic-[0-9a-z]{3}$`).MatchString(epicID) {
		t.Errorf("Expected epic ID to match 'customepic-XXX', got %q", epicID)
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
