package tui

import (
	"testing"

	"github.com/taxilian/tpg/internal/model"
)

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status model.Status
		want   string
	}{
		{model.StatusOpen, iconOpen},
		{model.StatusInProgress, iconInProgress},
		{model.StatusDone, iconDone},
		{model.StatusBlocked, iconBlocked},
		{model.StatusCanceled, iconCanceled},
		{model.Status("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := statusIcon(tt.status)
			if got != tt.want {
				t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusText(t *testing.T) {
	tests := []struct {
		status model.Status
		want   string
	}{
		{model.StatusOpen, "open"},
		{model.StatusInProgress, "prog"},
		{model.StatusBlocked, "block"},
		{model.StatusDone, "done"},
		{model.StatusCanceled, "cancel"},
		{model.Status("unknown"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := statusText(tt.status)
			if got != tt.want {
				t.Errorf("statusText(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	// formatStatus should return "icon text" format
	tests := []struct {
		status   model.Status
		wantIcon string
		wantText string
	}{
		{model.StatusOpen, iconOpen, "open"},
		{model.StatusInProgress, iconInProgress, "prog"},
		{model.StatusBlocked, iconBlocked, "block"},
		{model.StatusDone, iconDone, "done"},
		{model.StatusCanceled, iconCanceled, "cancel"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := formatStatus(tt.status)
			// Should contain both icon and text
			if got == "" {
				t.Errorf("formatStatus(%q) returned empty string", tt.status)
			}
			// The format should be "icon text" - check it contains the text
			wantContains := tt.wantText
			if !containsString(got, wantContains) {
				t.Errorf("formatStatus(%q) = %q, want to contain %q", tt.status, got, wantContains)
			}
		})
	}
}

// containsString checks if s contains substr (simple substring check)
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
