package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/model"
	"strings"
)

// buildGraph constructs the graph nodes from the current item's dependencies.
func (m *Model) buildGraph() {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		m.graphNodes = nil
		return
	}

	item := treeNodes[m.cursor].Item
	m.graphCurrentID = item.ID
	m.graphNodes = nil

	// Add blockers (column 0)
	for i, dep := range m.detailDeps {
		m.graphNodes = append(m.graphNodes, graphNode{
			ID:       dep.ID,
			Title:    dep.Title,
			Status:   dep.Status,
			Column:   0,
			Position: i,
		})
	}

	// Add current item (column 1)
	m.graphNodes = append(m.graphNodes, graphNode{
		ID:       item.ID,
		Title:    item.Title,
		Status:   string(item.Status),
		Column:   1,
		Position: 0,
	})

	// Add blocked items (column 2)
	for i, dep := range m.detailBlocks {
		m.graphNodes = append(m.graphNodes, graphNode{
			ID:       dep.ID,
			Title:    dep.Title,
			Status:   dep.Status,
			Column:   2,
			Position: i,
		})
	}

	m.graphCursor = len(m.detailDeps) // Start cursor on current item
}

func (m Model) handleGraphKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		m.viewMode = ViewDetail
		return m, nil

	case "j", "down":
		if m.graphCursor < len(m.graphNodes)-1 {
			m.graphCursor++
		}

	case "k", "up":
		if m.graphCursor > 0 {
			m.graphCursor--
		}

	case "enter":
		if m.graphCursor >= 0 && m.graphCursor < len(m.graphNodes) {
			targetID := m.graphNodes[m.graphCursor].ID
			treeNodes := m.buildTree()
			for i, node := range treeNodes {
				if node.Item.ID == targetID {
					m.cursor = i
					m.viewMode = ViewDetail
					return m, m.loadDetail()
				}
			}
			m.message = fmt.Sprintf("Item %s not in current filter", targetID)
		}
	}

	return m, nil
}

func (m Model) graphView() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Dependency Graph"))
	b.WriteString("\n\n")

	if len(m.graphNodes) == 0 {
		b.WriteString("No dependencies to display\n")
		b.WriteString("\n")
		b.WriteString(m.helpView())
		return b.String()
	}

	// Column headers
	b.WriteString(detailLabelStyle.Render("  Blockers"))
	b.WriteString("              ")
	b.WriteString(detailLabelStyle.Render("Current"))
	b.WriteString("               ")
	b.WriteString(detailLabelStyle.Render("Blocks"))
	b.WriteString("\n")

	// Collect nodes by column
	var blockers, current, blocked []graphNode
	for _, node := range m.graphNodes {
		switch node.Column {
		case 0:
			blockers = append(blockers, node)
		case 1:
			current = append(current, node)
		case 2:
			blocked = append(blocked, node)
		}
	}

	// Calculate max rows needed
	maxRows := max(len(blockers), max(len(current), len(blocked)))
	if maxRows == 0 {
		maxRows = 1
	}

	// Adapt column width to terminal (3 columns + 2 connectors of 6 chars each)
	contentWidth := m.width - (contentPadding * 2)
	colWidth := (contentWidth - 12) / 3
	if colWidth < 12 {
		colWidth = 12
	}
	if colWidth > 30 {
		colWidth = 30
	}
	for row := 0; row < maxRows; row++ {
		var leftPart, midPart, rightPart string
		var leftSelected, midSelected, rightSelected bool

		// Left column (blockers)
		if row < len(blockers) {
			node := blockers[row]
			icon := depStatusIcon(node.Status)
			title := node.ID
			if len(node.Title) > 0 {
				maxTitle := colWidth - len(node.ID) - 4
				if maxTitle > 0 && len(node.Title) > maxTitle {
					title = node.ID + " " + node.Title[:maxTitle-3] + "..."
				} else if maxTitle > 0 {
					title = node.ID + " " + node.Title
				}
			}
			leftPart = fmt.Sprintf("%s %s", icon, title)
			// Check if this node is selected
			for i, n := range m.graphNodes {
				if n.ID == node.ID && n.Column == 0 && i == m.graphCursor {
					leftSelected = true
					break
				}
			}
		}

		// Middle column (current)
		if row < len(current) {
			node := current[row]
			icon := depStatusIcon(node.Status)
			title := node.ID
			if len(node.Title) > 0 {
				maxTitle := colWidth - len(node.ID) - 4
				if maxTitle > 0 && len(node.Title) > maxTitle {
					title = node.ID + " " + node.Title[:maxTitle-3] + "..."
				} else if maxTitle > 0 {
					title = node.ID + " " + node.Title
				}
			}
			midPart = fmt.Sprintf("%s %s", icon, title)
			// Check if this node is selected
			for i, n := range m.graphNodes {
				if n.ID == node.ID && n.Column == 1 && i == m.graphCursor {
					midSelected = true
					break
				}
			}
		}

		// Right column (blocked)
		if row < len(blocked) {
			node := blocked[row]
			icon := depStatusIcon(node.Status)
			title := node.ID
			if len(node.Title) > 0 {
				maxTitle := colWidth - len(node.ID) - 4
				if maxTitle > 0 && len(node.Title) > maxTitle {
					title = node.ID + " " + node.Title[:maxTitle-3] + "..."
				} else if maxTitle > 0 {
					title = node.ID + " " + node.Title
				}
			}
			rightPart = fmt.Sprintf("%s %s", icon, title)
			// Check if this node is selected
			for i, n := range m.graphNodes {
				if n.ID == node.ID && n.Column == 2 && i == m.graphCursor {
					rightSelected = true
					break
				}
			}
		}

		// Render the row with connectors
		// Left part
		if leftSelected {
			b.WriteString(selectedRowStyle.Render(fmt.Sprintf("%-*s", colWidth, leftPart)))
		} else {
			b.WriteString(fmt.Sprintf("%-*s", colWidth, leftPart))
		}

		// Left connector
		if row < len(blockers) && len(current) > 0 {
			if row == 0 {
				b.WriteString(" ───▶ ")
			} else {
				b.WriteString(" ────┘")
			}
		} else {
			b.WriteString("      ")
		}

		// Middle part
		if midSelected {
			b.WriteString(selectedRowStyle.Render(fmt.Sprintf("%-*s", colWidth, midPart)))
		} else {
			b.WriteString(fmt.Sprintf("%-*s", colWidth, midPart))
		}

		// Right connector
		if row < len(blocked) && len(current) > 0 {
			if row == 0 {
				b.WriteString(" ───▶ ")
			} else {
				b.WriteString(" └────")
			}
		} else {
			b.WriteString("      ")
		}

		// Right part
		if rightSelected {
			b.WriteString(selectedRowStyle.Render(fmt.Sprintf("%-*s", colWidth, rightPart)))
		} else {
			b.WriteString(fmt.Sprintf("%-*s", colWidth, rightPart))
		}

		b.WriteString("\n")
	}

	// Legend
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Legend: "))
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusOpen]).Render(iconOpen + " open"))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusInProgress]).Render(iconInProgress + " in_progress"))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusDone]).Render(iconDone + " done"))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusBlocked]).Render(iconBlocked + " blocked"))
	b.WriteString("\n")

	// Help
	b.WriteString("\n")
	b.WriteString(m.helpView())

	return b.String()
}
