package model

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		itemType ItemType
		prefix   string
	}{
		{ItemTypeTask, "ts-"},
		{ItemTypeEpic, "ep-"},
	}

	for _, tt := range tests {
		t.Run(string(tt.itemType), func(t *testing.T) {
			id := GenerateID(tt.itemType)

			if !strings.HasPrefix(id, tt.prefix) {
				t.Errorf("expected prefix %q, got %q", tt.prefix, id)
			}

			// Should be prefix (3 chars) + DefaultIDLength alphanumeric chars
			expectedLen := len(tt.prefix) + DefaultIDLength
			if len(id) != expectedLen {
				t.Errorf("expected length %d, got %d (%q)", expectedLen, len(id), id)
			}
		})
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	// With 36^3 = 46656 possible values, keep iterations low to
	// avoid birthday-paradox collisions (~10% at 100, ~0.4% at 20).
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		id := GenerateID(ItemTypeTask)
		if seen[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestItemType_IsValid(t *testing.T) {
	tests := []struct {
		itemType ItemType
		valid    bool
	}{
		{ItemTypeTask, true},
		{ItemTypeEpic, true},
		{ItemType("task"), true},
		{ItemType("epic"), true},
		{ItemType(""), false},
		{ItemType("invalid"), false}, // only task/epic are valid
		{ItemType("Task"), false},    // case sensitive
		{ItemType("bug"), false},     // only task/epic are valid
		{ItemType("story"), false},   // only task/epic are valid
	}

	for _, tt := range tests {
		t.Run(string(tt.itemType), func(t *testing.T) {
			if got := tt.itemType.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// TestItemType_RestrictedToTaskEpic explicitly tests the type restriction behavior
// that was introduced to replace arbitrary type support.
func TestItemType_RestrictedToTaskEpic(t *testing.T) {
	// Valid types (should return true)
	validTypes := []ItemType{ItemTypeTask, ItemTypeEpic, "task", "epic"}
	for _, typ := range validTypes {
		if !typ.IsValid() {
			t.Errorf("ItemType(%q).IsValid() = false, want true", typ)
		}
	}

	// Invalid types - these were previously allowed but are now restricted
	invalidTypes := []ItemType{"story", "bug", "feature", "chore", "spike", "Task", "Epic", "TASK"}
	for _, typ := range invalidTypes {
		if typ.IsValid() {
			t.Errorf("ItemType(%q).IsValid() = true, want false (type restriction)", typ)
		}
	}
}

// TestGenerateID_HardcodedPrefixes verifies that ID generation uses hardcoded
// prefixes (ts- for task, ep- for epic) and doesn't depend on config.
func TestGenerateID_HardcodedPrefixes(t *testing.T) {
	// Task IDs should always start with ts-
	for i := 0; i < 5; i++ {
		id := GenerateID(ItemTypeTask)
		if !strings.HasPrefix(id, "ts-") {
			t.Errorf("Task ID %q doesn't have expected prefix 'ts-'", id)
		}
	}

	// Epic IDs should always start with ep-
	for i := 0; i < 5; i++ {
		id := GenerateID(ItemTypeEpic)
		if !strings.HasPrefix(id, "ep-") {
			t.Errorf("Epic ID %q doesn't have expected prefix 'ep-'", id)
		}
	}
}

// TestGenerateIDWithPrefixN_IgnoresEmptyPrefix verifies that when prefix is empty,
// the function falls back to hardcoded prefixes based on item type.
func TestGenerateIDWithPrefixN_IgnoresEmptyPrefix(t *testing.T) {
	// Empty prefix should use hardcoded defaults
	taskID := GenerateIDWithPrefixN("", ItemTypeTask, 3)
	if !strings.HasPrefix(taskID, "ts-") {
		t.Errorf("Task ID with empty prefix %q doesn't have expected 'ts-' prefix", taskID)
	}

	epicID := GenerateIDWithPrefixN("", ItemTypeEpic, 3)
	if !strings.HasPrefix(epicID, "ep-") {
		t.Errorf("Epic ID with empty prefix %q doesn't have expected 'ep-' prefix", epicID)
	}

	// Whitespace-only prefix should also use defaults
	taskID2 := GenerateIDWithPrefixN("  ", ItemTypeTask, 3)
	if !strings.HasPrefix(taskID2, "ts-") {
		t.Errorf("Task ID with whitespace prefix %q doesn't have expected 'ts-' prefix", taskID2)
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		status Status
		valid  bool
	}{
		{StatusOpen, true},
		{StatusInProgress, true},
		{StatusBlocked, true},
		{StatusDone, true},
		{Status("open"), true},
		{Status("in_progress"), true},
		{Status(""), false},
		{Status("invalid"), false},
		{Status("Open"), false}, // case sensitive
		{Status("pending"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}
