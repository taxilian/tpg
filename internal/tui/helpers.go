package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
	"strings"
)

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// wrapText wraps text to fit within maxWidth display columns, preserving word boundaries.
// Uses lipgloss.Width() for accurate column width calculation (handles ANSI codes).
func wrapText(text string, maxWidth int, indent string) string {
	if maxWidth <= 0 {
		return text
	}
	indentWidth := lipgloss.Width(indent)
	if indentWidth >= maxWidth {
		return text
	}
	contentWidth := maxWidth - indentWidth

	var result strings.Builder
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}
		if lipgloss.Width(line) <= contentWidth {
			result.WriteString(indent)
			result.WriteString(line)
			continue
		}
		// Wrap the line
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}
		currentLine := ""
		for _, word := range words {
			testLine := currentLine
			if testLine != "" {
				testLine += " "
			}
			testLine += word
			if lipgloss.Width(testLine) <= contentWidth {
				currentLine = testLine
			} else {
				if currentLine != "" {
					result.WriteString(indent)
					result.WriteString(currentLine)
					result.WriteString("\n")
				}
				// If single word is too long, truncate it
				if lipgloss.Width(word) > contentWidth {
					truncated := word
					runes := []rune(word)
					for j := len(runes); j > 0; j-- {
						if lipgloss.Width(string(runes[:j])) <= contentWidth-3 {
							truncated = string(runes[:j]) + "..."
							break
						}
					}
					result.WriteString(indent)
					result.WriteString(truncated)
					result.WriteString("\n")
					currentLine = ""
				} else {
					currentLine = word
				}
			}
		}
		if currentLine != "" {
			result.WriteString(indent)
			result.WriteString(currentLine)
		}
	}
	return result.String()
}

// truncateWidth truncates s to fit within maxWidth display columns, adding "..." if needed.
func truncateWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return "..."[:maxWidth]
	}
	target := maxWidth - 3
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		if lipgloss.Width(string(runes[:i])) <= target {
			return string(runes[:i]) + "..."
		}
	}
	return "..."
}

func trimLastRune(text string) string {
	if text == "" {
		return text
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

// depStatusIcon returns a colored icon for a dependency's status string.
func depStatusIcon(status string) string {
	s := model.Status(status)
	icon := statusIcon(s)
	if c, ok := statusColors[s]; ok {
		return lipgloss.NewStyle().Foreground(c).Render(icon)
	}
	return icon
}

func statusIcon(s model.Status) string {
	switch s {
	case model.StatusOpen:
		return iconOpen
	case model.StatusInProgress:
		return iconInProgress
	case model.StatusDone:
		return iconDone
	case model.StatusBlocked:
		return iconBlocked
	case model.StatusCanceled:
		return iconCanceled
	default:
		return "?"
	}
}

// statusText returns a short readable text for a status.
func statusText(s model.Status) string {
	switch s {
	case model.StatusOpen:
		return "open"
	case model.StatusInProgress:
		return "active"
	case model.StatusDone:
		return "done"
	case model.StatusBlocked:
		return "block"
	case model.StatusCanceled:
		return "cancel"
	default:
		return "?"
	}
}

// formatStatus returns "icon text" format for a status (e.g., "○ open").
func formatStatus(s model.Status) string {
	return statusIcon(s) + " " + statusText(s)
}

// templateInfo holds information about a template for display purposes.
type templateInfo struct {
	name        string
	stepNum     int                 // 1-based step number (0 if no step)
	totalSteps  int                 // total number of steps
	notFound    bool                // template doesn't exist
	invalidStep bool                // step index is out of range
	tmpl        *templates.Template // the loaded template (nil if not found)
}

// getTemplateInfo loads template information for an item.
func getTemplateInfo(item model.Item) templateInfo {
	info := templateInfo{
		name: item.TemplateID,
	}

	if item.TemplateID == "" {
		return info
	}

	tmpl, err := templates.LoadTemplate(item.TemplateID)
	if err != nil {
		info.notFound = true
		return info
	}

	info.tmpl = tmpl
	info.totalSteps = len(tmpl.Steps)

	if item.StepIndex != nil {
		stepIdx := *item.StepIndex
		info.stepNum = stepIdx + 1 // Convert to 1-based
		if stepIdx < 0 || stepIdx >= len(tmpl.Steps) {
			info.invalidStep = true
		}
	}

	return info
}

// getUnusedVariables returns variables that exist in item.TemplateVars but aren't used in the template.
func getUnusedVariables(tmpl *templates.Template, vars map[string]string, stepIndex *int) map[string]string {
	if tmpl == nil || vars == nil {
		return nil
	}

	// Collect all variable references from the template
	usedVars := make(map[string]bool)

	// Parse the template description for {{.varname}} patterns
	parseVarRefs := func(text string) {
		// Match {{.varname}} and {{ .varname }} patterns
		// Also match {{if .varname}}, {{with .varname}}, etc.
		for i := 0; i < len(text); i++ {
			if i+2 < len(text) && text[i:i+2] == "{{" {
				// Find the closing }}
				end := strings.Index(text[i:], "}}")
				if end == -1 {
					continue
				}
				content := text[i+2 : i+end]
				// Look for .varname patterns
				for j := 0; j < len(content); j++ {
					if content[j] == '.' && j+1 < len(content) {
						// Extract the variable name
						start := j + 1
						k := start
						for k < len(content) && (content[k] == '_' || (content[k] >= 'a' && content[k] <= 'z') || (content[k] >= 'A' && content[k] <= 'Z') || (content[k] >= '0' && content[k] <= '9')) {
							k++
						}
						if k > start {
							varName := content[start:k]
							usedVars[varName] = true
						}
					}
				}
				i += end + 1
			}
		}
	}

	// Parse template-level description
	parseVarRefs(tmpl.Description)

	// Parse the specific step if we have one
	if stepIndex != nil && *stepIndex >= 0 && *stepIndex < len(tmpl.Steps) {
		step := tmpl.Steps[*stepIndex]
		parseVarRefs(step.Title)
		parseVarRefs(step.Description)
	} else {
		// Parse all steps
		for _, step := range tmpl.Steps {
			parseVarRefs(step.Title)
			parseVarRefs(step.Description)
		}
	}

	// Also consider variables defined in the template as "used"
	for name := range tmpl.Variables {
		usedVars[name] = true
	}

	// Find unused variables
	unused := make(map[string]string)
	for name, value := range vars {
		if !usedVars[name] {
			unused[name] = value
		}
	}

	if len(unused) == 0 {
		return nil
	}
	return unused
}

// renderTemplateForItem renders the template description for an item using its stored variables.
// Returns the rendered description and a boolean indicating if rendering was successful.
func renderTemplateForItem(item model.Item) string {
	if item.TemplateID == "" {
		return item.Description
	}

	tmpl, err := templates.LoadTemplate(item.TemplateID)
	if err != nil {
		return item.Description
	}

	// Find the step for this item (if it has a step index)
	if item.StepIndex != nil && *item.StepIndex >= 0 && *item.StepIndex < len(tmpl.Steps) {
		step := tmpl.Steps[*item.StepIndex]
		return templates.RenderText(step.Description, item.TemplateVars)
	}

	// No step index, render the template description
	return templates.RenderText(tmpl.Description, item.TemplateVars)
}
