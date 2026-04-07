package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/model"
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
	m.syncVarPickerViewport(item, varNames)
	return m, nil
}

func (m *Model) syncVarPickerViewport(item model.Item, varNames []string) {
	setViewportContent(&m.varPickerViewport, m.width, varPickerViewportHeight(m.height), m.variablePickerContent(item, varNames))
}

func (m Model) variablePickerContent(item model.Item, varNames []string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit Template Variable") + "\n\n")
	b.WriteString(dimStyle.Render("Select a variable to edit:") + "\n\n")

	for i, name := range varNames {
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

	return b.String()
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

	varNames := m.getSortedVarNames(item)
	vp := m.varPickerViewport
	configureViewport(&vp, m.width, varPickerViewportHeight(m.height))
	syncViewportToCursor(&vp, m.varCursor, len(varNames))
	setViewportContent(&vp, m.width, varPickerViewportHeight(m.height), m.variablePickerContent(item, varNames))
	return vp.View() + "\n" + m.helpView()
}
