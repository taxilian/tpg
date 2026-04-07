package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
)

func newViewportModel() viewport.Model {
	vp := viewport.New(1, 1)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	return vp
}

func viewportWidth(width int) int {
	available := width - (contentPadding * 2)
	if available < 1 {
		return 1
	}
	return available
}

func detailViewportHeight(height int) int {
	visibleHeight := height - detailReservedRows
	if visibleHeight < 10 {
		visibleHeight = 10
	}
	return visibleHeight
}

func templateDetailViewportHeight(height int) int {
	visibleHeight := height - detailReservedRows
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	return visibleHeight
}

func configViewportHeight(height int) int {
	visibleHeight := height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	return visibleHeight
}

func varPickerViewportHeight(height int) int {
	visibleHeight := height - 6
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	return visibleHeight
}

func configureViewport(vp *viewport.Model, width, height int) {
	vp.Width = viewportWidth(width)
	vp.Height = max(1, height)
	if vp.MouseWheelDelta == 0 {
		vp.MouseWheelEnabled = true
		vp.MouseWheelDelta = 3
	}
}

func clampViewport(vp *viewport.Model) {
	maxOffset := vp.TotalLineCount() - vp.VisibleLineCount()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if vp.YOffset < 0 {
		vp.SetYOffset(0)
		return
	}
	if vp.YOffset > maxOffset {
		vp.SetYOffset(maxOffset)
	}
}

func setViewportContent(vp *viewport.Model, width, height int, content string) {
	prevOffset := vp.YOffset
	configureViewport(vp, width, height)
	vp.SetContent(content)
	vp.SetYOffset(prevOffset)
	clampViewport(vp)
}

func syncViewportToCursor(vp *viewport.Model, cursor, totalItems int) {
	if totalItems == 0 {
		vp.SetYOffset(0)
		return
	}
	visibleHeight := vp.VisibleLineCount()
	if visibleHeight < 1 {
		visibleHeight = max(1, vp.Height)
	}
	scrollPos := vp.YOffset
	calculateScrollRange(cursor, totalItems, visibleHeight, &scrollPos)
	vp.SetYOffset(scrollPos)
}

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
	configureViewport(&m.varPickerViewport, m.width, varPickerViewportHeight(m.height))
	syncViewportToCursor(&m.varPickerViewport, m.varCursor, itemCount)
}

func (m *Model) syncConfigScroll() {
	configureViewport(&m.configViewport, m.width, configViewportHeight(m.height))
	syncViewportToCursor(&m.configViewport, m.configCursor, len(m.configFields))
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
