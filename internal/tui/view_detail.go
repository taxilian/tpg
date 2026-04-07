package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/db"
	"strings"
)

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		if m.depNavActive {
			m.depNavActive = false
			return m, nil
		}
		m.viewMode = ViewList
		m.logsVisible = false
		m.varCursor = -1

	// Log toggle and scroll
	case "v":
		m.logsVisible = !m.logsVisible
	case "j", "down":
		if m.depNavActive {
			m.depCursor++
			section := m.currentDepSection()
			if m.depCursor >= len(section) {
				m.depCursor = len(section) - 1
			}
			if m.depCursor < 0 {
				m.depCursor = 0
			}
		} else {
			m.descScroll++
		}
	case "k", "up":
		if m.depNavActive {
			if m.depCursor > 0 {
				m.depCursor--
			}
		} else if m.descScroll > 0 {
			m.descScroll--
		}
	case "pgdown":
		if !m.depNavActive {
			m.descScroll += 10
		}
	case "pgup":
		if !m.depNavActive && m.descScroll > 0 {
			m.descScroll -= 10
			if m.descScroll < 0 {
				m.descScroll = 0
			}
		}

	// Dependency navigation
	case "tab":
		if len(m.detailDeps) > 0 || len(m.detailBlocks) > 0 {
			m.depNavActive = true
			m.depSection = (m.depSection + 1) % 2
			m.depCursor = 0
			// If switching to empty section, switch back
			if len(m.currentDepSection()) == 0 {
				m.depSection = (m.depSection + 1) % 2
			}
		}
	case "enter":
		if m.depNavActive {
			section := m.currentDepSection()
			if m.depCursor < len(section) {
				targetID := section[m.depCursor].ID
				// Find the target in tree nodes
				treeNodes := m.buildTree()
				for i, node := range treeNodes {
					if node.Item.ID == targetID {
						m.cursor = i
						m.depNavActive = false
						return m, m.loadDetail()
					}
				}
				m.message = fmt.Sprintf("Item %s not in current filter", targetID)
			}
		}

	// Actions work in detail view too - show status menu for confirmation
	case "s":
		return m.showStatusMenu(0) // Start selected
	case "d":
		return m.showStatusMenu(1) // Done selected
	case "b":
		return m.showStatusMenu(2) // Block selected
	case "L":
		return m.startInput(InputLog, "Log message: ")
	case "c":
		return m.showStatusMenu(3) // Cancel selected
	case "a":
		return m.startInput(InputAddDep, "Add blocker ID: ")
	case "e":
		// Open built-in textarea editor for description or selected variable
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			// If variable cursor is active, edit that variable
			if m.varCursor >= 0 && item.TemplateID != "" && len(item.TemplateVars) > 0 {
				varNames := m.getSortedVarNames(item)
				if m.varCursor < len(varNames) {
					varName := varNames[m.varCursor]
					return m.startTextareaEdit("var:"+varName, item.TemplateVars[varName])
				}
			}
			// Otherwise edit description
			return m.startTextareaEdit("description", item.Description)
		}

	case "V":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			if item.TemplateID != "" && len(item.TemplateVars) > 0 {
				m.viewMode = ViewVariablePicker
				m.varCursor = 0
				m.varPickerScroll = 0
				return m, nil
			}
		}

	case "r":
		return m, m.loadDetail()

	case "g":
		// Enter graph view
		m.buildGraph()
		m.viewMode = ViewGraph
		return m, nil

	// Refresh stored description from rendered template
	case "R":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			if item.TemplateID != "" {
				rendered := renderTemplateForItem(item)
				return m, func() tea.Msg {
					if err := m.db.SetDescription(item.ID, rendered); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{message: fmt.Sprintf("Updated description for %s from template", item.ID)}
				}
			}
		}
	}

	return m, nil
}

// currentDepSection returns the deps for the active section.
func (m Model) currentDepSection() []db.DepStatus {
	if m.depSection == 0 {
		return m.detailDeps
	}
	return m.detailBlocks
}

func (m Model) detailView() string {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return "No item selected"
	}

	item := treeNodes[m.cursor].Item
	var b strings.Builder

	// Title with status icon
	icon := statusIcon(item.Status)
	color := statusColors[item.Status]
	iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

	// Add stale indicator to title if stale
	title := item.Title
	if m.staleItems[item.ID] {
		title = staleStyle.Render("⚠ ") + title
	}

	b.WriteString(iconStyled + " " + titleStyle.Render(title) + "\n\n")

	b.WriteString(detailLabelStyle.Render("ID:       ") + item.ID + "\n")
	b.WriteString(detailLabelStyle.Render("Type:     ") + string(item.Type) + "\n")
	b.WriteString(detailLabelStyle.Render("Project:  ") + item.Project + "\n")

	// Show "stale" status for stale items
	if m.staleItems[item.ID] {
		statusStyled := staleStyle.Render("stale [STALE]")
		b.WriteString(detailLabelStyle.Render("Status:   ") + statusStyled)
	} else {
		statusStyled := lipgloss.NewStyle().Foreground(color).Render(string(item.Status))
		b.WriteString(detailLabelStyle.Render("Status:   ") + statusStyled)
	}
	b.WriteString("\n")

	b.WriteString(detailLabelStyle.Render("Priority: ") + fmt.Sprintf("%d", item.Priority) + "\n")

	if item.ParentID != nil {
		b.WriteString(detailLabelStyle.Render("Parent:   ") + *item.ParentID + "\n")
	}

	// Agent assignment
	if item.AgentID != nil && *item.AgentID != "" {
		b.WriteString(detailLabelStyle.Render("Agent:    ") + dimStyle.Render(*item.AgentID) + "\n")
	}

	// Labels
	if len(item.Labels) > 0 {
		labelsStr := ""
		for i, lbl := range item.Labels {
			if i > 0 {
				labelsStr += " "
			}
			labelsStr += labelStyle.Render("[" + lbl + "]")
		}
		b.WriteString(detailLabelStyle.Render("Labels:   ") + labelsStr + "\n")
	}

	// Template information - always show if item has a template
	var tmplInfo templateInfo
	if item.TemplateID != "" {
		tmplInfo = getTemplateInfo(item)

		// Format: "Template: <name>" or "Template: <name>, step <n>"
		tmplLine := "Template: " + tmplInfo.name
		if tmplInfo.notFound {
			tmplLine += " " + errorStyle.Render("[NOT FOUND]")
		} else if tmplInfo.invalidStep {
			tmplLine += fmt.Sprintf(", step %d", tmplInfo.stepNum) + " " + errorStyle.Render("[INVALID STEP]")
		} else if tmplInfo.totalSteps > 1 && tmplInfo.stepNum > 0 {
			tmplLine += fmt.Sprintf(", step %d", tmplInfo.stepNum)
		}
		b.WriteString(detailLabelStyle.Render("Template: ") + tmplLine[10:] + "\n") // Skip "Template: " prefix since we use detailLabelStyle

		// Show warning messages for error cases
		if tmplInfo.notFound {
			b.WriteString(errorStyle.Render("  ⚠ Template not found - showing raw variables") + "\n")
		} else if tmplInfo.invalidStep {
			b.WriteString(errorStyle.Render(fmt.Sprintf("  ⚠ Step %d is out of range (template has %d steps) - showing raw variables", tmplInfo.stepNum, tmplInfo.totalSteps)) + "\n")
		} else if tmplInfo.tmpl != nil {
			// Check for hash mismatch
			if item.TemplateHash != "" && item.TemplateHash != tmplInfo.tmpl.Hash {
				b.WriteString("  " + errorStyle.Render("[Template has changed since instantiation]") + "\n")
			}
		}
	}

	// Dependencies — "Blocked by" (what this depends on)
	if len(m.detailDeps) > 0 {
		header := "Blocked by:"
		if m.depNavActive && m.depSection == 0 {
			header = "▸ Blocked by:"
		}
		b.WriteString("\n" + detailLabelStyle.Render(header) + "\n")
		for i, dep := range m.detailDeps {
			depIcon := depStatusIcon(dep.Status)
			selected := m.depNavActive && m.depSection == 0 && i == m.depCursor
			line := fmt.Sprintf("  %s %s %s", depIcon, dep.ID, dep.Title)
			if selected {
				b.WriteString(selectedRowStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}

	// Dependencies — "Blocks" (what this item blocks)
	if len(m.detailBlocks) > 0 {
		header := "Blocks:"
		if m.depNavActive && m.depSection == 1 {
			header = "▸ Blocks:"
		}
		b.WriteString("\n" + detailLabelStyle.Render(header) + "\n")
		for i, dep := range m.detailBlocks {
			depIcon := depStatusIcon(dep.Status)
			selected := m.depNavActive && m.depSection == 1 && i == m.depCursor
			line := fmt.Sprintf("  %s %s %s", depIcon, dep.ID, dep.Title)
			if selected {
				b.WriteString(selectedRowStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}

	// Description section
	if item.TemplateID != "" {
		descLabel := "\nDescription"
		if tmplInfo.notFound || tmplInfo.invalidStep {
			descLabel += " " + dimStyle.Render("[stored - template error]")
		}
		descLabel += ":"
		b.WriteString(detailLabelStyle.Render(descLabel) + "\n")
		b.WriteString(renderTemplateForItem(item) + "\n")

		// Template Variables
		if len(item.TemplateVars) > 0 {
			b.WriteString("\n" + detailLabelStyle.Render("Template Variables:") + " " + dimStyle.Render("(V to edit)") + "\n")
			varNames := m.getSortedVarNames(item)
			for _, name := range varNames {
				value := item.TemplateVars[name]
				displayValue := value
				if len(value) > 60 {
					displayValue = value[:57] + "..."
				}
				if strings.Contains(value, "\n") {
					displayValue = strings.Split(value, "\n")[0] + "..."
				}
				b.WriteString(fmt.Sprintf("  %s: %s\n", labelStyle.Render(name), displayValue))
			}
		}
	} else {
		// Non-templated item
		if item.Description != "" {
			b.WriteString("\n" + detailLabelStyle.Render("Description:") + "\n")
			b.WriteString(item.Description + "\n")
		}
	}

	// Logs (toggle with v)
	logCount := len(m.detailLogs)
	if logCount > 0 {
		if m.logsVisible {
			b.WriteString("\n" + detailLabelStyle.Render(fmt.Sprintf("Logs (%d):", logCount)) + " " + dimStyle.Render("v:hide") + "\n")
			for _, log := range m.detailLogs {
				ts := dimStyle.Render(log.CreatedAt.Format("2006-01-02 15:04"))
				b.WriteString("  " + ts + " " + log.Message + "\n")
			}
		} else {
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Logs: %d entries (v to show)", logCount)) + "\n")
		}
	}

	content := b.String()

	maxVisible := m.height - detailReservedRows
	if maxVisible < 10 {
		maxVisible = 10
	}
	visibleContent, totalLines := scrollText(content, m.descScroll, maxVisible)

	// Build final output
	var result strings.Builder
	result.WriteString(visibleContent)

	// Scroll indicator if needed
	if m.descScroll+maxVisible < totalLines {
		remaining := totalLines - (m.descScroll + maxVisible)
		result.WriteString(dimStyle.Render(fmt.Sprintf("\n... %d more lines (j/k to scroll)", remaining)) + "\n")
	}

	// Help line
	help := "esc:back  s:start d:done b:block L:log c:cancel a:add-dep e:edit v:logs  j/k:scroll q:quit"
	if len(m.detailDeps) > 0 || len(m.detailBlocks) > 0 {
		help += "  tab:deps"
	}
	if item.TemplateID != "" {
		help += "  V:variables"
	}
	result.WriteString(helpStyle.Render(help))

	return result.String()
}
