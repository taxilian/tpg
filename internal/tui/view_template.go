package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m Model) handleTemplateListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.templateCursor > 0 {
			m.templateCursor--
		}

	case "down", "j":
		if m.templateCursor < len(m.templates)-1 {
			m.templateCursor++
		}

	case "g", "home":
		m.templateCursor = 0
		m.templateScroll = 0

	case "G", "end":
		m.templateCursor = max(0, len(m.templates)-1)

	case "pgup", "ctrl+b":
		pageSize := m.templateVisibleHeight()
		m.templateCursor -= pageSize
		if m.templateCursor < 0 {
			m.templateCursor = 0
		}

	case "pgdown", "ctrl+f":
		pageSize := m.templateVisibleHeight()
		m.templateCursor += pageSize
		if m.templateCursor >= len(m.templates) {
			m.templateCursor = max(0, len(m.templates)-1)
		}

	case "ctrl+u":
		pageSize := m.templateVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.templateCursor -= pageSize
		if m.templateCursor < 0 {
			m.templateCursor = 0
		}

	case "ctrl+d":
		pageSize := m.templateVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.templateCursor += pageSize
		if m.templateCursor >= len(m.templates) {
			m.templateCursor = max(0, len(m.templates)-1)
		}

	case "enter", "l":
		if len(m.templates) > 0 && m.templateCursor < len(m.templates) {
			if m.selectedTemplate == nil || m.selectedTemplate.ID != m.templates[m.templateCursor].ID {
				m.templateDetailViewport.GotoTop()
			}
			m.selectedTemplate = m.templates[m.templateCursor]
			m.viewMode = ViewTemplateDetail
		}

	case "esc", "h", "backspace":
		m.viewMode = ViewList
		m.templateCursor = 0

	case "r":
		return m, m.loadTemplates()
	}

	m.syncTemplateScroll()
	return m, nil
}

func (m Model) handleTemplateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.syncTemplateDetailViewport()

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		m.templateDetailViewport.LineDown(1)
	case "k", "up":
		m.templateDetailViewport.LineUp(1)
	case "pgdown", "ctrl+f":
		m.templateDetailViewport.PageDown()
	case "pgup", "ctrl+b":
		m.templateDetailViewport.PageUp()
	case "ctrl+d":
		m.templateDetailViewport.HalfPageDown()
	case "ctrl+u":
		m.templateDetailViewport.HalfPageUp()
	case "home":
		m.templateDetailViewport.GotoTop()
	case "end":
		m.templateDetailViewport.GotoBottom()
	case "esc", "h", "backspace":
		m.viewMode = ViewTemplateList
		m.selectedTemplate = nil
	}

	return m, nil
}

func (m Model) templateListView() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Templates"))
	b.WriteString(fmt.Sprintf("  %d templates\n\n", len(m.templates)))

	// Templates
	if len(m.templates) == 0 {
		b.WriteString("No templates found\n")
		b.WriteString(dimStyle.Render("Templates are loaded from .tpg/templates/\n"))
	} else {
		visibleHeight := m.height - 8
		if visibleHeight < 5 {
			visibleHeight = 5
		}
		start, end := calculateScrollRange(m.templateCursor, len(m.templates), visibleHeight, &m.templateScroll)

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 40 {
			rowWidth = 40
		}

		for i := start; i < end; i++ {
			tmpl := m.templates[i]
			selected := i == m.templateCursor

			// Format: id - title (description preview)
			line := fmt.Sprintf("%s  %s", tmpl.ID, tmpl.Title)
			if len(tmpl.Description) > 0 {
				desc := tmpl.Description
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}
				line += " - " + desc
			}

			// Truncate to fit width
			if len(line) > rowWidth {
				line = line[:rowWidth-3] + "..."
			}

			if selected {
				b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k:nav  enter:detail  r:refresh  esc:back  q:quit"))

	return b.String()
}

func (m *Model) syncTemplateDetailViewport() {
	content := m.templateDetailContent()
	if content == "" {
		return
	}
	setViewportContent(&m.templateDetailViewport, m.width, templateDetailViewportHeight(m.height), content)
}

func (m Model) templateDetailContent() string {
	if m.selectedTemplate == nil {
		return "No template selected"
	}

	tmpl := m.selectedTemplate
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(tmpl.Title) + "\n\n")

	// Basic info
	b.WriteString(detailLabelStyle.Render("ID:          ") + tmpl.ID + "\n")
	if tmpl.Source != "" {
		b.WriteString(detailLabelStyle.Render("Source:      ") + tmpl.Source + "\n")
	}
	b.WriteString("\n")

	// Description
	if tmpl.Description != "" {
		b.WriteString(detailLabelStyle.Render("Description:") + "\n")
		b.WriteString(tmpl.Description + "\n\n")
	}

	// Variables
	if len(tmpl.Variables) > 0 {
		b.WriteString(detailLabelStyle.Render("Variables:") + "\n")
		for name, v := range tmpl.Variables {
			optional := ""
			if v.Optional {
				optional = dimStyle.Render(" (optional)")
			}
			b.WriteString("  " + labelStyle.Render(name) + optional + "\n")
			if v.Description != "" {
				b.WriteString("    " + dimStyle.Render(v.Description) + "\n")
			}
			if v.Default != "" {
				b.WriteString("    " + dimStyle.Render("Default: "+v.Default) + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Steps
	if len(tmpl.Steps) > 0 {
		b.WriteString(detailLabelStyle.Render("Steps:") + "\n")
		for i, step := range tmpl.Steps {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step.Title))
			if len(step.Depends) > 0 {
				b.WriteString("     " + dimStyle.Render("Depends: "+strings.Join(step.Depends, ", ")) + "\n")
			}
		}
	}

	return b.String()
}

func (m Model) templateDetailView() string {
	if m.selectedTemplate == nil {
		return "No template selected"
	}

	vp := m.templateDetailViewport
	setViewportContent(&vp, m.width, templateDetailViewportHeight(m.height), m.templateDetailContent())
	return vp.View() + "\n" + helpStyle.Render("j/k:scroll  pgup/dn:page  home/end  esc:back  q:quit")
}
