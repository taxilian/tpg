package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m Model) handleVariablePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return m, nil
	}
	item := treeNodes[m.cursor].Item
	if item.TemplateID == "" || len(item.TemplateVars) == 0 {
		m.viewMode = ViewDetail
		return m, nil
	}

	varNames := m.getSortedVarNames(item)
	if m.varCursor >= len(varNames) {
		m.varCursor = max(0, len(varNames)-1)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.viewMode = ViewDetail
		m.varCursor = -1
		return m, nil
	case "j", "down":
		if m.varCursor < len(varNames)-1 {
			m.varCursor++
		}
	case "k", "up":
		if m.varCursor > 0 {
			m.varCursor--
		}
	case "enter":
		if m.varCursor >= 0 && m.varCursor < len(varNames) {
			varName := varNames[m.varCursor]
			m.viewMode = ViewDetail
			return m.startTextareaEdit("var:"+varName, item.TemplateVars[varName])
		}
	}
	m.syncVarPickerScroll(len(varNames))
	return m, nil
}

func (m Model) variablePickerView() string {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return "No item selected"
	}

	item := treeNodes[m.cursor].Item
	if item.TemplateID == "" || len(item.TemplateVars) == 0 {
		return "No template variables to edit"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Template Variable") + "\n\n")
	b.WriteString(dimStyle.Render("Select a variable to edit:") + "\n\n")

	varNames := m.getSortedVarNames(item)

	// Calculate visible area (header: 4 lines, footer: 2 lines)
	visibleHeight := m.height - 6
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	start, end := calculateScrollRange(m.varCursor, len(varNames), visibleHeight, &m.varPickerScroll)

	for i := start; i < end; i++ {
		name := varNames[i]
		value := item.TemplateVars[name]
		displayValue := value
		if len(value) > 60 {
			displayValue = value[:57] + "..."
		}
		if strings.Contains(value, "\n") {
			displayValue = strings.Split(value, "\n")[0] + "..."
		}

		if m.varCursor == i {
			line := fmt.Sprintf("▸ %s: %s", name, displayValue)
			b.WriteString(selectedRowStyle.Render(line) + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s: %s\n", labelStyle.Render(name), displayValue))
		}
	}

	b.WriteString("\n" + helpStyle.Render("j/k:nav  enter:edit  esc:back"))
	return b.String()
}
