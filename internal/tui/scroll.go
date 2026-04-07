package tui

import (
	"strings"
)

func (m Model) listVisibleHeight() int {
	visibleHeight := m.height - listReservedRows
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	return visibleHeight
}

func (m Model) templateVisibleHeight() int {
	visibleHeight := m.height - templateReservedRows
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	return visibleHeight
}

// scrollText clips text to show only visible lines based on scroll offset.
// Returns the visible portion and total line count.
// Clamps scrollOffset so the result is never empty when text has content.
func scrollText(text string, scrollOffset, maxVisible int) (visible string, totalLines int) {
	lines := strings.Split(text, "\n")
	totalLines = len(lines)

	if scrollOffset < 0 {
		scrollOffset = 0
	}
	// Clamp to last page instead of returning empty
	maxScroll := totalLines - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	end := scrollOffset + maxVisible
	if end > totalLines {
		end = totalLines
	}

	visible = strings.Join(lines[scrollOffset:end], "\n")
	return visible, totalLines
}

// syncListScroll ensures listScroll keeps the cursor visible.
// Must be called from Update (pointer receiver) so the mutation persists.
func (m *Model) syncListScroll() {
	treeNodes := m.buildTree()
	visibleHeight := m.height - listReservedRows
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	calculateScrollRange(m.cursor, len(treeNodes), visibleHeight, &m.listScroll)
}

func (m *Model) syncTemplateScroll() {
	visibleHeight := m.height - templateReservedRows
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	calculateScrollRange(m.templateCursor, len(m.templates), visibleHeight, &m.templateScroll)
}

func (m *Model) syncVarPickerScroll(itemCount int) {
	visibleHeight := m.height - 8
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	calculateScrollRange(m.varCursor, itemCount, visibleHeight, &m.varPickerScroll)
}

func (m *Model) syncConfigScroll() {
	visibleHeight := m.height - 6
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	calculateScrollRange(m.configCursor, len(m.configFields), visibleHeight, &m.configScroll)
}

// calculateScrollRange calculates the visible range for a scrolled list.
// Updates scrollPos to keep cursor visible with edge-based scrolling.
// Returns start and end indices for slicing the list.
func calculateScrollRange(cursor, totalItems, visibleHeight int, scrollPos *int) (start, end int) {
	if totalItems == 0 {
		*scrollPos = 0
		return 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= totalItems {
		cursor = totalItems - 1
	}
	if visibleHeight >= totalItems {
		*scrollPos = 0
		return 0, totalItems
	}

	// Scroll down if cursor below visible area
	for cursor >= *scrollPos+visibleHeight {
		(*scrollPos)++
	}
	// Scroll up if cursor above visible area
	for cursor < *scrollPos && *scrollPos > 0 {
		(*scrollPos)--
	}
	// Clamp to valid range
	if *scrollPos < 0 {
		*scrollPos = 0
	}
	maxScroll := totalItems - visibleHeight
	if *scrollPos > maxScroll {
		*scrollPos = maxScroll
	}

	start = *scrollPos
	end = start + visibleHeight
	if end > totalItems {
		end = totalItems
	}
	return start, end
}
