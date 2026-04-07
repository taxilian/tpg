package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/model"
	"strings"
)

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "ctrl+v":
		// Toggle selection mode
		m.selectMode = !m.selectMode
		if !m.selectMode {
			// Clear selections when exiting select mode
			m.selectedItems = make(map[string]bool)
		}
		return m, nil

	case " ":
		// Space: toggle selection of current item (only in select mode)
		treeNodes := m.buildTree()
		if m.selectMode && len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			id := treeNodes[m.cursor].Item.ID
			if m.selectedItems[id] {
				delete(m.selectedItems, id)
			} else {
				m.selectedItems[id] = true
			}
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		treeNodes := m.buildTree()
		if m.cursor < len(treeNodes)-1 {
			m.cursor++
		}

	case "g", "home":
		m.cursor = 0
		m.listScroll = 0

	case "G", "end":
		treeNodes := m.buildTree()
		m.cursor = max(0, len(treeNodes)-1)

	case "pgup", "ctrl+b":
		// Page up - move cursor up by visible height and sync scroll
		pageSize := m.listVisibleHeight()
		m.cursor -= pageSize
		if m.cursor < 0 {
			m.cursor = 0
		}
		// Sync scroll to show cursor at top of visible area
		m.listScroll = m.cursor

	case "pgdown", "ctrl+f":
		// Page down - move cursor down by visible height and sync scroll
		treeNodes := m.buildTree()
		pageSize := m.listVisibleHeight()
		m.cursor += pageSize
		if m.cursor >= len(treeNodes) {
			m.cursor = max(0, len(treeNodes)-1)
		}
		// Sync scroll to show cursor at bottom of visible area
		m.listScroll = max(0, m.cursor-pageSize+1)

	case "ctrl+u":
		pageSize := m.listVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.cursor -= pageSize
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.listScroll = m.cursor

	case "ctrl+d":
		treeNodes := m.buildTree()
		pageSize := m.listVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.cursor += pageSize
		if m.cursor >= len(treeNodes) {
			m.cursor = max(0, len(treeNodes)-1)
		}
		m.listScroll = max(0, m.cursor-pageSize+1)

	case "right":
		// Expand current node if it has children
		treeNodes := m.buildTree()
		if m.cursor < len(treeNodes) {
			node := treeNodes[m.cursor]
			if node.HasChildren && !m.treeExpanded[node.Item.ID] {
				m.treeExpanded[node.Item.ID] = true
			}
		}

	case "left":
		// Collapse current node or jump to parent
		treeNodes := m.buildTree()
		if m.cursor < len(treeNodes) {
			node := treeNodes[m.cursor]
			if node.HasChildren && m.treeExpanded[node.Item.ID] {
				// Collapse if expanded
				delete(m.treeExpanded, node.Item.ID)
			} else if node.Level > 0 {
				// Jump to parent if collapsed and has a parent
				if node.Item.ParentID != nil {
					for i, n := range treeNodes {
						if n.Item.ID == *node.Item.ParentID {
							m.cursor = i
							break
						}
					}
				}
			}
		}

	case "enter", "l":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 {
			m.viewMode = ViewDetail
			return m, m.loadDetail()
		}

	// Actions - show status menu for confirmation
	case "s":
		if m.selectMode && len(m.selectedItems) > 0 {
			return m.startInput(InputBatchStatus, "Batch status (o=open, i=in_progress, b=blocked, d=done, c=canceled): ")
		}
		return m.showStatusMenu(0) // Start selected
	case "p":
		if m.selectMode && len(m.selectedItems) > 0 {
			return m.startInput(InputBatchPriority, "Batch priority (1-5): ")
		}
		return m.startInput(InputProject, "Project: ")
	case "d":
		if m.selectMode && len(m.selectedItems) > 0 {
			return m.doBatchDone()
		}
		return m.showStatusMenu(1) // Done selected
	case "b":
		return m.showStatusMenu(2) // Block selected
	case "L":
		return m.startInput(InputLog, "Log message: ")
	case "c":
		return m.showStatusMenu(3) // Cancel selected
	case "D":
		return m.doDelete()

	// Filtering
	case "/":
		return m.startInput(InputSearch, "Search: ")
	case "t":
		return m.startInput(InputLabel, "Label: ")
	case "1":
		m.filterStatuses[model.StatusOpen] = !m.filterStatuses[model.StatusOpen]
		m.applyFilters()
	case "2":
		m.filterStatuses[model.StatusInProgress] = !m.filterStatuses[model.StatusInProgress]
		m.applyFilters()
	case "3":
		m.filterStatuses[model.StatusBlocked] = !m.filterStatuses[model.StatusBlocked]
		m.applyFilters()
	case "4":
		m.filterStatuses[model.StatusDone] = !m.filterStatuses[model.StatusDone]
		m.applyFilters()
	case "5":
		m.filterStatuses[model.StatusCanceled] = !m.filterStatuses[model.StatusCanceled]
		m.applyFilters()
	case "0":
		// Show all
		for s := range m.filterStatuses {
			m.filterStatuses[s] = true
		}
		m.applyFilters()

	case "esc":
		// If filters are set, clear them; otherwise quit
		if m.filterSearch != "" || m.filterProject != "" || m.filterLabel != "" || m.filterReady {
			m.filterSearch = ""
			m.filterProject = ""
			m.filterLabel = ""
			m.filterReady = false
			m.applyFilters()
		} else {
			return m, tea.Quit
		}

	case "r":
		var preserveID string
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			preserveID = treeNodes[m.cursor].Item.ID
		}
		return m, m.loadItemsPreserving(preserveID)

	case "R":
		m.filterReady = !m.filterReady
		m.applyFilters()
		m.syncListScroll()

	// Dependencies
	case "a":
		return m.startInput(InputAddDep, "Add blocker ID: ")

	// Create
	case "n":
		// Start the creation wizard instead of simple input
		m.viewMode = ViewCreateWizard
		m.createWizardStep = 1
		m.createWizardState = CreateWizardState{
			SelectedType: model.ItemTypeTask,
			TypeCursor:   0,
		}
		return m, nil

	// Templates
	case "T":
		m.viewMode = ViewTemplateList
		return m, m.loadTemplates()

	// Config
	case "C":
		m.viewMode = ViewConfig
		return m, m.loadConfig()
	}

	m.syncListScroll()
	return m, nil
}

// buildTree constructs a hierarchical tree from filtered items.
// Returns flattened list with level information.
func (m *Model) buildTree() []treeNode {
	// Create a map of all items for quick lookup
	itemMap := make(map[string]model.Item)
	for _, item := range m.filtered {
		itemMap[item.ID] = item
	}

	// Create a map of parent -> children relationships
	childrenMap := make(map[string][]model.Item)
	for _, item := range m.filtered {
		if item.ParentID != nil {
			childrenMap[*item.ParentID] = append(childrenMap[*item.ParentID], item)
		}
	}

	var nodes []treeNode

	// Find root items (no parent or parent not in filtered list)
	for _, item := range m.filtered {
		isRoot := item.ParentID == nil
		if item.ParentID != nil {
			if _, hasParent := itemMap[*item.ParentID]; !hasParent {
				isRoot = true // Parent not in filtered list, treat as root
			}
		}

		if isRoot {
			nodes = append(nodes, treeNode{
				Item:        item,
				Level:       0,
				HasChildren: len(childrenMap[item.ID]) > 0,
			})

			// Recursively add children if expanded
			if m.treeExpanded[item.ID] {
				nodes = append(nodes, m.getChildNodes(item.ID, 1, childrenMap)...)
			}
		}
	}

	return nodes
}

// getChildNodes recursively gets child nodes for a parent.
func (m *Model) getChildNodes(parentID string, level int, childrenMap map[string][]model.Item) []treeNode {
	var nodes []treeNode
	children := childrenMap[parentID]

	for i, child := range children {
		nodes = append(nodes, treeNode{
			Item:        child,
			Level:       level,
			HasChildren: len(childrenMap[child.ID]) > 0,
			IsLastChild: i == len(children)-1,
		})

		// Recursively add grandchildren if expanded
		if m.treeExpanded[child.ID] {
			nodes = append(nodes, m.getChildNodes(child.ID, level+1, childrenMap)...)
		}
	}

	return nodes
}

func (m Model) listView() string {
	var b strings.Builder

	// Header
	title := "tpg"
	b.WriteString(titleStyle.Render(title))
	b.WriteString(fmt.Sprintf("  %d/%d items", len(m.filtered), len(m.items)))

	// Selection mode indicator
	if m.selectMode {
		b.WriteString("  ")
		b.WriteString(selectModeStyle.Render(fmt.Sprintf("[SELECT MODE] (%d selected)", len(m.selectedItems))))
	}

	// Active filters
	filters := m.activeFiltersString()
	if filters != "" {
		b.WriteString("  ")
		b.WriteString(filterStyle.Render(filters))
	}
	b.WriteString("\n\n")

	// Build tree from filtered items
	treeNodes := m.buildTree()

	// Items
	if len(treeNodes) == 0 {
		b.WriteString("No items match filters\n")
	} else {
		visibleHeight := m.height - listReservedRows
		if visibleHeight < 3 {
			visibleHeight = 3
		}

		var start, end int
		if m.cursor != m.prevCursor {
			start, end = calculateScrollRange(m.cursor, len(treeNodes), visibleHeight, &m.listScroll)
			m.prevCursor = m.cursor
		} else {
			start = m.listScroll
			end = min(m.listScroll+visibleHeight, len(treeNodes))
		}

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 40 {
			rowWidth = 40
		}

		for i := start; i < end; i++ {
			node := treeNodes[i]
			selected := i == m.cursor

			if selected {
				// For selected row: plain text, then apply highlight to full width
				line := m.formatTreeNodeLinePlain(node, rowWidth)
				b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
			} else {
				// For non-selected: use styled version
				line := m.formatTreeNodeLineStyled(node, rowWidth)
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(m.helpView())

	return b.String()
}

// formatTreeNodeLinePlain returns a plain text line for a tree node without ANSI styling.
// Used for selected rows where we apply a single highlight style.
func (m Model) formatTreeNodeLinePlain(node treeNode, width int) string {
	item := node.Item

	// Check if item is stale and use "stale" status display
	var status string
	if m.staleItems[item.ID] {
		status = "⚠ stale"
	} else {
		status = formatStatus(item.Status)
	}

	treePrefix := m.buildTreePrefix(node)
	treePrefixWidth := lipgloss.Width(treePrefix)

	selectPrefix := ""
	selectWidth := 0
	if m.selectMode {
		if m.selectedItems[item.ID] {
			selectPrefix = "✓ "
		} else {
			selectPrefix = "  "
		}
		selectWidth = 2
	}

	agent := ""
	agentWidth := 0
	if item.AgentID != nil && *item.AgentID != "" {
		agent = "◈"
		agentWidth = 2
	}

	itemType := string(item.Type)
	if len(itemType) > 4 {
		itemType = itemType[:4]
	}
	typeWidth := 5

	statusWidth := 9

	project := ""
	projectWidth := 0
	if item.Project != "" {
		project = "[" + item.Project + "]"
		projectWidth = lipgloss.Width(project) + 1
	}

	labels := ""
	labelsWidth := 0
	for _, lbl := range item.Labels {
		labels += " [" + lbl + "]"
		labelsWidth += lipgloss.Width("["+lbl+"]") + 1
	}

	fixedWidth := 10 + labelsWidth + projectWidth + agentWidth + typeWidth + statusWidth + selectWidth + treePrefixWidth
	titleWidth := width - fixedWidth
	if titleWidth < 10 {
		titleWidth = 10
	}

	title := item.Title
	title = truncateWidth(title, titleWidth)

	if agent != "" {
		return fmt.Sprintf("%s%s%-8s %-4s %s %s %-*s%s %s", selectPrefix, treePrefix, status, itemType, item.ID, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s%-8s %-4s %s  %-*s%s %s", selectPrefix, treePrefix, status, itemType, item.ID, titleWidth, title, labels, project)
}

// formatTreeNodeLineStyled returns a styled line with colors for non-selected rows.
func (m Model) formatTreeNodeLineStyled(node treeNode, width int) string {
	item := node.Item

	// Check if item is stale and use "stale" status display with orange styling
	var statusStyled string
	if m.staleItems[item.ID] {
		statusStyled = staleStyle.Render(fmt.Sprintf("%-8s", "⚠ stale"))
	} else {
		icon := statusIcon(item.Status)
		text := statusText(item.Status)
		color := statusColors[item.Status]
		statusStyled = lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%-8s", icon+" "+text))
	}

	treePrefix := m.buildTreePrefix(node)
	treePrefixWidth := lipgloss.Width(treePrefix)

	selectPrefix := ""
	selectWidth := 0
	if m.selectMode {
		if m.selectedItems[item.ID] {
			selectPrefix = selectModeStyle.Render("✓") + " "
		} else {
			selectPrefix = "  "
		}
		selectWidth = 2
	}

	agent := ""
	agentWidth := 0
	if item.AgentID != nil && *item.AgentID != "" {
		agent = dimStyle.Render("◈")
		agentWidth = 2
	}

	id := dimStyle.Render(item.ID)

	itemType := string(item.Type)
	if len(itemType) > 4 {
		itemType = itemType[:4]
	}
	typeStyled := dimStyle.Render(fmt.Sprintf("%-4s", itemType))
	typeWidth := 5

	statusWidth := 9

	project := ""
	projectWidth := 0
	if item.Project != "" {
		project = dimStyle.Render("[" + item.Project + "]")
		projectWidth = lipgloss.Width("["+item.Project+"]") + 1
	}

	labels := ""
	labelsWidth := 0
	for _, lbl := range item.Labels {
		labels += " " + labelStyle.Render("["+lbl+"]")
		labelsWidth += lipgloss.Width("["+lbl+"]") + 1
	}

	fixedWidth := 10 + labelsWidth + projectWidth + agentWidth + typeWidth + statusWidth + selectWidth + treePrefixWidth
	titleWidth := width - fixedWidth
	if titleWidth < 10 {
		titleWidth = 10
	}

	title := item.Title
	title = truncateWidth(title, titleWidth)

	// Render tree prefix with appropriate styling
	styledTreePrefix := dimStyle.Render(treePrefix)

	if agent != "" {
		return fmt.Sprintf("%s%s%s %s %s %s %-*s%s %s", selectPrefix, styledTreePrefix, statusStyled, typeStyled, id, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s%s %s %s  %-*s%s %s", selectPrefix, styledTreePrefix, statusStyled, typeStyled, id, titleWidth, title, labels, project)
}

// buildTreePrefix creates the indentation and branch indicators for a tree node.
func (m Model) buildTreePrefix(node treeNode) string {
	if node.Level == 0 {
		// Root level - no indentation
		if node.HasChildren {
			if m.treeExpanded[node.Item.ID] {
				return "▼ "
			}
			return "▶ "
		}
		return "○ "
	}

	// Build indentation based on level
	prefix := ""
	for i := 0; i < node.Level-1; i++ {
		prefix += "│  "
	}

	// Add branch connector
	if node.IsLastChild {
		prefix += "└─ "
	} else {
		prefix += "├─ "
	}

	// Add expand/collapse indicator for nodes with children
	if node.HasChildren {
		if m.treeExpanded[node.Item.ID] {
			prefix += "▼ "
		} else {
			prefix += "▶ "
		}
	} else {
		prefix += "○ "
	}

	return prefix
}
