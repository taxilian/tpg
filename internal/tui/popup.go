package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

// renderPopup renders content in a centered box over the base view.
func (m Model) renderPopup(title string, content string, width int) string {
	base := m.renderBaseView()
	return m.renderPopupOver(base, title, content, width)
}

// renderPopupOver renders content in a centered box over the given base view.
func (m Model) renderPopupOver(base string, title string, content string, width int) string {
	if width <= 0 {
		width = 50
	}
	contentWidth := m.width - (contentPadding * 2)
	if contentWidth > 0 && width > contentWidth {
		width = max(20, contentWidth)
	}

	var popupContent strings.Builder
	if title != "" {
		popupContent.WriteString(titleStyle.Render(title))
		popupContent.WriteString("\n\n")
	}
	popupContent.WriteString(content)

	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(width)

	popup := popupStyle.Render(popupContent.String())
	if base == "" {
		return popup
	}
	return overlayCentered(dimStyle.Render(base), popup, contentWidth, m.height-1)
}

func overlayCentered(base, popup string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	popupLines := strings.Split(popup, "\n")

	baseWidth := maxLineWidth(baseLines)
	popupWidth := maxLineWidth(popupLines)
	width = max(width, max(baseWidth, popupWidth))
	if width == 0 {
		width = max(baseWidth, popupWidth)
	}

	baseHeight := len(baseLines)
	popupHeight := len(popupLines)
	height = max(height, max(baseHeight, popupHeight))
	if height == 0 {
		height = max(baseHeight, popupHeight)
	}

	for i, line := range baseLines {
		baseLines[i] = padLine(line, width)
	}
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}

	startX := (width - popupWidth) / 2
	startY := (height - popupHeight) / 2

	for i, line := range popupLines {
		targetY := startY + i
		if targetY < 0 || targetY >= len(baseLines) {
			continue
		}
		popupLine := padLine(line, popupWidth)
		baseLine := baseLines[targetY]
		left := ansi.Cut(baseLine, 0, startX)
		right := ansi.Cut(baseLine, startX+popupWidth, width)
		baseLines[targetY] = left + popupLine + right
	}

	return strings.Join(baseLines, "\n")
}

func padLine(line string, width int) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth == width {
		return line
	}
	if lineWidth > width {
		return ansi.Cut(line, 0, width)
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func maxLineWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}
