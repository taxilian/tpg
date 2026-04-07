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
			m.configInput.SetValue(m.inputOriginal)
			m.configInput.Blur()
			return m, nil
		case "enter":
			// Save the value
			if m.configCursor < len(m.configFields) {
				field := m.configFields[m.configCursor]
				value := m.configInput.Value()
				config, err := db.LoadConfig()
				if err != nil {
					m.err = err
					m.configEditing = false
					m.configInput.Blur()
					return m, nil
				}
				if err := db.SetConfigField(config, field.Path, value); err != nil {
					m.err = err
					m.configEditing = false
					m.configInput.Blur()
					return m, nil
				}
				if err := db.SaveConfig(config); err != nil {
					m.err = err
					m.configEditing = false
					m.configInput.Blur()
					return m, nil
				}
				m.message = fmt.Sprintf("Set %s = %s", field.Path, value)
				m.configEditing = false
				m.configInput.Blur()
				return m, m.loadConfig()
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.configInput, cmd = m.configInput.Update(msg)
		return m, cmd
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
			value := ""
			if field.Value != nil {
				value = fmt.Sprintf("%v", field.Value)
			}
			m.inputOriginal = value
			m.configInput.SetValue(value)
			m.configInput.CursorEnd()
			m.configInput.Width = rowValueWidth(m.width-(contentPadding*2), field.Path)
			return m, m.configInput.Focus()
		}

	case "r":
		return m, m.loadConfig()
	}

	m.syncConfigScroll()
	m.syncConfigViewport()
	return m, nil
}

func (m *Model) syncConfigViewport() {
	setViewportContent(&m.configViewport, m.width, configViewportHeight(m.height), m.configViewContent())
}

func (m Model) configViewContent() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Configuration"))
	_, _ = fmt.Fprintf(&b, "  %d settings\n\n", len(m.configFields))

	// Config fields
	if len(m.configFields) == 0 {
		b.WriteString("No configuration found\n")
		b.WriteString(dimStyle.Render("Run 'tpg init' to create a config file\n"))
	} else {
		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 40 {
			rowWidth = 40
		}

		for i := range m.configFields {
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
					input := m.configInput
					input.Width = rowValueWidth(rowWidth, field.Path)
					b.WriteString(input.View())
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

	return b.String()
}

func (m Model) configView() string {
	vp := m.configViewport
	configureViewport(&vp, m.width, configViewportHeight(m.height))
	syncViewportToCursor(&vp, m.configCursor, len(m.configFields))
	setViewportContent(&vp, m.width, configViewportHeight(m.height), m.configViewContent())

	// Footer
	var b strings.Builder
	b.WriteString(vp.View())
	b.WriteString("\n")
	if m.configEditing {
		b.WriteString(helpStyle.Render("enter:save  esc:cancel"))
	} else {
		b.WriteString(helpStyle.Render("j/k:nav  enter/e:edit  r:refresh  esc:back  q:quit"))
	}

	return b.String()
}

func rowValueWidth(width int, _ string) int {
	valueWidth := width - 38
	if valueWidth < 20 {
		valueWidth = 20
	}
	return valueWidth
}
