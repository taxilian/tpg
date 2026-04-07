package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
	"sort"
	"strings"
	"time"
)

func (m Model) handleCreateWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.isWizardDescriptionStep() {
		switch msg.String() {
		case "esc":
			m.textarea.Blur()
			if m.createWizardStep > 1 {
				m.createWizardStep--
			}
			return m, nil
		case "ctrl+s", "ctrl+enter":
			m.createWizardState.Description = m.textarea.Value()
			return m.advanceWizardStep()
		}

		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.createWizardState.Description = m.textarea.Value()
		return m, cmd
	}

	switch m.createWizardStep {
	case 1:
		switch msg.String() {
		case "esc":
			m.viewMode = ViewList
			m.createWizardStep = 0
			m.message = "Creation canceled"
			return m, nil
		case "enter":
			return m.advanceWizardStep()
		case "up", "k":
			return m.handleWizardUp()
		case "down", "j":
			return m.handleWizardDown()
		}
	case 2:
		switch msg.String() {
		case "esc":
			m.wizardTitleInput.Blur()
			m.createWizardStep = 1
			return m, nil
		case "enter":
			m.createWizardState.Title = m.wizardTitleInput.Value()
			return m.advanceWizardStep()
		}
		return m.updateWizardTextInput(msg, &m.wizardTitleInput, func(value string) {
			m.createWizardState.Title = value
		})
	case 3:
		switch msg.String() {
		case "esc":
			m.blurWizardWorktreeInputs()
			m.createWizardStep = 2
			return m, m.focusWizardTitleInput()
		case "enter":
			m.syncWizardWorktreeState()
			return m.advanceWizardStep()
		case "tab":
			return m.handleWizardTab()
		case "up", "k":
			return m.handleWizardUp()
		case "down", "j":
			return m.handleWizardDown()
		}
		if m.createWizardState.SelectedType == model.ItemTypeEpic && m.createWizardState.UseWorktree {
			return m.updateActiveWizardWorktreeInput(msg)
		}
	}

	return m, nil
}

func (m Model) handleWizardTab() (tea.Model, tea.Cmd) {
	if m.createWizardStep != 3 || m.createWizardState.SelectedType != model.ItemTypeEpic || !m.createWizardState.UseWorktree {
		return m, nil
	}
	if m.createWizardState.WorktreeField == 0 {
		m.createWizardState.WorktreeField = 1
	} else {
		m.createWizardState.WorktreeField = 0
	}
	return m, m.focusActiveWizardWorktreeInput()
}

func (m Model) getAvailableTypes() []TypeOption {
	options := []TypeOption{}
	seen := make(map[model.ItemType]bool)

	prefixForType := func(itemType model.ItemType) string {
		switch itemType {
		case model.ItemTypeTask:
			return db.DefaultTaskPrefix
		case model.ItemTypeEpic:
			return db.DefaultEpicPrefix
		default:
			return "it"
		}
	}

	addType := func(itemType model.ItemType, desc string) {
		if itemType == "" || seen[itemType] {
			return
		}
		options = append(options, TypeOption{
			Type:   itemType,
			Prefix: prefixForType(itemType),
			Desc:   desc,
		})
		seen[itemType] = true
	}

	// Default types
	addType(model.ItemTypeTask, "Standard task (default)")
	addType(model.ItemTypeEpic, "Large body of work with child tasks")

	// Database types (for backward compatibility with existing items of old types)
	if m.db != nil {
		if dbTypes, err := m.db.GetDistinctTypes(); err == nil {
			for _, itemType := range dbTypes {
				addType(itemType, "")
			}
		}
	}

	// Sort: task, epic, then alphabetical
	sort.Slice(options, func(i, j int) bool {
		order := func(itemType model.ItemType) int {
			switch itemType {
			case model.ItemTypeTask:
				return 0
			case model.ItemTypeEpic:
				return 1
			default:
				return 2
			}
		}
		left := order(options[i].Type)
		right := order(options[j].Type)
		if left != right {
			return left < right
		}
		return string(options[i].Type) < string(options[j].Type)
	})

	return options
}

func (m Model) handleWizardUp() (tea.Model, tea.Cmd) {
	switch m.createWizardStep {
	case 1: // Type selection
		types := m.getAvailableTypes()
		if len(types) == 0 {
			return m, nil
		}
		if m.createWizardState.TypeCursor > 0 {
			m.createWizardState.TypeCursor--
			m.createWizardState.SelectedType = types[m.createWizardState.TypeCursor].Type
		}
	case 3: // Worktree toggle (epics only)
		if m.createWizardState.SelectedType == model.ItemTypeEpic {
			m.createWizardState.UseWorktree = true
			m = m.ensureWizardWorktreeDefaults()
			return m, m.focusActiveWizardWorktreeInput()
		}
	}
	return m, nil
}

func (m Model) handleWizardDown() (tea.Model, tea.Cmd) {
	switch m.createWizardStep {
	case 1: // Type selection
		types := m.getAvailableTypes()
		if len(types) == 0 {
			return m, nil
		}
		if m.createWizardState.TypeCursor < len(types)-1 {
			m.createWizardState.TypeCursor++
			m.createWizardState.SelectedType = types[m.createWizardState.TypeCursor].Type
		}
	case 3: // Worktree toggle (epics only)
		if m.createWizardState.SelectedType == model.ItemTypeEpic {
			m.createWizardState.UseWorktree = false
			m.blurWizardWorktreeInputs()
		}
	}
	return m, nil
}

func (m Model) advanceWizardStep() (tea.Model, tea.Cmd) {
	state := &m.createWizardState

	switch m.createWizardStep {
	case 1: // Type selected
		types := m.getAvailableTypes()
		if len(types) == 0 {
			return m, nil
		}
		if state.TypeCursor < 0 || state.TypeCursor >= len(types) {
			state.TypeCursor = 0
		}
		state.SelectedType = types[state.TypeCursor].Type
		m.createWizardStep = 2
		return m, m.focusWizardTitleInput()

	case 2: // Title
		state.Title = m.wizardTitleInput.Value()
		if strings.TrimSpace(state.Title) == "" {
			m.err = fmt.Errorf("title is required")
			return m, nil
		}
		m.wizardTitleInput.Blur()
		m.createWizardStep = 3
		if state.SelectedType != model.ItemTypeEpic {
			return m.startWizardDescription()
		}

	case 3: // Worktree (epics only) or Description (non-epic)
		if state.SelectedType == model.ItemTypeEpic {
			m.syncWizardWorktreeState()
			m.blurWizardWorktreeInputs()
			m.createWizardStep = 4
			return m.startWizardDescription()
		}
		if !validateDescription(state.Description) {
			m.err = fmt.Errorf("description must be at least 3 words or 20 characters")
			return m, nil
		}
		m.textarea.Blur()
		return m.createItemFromWizard()

	case 4: // Description (epic)
		if !validateDescription(state.Description) {
			m.err = fmt.Errorf("description must be at least 3 words or 20 characters")
			return m, nil
		}
		m.textarea.Blur()
		return m.createItemFromWizard()
	}

	return m, nil
}

func validateDescription(desc string) bool {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return false
	}

	// Check word count
	words := strings.Fields(desc)
	if len(words) >= 3 {
		return true
	}

	// Check character count
	if len(desc) >= 20 {
		return true
	}

	return false
}

// generateWorktreeBranch generates a branch name from epic ID and title.
// Format: feature/<epic-id>-<slug> where slug is lowercase title with non-alnum->hyphens
func generateWorktreeBranch(epicID, title string) string {
	// Convert title to lowercase
	slug := strings.ToLower(title)

	// Replace non-alphanumeric characters with hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	// Collapse multiple hyphens
	slug = result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
	}

	return fmt.Sprintf("feature/%s-%s", epicID, slug)
}

func generateWorktreeBranchPreview(title string) string {
	return generateWorktreeBranch("ep-xxx", title)
}

func (m Model) ensureWizardWorktreeDefaults() Model {
	if m.createWizardState.WorktreeBranch == "" {
		m.createWizardState.WorktreeBranch = generateWorktreeBranchPreview(m.createWizardState.Title)
		m.createWizardState.WorktreeBranchAuto = true
	}
	if m.createWizardState.WorktreeBase == "" {
		m.createWizardState.WorktreeBase = "main"
	}
	if m.createWizardState.WorktreeField < 0 || m.createWizardState.WorktreeField > 1 {
		m.createWizardState.WorktreeField = 0
	}
	m.wizardBranchInput.SetValue(m.createWizardState.WorktreeBranch)
	m.wizardBranchInput.CursorEnd()
	m.wizardBaseInput.SetValue(m.createWizardState.WorktreeBase)
	m.wizardBaseInput.CursorEnd()
	return m
}

func (m Model) createItemFromWizard() (tea.Model, tea.Cmd) {
	state := m.createWizardState
	selectedType := state.SelectedType
	if selectedType == "" {
		selectedType = model.ItemTypeTask
	}

	itemID, err := m.db.GenerateItemID(selectedType)
	if err != nil {
		m.err = err
		return m, nil
	}

	now := time.Now()
	item := model.Item{
		ID:          itemID,
		Project:     m.project,
		Type:        selectedType,
		Title:       state.Title,
		Description: state.Description,
		Status:      model.StatusOpen,
		Priority:    2,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if selectedType == model.ItemTypeEpic && state.UseWorktree {
		item.WorktreeBranch = state.WorktreeBranch
		item.WorktreeBase = state.WorktreeBase
	}

	if err := m.db.CreateItem(&item); err != nil {
		m.err = err
		return m, nil
	}

	m.viewMode = ViewList
	m.createWizardStep = 0
	m.wizardTitleInput.Blur()
	m.blurWizardWorktreeInputs()
	m.message = fmt.Sprintf("Created %s: %s", selectedType, itemID)

	return m, m.loadItems()
}

func (m Model) createWizardView() string {
	switch m.createWizardStep {
	case 1:
		return m.wizardTypeView()
	case 2:
		return m.wizardTitleView()
	case 3:
		if m.createWizardState.SelectedType == model.ItemTypeEpic {
			return m.wizardWorktreeView()
		}
		return m.wizardDescriptionView()
	case 4:
		return m.wizardDescriptionView()
	default:
		return ""
	}
}

func (m Model) isWizardDescriptionStep() bool {
	if m.createWizardStep == 3 && m.createWizardState.SelectedType != model.ItemTypeEpic {
		return true
	}
	return m.createWizardStep == 4
}

func (m Model) wizardPopupWidth() int {
	width := 60
	contentWidth := m.width - (contentPadding * 2)
	if contentWidth > 0 && width > contentWidth {
		width = max(30, contentWidth)
	}
	if width < 30 {
		width = 30
	}
	return width
}

func (m Model) wizardPopupTitle() string {
	selectedType := m.createWizardState.SelectedType
	if selectedType == "" {
		selectedType = model.ItemTypeTask
	}
	return fmt.Sprintf("Create %s", selectedType)
}

func (m Model) wizardPopupBase() string {
	return m.listView()
}

func (m Model) startWizardDescription() (Model, tea.Cmd) {
	m.textarea.SetValue(m.createWizardState.Description)
	m.textarea.Focus()
	popupWidth := m.wizardPopupWidth()
	width := max(20, popupWidth-6)
	height := 5
	if m.height >= 24 {
		height = 7
	} else if m.height <= 16 {
		height = 4
	}
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(height)
	return m, textarea.Blink
}

func (m *Model) setWizardInputWidths() {
	width := max(20, m.wizardPopupWidth()-8)
	m.wizardTitleInput.Width = width
	m.wizardBranchInput.Width = width
	m.wizardBaseInput.Width = width
}

func (m *Model) focusWizardTitleInput() tea.Cmd {
	m.setWizardInputWidths()
	m.wizardTitleInput.SetValue(m.createWizardState.Title)
	m.wizardTitleInput.CursorEnd()
	m.blurWizardWorktreeInputs()
	return m.wizardTitleInput.Focus()
}

func (m *Model) blurWizardWorktreeInputs() {
	m.wizardBranchInput.Blur()
	m.wizardBaseInput.Blur()
}

func (m *Model) activeWizardWorktreeInput() *textinput.Model {
	if m.createWizardState.WorktreeField == 0 {
		return &m.wizardBranchInput
	}
	return &m.wizardBaseInput
}

func (m *Model) focusActiveWizardWorktreeInput() tea.Cmd {
	m.setWizardInputWidths()
	m.blurWizardWorktreeInputs()
	return m.activeWizardWorktreeInput().Focus()
}

func (m *Model) syncWizardWorktreeState() {
	m.createWizardState.Title = m.wizardTitleInput.Value()
	m.createWizardState.WorktreeBranch = m.wizardBranchInput.Value()
	m.createWizardState.WorktreeBase = m.wizardBaseInput.Value()
}

func (m *Model) updateWizardTextInput(msg tea.KeyMsg, input *textinput.Model, sync func(string)) (tea.Model, tea.Cmd) {
	m.setWizardInputWidths()
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	sync(input.Value())
	return *m, cmd
}

func (m *Model) updateActiveWizardWorktreeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	active := m.activeWizardWorktreeInput()
	previous := active.Value()
	_, cmd := m.updateWizardTextInput(msg, active, func(string) {})
	if m.createWizardState.WorktreeField == 0 && m.wizardBranchInput.Value() != previous {
		m.createWizardState.WorktreeBranchAuto = false
	}
	m.syncWizardWorktreeState()
	return *m, cmd
}

func (m Model) wizardTypeView() string {
	var b strings.Builder

	types := m.getAvailableTypes()
	if m.createWizardState.TypeCursor < 0 || m.createWizardState.TypeCursor >= len(types) {
		m.createWizardState.TypeCursor = 0
	}

	b.WriteString("Select item type:\n\n")
	for i, t := range types {
		selected := i == m.createWizardState.TypeCursor
		icon := "○"
		if selected {
			icon = "●"
		}
		label := fmt.Sprintf("%s %s (%s-)", icon, t.Type, t.Prefix)
		if t.Desc != "" {
			label = fmt.Sprintf("%s - %s", label, t.Desc)
		}
		if selected {
			b.WriteString(selectedRowStyle.Render(label))
		} else {
			b.WriteString(label)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" [↑↓] Navigate  [Enter] Select  [Esc] Cancel"))

	return m.renderPopup("Create New Item", b.String(), m.wizardPopupWidth())
}

func (m Model) wizardWorktreeView() string {
	var b strings.Builder

	b.WriteString("Would you like to create a worktree?\n\n")

	// Use worktree toggle
	if m.createWizardState.UseWorktree {
		b.WriteString("  ○ No worktree\n")
		b.WriteString("  ● Yes, create worktree\n")
	} else {
		b.WriteString("  ● No worktree\n")
		b.WriteString("  ○ Yes, create worktree\n")
	}

	b.WriteString("\n")

	if m.createWizardState.UseWorktree {
		m.setWizardInputWidths()
		branchInput := m.wizardBranchInput
		baseInput := m.wizardBaseInput
		b.WriteString("  Branch: ")
		b.WriteString(branchInput.View())
		b.WriteString("\n")
		b.WriteString("  Base:   ")
		b.WriteString(baseInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" [Up/Down] Toggle  [Tab] Switch Field  [Enter] Continue  [Esc] Back"))

	return m.renderPopupOver(m.wizardPopupBase(), m.wizardPopupTitle(), b.String(), m.wizardPopupWidth())
}

func (m Model) wizardTitleView() string {
	var b strings.Builder

	m.setWizardInputWidths()
	inputWidth := max(20, m.wizardPopupWidth()-6)
	input := m.wizardTitleInput
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(inputWidth).
		Render(input.View())

	b.WriteString("Title:\n")
	b.WriteString(inputBox)
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(" [Enter] Continue  [Esc] Cancel"))

	return m.renderPopupOver(m.wizardPopupBase(), m.wizardPopupTitle(), b.String(), m.wizardPopupWidth())
}

func (m Model) wizardDescriptionView() string {
	var b strings.Builder

	b.WriteString("Description (required):\n")
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(" [Ctrl+S] Save  [Esc] Cancel"))

	return m.renderPopupOver(m.wizardPopupBase(), m.wizardPopupTitle(), b.String(), m.wizardPopupWidth())
}
