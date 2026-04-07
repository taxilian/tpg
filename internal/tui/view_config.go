package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/db"
	"strings"
)

func (m Model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle editing mode
	if m.configEditing {
		switch msg.String() {
		case "esc":
			m.configEditing = false
			m.inputText = ""
			return m, nil
		case "enter":
			// Save the value
			if m.configCursor < len(m.configFields) {
				field := m.configFields[m.configCursor]
				config, err := db.LoadConfig()
				if err != nil {
					m.err = err
					m.configEditing = false
					return m, nil
				}
				if err := db.SetConfigField(config, field.Path, m.inputText); err != nil {
					m.err = err
					m.configEditing = false
					return m, nil
				}
				if err := db.SaveConfig(config); err != nil {
					m.err = err
					m.configEditing = false
					return m, nil
				}
				m.message = fmt.Sprintf("Set %s = %s", field.Path, m.inputText)
				m.configEditing = false
				m.inputText = ""
				return m, m.loadConfig()
			}
			return m, nil
		case "backspace":
			if len(m.inputText) > 0 {
				m.inputText = m.inputText[:len(m.inputText)-1]
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.inputText += msg.String()
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		m.viewMode = ViewList
		m.configCursor = 0
		return m, nil

	case "up", "k":
		if m.configCursor > 0 {
			m.configCursor--
		}

	case "down", "j":
		if m.configCursor < len(m.configFields)-1 {
			m.configCursor++
		}

	case "g", "home":
		m.configCursor = 0
		m.configScroll = 0

	case "G", "end":
		m.configCursor = max(0, len(m.configFields)-1)

	case "enter", "e":
		// Start editing the current field
		if m.configCursor < len(m.configFields) {
			field := m.configFields[m.configCursor]
			// Don't allow editing map fields directly
			if field.Type == "map" {
				m.message = "Cannot edit map fields directly; edit config.json"
				return m, nil
			}
			m.configEditing = true
			// Pre-fill with current value
			if field.Value != nil {
				m.inputText = fmt.Sprintf("%v", field.Value)
			} else {
				m.inputText = ""
			}
		}

	case "r":
		return m, m.loadConfig()
	}

	m.syncConfigScroll()
	return m, nil
}

func (m Model) configView() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Configuration"))
	b.WriteString(fmt.Sprintf("  %d settings\n\n", len(m.configFields)))

	// Config fields
	if len(m.configFields) == 0 {
		b.WriteString("No configuration found\n")
		b.WriteString(dimStyle.Render("Run 'tpg init' to create a config file\n"))
	} else {
		visibleHeight := m.height - 8
		if visibleHeight < 5 {
			visibleHeight = 5
		}
		start, end := calculateScrollRange(m.configCursor, len(m.configFields), visibleHeight, &m.configScroll)

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 40 {
			rowWidth = 40
		}

		for i := start; i < end; i++ {
			field := m.configFields[i]
			selected := i == m.configCursor

			// Format value for display
			valueStr := db.FormatConfigValue(field.Value)

			// Format: path = value (type)
			line := fmt.Sprintf("%-35s = %-20s", field.Path, valueStr)
			if field.Type != "" && field.Type != "string" {
				line += " " + dimStyle.Render("("+field.Type+")")
			}

			// Truncate to fit width
			if len(line) > rowWidth {
				line = line[:rowWidth-3] + "..."
			}

			if selected {
				if m.configEditing {
					// Show edit mode
					editLine := fmt.Sprintf("%-35s = ", field.Path)
					b.WriteString(editLine)
					b.WriteString(inputStyle.Render(m.inputText + "█"))
					b.WriteString("\n")
				} else {
					b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
					b.WriteString("\n")
				}
			} else {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}

	// Footer
	b.WriteString("\n")
	if m.configEditing {
		b.WriteString(helpStyle.Render("enter:save  esc:cancel"))
	} else {
		b.WriteString(helpStyle.Render("j/k:nav  enter/e:edit  r:refresh  esc:back  q:quit"))
	}

	return b.String()
}
