package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/db"
	"strings"
)

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	switch m.inputMode {
	case InputTextarea:
		b.WriteString(m.textareaView())
	case InputStatusMenu:
		b.WriteString(m.statusMenuView())
	default:
		switch m.viewMode {
		case ViewList:
			b.WriteString(m.listView())
		case ViewDetail:
			b.WriteString(m.detailView())
		case ViewGraph:
			b.WriteString(m.graphView())
		case ViewTemplateList:
			b.WriteString(m.templateListView())
		case ViewTemplateDetail:
			b.WriteString(m.templateDetailView())
		case ViewConfig:
			b.WriteString(m.configView())
		case ViewCreateWizard:
			b.WriteString(m.createWizardView())
		case ViewVariablePicker:
			b.WriteString(m.variablePickerView())
		}

		// Input line (for non-textarea input modes)
		if m.inputMode != InputNone {
			b.WriteString("\n")
			b.WriteString(m.promptOverlayView())
		}
	}

	// Status message
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	} else if m.message != "" {
		b.WriteString("\n")
		b.WriteString(messageStyle.Render(m.message))
	}

	// Apply padding to entire content
	padStyle := lipgloss.NewStyle().
		PaddingLeft(contentPadding).
		PaddingRight(contentPadding).
		PaddingTop(1)

	return padStyle.Render(b.String())
}

// renderBaseView renders the current view without input overlays.
func (m Model) renderBaseView() string {
	switch m.viewMode {
	case ViewList:
		return m.listView()
	case ViewDetail:
		return m.detailView()
	case ViewGraph:
		return m.graphView()
	case ViewTemplateList:
		return m.templateListView()
	case ViewTemplateDetail:
		return m.templateDetailView()
	case ViewConfig:
		return m.configView()
	case ViewCreateWizard:
		return m.wizardPopupBase()
	default:
		return ""
	}
}

// textareaView renders the textarea editing view.
func (m Model) textareaView() string {
	var b strings.Builder

	// Header showing what we're editing
	var title string
	treeNodes := m.buildTree()
	if m.textareaTarget == "description" {
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			title = fmt.Sprintf("Editing description for %s", item.ID)
		} else {
			title = "Editing description"
		}
	} else if varName, ok := strings.CutPrefix(m.textareaTarget, "var:"); ok {
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			title = fmt.Sprintf("Editing variable '%s' for %s", varName, item.ID)
		} else {
			title = fmt.Sprintf("Editing variable '%s'", varName)
		}
	}

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Textarea
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n")

	// Help text
	b.WriteString(m.helpView())

	return b.String()
}

// statusMenuView renders the status change confirmation menu.
func (m Model) statusMenuView() string {
	// Get current item info for context
	var itemInfo string
	treeNodes := m.buildTree()
	if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
		item := treeNodes[m.cursor].Item
		itemInfo = fmt.Sprintf("%s: %s", item.ID, item.Title)
		if len(itemInfo) > 50 {
			itemInfo = itemInfo[:47] + "..."
		}
	}

	// Menu options
	options := []struct {
		key    string
		label  string
		status string
	}{
		{"s", "Start", "in_progress"},
		{"d", "Done", "done"},
		{"b", "Block", "blocked"},
		{"c", "Cancel", "canceled"},
	}

	var menuContent strings.Builder
	if itemInfo != "" {
		menuContent.WriteString(dimStyle.Render(itemInfo) + "\n\n")
	}
	for i, opt := range options {
		prefix := "  "
		if i == m.statusMenuCursor {
			prefix = "▸ "
		}

		line := fmt.Sprintf("%s[%s] %s", prefix, opt.key, opt.label)
		if opt.status != "" {
			line += " " + dimStyle.Render("("+opt.status+")")
		}

		if i == m.statusMenuCursor {
			menuContent.WriteString(selectedRowStyle.Render(line) + "\n")
		} else {
			menuContent.WriteString(line + "\n")
		}
	}

	menuContent.WriteString("\n")
	menuContent.WriteString(m.helpViewWidth(46))

	return m.renderPopup("Change Status", menuContent.String(), 50)
}

// Run starts the TUI with the given project filter.
func Run(database *db.DB, project string) error {
	m := New(database, project)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
