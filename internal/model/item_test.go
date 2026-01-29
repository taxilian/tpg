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
	// With 36^3 = 46656 possible values and 100 iterations,
	// collision probability is low enough for a smoke test.
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
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
		{ItemType("invalid"), false},
		{ItemType("Task"), false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.itemType), func(t *testing.T) {
			if got := tt.itemType.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
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
