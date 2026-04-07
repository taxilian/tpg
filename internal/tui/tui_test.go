package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
)

func newTestModel(items ...model.Item) Model {
	m := New(nil, "")
	m.items = items
	m.filtered = items
	m.width = 120
	m.height = 30
	m.filterStatuses = map[model.Status]bool{
		model.StatusOpen:       true,
		model.StatusInProgress: true,
		model.StatusBlocked:    true,
		model.StatusDone:       true,
		model.StatusCanceled:   true,
	}
	m.applyFilters()
	return m
}

func sendTextInput(model tea.Model, text string) Model {
	m := model.(Model)
	for _, r := range text {
		updated, _ := m.handleInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	return m
}

func assertBindingRendered(t *testing.T, view string, binding key.Binding) {
	t.Helper()
	help := binding.Help()
	if !strings.Contains(view, help.Key) {
		t.Fatalf("view missing binding key %q in:\n%s", help.Key, view)
	}
	if !strings.Contains(view, help.Desc) {
		t.Fatalf("view missing binding description %q in:\n%s", help.Desc, view)
	}
}

func statusMenuLabel(binding key.Binding) string {
	desc := strings.TrimSuffix(binding.Help().Desc, " item")
	if desc == "" {
		return ""
	}
	return strings.ToUpper(desc[:1]) + desc[1:]
}

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
			want := tt.wantIcon + " " + tt.wantText
			if got != want {
				t.Errorf("formatStatus(%q) = %q, want %q", tt.status, got, want)
			}
		})
	}
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
		{
			name:       "variable used only in step description",
			tmplDesc:   "Template intro",
			stepDesc:   "Deploy to {{.env}}",
			vars:       map[string]string{"env": "prod", "extra": "unused"},
			wantUnused: []string{"extra"},
		},
		{
			name:       "variables split across template and step descriptions",
			tmplDesc:   "Hello {{.name}}",
			stepDesc:   "Deploy to {{.env}}",
			vars:       map[string]string{"name": "John", "env": "prod", "extra": "unused"},
			wantUnused: []string{"extra"},
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
				if len(got) > 0 {
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
	t.Run("no template", func(t *testing.T) {
		info := getTemplateInfo(model.Item{})
		if info.name != "" {
			t.Fatalf("getTemplateInfo() for non-templated item should have empty name")
		}
		if info.tmpl != nil || info.notFound || info.invalidStep {
			t.Fatalf("getTemplateInfo() for non-templated item returned unexpected flags: %+v", info)
		}
	})

	t.Run("missing template", func(t *testing.T) {
		info := getTemplateInfo(model.Item{TemplateID: "nonexistent-template-xyz"})
		if !info.notFound {
			t.Fatalf("getTemplateInfo() for non-existent template should set notFound=true")
		}
		if info.name != "nonexistent-template-xyz" {
			t.Fatalf("getTemplateInfo() should preserve template name even when not found")
		}
	})

	t.Run("existing template", func(t *testing.T) {
		stepIndex := 1
		info := getTemplateInfo(model.Item{TemplateID: "test-review", StepIndex: &stepIndex})
		if info.notFound {
			t.Fatalf("getTemplateInfo() unexpectedly marked valid template as missing")
		}
		if info.tmpl == nil {
			t.Fatalf("getTemplateInfo() did not load template metadata")
		}
		if info.name != "test-review" {
			t.Fatalf("getTemplateInfo() name = %q, want %q", info.name, "test-review")
		}
		if info.stepNum != 2 {
			t.Fatalf("getTemplateInfo() stepNum = %d, want 2", info.stepNum)
		}
		if info.totalSteps != len(info.tmpl.Steps) || info.totalSteps == 0 {
			t.Fatalf("getTemplateInfo() totalSteps = %d, tmpl steps = %d", info.totalSteps, len(info.tmpl.Steps))
		}
	})
}

func TestShowStatusMenu(t *testing.T) {
	m := newTestModel(model.Item{ID: "test-1", Title: "Test Item", Status: model.StatusOpen})

	tests := []struct {
		cursor int
		name   string
		want   key.Binding
	}{
		{0, "start", statusMenuBindings.Start},
		{1, "done", statusMenuBindings.Done},
		{2, "block", statusMenuBindings.Block},
		{3, "cancel", statusMenuBindings.Stop},
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

			view := resultModel.statusMenuView()
			help := tt.want.Help()
			selectedLine := "▸ [" + help.Key + "] " + statusMenuLabel(tt.want)
			if !strings.Contains(view, selectedLine) {
				t.Fatalf("statusMenuView() missing selected option %q in:\n%s", selectedLine, view)
			}
			if !strings.Contains(view, "test-1: Test Item") {
				t.Fatalf("statusMenuView() missing current item context in:\n%s", view)
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
	m := newTestModel(model.Item{ID: "test-1", Title: "Test Item", Status: model.StatusOpen})
	m.inputMode = InputStatusMenu
	m.statusMenuCursor = 0

	view := m.statusMenuView()

	if !strings.Contains(view, "Change Status") {
		t.Fatalf("statusMenuView() should contain title, got:\n%s", view)
	}

	for _, binding := range []key.Binding{statusMenuBindings.Start, statusMenuBindings.Done, statusMenuBindings.Block, statusMenuBindings.Stop} {
		help := binding.Help()
		label := "[" + help.Key + "] " + statusMenuLabel(binding)
		if !strings.Contains(view, label) {
			t.Fatalf("statusMenuView() should contain %q in:\n%s", label, view)
		}
	}

	assertBindingRendered(t, view, statusMenuBindings.Up)
	assertBindingRendered(t, view, statusMenuBindings.Down)
	assertBindingRendered(t, view, statusMenuBindings.Confirm)
	assertBindingRendered(t, view, statusMenuBindings.Cancel)
}

func TestMainViewHelpRendering(t *testing.T) {
	t.Run("list short and full help", func(t *testing.T) {
		m := newTestModel(model.Item{ID: "ts-1", Title: "Alpha", Status: model.StatusOpen})
		m.viewMode = ViewList

		shortHelp := m.helpViewWidth(160)
		assertBindingRendered(t, shortHelp, listBindings.Up)
		assertBindingRendered(t, shortHelp, listBindings.Detail)
		assertBindingRendered(t, shortHelp, listBindings.New)
		if strings.Contains(shortHelp, listBindings.AddDep.Help().Desc) {
			t.Fatalf("short help should not render full-only binding %q:\n%s", listBindings.AddDep.Help().Desc, shortHelp)
		}

		m.help.ShowAll = true
		fullHelp := m.helpViewWidth(160)
		assertBindingRendered(t, fullHelp, listBindings.AddDep)
		assertBindingRendered(t, fullHelp, listBindings.Expand)
		assertBindingRendered(t, fullHelp, appBindings.ToggleHelp)
	})

	t.Run("detail short and full help", func(t *testing.T) {
		m := newTestModel(model.Item{ID: "ts-1", Title: "Alpha", Status: model.StatusOpen, TemplateID: "simple-task"})
		m.viewMode = ViewDetail
		m.syncDetailViewport()

		shortHelp := m.helpViewWidth(160)
		assertBindingRendered(t, shortHelp, appBindings.Back)
		assertBindingRendered(t, shortHelp, detailBindings.Start)
		assertBindingRendered(t, shortHelp, detailBindings.PageDown)
		if strings.Contains(shortHelp, detailBindings.Refresh.Help().Desc) {
			t.Fatalf("short help should not render full-only binding %q:\n%s", detailBindings.Refresh.Help().Desc, shortHelp)
		}

		m.help.ShowAll = true
		fullHelp := m.helpViewWidth(160)
		assertBindingRendered(t, fullHelp, detailBindings.Refresh)
		assertBindingRendered(t, fullHelp, detailBindings.Graph)
		assertBindingRendered(t, fullHelp, detailBindings.Rerender)
	})
}

func TestPromptTextInputFlows(t *testing.T) {
	items := []model.Item{
		{ID: "ts-1", Title: "Alpha task", Status: model.StatusOpen},
		{ID: "ts-2", Title: "Beta task", Status: model.StatusOpen},
	}

	t.Run("focus update and submit search", func(t *testing.T) {
		m := newTestModel(items...)
		m, _ = m.startInput(InputSearch, "Search: ")
		if !m.searchInput.Focused() {
			t.Fatalf("search input should be focused after startInput")
		}

		m = sendTextInput(m, "Alpha")
		if m.searchInput.Value() != "Alpha" {
			t.Fatalf("searchInput value = %q, want %q", m.searchInput.Value(), "Alpha")
		}
		if m.filterSearch != "Alpha" {
			t.Fatalf("filterSearch = %q, want %q", m.filterSearch, "Alpha")
		}
		if len(m.filtered) != 1 || m.filtered[0].ID != "ts-1" {
			t.Fatalf("filtered items = %+v, want only ts-1", m.filtered)
		}

		updated, _ := m.handleInputKey(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(Model)
		if m.inputMode != InputNone {
			t.Fatalf("inputMode = %v, want InputNone", m.inputMode)
		}
		if m.searchInput.Focused() {
			t.Fatalf("search input should blur on submit")
		}
	})

	t.Run("cancel restores previous filter", func(t *testing.T) {
		m := newTestModel(items...)
		m.filterSearch = "Beta"
		m.applyFilters()
		m, _ = m.startInput(InputSearch, "Search: ")
		m = sendTextInput(m, " Alpha")

		updated, _ := m.handleInputKey(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(Model)
		if m.inputMode != InputNone {
			t.Fatalf("inputMode = %v, want InputNone", m.inputMode)
		}
		if m.filterSearch != "Beta" {
			t.Fatalf("filterSearch = %q, want restored value %q", m.filterSearch, "Beta")
		}
		if m.searchInput.Value() != "Beta" {
			t.Fatalf("searchInput value = %q, want restored value %q", m.searchInput.Value(), "Beta")
		}
	})

	t.Run("tab cycles through filter input modes", func(t *testing.T) {
		m := newTestModel(items...)
		m, _ = m.startInput(InputSearch, "Search: ")
		m = sendTextInput(m, "shared")

		updated, _ := m.handleInputKey(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.inputMode != InputProject || !m.projectInput.Focused() {
			t.Fatalf("expected project input to be focused after first tab")
		}
		if m.searchInput.Value() != "shared" {
			t.Fatalf("search input value = %q, want %q after leaving search mode", m.searchInput.Value(), "shared")
		}

		updated, _ = m.handleInputKey(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.inputMode != InputLabel || !m.labelInput.Focused() {
			t.Fatalf("expected label input to be focused after second tab")
		}
		if m.projectInput.Value() != "" {
			t.Fatalf("project input value = %q, want empty default", m.projectInput.Value())
		}

		updated, _ = m.handleInputKey(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.inputMode != InputSearch || !m.searchInput.Focused() {
			t.Fatalf("expected search input to be focused after cycling back")
		}
		if m.searchInput.Value() != "shared" {
			t.Fatalf("search input value = %q, want %q", m.searchInput.Value(), "shared")
		}
	})
}

func TestWizardTitleTextInputFlow(t *testing.T) {
	m := newTestModel()
	m.viewMode = ViewCreateWizard
	m.createWizardStep = 2
	m.createWizardState.SelectedType = model.ItemTypeTask
	_ = m.focusWizardTitleInput()
	if !m.wizardTitleInput.Focused() {
		t.Fatalf("wizard title input should be focused")
	}

	updated, _ := m.handleCreateWizardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	m = updated.(Model)
	updated, _ = m.handleCreateWizardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)
	updated, _ = m.handleCreateWizardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(Model)
	if m.createWizardState.Title != "New" {
		t.Fatalf("wizard title = %q, want %q", m.createWizardState.Title, "New")
	}

	updated, _ = m.handleCreateWizardKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.createWizardStep != 3 {
		t.Fatalf("createWizardStep = %d, want 3", m.createWizardStep)
	}
	if m.createWizardState.Title != "New" {
		t.Fatalf("wizard title = %q, want %q after continue", m.createWizardState.Title, "New")
	}
}
