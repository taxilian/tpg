package tui

import (
	"testing"

	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
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
		{model.StatusInProgress, "active"},
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
		{model.StatusInProgress, iconInProgress, "active"},
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

func TestGetUnusedVariables(t *testing.T) {
	tests := []struct {
		name       string
		tmplDesc   string
		stepDesc   string
		vars       map[string]string
		wantUnused []string
	}{
		{
			name:       "all variables used",
			tmplDesc:   "Hello {{.name}}, your age is {{.age}}",
			vars:       map[string]string{"name": "John", "age": "30"},
			wantUnused: nil,
		},
		{
			name:       "one unused variable",
			tmplDesc:   "Hello {{.name}}",
			vars:       map[string]string{"name": "John", "extra": "unused"},
			wantUnused: []string{"extra"},
		},
		{
			name:       "variable in conditional",
			tmplDesc:   "{{if .show}}Visible{{end}}",
			vars:       map[string]string{"show": "true", "hidden": "value"},
			wantUnused: []string{"hidden"},
		},
		{
			name:       "no variables in template",
			tmplDesc:   "Static text",
			vars:       map[string]string{"unused1": "a", "unused2": "b"},
			wantUnused: []string{"unused1", "unused2"},
		},
		{
			name:       "empty vars",
			tmplDesc:   "Hello {{.name}}",
			vars:       map[string]string{},
			wantUnused: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock template
			tmpl := &templates.Template{
				Description: tt.tmplDesc,
				Steps: []templates.Step{
					{Description: tt.stepDesc},
				},
			}

			got := getUnusedVariables(tmpl, tt.vars, nil)

			// Check if we got the expected unused variables
			if tt.wantUnused == nil {
				if got != nil && len(got) > 0 {
					t.Errorf("getUnusedVariables() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.wantUnused) {
				t.Errorf("getUnusedVariables() returned %d vars, want %d", len(got), len(tt.wantUnused))
				return
			}

			for _, name := range tt.wantUnused {
				if _, ok := got[name]; !ok {
					t.Errorf("getUnusedVariables() missing expected unused var %q", name)
				}
			}
		})
	}
}

func TestGetTemplateInfo(t *testing.T) {
	// Test with no template
	item := model.Item{}
	info := getTemplateInfo(item)
	if info.name != "" {
		t.Errorf("getTemplateInfo() for non-templated item should have empty name")
	}

	// Test with non-existent template
	item = model.Item{TemplateID: "nonexistent-template-xyz"}
	info = getTemplateInfo(item)
	if !info.notFound {
		t.Errorf("getTemplateInfo() for non-existent template should set notFound=true")
	}
	if info.name != "nonexistent-template-xyz" {
		t.Errorf("getTemplateInfo() should preserve template name even when not found")
	}
}

func TestShowStatusMenu(t *testing.T) {
	// Test that showStatusMenu sets the correct state
	m := Model{
		filtered: []model.Item{{ID: "test-1", Title: "Test Item"}},
	}

	// Test showing menu with different cursor positions
	tests := []struct {
		cursor int
		name   string
	}{
		{0, "start"},
		{1, "done"},
		{2, "block"},
		{3, "cancel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultModel, _ := m.showStatusMenu(tt.cursor)

			if resultModel.inputMode != InputStatusMenu {
				t.Errorf("showStatusMenu(%d) should set inputMode to InputStatusMenu", tt.cursor)
			}
			if resultModel.statusMenuCursor != tt.cursor {
				t.Errorf("showStatusMenu(%d) should set statusMenuCursor to %d, got %d", tt.cursor, tt.cursor, resultModel.statusMenuCursor)
			}
		})
	}
}

func TestShowStatusMenuNoItems(t *testing.T) {
	// Test that showStatusMenu does nothing when there are no items
	m := Model{
		filtered: []model.Item{},
	}

	resultModel, _ := m.showStatusMenu(0)

	if resultModel.inputMode == InputStatusMenu {
		t.Errorf("showStatusMenu() should not show menu when there are no items")
	}
}

func TestStatusMenuView(t *testing.T) {
	// Test that statusMenuView renders correctly
	m := Model{
		filtered:         []model.Item{{ID: "test-1", Title: "Test Item"}},
		inputMode:        InputStatusMenu,
		statusMenuCursor: 0,
	}

	view := m.statusMenuView()

	// Check that the view contains expected elements
	expectedStrings := []string{
		"Change Status",
		"[s] Start",
		"[d] Done",
		"[b] Block",
		"[c] Cancel",
	}

	for _, expected := range expectedStrings {
		if !containsString(view, expected) {
			t.Errorf("statusMenuView() should contain %q", expected)
		}
	}
}
